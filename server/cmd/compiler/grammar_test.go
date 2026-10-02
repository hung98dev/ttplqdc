package main

import (
	"testing"

	"thinhthan/internal/config"
)

func TSpec(name string) TypeSpec { return TypeSpec{Name: name} }

func TestGrammar_ID(t *testing.T) {
	for _, ok := range []string{"monster.rung_u_minh.wolf", "x.y_2", "a.b"} {
		if _, err := TSpec("id").ParseValue(ok); err != nil {
			t.Fatalf("id %q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"A.b", "a..b", "-a.b", "a b", "1"} {
		// note: "1" is legal per contract grammar ([a-z0-9_]+ single segment);
		// static-id strictness is enforced separately at emit (F-2.4)
		if bad == "1" {
			if _, err := TSpec("id").ParseValue(bad); err != nil {
				t.Fatalf("contract id %q rejected", bad)
			}
			continue
		}
		if _, err := TSpec("id").ParseValue(bad); err == nil {
			t.Fatalf("id %q accepted", bad)
		}
	}
}

func TestGrammar_Int(t *testing.T) {
	v, err := TSpec("int").ParseValue("-42")
	if err != nil || v.Int != -42 {
		t.Fatalf("%v %v", v, err)
	}
	if _, err := TSpec("int").ParseValue("1.5"); err == nil {
		t.Fatal("decimal accepted as int")
	}
	if _, err := TSpec("int").ParseValue("1,000"); err == nil {
		t.Fatal("grouped accepted as plain int")
	}
}

func TestGrammar_GroupedIntAdapter(t *testing.T) {
	g := TypeSpec{Name: "grouped_int"}
	v, err := g.ParseValue("1,091,400")
	if err != nil || v.Int != 1091400 {
		t.Fatalf("%v %v", v, err)
	}
	if _, err := g.ParseValue("12345"); err == nil {
		t.Fatal("ungrouped accepted")
	}
	if _, err := g.ParseValue("1,00"); err == nil {
		t.Fatal("bad grouping accepted")
	}
}

func TestGrammar_DecimalExactRational(t *testing.T) {
	v, err := TSpec("ratio").ParseValue("0.20")
	if err != nil {
		t.Fatal(err)
	}
	if v.Rat.Num != 1 || v.Rat.Den != 5 {
		t.Fatalf("rat: %v", v.Rat)
	}
	if _, err := TSpec("decimal").ParseValue("1e3"); err == nil {
		t.Fatal("scientific accepted")
	}
	if _, err := TSpec("decimal").ParseValue(".5"); err == nil {
		t.Fatal("missing int part accepted")
	}
}

func TestGrammar_BPAndPercentAlias(t *testing.T) {
	v, err := TSpec("bp").ParseValue("8500")
	if err != nil || v.Int != 8500 {
		t.Fatalf("%v %v", v, err)
	}
	if _, err := TSpec("bp").ParseValue("10001"); err == nil {
		t.Fatal("bp >10000 accepted")
	}
	p := TypeSpec{Name: "percent"}
	v, err = p.ParseValue("85%")
	if err != nil || v.Int != 8500 {
		t.Fatalf("percent: %v %v", v, err)
	}
	if _, err := p.ParseValue("0.555%"); err == nil {
		t.Fatal("nonintegral bp accepted")
	}
}

func TestGrammar_EnumBool(t *testing.T) {
	e := TypeSpec{Name: "enum", Enum: []string{"KIM", "MOC"}}
	if _, err := e.ParseValue("KIM"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ParseValue("kim"); err == nil {
		t.Fatal("enum case-insensitive accepted")
	}
	if _, err := TSpec("bool").ParseValue("True"); err == nil {
		t.Fatal("bool uppercase accepted")
	}
}

func TestGrammar_RangeSetOrderedPair(t *testing.T) {
	r := TypeSpec{Name: "range", Elem: &TypeSpec{Name: "int"}}
	v, err := r.ParseValue("1..10")
	if err != nil {
		t.Fatal(err)
	}
	if v.Rec["min"].Int != 1 || v.Rec["max"].Int != 10 {
		t.Fatalf("range: %v", v)
	}
	if _, err := r.ParseValue("10..1"); err == nil {
		t.Fatal("inverted range accepted")
	}
	s := TypeSpec{Name: "set", Elem: &TypeSpec{Name: "id"}}
	vs, err := s.ParseValue("a.b, c.d, e.f")
	if err != nil {
		t.Fatal(err)
	}
	if vs.Kind != config.KindSet || len(vs.Elems) != 3 {
		t.Fatalf("set: %v", vs)
	}
	if _, err := s.ParseValue("a.b, a.b"); err == nil {
		t.Fatal("dup set element accepted")
	}
	o := TypeSpec{Name: "ordered", Elem: &TypeSpec{Name: "id"}}
	vo, err := o.ParseValue("c.d, a.b")
	if err != nil {
		t.Fatal(err)
	}
	if vo.Elems[0].Str != "c.d" {
		t.Fatalf("ordered reordered: %v", vo)
	}
	p := TypeSpec{Name: "pair", Elem: &TypeSpec{Name: "int"}}
	vp2, err := p.ParseValue("1280 x 720")
	if err != nil || vp2.Elems[0].Int != 1280 || vp2.Elems[1].Int != 720 {
		t.Fatalf("pair: %v %v", vp2, err)
	}
	vp3, err := p.ParseValue("1280x720")
	if err != nil || vp3.Elems[0].Int != 1280 {
		t.Fatalf("pair axb: %v %v", vp3, err)
	}
}

func TestGrammar_StringNFC(t *testing.T) {
	// NFC canonical form: precomposed where possible
	v2, _ := TSpec("string").ParseValue("Cafe\u0301")
	if v2.Str != "Caf\u00e9" {
		t.Fatalf("NFC failed: %q", v2.Str)
	}
	// Vietnamese combining marks normalize too
	v3, _ := TSpec("string").ParseValue("Thu\u00f4\u0300ng")
	if v3.Str != "Thu\u1ed3ng" {
		t.Fatalf("NFC vi: %q", v3.Str)
	}
}

func TestGrammar_NullableNONE(t *testing.T) {
	sp := TypeSpec{Name: "string", Nullable: true}
	v, err := sp.ParseValue("NONE")
	if err != nil || v.Kind != config.KindNull {
		t.Fatalf("%v %v", v, err)
	}
	sp2 := TypeSpec{Name: "int", Nullable: true}
	v2, err := sp2.ParseValue("NONE")
	if err != nil || v2.Kind != config.KindNull {
		t.Fatalf("%v %v", v2, err)
	}
	// NONE rejected when not nullable
	if _, err := TSpec("int").ParseValue("NONE"); err == nil {
		t.Fatal("NONE accepted without nullable")
	}
	if _, err := TSpec("int").ParseValue(""); err == nil {
		t.Fatal("empty cell accepted")
	}
}

func TestGrammar_Adapters(t *testing.T) {
	lv := TypeSpec{Name: "int", IntPrefix: "Lv"}
	v, err := lv.ParseValue("Lv60")
	if err != nil || v.Int != 60 {
		t.Fatalf("%v %v", v, err)
	}
	if _, err := lv.ParseValue("60"); err == nil {
		t.Fatal("Lv prefix omitted accepted")
	}
	secs := TypeSpec{Name: "int", IntSuffix: "s", IntScale: 1000}
	v, err = secs.ParseValue("12s")
	if err != nil || v.Int != 12000 {
		t.Fatalf("%v %v", v, err)
	}
}

func TestExpr_PrecedenceAndEval(t *testing.T) {
	e, err := ParseExpr("10000*L*L")
	if err != nil {
		t.Fatal(err)
	}
	v, err := EvalExpr(e, map[string]config.Rat{"L": {Num: 12, Den: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if i, _ := v.Int(); i != 1440000 {
		t.Fatalf("v=%s", v)
	}
	e2, _ := ParseExpr("1+2*3")
	v2, _ := EvalExpr(e2, nil)
	if i, _ := v2.Int(); i != 7 {
		t.Fatalf("precedence: %s", v2)
	}
	e3, _ := ParseExpr("10-4-3")
	v3, _ := EvalExpr(e3, nil)
	if i, _ := v3.Int(); i != 3 {
		t.Fatalf("left assoc: %s", v3)
	}
}

func TestExpr_CallsAndDivision(t *testing.T) {
	e, _ := ParseExpr("floor(7/2)")
	v, _ := EvalExpr(e, nil)
	if i, _ := v.Int(); i != 3 {
		t.Fatalf("floor: %s", v)
	}
	e, _ = ParseExpr("ceil(7/2)")
	v, _ = EvalExpr(e, nil)
	if i, _ := v.Int(); i != 4 {
		t.Fatalf("ceil: %s", v)
	}
	e, _ = ParseExpr("clamp(15,1,10)")
	v, _ = EvalExpr(e, nil)
	if i, _ := v.Int(); i != 10 {
		t.Fatalf("clamp: %s", v)
	}
	e, _ = ParseExpr("min(2,5,1)")
	v, _ = EvalExpr(e, nil)
	if i, _ := v.Int(); i != 1 {
		t.Fatalf("min: %s", v)
	}
	if _, err := EvalExpr(mustParse("1/0"), nil); err == nil {
		t.Fatal("div by zero accepted")
	}
	// exact rational division: 1/3 + 1/3 + 1/3 = 1
	e, _ = ParseExpr("1/3+1/3+1/3")
	v, _ = EvalExpr(e, nil)
	if i, ok := v.Int(); !ok || i != 1 {
		t.Fatalf("rational: %s", v)
	}
}

func TestExpr_Rejects(t *testing.T) {
	for _, bad := range []string{"foo(x)", "x=1", "2x", "1 2", "sin(0)", "x!"} {
		if _, err := ParseExpr(bad); err == nil {
			t.Fatalf("expr %q accepted", bad)
		}
	}
}

func mustParse(s string) *config.ExprNode {
	e, err := ParseExpr(s)
	if err != nil {
		panic(err)
	}
	return e
}

func TestExpr_RoundHalfUpAdapter(t *testing.T) {
	neg, _ := ParseExpr("-1/2")
	n, err := RoundHalfUpExpr(neg, nil)
	if err != nil || n != -1 {
		t.Fatalf("%d %v", n, err)
	}
	pos, _ := ParseExpr("5/2")
	p, _ := RoundHalfUpExpr(pos, nil)
	if p != 3 {
		t.Fatalf("%d", p)
	}
}
