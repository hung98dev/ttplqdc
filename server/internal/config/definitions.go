package config

import (
	"fmt"
	"strings"

	"thinhthan/internal/core/id"
)

// Kind tags one typed value in the compiled semantic model
// (content_authoring_contract.md §1 typed grammar, §5 canonical payload).
type Kind int

const (
	KindNull Kind = iota
	KindInt
	KindRational
	KindString
	KindBool
	KindList   // semantic list; element order is the declared order
	KindSet    // comma-token set; elements hold canonical order at emit
	KindRecord // object; member order is irrelevant (sorted at write)
	KindExpr   // typed expression tree
)

// Rat is an exact base-10 rational (contract §1 `decimal`/`ratio`): reduced
// numerator/positive denominator. No binary float anywhere.
type Rat struct {
	Num int64
	Den int64
}

// Int returns the integer value when the rational is integral.
func (r Rat) Int() (int64, bool) {
	if r.Den == 0 || r.Num%r.Den != 0 {
		return 0, false
	}
	return r.Num / r.Den, true
}

func (r Rat) String() string {
	if i, ok := r.Int(); ok {
		return fmt.Sprintf("%d", i)
	}
	return fmt.Sprintf("%d/%d", r.Num, r.Den)
}

// ReduceRat returns the reduced form with positive denominator.
func ReduceRat(num, den int64) (Rat, error) {
	if den == 0 {
		return Rat{}, fmt.Errorf("config: rational with zero denominator")
	}
	if den < 0 {
		num, den = -num, -den
	}
	g := gcd(abs64(num), den)
	return Rat{Num: num / g, Den: den / g}, nil
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// Value is the compiler's typed semantic value.
type Value struct {
	Kind  Kind
	Int   int64            // KindInt
	Rat   Rat              // KindRational
	Str   string           // KindString
	Bool  bool             // KindBool
	Elems []Value          // KindList, KindSet
	Rec   map[string]Value // KindRecord
	Expr  *ExprNode        // KindExpr
}

// ExprNode is one node of the §1 typed expression grammar: literals, field
// references, unary negation, binary + - * /, and the finite call set
// floor/ceil/min/max/clamp.
type ExprNode struct {
	Op    string // "lit", "ref", "neg", "+", "-", "*", "/", or call name
	Num   int64  // Op=="lit" integer literal
	Rat   Rat    // Op=="lit" rational literal (preferred when set)
	IsRat bool
	Name  string      // Op=="ref" field name; Op==call call name
	Args  []*ExprNode // operand(s)
}

// V constructors keep emission sites terse and total.
func VNull() Value       { return Value{Kind: KindNull} }
func VInt(v int64) Value { return Value{Kind: KindInt, Int: v} }
func VRat(num, den int64) (Value, error) {
	r, err := ReduceRat(num, den)
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: KindRational, Rat: r}, nil
}
func VStr(s string) Value { return Value{Kind: KindString, Str: s} }
func VBool(b bool) Value  { return Value{Kind: KindBool, Bool: b} }
func VList(e ...Value) Value {
	if e == nil {
		e = []Value{}
	}
	return Value{Kind: KindList, Elems: e}
}
func VSet(e ...Value) Value {
	if e == nil {
		e = []Value{}
	}
	return Value{Kind: KindSet, Elems: e}
}
func VRec(m map[string]Value) Value {
	if m == nil {
		m = map[string]Value{}
	}
	return Value{Kind: KindRecord, Rec: m}
}
func VExpr(e *ExprNode) Value { return Value{Kind: KindExpr, Expr: e} }

// Strs converts string values to a KindList for one call site.
func VStrs(ss ...string) Value {
	v := make([]Value, len(ss))
	for i, s := range ss {
		v[i] = VStr(s)
	}
	return VList(v...)
}

