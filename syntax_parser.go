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
	if err := validateStatementList(out); err != nil {
		return nil, err
	}
	return out, nil
}

func validateStatementList(statements []stmt) error {
	lexicalNames := make(map[string]Span)
	varNames := make(map[string]Span)
	for _, statement := range statements {
		var declarations []*varStmt
		switch node := statement.(type) {
		case *varStmt:
			declarations = []*varStmt{node}
		case *varsStmt:
			declarations = node.declarations
		case *functionStmt:
			if _, found := lexicalNames[node.name]; found {
				return declarationSyntaxError(node.name, node.span())
			}
			varNames[node.name] = node.span()
			if err := validateStatementList(node.fn.body); err != nil {
				return err
			}
		case *blockStmt:
			if err := validateStatementList(node.body); err != nil {
				return err
			}
		case *forStmt:
			if block, ok := node.body.(*blockStmt); ok {
				if err := validateStatementList(block.body); err != nil {
					return err
				}
			}
		}
		for _, declaration := range declarations {
			if declaration.declaration == TokVar {
				if _, found := lexicalNames[declaration.name]; found {
					return declarationSyntaxError(declaration.name, declaration.span())
				}
				varNames[declaration.name] = declaration.span()
				continue
			}
			if _, found := lexicalNames[declaration.name]; found {
				return declarationSyntaxError(declaration.name, declaration.span())
			}
			if _, found := varNames[declaration.name]; found {
				return declarationSyntaxError(declaration.name, declaration.span())
			}
			lexicalNames[declaration.name] = declaration.span()
		}
	}
	return nil
}

