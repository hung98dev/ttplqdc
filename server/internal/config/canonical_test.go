package config

import (
	"strings"
	"testing"
)

func TestCanonical_KeySortByUTF8Bytes(t *testing.T) {
	v := VRec(map[string]Value{
		"b":  VInt(1),
		"a":  VInt(2),
		"aa": VInt(3),
		"B":  VInt(4),
	})
	b, err := CanonicalBytes(v)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"B":4,"a":2,"aa":3,"b":1}` + "\n"
	if string(b) != want {
		t.Fatalf("got %q want %q", b, want)
	}
}

func TestCanonical_MinimalInts(t *testing.T) {
	b, err := CanonicalBytes(VList(VInt(0), VInt(-12), VInt(9007199254740993)))
	if err != nil {
		t.Fatal(err)
	}
	want := `[0,-12,9007199254740993]` + "\n"
	if string(b) != want {
		t.Fatalf("got %q want %q", b, want)
	}
}

func TestCanonical_RationalReduced(t *testing.T) {
	// 0.20 == 0.2: reduced 1/5
	v1, err := VRat(20, 100)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := VRat(2, 10)
	if err != nil {
		t.Fatal(err)
	}
	b1, _ := CanonicalBytes(v1)
	b2, _ := CanonicalBytes(v2)
	want := `{"denominator":5,"numerator":1}` + "\n"
	if string(b1) != want || string(b2) != want {
		t.Fatalf("got %q / %q want %q", b1, b2, want)
	}
	// negative denominator normalized
	v3, err := VRat(3, -7)
	if err != nil {
		t.Fatal(err)
	}
	b3, _ := CanonicalBytes(v3)
	want3 := `{"denominator":7,"numerator":-3}` + "\n"
	if string(b3) != want3 {
		t.Fatalf("got %q want %q", b3, want3)
	}
	// integral rational serializes as int
	v4, err := VRat(8, 4)
	if err != nil {
		t.Fatal(err)
	}
	b4, _ := CanonicalBytes(v4)
	if string(b4) != "2\n" {
		t.Fatalf("got %q want 2", b4)
	}
}

func TestCanonical_ControlEscapesLowercase(t *testing.T) {
	b, err := CanonicalBytes(VStr("\x01\x1f\"\\/\b\f\n\r\t"))
	if err != nil {
		t.Fatal(err)
	}
	want := `"\u0001\u001f\"\\/\b\f\n\r\t"` + "\n"
	if string(b) != want {
		t.Fatalf("got %q want %q", b, want)
	}
}

func TestCanonical_NoSlashEscaping(t *testing.T) {
	b, err := CanonicalBytes(VStr("a/b/c"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `a/b/c`) {
		t.Fatalf("slash escaped: %q", b)
	}
}

func TestCanonical_ExplicitNull(t *testing.T) {
	b, err := CanonicalBytes(VRec(map[string]Value{"x": VNull()}))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "{\"x\":null}\n" {
		t.Fatalf("got %q", b)
	}
}

func TestCanonical_SingleTrailingLF(t *testing.T) {
	b, err := CanonicalBytes(VInt(1))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(b), "}\n") && !strings.HasSuffix(string(b), "1\n") {
		t.Fatalf("no trailing LF: %q", b)
	}
	if strings.HasSuffix(string(b), "\n\n") {
		t.Fatalf("extra trailing LF: %q", b)
	}
}

func TestCanonical_MapOrderIndependent(t *testing.T) {
	for i := 0; i < 50; i++ {
		m := map[string]Value{}
		for _, k := range []string{"z", "y", "x", "w", "v"} {
			m[k] = VInt(int64(i))
		}
		b, err := CanonicalBytes(VRec(m))
		if err != nil {
			t.Fatal(err)
		}
		want := `{"v":` + itoa(i) + `,"w":` + itoa(i) + `,"x":` + itoa(i) + `,"y":` + itoa(i) + `,"z":` + itoa(i) + "}\n"
		if string(b) != want {
			t.Fatalf("got %q want %q", b, want)
		}
	}
}

func TestCanonical_SetSortsCanonically(t *testing.T) {
	b, err := CanonicalBytes(VSet(VStr("b"), VStr("a"), VStr("c")))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "[\"a\",\"b\",\"c\"]\n" {
		t.Fatalf("got %q", b)
	}
	bi, err := CanonicalBytes(VSet(VInt(10), VInt(2), VInt(-1)))
	if err != nil {
		t.Fatal(err)
	}
	if string(bi) != "[-1,2,10]\n" {
		t.Fatalf("got %q", bi)
	}
}

func TestCanonical_ExprTreeSerialization(t *testing.T) {
	e := VExpr(&ExprNode{
		Op: "+",
		Args: []*ExprNode{
			{Op: "ref", Name: "L"},
			{Op: "lit", Num: 5},
		},
	})
	b, err := CanonicalBytes(e)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"args":[{"ref":"L"},{"lit":5}],"op":"+"}` + "\n"
	if string(b) != want {
		t.Fatalf("got %q want %q", b, want)
	}
}

func TestCanonical_UnicodeUnescaped(t *testing.T) {
	b, err := CanonicalBytes(VStr("Rừng U Minh © α"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "Rừng U Minh") {
		t.Fatalf("unicode escaped: %q", b)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var d [20]byte
	p := len(d)
	for i > 0 {
		p--
		d[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		d[p] = '-'
	}
	return string(d[p:])
}
