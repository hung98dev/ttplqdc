package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/unicode/norm"

	"thinhthan/internal/config"
)

// TypeSpec is the §1 typed grammar for one column/field.
// Drivers declare each column's spec; the parser produces a typed
// config.Value. Registered column adapters (prefix/suffix/units) are
// explicit fields of the spec — never inferred.
type TypeSpec struct {
	Name       string // id, int, grouped_int, decimal, ratio, bp, percent, enum, bool, range, set, ordered, pair, string, expr, ints_ms, deg
	Enum       []string
	Elem       *TypeSpec // element type for set/ordered/pair/range
	Min        *config.Rat
	Max        *config.Rat
	IntPrefix  string // adapter: e.g. "Lv", "T"
	IntSuffix  string // adapter: e.g. "s", "ms", "m", "px", "%"
	IntScale   int64  // adapter unit scale (e.g. s → ms = 1000); default 1
	Nullable   bool   // accepts the NONE token -> KindNull
	EmptyIsSet bool   // empty cell legal (declared empty default)
}

var (
	idRe         = regexp.MustCompile(`^[a-z0-9_]+(\.[a-z0-9_]+)*$`)
	intRe        = regexp.MustCompile(`^-?[0-9]+$`)
	groupedIntRe = regexp.MustCompile(`^[0-9]{1,3}(,[0-9]{3})*$`)
	decimalRe    = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)
	percentRe    = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?%$`)
)

// ParseValue parses one scalar token under spec. loc describes the source
// (used in error text); diags are emitted by the caller.
func (t TypeSpec) ParseValue(raw string) (config.Value, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		if t.EmptyIsSet {
			return config.VNull(), nil
		}
		return config.Value{}, fmt.Errorf("empty cell")
	}
	if s == "NONE" && t.Nullable {
		return config.VNull(), nil
	}
	// strip a single matching outer backtick pair
	if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' && !strings.Contains(s[1:len(s)-1], "`") {
		s = s[1 : len(s)-1]
	}
	v, err := t.parse(s)
	if err != nil {
		return config.Value{}, err
	}
	if err := t.checkBounds(v); err != nil {
		return config.Value{}, err
	}
	return v, nil
}

func (t TypeSpec) parse(s string) (config.Value, error) {
	switch t.Name {
	case "id":
		if !idRe.MatchString(s) {
			return config.Value{}, fmt.Errorf("id %q: want [a-z0-9_]+(\\.[a-z0-9_]+)*", s)
		}
		return config.VStr(s), nil
	case "int", "grouped_int":
		return t.parseInt(s)
	case "decimal", "ratio":
		return t.parseRat(s)
	case "bp":
		v, err := t.parseInt(s)
		if err != nil {
			return config.Value{}, err
		}
		if v.Int < 0 || v.Int > 10000 {
			return config.Value{}, fmt.Errorf("bp %d out of 0..10000", v.Int)
		}
		return v, nil
	case "percent":
		// registered alias: exact % decimal * 100 -> bp int
		if !percentRe.MatchString(s) {
			return config.Value{}, fmt.Errorf("percent %q: want `N%%`", s)
		}
		r, err := parseDecimal(strings.TrimSuffix(s, "%"))
		if err != nil {
			return config.Value{}, err
		}
		num := r.Num * 100
		if num%r.Den != 0 {
			return config.Value{}, fmt.Errorf("percent %q: nonintegral bp", s)
		}
		return config.VInt(num / r.Den), nil
	case "enum":
		for _, e := range t.Enum {
			if s == e {
				return config.VStr(s), nil
			}
		}
		return config.Value{}, fmt.Errorf("enum %q not in %v", s, t.Enum)
	case "bool":
		switch s {
		case "true":
			return config.VBool(true), nil
		case "false":
			return config.VBool(false), nil
		}
		return config.Value{}, fmt.Errorf("bool %q: want true|false", s)
	case "range":
		return t.parseRange(s)
	case "set", "ordered":
		return t.parseList(s)
	case "pair":
		return t.parsePair(s)
	case "string":
		return config.VStr(norm.NFC.String(s)), nil
	case "expr":
		e, err := ParseExpr(s)
		if err != nil {
			return config.Value{}, err
		}
		return config.VExpr(e), nil
	}
	return config.Value{}, fmt.Errorf("unknown type %q", t.Name)
}

func (t TypeSpec) parseInt(s string) (config.Value, error) {
	if t.IntPrefix != "" {
		if !strings.HasPrefix(s, t.IntPrefix) {
			return config.Value{}, fmt.Errorf("int %q: want %s prefix", s, t.IntPrefix)
		}
		s = strings.TrimPrefix(s, t.IntPrefix)
	}
	if t.IntSuffix != "" {
		if !strings.HasSuffix(s, t.IntSuffix) {
			return config.Value{}, fmt.Errorf("int %q: want %s suffix", s, t.IntSuffix)
		}
		s = strings.TrimSuffix(s, t.IntSuffix)
	}
	var digits string
	if t.Name == "grouped_int" {
		if !groupedIntRe.MatchString(s) {
			return config.Value{}, fmt.Errorf("grouped_int %q: want 1,000 form", s)
		}
		digits = strings.ReplaceAll(s, ",", "")
	} else {
		digits = s
	}
	if !intRe.MatchString(digits) {
		return config.Value{}, fmt.Errorf("int %q: want -?[0-9]+", s)
	}
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return config.Value{}, fmt.Errorf("int %q: %v", s, err)
	}
	scale := t.IntScale
	if scale == 0 {
		scale = 1
	}
	if n > (1<<63-1)/scale || n < (-1<<63)/scale {
		return config.Value{}, fmt.Errorf("int %q: overflow on unit scale", s)
	}
	return config.VInt(n * scale), nil
}

