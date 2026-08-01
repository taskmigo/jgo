package gots

import "strconv"

type parser struct {
	tokens []Token
	pos    int
}

func parse(source string) ([]stmt, error) {
	ts, e := lex(source)
	if e != nil {
		return nil, e
	}
	p := &parser{tokens: ts}
	var out []stmt
	for !p.at(TokEOF) {
		s, e := p.statement()
		if e != nil {
			return nil, e
		}
		out = append(out, s)
	}
	return out, nil
}
func (p *parser) statement() (stmt, error) {
	if p.match(TokSemi) {
		return &exprStmt{base: base{p.prev().Span}, e: &literalExpr{value: Undefined()}}, nil
	}
	if p.match(TokLet, TokVar, TokConst) {
		k := p.prev()
		var declarations []*varStmt
		declared := map[string]struct{}{}
		for {
			n, e := p.need(TokIdent, "expected variable name")
			if e != nil {
				return nil, e
			}
			if k.Type != TokVar {
				if _, exists := declared[n.Literal]; exists {
					return nil, p.err(n, "duplicate lexical declaration")
				}
				declared[n.Literal] = struct{}{}
			}
			var v expr = &literalExpr{base: base{n.Span}, value: Undefined()}
			if p.match(TokAssign) {
				v, e = p.assign()
				if e != nil {
					return nil, e
				}
			} else if k.Type == TokConst {
				return nil, p.err(n, "const declaration requires an initializer")
			}
			declarations = append(declarations, &varStmt{base: base{k.Span}, name: n.Literal, value: v, constant: k.Type == TokConst})
			if !p.match(TokComma) {
				break
			}
		}
		p.match(TokSemi)
		if len(declarations) == 1 {
			return declarations[0], nil
		}
		return &varsStmt{base: base{k.Span}, declarations: declarations}, nil
	}
	if p.match(TokFunction) {
		start := p.prev()
		name, e := p.need(TokIdent, "expected function name")
		if e != nil {
			return nil, e
		}
		fn, e := p.function(start, name.Literal)
		if e != nil {
			return nil, e
		}
		return &functionStmt{base: base{start.Span}, name: name.Literal, fn: fn}, nil
	}
	if p.match(TokReturn) {
		t := p.prev()
		var v expr = &literalExpr{base: base{t.Span}, value: Undefined()}
		var e error
		if !p.at(TokSemi) && !p.at(TokRBrace) && !p.at(TokEOF) {
			v, e = p.expression()
			if e != nil {
				return nil, e
			}
		}
		p.match(TokSemi)
		return &returnStmt{base: base{t.Span}, value: v}, nil
	}
	if p.match(TokIf) {
		t := p.prev()
		if _, e := p.need(TokLParen, "expected '('"); e != nil {
			return nil, e
		}
		test, e := p.expression()
		if e != nil {
			return nil, e
		}
		if _, e = p.need(TokRParen, "expected ')'"); e != nil {
			return nil, e
		}
		then, e := p.statement()
		if e != nil {
			return nil, e
		}
		var other stmt
		if p.match(TokElse) {
			other, e = p.statement()
		}
		return &ifStmt{base: base{t.Span}, test: test, then: then, otherwise: other}, e
	}
	if p.match(TokWhile) {
		t := p.prev()
		if _, e := p.need(TokLParen, "expected '('"); e != nil {
			return nil, e
		}
		test, e := p.expression()
		if e != nil {
			return nil, e
		}
		if _, e = p.need(TokRParen, "expected ')'"); e != nil {
			return nil, e
		}
		body, e := p.statement()
		return &whileStmt{base: base{t.Span}, test: test, body: body}, e
	}
	if p.match(TokLBrace) {
		return p.block(p.prev())
	}
	e, err := p.expression()
	if err != nil {
		return nil, err
	}
	p.match(TokSemi)
	return &exprStmt{base: base{e.span()}, e: e}, nil
}
func (p *parser) block(start Token) (stmt, error) {
	var b []stmt
	for !p.at(TokRBrace) && !p.at(TokEOF) {
		s, e := p.statement()
		if e != nil {
			return nil, e
		}
		b = append(b, s)
	}
	end, e := p.need(TokRBrace, "expected '}'")
	if e != nil {
		return nil, e
	}
	return &blockStmt{base: base{Span{start.Span.Start, end.Span.End}}, body: b}, nil
}
func (p *parser) expression() (expr, error) { return p.assign() }
func (p *parser) assign() (expr, error) {
	x, e := p.binary(1)
	if e == nil && p.match(TokAssign) {
		v, er := p.assign()
		if er != nil {
			return nil, er
		}
		switch x.(type) {
		case *identExpr, *memberExpr:
			return &assignExpr{base: base{x.span()}, target: x, value: v}, nil
		default:
			return nil, p.err(p.prev(), "invalid assignment target")
		}
	}
	return x, e
}

var prec = map[TokenType]int{TokOr: 1, TokAnd: 2, TokEQ: 3, TokNE: 3, TokStrictEQ: 3, TokStrictNE: 3, TokLT: 4, TokLE: 4, TokGT: 4, TokGE: 4, TokPlus: 5, TokMinus: 5, TokStar: 6, TokSlash: 6}