// CompareValues orders values for canonical composite-key and set sorting:
// KindInt numerically, strings/enums/ids by UTF-8 bytes, rationals by exact
// cross-product, bools false<true, then by kind rank for mixed inputs.
func CompareValues(a, b Value) int {
	if a.Kind != b.Kind {
		if a.Kind == KindInt || b.Kind == KindInt {
			// typed integers always sort numerically ahead of other kinds
			if a.Kind == KindInt {
				return -1
			}
			return 1
		}
		if a.Kind < b.Kind {
			return -1
		}
		return 1
	}
	switch a.Kind {
	case KindInt:
		switch {
		case a.Int < b.Int:
			return -1
		case a.Int > b.Int:
			return 1
		}
	case KindRational:
		l := a.Rat.Num * b.Rat.Den
		r := b.Rat.Num * a.Rat.Den
		switch {
		case l < r:
			return -1
		case l > r:
			return 1
		}
	case KindString:
		return strings.Compare(a.Str, b.Str)
	case KindBool:
		if a.Bool != b.Bool {
			if !a.Bool {
				return -1
			}
			return 1
		}
	case KindList, KindSet:
		for i := 0; i < len(a.Elems) && i < len(b.Elems); i++ {
			if c := CompareValues(a.Elems[i], b.Elems[i]); c != 0 {
				return c
			}
		}
		switch {
		case len(a.Elems) < len(b.Elems):
			return -1
		case len(a.Elems) > len(b.Elems):
			return 1
		}
	}
	return 0
}

// KeyString renders a composite record key deterministically for indexes.
func KeyString(key []Value) string {
	var b strings.Builder
	for i, v := range key {
		if i > 0 {
			b.WriteByte(0)
		}
		switch v.Kind {
		case KindInt:
			fmt.Fprintf(&b, "i:%d", v.Int)
		case KindRational:
			fmt.Fprintf(&b, "r:%s", v.Rat)
		case KindString:
			fmt.Fprintf(&b, "s:%s", v.Str)
		case KindBool:
			fmt.Fprintf(&b, "b:%t", v.Bool)
		default:
			fmt.Fprintf(&b, "k:%d", v.Kind)
		}
	}
	return b.String()
}

// Record is one emitted definition row keyed by the family's declared
// composite key.
type Record struct {
	Key    []Value
	Fields map[string]Value
}

// Family groups records under one stable family name (contract §3 keys).
// Records are keyed by their serialized composite key; emission sorts keys
// with CompareValues (typed ints numerically, IDs by UTF-8 bytes).
type Family struct {
	Name       string
	KeyColumns []string
	Records    map[string]Record
	keys       map[string][]Value
}

// NewFamily declares a family and its composite key columns.
func NewFamily(name string, keyColumns ...string) *Family {
	return &Family{
		Name:       name,
		KeyColumns: keyColumns,
		Records:    map[string]Record{},
		keys:       map[string][]Value{},
	}
}

// Put inserts a record; a duplicate key reports the would-be
// DUPLICATE_PRIMARY_KEY location to the caller.
func (f *Family) Put(key []Value, fields map[string]Value) (dup bool, existing Record) {
	ks := KeyString(key)
	if old, ok := f.Records[ks]; ok {
		return true, old
	}
	f.Records[ks] = Record{Key: key, Fields: fields}
	f.keys[ks] = key
	return false, Record{}
}

// SortedKeys returns composite keys in canonical order.
func (f *Family) SortedKeys() [][]Value {
	keys := make([][]Value, 0, len(f.keys))
	for _, k := range f.keys {
		keys = append(keys, k)
	}
	sortKeys(keys)
	return keys
}

func sortKeys(keys [][]Value) {
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && compareKeys(keys[j], keys[j-1]) < 0; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
}

func compareKeys(a, b []Value) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := CompareValues(a[i], b[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}

// CheckStableID asserts an emitted identity field satisfies the canonical
// static content ID grammar (ids.md, F-2.4 dual-check rule).
func CheckStableID(s string) error {
	return id.ValidateStaticContentID(s)
}