func (t TypeSpec) parseRat(s string) (config.Value, error) {
	r, err := parseDecimal(s)
	if err != nil {
		return config.Value{}, err
	}
	return config.VRat(r.Num, r.Den)
}

// parseDecimal parses `-?[0-9]+(\.[0-9]+)?` into a reduced rational — exact
// base-10, no binary float, no scientific notation.
func parseDecimal(s string) (config.Rat, error) {
	if !decimalRe.MatchString(s) {
		return config.Rat{}, fmt.Errorf("decimal %q: want -?[0-9]+(\\.[0-9]+)?", s)
	}
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	intPart := s
	fracPart := ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart = s[:i]
		fracPart = s[i+1:]
	}
	ip, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return config.Rat{}, fmt.Errorf("decimal %q: %v", s, err)
	}
	den := int64(1)
	frac := int64(0)
	for _, c := range fracPart {
		frac = frac*10 + int64(c-'0')
		den *= 10
	}
	num := ip*den + frac
	if neg {
		num = -num
	}
	return config.ReduceRat(num, den)
}

func (t TypeSpec) parseRange(s string) (config.Value, error) {
	i := strings.Index(s, "..")
	if i < 0 {
		return config.Value{}, fmt.Errorf("range %q: want min..max", s)
	}
	if t.Elem == nil {
		return config.Value{}, fmt.Errorf("range spec without elem type")
	}
	lo, err := t.Elem.parse(strings.TrimSpace(s[:i]))
	if err != nil {
		return config.Value{}, fmt.Errorf("range %q lo: %v", s, err)
	}
	hi, err := t.Elem.parse(strings.TrimSpace(s[i+2:]))
	if err != nil {
		return config.Value{}, fmt.Errorf("range %q hi: %v", s, err)
	}
	if config.CompareValues(lo, hi) > 0 {
		return config.Value{}, fmt.Errorf("range %q: min > max", s)
	}
	return config.VRec(map[string]config.Value{"min": lo, "max": hi}), nil
}

func (t TypeSpec) parseList(s string) (config.Value, error) {
	if t.Elem == nil {
		return config.Value{}, fmt.Errorf("%s spec without elem type", t.Name)
	}
	parts := strings.Split(s, ",")
	var vals []config.Value
	seen := map[string]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		v, err := t.Elem.ParseValue(p)
		if err != nil {
			return config.Value{}, err
		}
		if t.Name == "set" {
			key := config.KeyString([]config.Value{v})
			if seen[key] {
				return config.Value{}, fmt.Errorf("set %q: duplicate element %q", s, p)
			}
			seen[key] = true
		}
		vals = append(vals, v)
	}
	if t.Name == "set" {
		return config.VSet(vals...), nil
	}
	return config.VList(vals...), nil
}

func (t TypeSpec) parsePair(s string) (config.Value, error) {
	if t.Elem == nil {
		return config.Value{}, fmt.Errorf("pair spec without elem type")
	}
	// `a x b` or `axb`
	var a, b string
	if i := strings.Index(s, " x "); i >= 0 {
		a, b = s[:i], s[i+3:]
	} else if i := strings.Index(s, "x"); i > 0 && i < len(s)-1 {
		a, b = s[:i], s[i+1:]
	} else {
		return config.Value{}, fmt.Errorf("pair %q: want `a x b`", s)
	}
	va, err := t.Elem.parse(strings.TrimSpace(a))
	if err != nil {
		return config.Value{}, err
	}
	vb, err := t.Elem.parse(strings.TrimSpace(b))
	if err != nil {
		return config.Value{}, err
	}
	return config.VList(va, vb), nil
}

func (t TypeSpec) checkBounds(v config.Value) error {
	if v.Kind != config.KindInt && v.Kind != config.KindRational {
		return nil
	}
	asRat := func() config.Rat {
		if v.Kind == config.KindInt {
			return config.Rat{Num: v.Int, Den: 1}
		}
		return v.Rat
	}()
	if t.Min != nil && compareRat(asRat, *t.Min) < 0 {
		return fmt.Errorf("value %s below min %s", asRat, *t.Min)
	}
	if t.Max != nil && compareRat(asRat, *t.Max) > 0 {
		return fmt.Errorf("value %s above max %s", asRat, *t.Max)
	}
	return nil
}

func compareRat(a, b config.Rat) int {
	l := a.Num * b.Den
	r := b.Num * a.Den
	switch {
	case l < r:
		return -1
	case l > r:
		return 1
	}
	return 0
}

// RoundHalfUp is the explicitly declared field adapter for decimal→int
// fields: round at the field boundary, halves away from zero.
func RoundHalfUp(r config.Rat) int64 {
	if r.Den == 0 {
		return 0
	}
	neg := r.Num < 0
	a := r.Num
	if neg {
		a = -a
	}
	q := a / r.Den
	if 2*(a%r.Den) >= r.Den {
		q++
	}
	if neg {
		return -q
	}
	return q
}
