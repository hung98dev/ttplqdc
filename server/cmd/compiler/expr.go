package main

import (
	"fmt"
	"strings"

	"thinhthan/internal/config"
)

// Expr parser for the §1 typed expression grammar:
// literals, declared field references, parentheses, unary -, binary
// + - * / (left-assoc, * / before + -), and finite calls
// floor/ceil/min/max/clamp. No implicit multiplication, assignment,
// randomness or other calls. Division is exact rational until an explicit
// rounding operator; division by zero rejects.

type exprParser struct {
	s   string
	pos int
}

// ParseExpr parses source text into a typed AST.
func ParseExpr(s string) (*config.ExprNode, error) {
	p := &exprParser{s: s}
	e, err := p.addSub()
	if err != nil {
		return nil, err
	}
	p.ws()
	if p.pos != len(p.s) {
		return nil, fmt.Errorf("expr %q: trailing input at %d", s, p.pos)
	}
	return e, nil
}

func (p *exprParser) ws() {
	for p.pos < len(p.s) && (p.s[p.pos] == ' ' || p.s[p.pos] == '\t') {
		p.pos++
	}
}

func (p *exprParser) peek() byte {
	if p.pos >= len(p.s) {
		return 0
	}
	return p.s[p.pos]
}

func (p *exprParser) addSub() (*config.ExprNode, error) {
	l, err := p.mulDiv()
	if err != nil {
		return nil, err
	}
	for {
		p.ws()
		c := p.peek()
		if c != '+' && c != '-' {
			return l, nil
		}
		p.pos++
		r, err := p.mulDiv()
		if err != nil {
			return nil, err
		}
		l = &config.ExprNode{Op: string(c), Args: []*config.ExprNode{l, r}}
	}
}

func (p *exprParser) mulDiv() (*config.ExprNode, error) {
	l, err := p.unary()
	if err != nil {
		return nil, err
	}
	for {
		p.ws()
		c := p.peek()
		if c != '*' && c != '/' {
			return l, nil
		}
		p.pos++
		r, err := p.unary()
		if err != nil {
			return nil, err
		}
		l = &config.ExprNode{Op: string(c), Args: []*config.ExprNode{l, r}}
	}
}

func (p *exprParser) unary() (*config.ExprNode, error) {
	p.ws()
	if p.peek() == '-' {
		p.pos++
		e, err := p.unary()
		if err != nil {
			return nil, err
		}
		return &config.ExprNode{Op: "neg", Args: []*config.ExprNode{e}}, nil
	}
	return p.atom()
}

var callArity = map[string]int{"floor": 1, "ceil": 1, "min": -1, "max": -1, "clamp": 3}

func (p *exprParser) atom() (*config.ExprNode, error) {
	p.ws()
	c := p.peek()
	switch {
	case c == '(':
		p.pos++
		e, err := p.addSub()
		if err != nil {
			return nil, err
		}
		p.ws()
		if p.peek() != ')' {
			return nil, fmt.Errorf("expr %q: expected ')' at %d", p.s, p.pos)
		}
		p.pos++
		return e, nil
	case c >= '0' && c <= '9' || c == '.':
		return p.number()
	case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_':
		return p.ident()
	}
	return nil, fmt.Errorf("expr %q: unexpected %q at %d", p.s, string(c), p.pos)
}

func (p *exprParser) number() (*config.ExprNode, error) {
	start := p.pos
	for p.pos < len(p.s) && (p.s[p.pos] >= '0' && p.s[p.pos] <= '9' || p.s[p.pos] == '.') {
		p.pos++
	}
	s := p.s[start:p.pos]
	r, err := parseDecimal(s)
	if err != nil {
		return nil, fmt.Errorf("expr literal %q: %v", s, err)
	}
	if i, ok := r.Int(); ok {
		return &config.ExprNode{Op: "lit", Num: i}, nil
	}
	return &config.ExprNode{Op: "lit", Rat: r, IsRat: true}, nil
}

func (p *exprParser) ident() (*config.ExprNode, error) {
	start := p.pos
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c >= '0' && c <= '9') {
			break
		}
		p.pos++
	}
	name := p.s[start:p.pos]
	p.ws()
	if p.peek() != '(' {
		return &config.ExprNode{Op: "ref", Name: name}, nil
	}
	arity, ok := callArity[name]
	if !ok {
		return nil, fmt.Errorf("expr %q: unknown call %q (allowed: floor,ceil,min,max,clamp)", p.s, name)
	}
	p.pos++
	var args []*config.ExprNode
	p.ws()
	for {
		e, err := p.addSub()
		if err != nil {
			return nil, err
		}
		args = append(args, e)
		p.ws()
		if p.peek() == ',' {
			p.pos++
			p.ws()
			continue
		}
		break
	}
	if p.peek() != ')' {
		return nil, fmt.Errorf("expr %q: call %q expected ')' at %d", p.s, name, p.pos)
	}
	p.pos++
	if arity >= 0 && len(args) != arity {
		return nil, fmt.Errorf("expr %q: %s wants %d args, got %d", p.s, name, arity, len(args))
	}
	if arity < 0 && len(args) < 2 {
		return nil, fmt.Errorf("expr %q: %s wants >=2 args, got %d", p.s, name, len(args))
	}
	return &config.ExprNode{Op: name, Args: args}, nil
}

