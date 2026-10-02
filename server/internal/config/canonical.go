package config

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
)

// WriteCanonical serializes one typed value as canonical RFC 8259 JSON per
// content_authoring_contract.md §5: UTF-8 without BOM/whitespace, object keys
// sorted by UTF-8 bytes, minimal-decimal integers, reduced
// {"denominator","numerator"} rationals, \" \\ \b \f \n \r \t plus lowercase
// \u00xx only for U+0000..001F, no escaping of '/' or other Unicode.
// A top-level document gets exactly one trailing LF.
func WriteCanonical(w io.Writer, v Value) error {
	var b bytes.Buffer
	if err := writeValue(&b, v); err != nil {
		return err
	}
	b.WriteByte('\n')
	_, err := w.Write(b.Bytes())
	return err
}

// CanonicalBytes returns the canonical payload bytes (one trailing LF).
func CanonicalBytes(v Value) ([]byte, error) {
	var b bytes.Buffer
	if err := WriteCanonical(&b, v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func writeValue(b *bytes.Buffer, v Value) error {
	switch v.Kind {
	case KindNull:
		b.WriteString("null")
	case KindInt:
		b.WriteString(strconv.FormatInt(v.Int, 10))
	case KindRational:
		if i, ok := v.Rat.Int(); ok {
			// integral rationals serialize as minimal ints
			b.WriteString(strconv.FormatInt(i, 10))
			return nil
		}
		b.WriteString(`{"denominator":`)
		b.WriteString(strconv.FormatInt(v.Rat.Den, 10))
		b.WriteString(`,"numerator":`)
		b.WriteString(strconv.FormatInt(v.Rat.Num, 10))
		b.WriteByte('}')
	case KindString:
		writeString(b, v.Str)
	case KindBool:
		if v.Bool {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case KindList:
		b.WriteByte('[')
		for i, e := range v.Elems {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := writeValue(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case KindSet:
		b.WriteByte('[')
		sorted := make([]Value, len(v.Elems))
		copy(sorted, v.Elems)
		sort.Slice(sorted, func(i, j int) bool { return CompareValues(sorted[i], sorted[j]) < 0 })
		for i, e := range sorted {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := writeValue(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case KindRecord:
		b.WriteByte('{')
		keys := make([]string, 0, len(v.Rec))
		for k := range v.Rec {
			keys = append(keys, k)
		}
		sort.Strings(keys) // UTF-8 byte order
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			writeString(b, k)
			b.WriteByte(':')
			if err := writeValue(b, v.Rec[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	case KindExpr:
		if v.Expr == nil {
			b.WriteString("null")
			return nil
		}
		writeExpr(b, v.Expr)
	default:
		return fmt.Errorf("config: cannot serialize value kind %d", v.Kind)
	}
	return nil
}

func writeString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// writeExpr serializes the typed expression syntax tree: field names,
// operators and arguments, never source whitespace.
func writeExpr(b *bytes.Buffer, e *ExprNode) {
	b.WriteByte('{')
	switch e.Op {
	case "lit":
		b.WriteString(`"lit":`)
		if e.IsRat {
			if i, ok := e.Rat.Int(); ok {
				b.WriteString(strconv.FormatInt(i, 10))
			} else {
				b.WriteString(`{"denominator":`)
				b.WriteString(strconv.FormatInt(e.Rat.Den, 10))
				b.WriteString(`,"numerator":`)
				b.WriteString(strconv.FormatInt(e.Rat.Num, 10))
				b.WriteByte('}')
			}
		} else {
			b.WriteString(strconv.FormatInt(e.Num, 10))
		}
	case "ref":
		b.WriteString(`"ref":`)
		writeString(b, e.Name)
	default:
		b.WriteString(`"args":[`)
		for i, a := range e.Args {
			if i > 0 {
				b.WriteByte(',')
			}
			writeExpr(b, a)
		}
		b.WriteString(`],"op":`)
		writeString(b, e.Op)
	}
	b.WriteByte('}')
}

// CanonicalPayload builds the §5 top-level payload record for a candidate:
// definitions grouped by family and sorted by declared composite key.
// The geometry family itself is a member; its own revision field is excluded
// upstream so no recursive hash can occur.
func CanonicalPayload(snap *CandidateSnapshot) Value {
	defs := map[string]Value{}
	for name, fam := range snap.Definitions {
		defs[name] = familyValue(fam)
	}
	var vp Value = VNull()
	if snap.ValidationParameters != nil {
		vp = familyValue(snap.ValidationParameters)
	}
	var geom Value = VNull()
	if snap.Geometry != nil {
		geom = familyValue(snap.Geometry)
	}
	rv := map[string]Value{}
	for k, v := range snap.RuleVersions {
		rv[k] = VInt(int64(v))
	}
	return VRec(map[string]Value{
		"authoring_schema_version": VInt(int64(snap.AuthoringSchemaVersion)),
		"content_schema_version":   VInt(int64(snap.ContentSchemaVersion)),
		"rule_versions":            VRec(rv),
		"definitions":              VRec(defs),
		"validation_parameters":    vp,
		"geometry":                 geom,
	})
}

// familyValue emits one family's records as a list sorted by composite key.
func familyValue(f *Family) Value {
	recs := make([]Value, 0, len(f.Records))
	for _, key := range f.SortedKeys() {
		rec := f.Records[KeyString(key)]
		recs = append(recs, VRec(rec.Fields))
	}
	return VList(recs...)
}