func (p *parser) binary(min int) (expr, error) {
	left, e := p.unary()
	if e != nil {
		return nil, e
	}
	for prec[p.peek().Type] >= min {
		op := p.take()
		right, e := p.binary(prec[op.Type] + 1)
		if e != nil {
			return nil, e
		}
		left = &binaryExpr{base: base{Span{left.span().Start, right.span().End}}, left: left, op: op.Type, right: right}
	}
	return left, nil
}
func (p *parser) unary() (expr, error) {
	if p.match(TokBang, TokMinus, TokPlus, TokTypeof) {
		o := p.prev()
		r, e := p.unary()
		if e != nil {
			return nil, e
		}
		return &unaryExpr{base: base{Span{o.Span.Start, r.span().End}}, op: o.Type, right: r}, nil
	}
	construct := p.match(TokNew)
	x, e := p.primary()
	if e != nil {
		return nil, e
	}
	for {
		if p.match(TokLParen) {
			args := []expr{}
			if !p.at(TokRParen) {
				for {
					a, e := p.expression()
					if e != nil {
						return nil, e
					}
					args = append(args, a)
					if !p.match(TokComma) {
						break
					}
				}
			}
			end, e := p.need(TokRParen, "expected ')'")
			if e != nil {
				return nil, e
			}
			x = &callExpr{base: base{Span{x.span().Start, end.Span.End}}, callee: x, args: args, construct: construct}
			construct = false
		} else if p.match(TokDot) {
			n, e := p.need(TokIdent, "expected property name")
			if e != nil {
				return nil, e
			}
			x = &memberExpr{base: base{Span{x.span().Start, n.Span.End}}, object: x, property: &literalExpr{base: base{n.Span}, value: String(n.Literal)}}
		} else if p.match(TokLBracket) {
			q, e := p.expression()
			if e != nil {
				return nil, e
			}
			end, e := p.need(TokRBracket, "expected ']'")
			if e != nil {
				return nil, e
			}
			x = &memberExpr{base: base{Span{x.span().Start, end.Span.End}}, object: x, property: q}
		} else {
			break
		}
	}
	if construct {
		return nil, p.err(p.peek(), "expected constructor call")
	}
	return x, nil
}
func (p *parser) primary() (expr, error) {
	t := p.take()
	switch t.Type {
	case TokNumber:
		n, _ := strconv.ParseFloat(t.Literal, 64)
		return &literalExpr{base: base{t.Span}, value: Number(n)}, nil
	case TokString:
		return &literalExpr{base: base{t.Span}, value: String(t.Literal)}, nil
	case TokTrue:
		return &literalExpr{base: base{t.Span}, value: Boolean(true)}, nil
	case TokFalse:
		return &literalExpr{base: base{t.Span}, value: Boolean(false)}, nil
	case TokNull:
		return &literalExpr{base: base{t.Span}, value: Null()}, nil
	case TokUndefined:
		return &literalExpr{base: base{t.Span}, value: Undefined()}, nil
	case TokIdent:
		return &identExpr{base: base{t.Span}, name: t.Literal}, nil
	case TokLParen:
		x, e := p.expression()
		if e != nil {
			return nil, e
		}
		_, e = p.need(TokRParen, "expected ')'")
		return x, e
	case TokLBracket:
		var a []expr
		if !p.at(TokRBracket) {
			for {
				x, e := p.expression()
				if e != nil {
					return nil, e
				}
				a = append(a, x)
				if !p.match(TokComma) {
					break
				}
			}
		}
		end, e := p.need(TokRBracket, "expected ']'")
		return &arrayExpr{base: base{Span{t.Span.Start, end.Span.End}}, values: a}, e
	case TokLBrace:
		var es []objectEntry
		if !p.at(TokRBrace) {
			for {
				k, e := p.needAny([]TokenType{TokIdent, TokString}, "expected property")
				if e != nil {
					return nil, e
				}
				if _, e = p.need(TokColon, "expected ':'"); e != nil {
					return nil, e
				}
				v, e := p.expression()
				if e != nil {
					return nil, e
				}
				es = append(es, objectEntry{k.Literal, v})
				if !p.match(TokComma) {
					break
				}
			}
		}
		end, e := p.need(TokRBrace, "expected '}'")
		return &objectExpr{base: base{Span{t.Span.Start, end.Span.End}}, entries: es}, e
	case TokFunction:
		return p.function(t, "")
	default:
		return nil, p.err(t, "expected expression")
	}
}
func (p *parser) function(start Token, name string) (*functionExpr, error) {
	if _, e := p.need(TokLParen, "expected '('"); e != nil {
		return nil, e
	}
	var ps []string
	if !p.at(TokRParen) {
		for {
			n, e := p.need(TokIdent, "expected parameter")
			if e != nil {
				return nil, e
			}
			ps = append(ps, n.Literal)
			if !p.match(TokComma) {
				break
			}
		}
	}
	if _, e := p.need(TokRParen, "expected ')'"); e != nil {
		return nil, e
	}
	lb, e := p.need(TokLBrace, "expected function body")
	if e != nil {
		return nil, e
	}
	b, e := p.block(lb)
	if e != nil {
		return nil, e
	}
	return &functionExpr{base: base{Span{start.Span.Start, b.span().End}}, name: name, params: ps, body: b.(*blockStmt).body}, nil
}
func (p *parser) peek() Token { return p.tokens[p.pos] }
func (p *parser) take() Token {
	t := p.peek()
	if p.pos < len(p.tokens)-1 {
		p.pos++
	}
	return t
}
func (p *parser) prev() Token         { return p.tokens[p.pos-1] }
func (p *parser) at(t TokenType) bool { return p.peek().Type == t }
func (p *parser) match(ts ...TokenType) bool {
	for _, t := range ts {
		if p.at(t) {
			p.take()
			return true
		}
	}
	return false
}
func (p *parser) need(t TokenType, m string) (Token, error) {
	if p.at(t) {
		return p.take(), nil
	}
	return Token{}, p.err(p.peek(), m)
}
func (p *parser) needAny(ts []TokenType, m string) (Token, error) {
	for _, t := range ts {
		if p.at(t) {
			return p.take(), nil
		}
	}
	return Token{}, p.err(p.peek(), m)
}
func (p *parser) err(t Token, m string) error { return &SyntaxError{t.Span, t, m} }