func declarationSyntaxError(name string, span Span) error {
	token := Token{Type: TokIdent, Literal: name, Span: span}
	return &SyntaxError{Span: span, Token: token, Message: "identifier has already been declared"}
}
func (p *parser) statement() (stmt, error) {
	if p.match(TokSemi) {
		return &exprStmt{base: base{p.prev().Span}, e: &literalExpr{value: Undefined()}}, nil
	}
	if p.match(TokLet, TokVar, TokConst) {
		return p.variableStatement(p.prev())
	}
	if p.match(TokFunction) {
		return p.functionDeclaration(p.prev())
	}
	if p.match(TokReturn) {
		return p.returnStatement(p.prev())
	}
	if p.match(TokIf) {
		return p.ifStatement(p.prev())
	}
	if p.match(TokWhile) {
		return p.whileStatement(p.prev())
	}
	if p.match(TokFor) {
		return p.forStatement(p.prev())
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

func (p *parser) variableStatement(keyword Token) (stmt, error) {
	var declarations []*varStmt
	declaredNames := map[string]struct{}{}
	for {
		name, err := p.need(TokIdent, "expected variable name")
		if err != nil {
			return nil, err
		}
		if keyword.Type != TokVar {
			if _, exists := declaredNames[name.Literal]; exists {
				return nil, p.err(name, "duplicate lexical declaration")
			}
			declaredNames[name.Literal] = struct{}{}
		}

		var initializer expr = &literalExpr{base: base{name.Span}, value: Undefined()}
		if p.match(TokAssign) {
			initializer, err = p.assign()
			if err != nil {
				return nil, err
			}
		} else if keyword.Type == TokConst {
			return nil, p.err(name, "const declaration requires an initializer")
		}
		declarations = append(declarations, &varStmt{
			base: base{keyword.Span}, name: name.Literal, value: initializer,
			declaration: keyword.Type,
		})
		if !p.match(TokComma) {
			break
		}
	}
	p.match(TokSemi)
	if len(declarations) == 1 {
		return declarations[0], nil
	}
	return &varsStmt{base: base{keyword.Span}, declarations: declarations}, nil
}

func (p *parser) functionDeclaration(start Token) (stmt, error) {
	name, err := p.need(TokIdent, "expected function name")
	if err != nil {
		return nil, err
	}
	function, err := p.function(start, name.Literal)
	if err != nil {
		return nil, err
	}
	return &functionStmt{base: base{start.Span}, name: name.Literal, fn: function}, nil
}

func (p *parser) returnStatement(keyword Token) (stmt, error) {
	var value expr = &literalExpr{base: base{keyword.Span}, value: Undefined()}
	if !p.at(TokSemi) && !p.at(TokRBrace) && !p.at(TokEOF) {
		var err error
		value, err = p.expression()
		if err != nil {
			return nil, err
		}
	}
	p.match(TokSemi)
	return &returnStmt{base: base{keyword.Span}, value: value}, nil
}

func (p *parser) ifStatement(keyword Token) (stmt, error) {
	test, err := p.parenthesizedExpression()
	if err != nil {
		return nil, err
	}
	thenBranch, err := p.statement()
	if err != nil {
		return nil, err
	}
	var elseBranch stmt
	if p.match(TokElse) {
		elseBranch, err = p.statement()
	}
	return &ifStmt{base: base{keyword.Span}, test: test, then: thenBranch, otherwise: elseBranch}, err
}

func (p *parser) whileStatement(keyword Token) (stmt, error) {
	test, err := p.parenthesizedExpression()
	if err != nil {
		return nil, err
	}
	body, err := p.statement()
	return &whileStmt{base: base{keyword.Span}, test: test, body: body}, err
}

func (p *parser) parenthesizedExpression() (expr, error) {
	if _, err := p.need(TokLParen, "expected '('"); err != nil {
		return nil, err
	}
	value, err := p.expression()
	if err != nil {
		return nil, err
	}
	if _, err = p.need(TokRParen, "expected ')'"); err != nil {
		return nil, err
	}
	return value, nil
}

func (p *parser) forStatement(keyword Token) (stmt, error) {
	if _, err := p.need(TokLParen, "expected '('"); err != nil {
		return nil, err
	}
	initializer, lexical, err := p.forInitializer(keyword)
	if err != nil {
		return nil, err
	}

	var test expr = &literalExpr{base: base{keyword.Span}, value: Boolean(true)}
	if !p.at(TokSemi) {
		test, err = p.expression()
		if err != nil {
			return nil, err
		}
	}
	if _, err = p.need(TokSemi, "expected ';'"); err != nil {
		return nil, err
	}

	var update expr = &literalExpr{base: base{keyword.Span}, value: Undefined()}
	if !p.at(TokRParen) {
		update, err = p.expression()
		if err != nil {
			return nil, err
		}
	}
	if _, err = p.need(TokRParen, "expected ')'"); err != nil {
		return nil, err
	}
	body, err := p.statement()
	if err != nil {
		return nil, err
	}
	return &forStmt{base: base{keyword.Span}, init: initializer, test: test, update: update, body: body, lexical: lexical}, nil
}

func (p *parser) forInitializer(keyword Token) (stmt, bool, error) {
	lexical := p.at(TokLet) || p.at(TokConst)
	if p.match(TokSemi) {
		return &exprStmt{base: base{keyword.Span}, e: &literalExpr{value: Undefined()}}, lexical, nil
	}
	if p.at(TokLet) || p.at(TokConst) || p.at(TokVar) {
		initializer, err := p.statement()
		return initializer, lexical, err
	}
	value, err := p.expression()
	if err != nil {
		return nil, lexical, err
	}
	if _, err = p.need(TokSemi, "expected ';'"); err != nil {
		return nil, lexical, err
	}
	return &exprStmt{base: base{value.span()}, e: value}, lexical, nil
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
func (p *parser) expression() (expr, error) {
	first, e := p.assign()
	if e != nil {
		return nil, e
	}
	values := []expr{first}
	for p.match(TokComma) {
		next, e := p.assign()
		if e != nil {
			return nil, e
		}
		values = append(values, next)
	}
	if len(values) == 1 {
		return first, nil
	}
	return &sequenceExpr{base: base{Span{first.span().Start, values[len(values)-1].span().End}}, values: values}, nil
}
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
	if p.match(TokPlusPlus, TokMinusMinus) {
		operator := p.prev()
		target, err := p.unary()
		if err != nil {
			return nil, err
		}
		switch target.(type) {
		case *identExpr, *memberExpr:
			return &updateExpr{base: base{Span{operator.Span.Start, target.span().End}}, op: operator.Type, target: target}, nil
		default:
			return nil, p.err(operator, "invalid update target")
		}
	}
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
					a, e := p.assign()
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
			n, e := p.needIdentifierName("expected property name")
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

func (p *parser) needIdentifierName(message string) (Token, error) {
	token := p.peek()
	if token.Type == TokIdent || isKeyword(token.Type) {
		p.take()
		return token, nil
	}
	return Token{}, p.err(token, message)
}

func isKeyword(tokenType TokenType) bool {
	switch tokenType {
	case TokLet, TokVar, TokConst, TokFunction, TokReturn, TokIf, TokElse, TokWhile,
		TokFor, TokTrue, TokFalse, TokNull, TokUndefined, TokNew, TokTypeof:
		return true
	default:
		return false
	}
}
func (p *parser) primary() (expr, error) {
	t := p.take()
	switch t.Type {
	case TokNumber:
		if t.numberValue != nil {
			return &literalExpr{base: base{t.Span}, value: Number(*t.numberValue)}, nil
		}
		n, _ := strconv.ParseFloat(t.Literal, 64)
		return &literalExpr{base: base{t.Span}, value: Number(n)}, nil
	case TokString:
		if t.stringValue != nil {
			return &literalExpr{base: base{t.Span}, value: StringUTF16(*t.stringValue)}, nil
		}
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
				x, e := p.assign()
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
				v, e := p.assign()
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
