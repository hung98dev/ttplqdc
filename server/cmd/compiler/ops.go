package main

import (
	"fmt"
	"strings"

	"thinhthan/internal/config"
)

// ops.go — typed `KIND(arg;...)` op-list parser shared by the mechanic-ops
// grammars (monster Typed Mechanic Operations, Typed Roster Overrides).

// opCtor declares one constructor's name and argument spec: 'i' int,
// 'r' ratio/decimal, 'e' enum token, 's' id string.
type opCtor struct {
	Name string
	Args string // e.g. "errr"
}

// mechanicOpCtors is the closed constructor set of the monster catalog's
// Typed Mechanic Operations table + the FOLLOWUP op the Typed Roster
// Overrides section declares. Unknown constructors reject.
var mechanicOpCtors = map[string]string{
	"VISUAL":      "e",
	"STATUS":      "eiie",
	"PULL":        "r",
	"DOT":         "erii",
	"TRAIL":       "riii",
	"RANGE":       "r",
	"SHIELD":      "rii",
	"SHOTS":       "irr",
	"FEINT":       "ii",
	"BOUNCE":      "ir",
	"EXPAND":      "rri",
	"COUNTER":     "err",
	"EXIT_EXTEND": "iii",
	"FOLLOWUP":    "eri",
}

// parseOpArgs parses one arg token per its spec char.
func parseOpArg(spec byte, raw string) (config.Value, error) {
	tok := strings.TrimSpace(raw)
	switch spec {
	case 'i':
		return (TypeSpec{Name: "int"}).ParseValue(tok)
	case 'r':
		r, err := parseDecimal(tok)
		if err != nil {
			if v, e2 := (TypeSpec{Name: "int"}).ParseValue(tok); e2 == nil {
				return v, nil
			}
			return config.Value{}, err
		}
		return config.VRat(r.Num, r.Den)
	case 'e':
		if !idToken(tok) {
			return config.Value{}, fmt.Errorf("enum token %q", tok)
		}
		return config.VStr(tok), nil
	case 's':
		return config.VStr(tok), nil
	}
	return config.Value{}, fmt.Errorf("op arg spec %q", spec)
}

func idToken(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

// ParseOps parses `KIND(arg;...),KIND2(...)` into an ordered list of op
// records {kind, args}. Constructor set comes from allowed (defaults to the
// mechanic set when nil). Unknown constructor/arity rejects.
func ParseOps(src string, allowed map[string]string) ([]config.Value, error) {
	if allowed == nil {
		allowed = mechanicOpCtors
	}
	var out []config.Value
	for _, tok := range splitTop(src, ',') {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		open := strings.IndexByte(tok, '(')
		if open < 0 || !strings.HasSuffix(tok, ")") {
			return nil, fmt.Errorf("op %q is not KIND(args)", tok)
		}
		kind := strings.TrimSpace(tok[:open])
		sig, ok := allowed[kind]
		if !ok {
			return nil, fmt.Errorf("unknown op constructor %q", kind)
		}
		argSrc := tok[open+1 : len(tok)-1]
		var args []string
		if argSrc != "" {
			args = splitTop(argSrc, ';')
		}
		if len(args) != len(sig) {
			return nil, fmt.Errorf("op %s takes %d args, got %d", kind, len(sig), len(args))
		}
		av := make([]config.Value, len(args))
		for i, a := range args {
			v, err := parseOpArg(sig[i], a)
			if err != nil {
				return nil, fmt.Errorf("op %s arg %d: %w", kind, i+1, err)
			}
			av[i] = v
		}
		out = append(out, config.VRec(map[string]config.Value{
			"args": config.VList(av...),
			"kind": config.VStr(kind),
		}))
	}
	return out, nil
}