// EvalExpr evaluates a typed AST against a field environment; all arithmetic
// is exact rational. floor/ceil/min/max/clamp resolve; division by zero and
// non-integer floor/ceil operand semantics are enforced.
func EvalExpr(e *config.ExprNode, env map[string]config.Rat) (config.Rat, error) {
	switch e.Op {
	case "lit":
		if e.IsRat {
			return e.Rat, nil
		}
		return config.Rat{Num: e.Num, Den: 1}, nil
	case "ref":
		v, ok := env[e.Name]
		if !ok {
			return config.Rat{}, fmt.Errorf("expr: unknown ref %q", e.Name)
		}
		return v, nil
	case "neg":
		v, err := EvalExpr(e.Args[0], env)
		if err != nil {
			return config.Rat{}, err
		}
		return config.Rat{Num: -v.Num, Den: v.Den}, nil
	case "+", "-", "*", "/":
		l, err := EvalExpr(e.Args[0], env)
		if err != nil {
			return config.Rat{}, err
		}
		r, err := EvalExpr(e.Args[1], env)
		if err != nil {
			return config.Rat{}, err
		}
		return ratOp(e.Op, l, r)
	case "floor", "ceil", "min", "max", "clamp":
		vals := make([]config.Rat, len(e.Args))
		for i, a := range e.Args {
			v, err := EvalExpr(a, env)
			if err != nil {
				return config.Rat{}, err
			}
			vals[i] = v
		}
		switch e.Op {
		case "floor":
			return config.Rat{Num: floorDiv(vals[0].Num, vals[0].Den), Den: 1}, nil
		case "ceil":
			return config.Rat{Num: -floorDiv(-vals[0].Num, vals[0].Den), Den: 1}, nil
		case "min":
			m := vals[0]
			for _, v := range vals[1:] {
				if compareRat(v, m) < 0 {
					m = v
				}
			}
			return m, nil
		case "max":
			m := vals[0]
			for _, v := range vals[1:] {
				if compareRat(v, m) > 0 {
					m = v
				}
			}
			return m, nil
		case "clamp":
			lo, hi := vals[1], vals[2]
			v := vals[0]
			if compareRat(v, lo) < 0 {
				return lo, nil
			}
			if compareRat(v, hi) > 0 {
				return hi, nil
			}
			return v, nil
		}
	}
	return config.Rat{}, fmt.Errorf("expr: bad op %q", e.Op)
}

func ratOp(op string, l, r config.Rat) (config.Rat, error) {
	switch op {
	case "+":
		return config.ReduceRat(l.Num*r.Den+r.Num*l.Den, l.Den*r.Den)
	case "-":
		return config.ReduceRat(l.Num*r.Den-r.Num*l.Den, l.Den*r.Den)
	case "*":
		return config.ReduceRat(l.Num*r.Num, l.Den*r.Den)
	case "/":
		if r.Num == 0 {
			return config.Rat{}, fmt.Errorf("expr: division by zero")
		}
		return config.ReduceRat(l.Num*r.Den, l.Den*r.Num)
	}
	return config.Rat{}, fmt.Errorf("expr: bad op %q", op)
}

// floorDiv is mathematical floor division (den > 0).
func floorDiv(n, d int64) int64 {
	q := n / d
	if (n%d != 0) && (n < 0) != (d < 0) {
		q--
	}
	return q
}

// RoundHalfUpExpr applies the declared `round_half_up` field adapter: the
// only legal decimal→int stage unless the owning field declares otherwise.
func RoundHalfUpExpr(e *config.ExprNode, env map[string]config.Rat) (int64, error) {
	r, err := EvalExpr(e, env)
	if err != nil {
		return 0, err
	}
	return RoundHalfUp(r), nil
}

// ExprString renders an AST back to a readable form for diagnostics.
func ExprString(e *config.ExprNode) string {
	var b strings.Builder
	writeExprStr(&b, e)
	return b.String()
}

func writeExprStr(b *strings.Builder, e *config.ExprNode) {
	switch e.Op {
	case "lit":
		if e.IsRat {
			b.WriteString(e.Rat.String())
		} else {
			fmt.Fprintf(b, "%d", e.Num)
		}
	case "ref":
		b.WriteString(e.Name)
	case "neg":
		b.WriteString("-(")
		writeExprStr(b, e.Args[0])
		b.WriteString(")")
	case "+", "-", "*", "/":
		b.WriteString("(")
		writeExprStr(b, e.Args[0])
		b.WriteString(e.Op)
		writeExprStr(b, e.Args[1])
		b.WriteString(")")
	default:
		b.WriteString(e.Op)
		b.WriteString("(")
		for i, a := range e.Args {
			if i > 0 {
				b.WriteString(",")
			}
			writeExprStr(b, a)
		}
		b.WriteString(")")
	}
}
