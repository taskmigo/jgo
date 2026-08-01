package gots

import "strconv"

type parser struct {
	tokens []Token
	pos    int
}

type parsedScript struct {
	body   []stmt
	strict bool
}

func parse(source string) (parsedScript, error) {
	ts, e := lex(source)
	if e != nil {
		return parsedScript{}, e
	}
	if err := validateRecognizableClassEarlyErrors(ts); err != nil {
		return parsedScript{}, err
	}
	p := &parser{tokens: ts}
	var out []stmt
	for !p.at(TokEOF) {
		s, e := p.statement()
		if e != nil {
			return parsedScript{}, e
		}
		out = append(out, s)
	}
	if err := validateStatementList(out); err != nil {
		return parsedScript{}, err
	}
	strict := statementListStrict(out)
	if err := validateStrictTokenPatterns(ts, strict); err != nil {
		return parsedScript{}, err
	}
	annotateFunctionStrictness(out, strict)
	if err := validateStrictStatements(out, strict); err != nil {
		return parsedScript{}, err
	}
	if err := validateControlFlow(out, false); err != nil {
		return parsedScript{}, err
	}
	if err := validateNewTargetStatements(out, false); err != nil {
		return parsedScript{}, err
	}
	if err := validatePrivateNames(out, nil); err != nil {
		return parsedScript{}, err
	}
	return parsedScript{body: out, strict: strict}, nil
}

func validateStrictTokenPatterns(tokens []Token, strict bool) error {
	if !strict {
		return nil
	}
	for index := 0; index+3 < len(tokens); index++ {
		if tokens[index].Type != TokIdent {
			continue
		}
		if tokens[index].Literal == "catch" && tokens[index+1].Type == TokLParen && restrictedStrictBinding(tokens[index+2].Literal) && tokens[index+3].Type == TokRParen {
			return strictSyntaxError(tokens[index+2].Literal, tokens[index+2].Span, "invalid catch binding in strict mode")
		}
		if index+4 < len(tokens) && tokens[index].Literal == "set" && tokens[index+2].Type == TokLParen && restrictedStrictBinding(tokens[index+3].Literal) && tokens[index+4].Type == TokRParen {
			return strictSyntaxError(tokens[index+3].Literal, tokens[index+3].Span, "invalid setter parameter in strict mode")
		}
	}
	return nil
}

func statementListStrict(statements []stmt) bool {
	for _, statement := range statements {
		expressionStatement, ok := statement.(*exprStmt)
		if !ok {
			return false
		}
		literal, ok := expressionStatement.e.(*literalExpr)
		if !ok || literal.value.k != KindString || !literal.directiveEligible {
			return false
		}
		if !literal.directiveEscaped && jsString(literal.value.s).equal(jsString(String("use strict").s)) {
			return true
		}
	}
	return false
}

func annotateFunctionStrictness(statements []stmt, inheritedStrict bool) {
	for _, statement := range statements {
		switch node := statement.(type) {
		case *exprStmt:
			annotateExpressionStrictness(node.e, inheritedStrict)
		case *varStmt:
			annotateExpressionStrictness(node.value, inheritedStrict)
		case *varsStmt:
			for _, declaration := range node.declarations {
				annotateExpressionStrictness(declaration.value, inheritedStrict)
			}
		case *functionStmt:
			node.fn.strict = inheritedStrict || statementListStrict(node.fn.body)
			annotateFunctionStrictness(node.fn.body, node.fn.strict)
		case *classStmt:
			annotateClassStrictness(node.definition)
		case *blockStmt:
			annotateFunctionStrictness(node.body, inheritedStrict)
		case *ifStmt:
			annotateExpressionStrictness(node.test, inheritedStrict)
			annotateNestedStatement(node.then, inheritedStrict)
			if node.otherwise != nil {
				annotateNestedStatement(node.otherwise, inheritedStrict)
			}
		case *whileStmt:
			annotateExpressionStrictness(node.test, inheritedStrict)
			annotateNestedStatement(node.body, inheritedStrict)
		case *withStmt:
			annotateExpressionStrictness(node.object, inheritedStrict)
			annotateNestedStatement(node.body, inheritedStrict)
		case *forStmt:
			annotateNestedStatement(node.init, inheritedStrict)
			annotateExpressionStrictness(node.test, inheritedStrict)
			annotateExpressionStrictness(node.update, inheritedStrict)
			annotateNestedStatement(node.body, inheritedStrict)
		case *returnStmt:
			annotateExpressionStrictness(node.value, inheritedStrict)
		case *labelledStmt:
			annotateNestedStatement(node.body, inheritedStrict)
		}
	}
}

func annotateNestedStatement(statement stmt, inheritedStrict bool) {
	annotateFunctionStrictness([]stmt{statement}, inheritedStrict)
}

func annotateExpressionStrictness(expression expr, inheritedStrict bool) {
	switch node := expression.(type) {
	case *functionExpr:
		node.strict = inheritedStrict || statementListStrict(node.body)
		annotateFunctionStrictness(node.body, node.strict)
	case *classExpr:
		annotateClassStrictness(node)
	case *unaryExpr:
		annotateExpressionStrictness(node.right, inheritedStrict)
	case *updateExpr:
		annotateExpressionStrictness(node.target, inheritedStrict)
	case *binaryExpr:
		annotateExpressionStrictness(node.left, inheritedStrict)
		annotateExpressionStrictness(node.right, inheritedStrict)
	case *sequenceExpr:
		for _, value := range node.values {
			annotateExpressionStrictness(value, inheritedStrict)
		}
	case *assignExpr:
		annotateExpressionStrictness(node.target, inheritedStrict)
		annotateExpressionStrictness(node.value, inheritedStrict)
	case *callExpr:
		annotateExpressionStrictness(node.callee, inheritedStrict)
		for _, argument := range node.args {
			annotateExpressionStrictness(argument, inheritedStrict)
		}
	case *memberExpr:
		annotateExpressionStrictness(node.object, inheritedStrict)
		annotateExpressionStrictness(node.property, inheritedStrict)
	case *arrayExpr:
		for _, value := range node.values {
			annotateExpressionStrictness(value, inheritedStrict)
		}
	case *objectExpr:
		for _, entry := range node.entries {
			annotateExpressionStrictness(entry.value, inheritedStrict)
		}
	}
}

func annotateClassStrictness(class *classExpr) {
	if class.heritage != nil {
		annotateExpressionStrictness(class.heritage, true)
	}
	for index := range class.elements {
		element := &class.elements[index]
		if element.staticBlock != nil {
			annotateFunctionStrictness(element.staticBlock, true)
			continue
		}
		annotateExpressionStrictness(element.key, true)
		if element.field {
			if element.initializer != nil {
				annotateExpressionStrictness(element.initializer, true)
			}
			continue
		}
		element.method.strict = true
		annotateFunctionStrictness(element.method.body, true)
	}
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
		case *classStmt:
			if _, found := lexicalNames[node.name]; found {
				return declarationSyntaxError(node.name, node.span())
			}
			if _, found := varNames[node.name]; found {
				return declarationSyntaxError(node.name, node.span())
			}
			lexicalNames[node.name] = node.span()
			for _, element := range node.definition.elements {
				if element.staticBlock != nil {
					if err := validateStatementList(element.staticBlock); err != nil {
						return err
					}
				}
				if element.method != nil {
					if err := validateStatementList(element.method.body); err != nil {
						return err
					}
				}
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
		case *labelledStmt:
			if err := validateStatementList([]stmt{node.body}); err != nil {
				return err
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
	if p.at(TokIdent) && p.peekN(1).Type == TokColon {
		label := p.take()
		p.take()
		if p.at(TokIdent) && (p.peek().Literal == "using" || p.peek().Literal == "await" && p.peekN(1).Type == TokIdent && p.peekN(1).Literal == "using") {
			return nil, p.err(p.peek(), "lexical declaration is not allowed as a labelled item")
		}
		if p.at(TokClass) {
			return nil, p.err(p.peek(), "class declaration is not allowed as a labelled item")
		}
		body, err := p.statement()
		if err != nil {
			return nil, err
		}
		return &labelledStmt{base: base{Span{label.Span.Start, body.span().End}}, label: label.Literal, body: body}, nil
	}
	if p.match(TokSemi) {
		return &emptyStmt{base: base{p.prev().Span}}, nil
	}
	if p.match(TokLet, TokVar, TokConst) {
		return p.variableStatement(p.prev())
	}
	if p.match(TokFunction) {
		return p.functionDeclaration(p.prev())
	}
	if p.match(TokClass) {
		return p.classDeclaration(p.prev())
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
	if p.at(TokIdent) && p.peek().Literal == "with" && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == TokLParen {
		return p.withStatement(p.take())
	}
	if p.match(TokFor) {
		return p.forStatement(p.prev())
	}
	if p.match(TokBreak) {
		return p.breakOrContinueStatement(p.prev(), false)
	}
	if p.match(TokContinue) {
		return p.breakOrContinueStatement(p.prev(), true)
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

func (p *parser) withStatement(keyword Token) (stmt, error) {
	if _, err := p.need(TokLParen, "expected '('"); err != nil {
		return nil, err
	}
	object, err := p.expression()
	if err != nil {
		return nil, err
	}
	if _, err := p.need(TokRParen, "expected ')'"); err != nil {
		return nil, err
	}
	body, err := p.statement()
	if err != nil {
		return nil, err
	}
	return &withStmt{base: base{Span{keyword.Span.Start, body.span().End}}, object: object, body: body}, nil
}

func (p *parser) breakOrContinueStatement(keyword Token, isContinue bool) (stmt, error) {
	target := ""
	end := keyword.Span.End
	if p.at(TokIdent) && p.peek().Span.Start.Line == keyword.Span.End.Line {
		label := p.take()
		target, end = label.Literal, label.Span.End
	}
	if semicolon, matched := p.matchToken(TokSemi); matched {
		end = semicolon.Span.End
	}
	statementBase := base{Span{keyword.Span.Start, end}}
	if isContinue {
		return &continueStmt{base: statementBase, target: target}, nil
	}
	return &breakStmt{base: statementBase, target: target}, nil
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
		hasInitializer := false
		if p.match(TokAssign) {
			hasInitializer = true
			initializer, err = p.assign()
			if err != nil {
				return nil, err
			}
			setAnonymousClassName(initializer, name.Literal)
		} else if keyword.Type == TokConst {
			return nil, p.err(name, "const declaration requires an initializer")
		}
		declarations = append(declarations, &varStmt{
			base: base{keyword.Span}, name: name.Literal, value: initializer,
			declaration: keyword.Type, hasInitializer: hasInitializer,
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

func (p *parser) classDeclaration(start Token) (stmt, error) {
	name, err := p.need(TokIdent, "expected class name")
	if err != nil {
		return nil, err
	}
	definition, err := p.classTail(start, name.Literal)
	if err != nil {
		return nil, err
	}
	return &classStmt{base: base{definition.span()}, name: name.Literal, definition: definition}, nil
}

func (p *parser) classTail(start Token, name string) (*classExpr, error) {
	var heritage expr
	if p.at(TokIdent) && p.peek().Literal == "extends" {
		p.take()
		var err error
		heritage, err = p.assign()
		if err != nil {
			return nil, err
		}
	}
	if _, err := p.need(TokLBrace, "expected class body"); err != nil {
		return nil, err
	}

	elements := make([]classElement, 0)
	constructorFound := false
	privateDeclarations := make(map[string]string)
	for !p.at(TokRBrace) && !p.at(TokEOF) {
		if p.match(TokSemi) {
			continue
		}
		isStatic := false
		if p.at(TokIdent) && p.peek().Literal == "static" && p.peekN(1).Type != TokLParen {
			p.take()
			isStatic = true
			if p.at(TokLBrace) {
				statement, err := p.block(p.take())
				if err != nil {
					return nil, err
				}
				block := statement.(*blockStmt)
				if statementsContainDirectCall(block.body, "super") {
					return nil, p.err(p.prev(), "super() is not allowed in a class static block")
				}
				elements = append(elements, classElement{static: true, staticBlock: append([]stmt{}, block.body...)})
				continue
			}
		}
		accessor := ""
		if p.at(TokIdent) && (p.peek().Literal == "get" || p.peek().Literal == "set") && p.peekN(1).Type != TokLParen && p.peekN(1).Type != TokStar {
			accessor = p.take().Literal
		}
		generator := p.match(TokStar)
		key, keyName, err := p.classElementName()
		if err != nil {
			return nil, err
		}
		if _, private := key.(*privateNameExpr); private {
			kind := "element"
			if accessor != "" {
				kind = accessor
			}
			declaration := kind
			if isStatic {
				declaration = "static:" + declaration
			}
			if previous, exists := privateDeclarations[keyName]; exists {
				pair := previous == "get" && declaration == "set" || previous == "set" && declaration == "get" ||
					previous == "static:get" && declaration == "static:set" || previous == "static:set" && declaration == "static:get"
				if !pair {
					return nil, p.err(p.prev(), "duplicate private class element")
				}
				privateDeclarations[keyName] = previous + "+" + declaration
			} else {
				privateDeclarations[keyName] = declaration
			}
		}
		if !p.at(TokLParen) {
			if accessor != "" || generator {
				return nil, p.err(p.peek(), "expected class method parameters")
			}
			if !isStatic && keyName == "constructor" {
				return nil, p.err(p.peek(), "class field cannot be named constructor")
			}
			if isStatic && (keyName == "prototype" || keyName == "constructor") {
				return nil, p.err(p.peek(), "invalid static class field name")
			}
			var initializer expr
			if p.match(TokAssign) {
				initializer, err = p.assign()
				if err != nil {
					return nil, err
				}
				if expressionContainsIdentifier(initializer, "arguments") {
					return nil, p.err(p.prev(), "arguments is not allowed in a class field initializer")
				}
				if expressionContainsDirectCall(initializer, "super") {
					return nil, p.err(p.prev(), "super() is not allowed in a class field initializer")
				}
			}
			if _, hasSemicolon := p.matchToken(TokSemi); !hasSemicolon && !p.at(TokRBrace) {
				end := key.span().End
				if initializer != nil {
					end = initializer.span().End
				}
				if p.peek().Span.Start.Line == end.Line {
					return nil, p.err(p.peek(), "class fields on the same line must be separated by a semicolon")
				}
			}
			elements = append(elements, classElement{key: key, initializer: initializer, static: isStatic, field: true})
			continue
		}
		method, err := p.function(start, keyName)
		if err != nil {
			return nil, err
		}
		method.strict = true
		if accessor == "get" && len(method.params) != 0 {
			return nil, p.err(p.prev(), "getter must not have parameters")
		}
		if accessor == "set" && len(method.params) != 1 {
			return nil, p.err(p.prev(), "setter must have exactly one parameter")
		}
		if !isStatic && keyName == "constructor" {
			if accessor != "" || generator {
				return nil, p.err(p.prev(), "class constructor cannot be an accessor")
			}
			if constructorFound {
				return nil, p.err(p.prev(), "duplicate class constructor")
			}
			constructorFound = true
			if heritage == nil && statementsContainDirectCall(method.body, "super") {
				return nil, p.err(p.prev(), "super() is not allowed in a base class constructor")
			}
		}
		if keyName != "constructor" && statementsContainDirectCall(method.body, "super") {
			return nil, p.err(p.prev(), "super() is only allowed in a class constructor")
		}
		if isStatic && keyName == "prototype" {
			return nil, p.err(p.prev(), "static class method cannot be named prototype")
		}
		elements = append(elements, classElement{key: key, method: method, static: isStatic, accessor: accessor, generator: generator})
	}
	end, err := p.need(TokRBrace, "expected '}'")
	if err != nil {
		return nil, err
	}
	return &classExpr{base: base{Span{start.Span.Start, end.Span.End}}, name: name, heritage: heritage, elements: elements}, nil
}

func (p *parser) classElementName() (expr, string, error) {
	if p.match(TokLBracket) {
		key, err := p.assign()
		if err != nil {
			return nil, "", err
		}
		if _, err := p.need(TokRBracket, "expected ']'"); err != nil {
			return nil, "", err
		}
		return key, "", nil
	}
	name, err := p.needAny([]TokenType{TokIdent, TokPrivateIdent, TokString, TokNumber}, "expected class element name")
	if err != nil {
		return nil, "", err
	}
	if name.Type == TokPrivateIdent {
		if name.Literal == "#constructor" {
			return nil, "", p.err(name, "private class element cannot be named #constructor")
		}
		return &privateNameExpr{base: base{name.Span}, name: name.Literal}, name.Literal, nil
	}
	if name.Type == TokNumber {
		if name.numberValue != nil {
			return &literalExpr{base: base{name.Span}, value: Number(*name.numberValue)}, name.Literal, nil
		}
		number, _ := strconv.ParseFloat(name.Literal, 64)
		return &literalExpr{base: base{name.Span}, value: Number(number)}, name.Literal, nil
	}
	return &literalExpr{base: base{name.Span}, value: String(name.Literal)}, name.Literal, nil
}

func (p *parser) returnStatement(keyword Token) (stmt, error) {
	var value expr = &literalExpr{base: base{keyword.Span}, value: Undefined()}
	if !p.at(TokSemi) && !p.at(TokRBrace) && !p.at(TokEOF) && p.peek().Span.Start.Line == keyword.Span.End.Line {
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
	if _, classDeclaration := thenBranch.(*classStmt); classDeclaration {
		return nil, p.err(keyword, "class declaration is not allowed in statement position")
	}
	var elseBranch stmt
	if p.match(TokElse) {
		elseBranch, err = p.statement()
		if _, classDeclaration := elseBranch.(*classStmt); classDeclaration {
			return nil, p.err(keyword, "class declaration is not allowed in statement position")
		}
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
			if identifier, ok := x.(*identExpr); ok {
				setAnonymousClassName(v, identifier.name)
			}
			return &assignExpr{base: base{x.span()}, target: x, value: v}, nil
		default:
			return nil, p.err(p.prev(), "invalid assignment target")
		}
	}
	return x, e
}

func setAnonymousClassName(expression expr, name string) {
	if class, ok := expression.(*classExpr); ok && class.name == "" {
		class.name = name
	}
}

var prec = map[TokenType]int{TokOr: 1, TokAnd: 2, TokEQ: 3, TokNE: 3, TokStrictEQ: 3, TokStrictNE: 3, TokLT: 4, TokLE: 4, TokGT: 4, TokGE: 4, TokIn: 4, TokPlus: 5, TokMinus: 5, TokStar: 6, TokSlash: 6}

func (p *parser) binary(min int) (expr, error) {
	left, e := p.unary()
	if e != nil {
		return nil, e
	}
	for {
		operatorType := p.peek().Type
		if operatorType == TokIdent && p.peek().Literal == "in" {
			operatorType = TokIn
		}
		if prec[operatorType] < min {
			break
		}
		op := p.take()
		op.Type = operatorType
		right, e := p.binary(prec[operatorType] + 1)
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
	if p.match(TokBang, TokMinus, TokPlus, TokTypeof, TokDelete) {
		o := p.prev()
		if o.Type == TokTypeof && p.at(TokIdent) && p.peek().Literal == "import" {
			return nil, p.err(p.peek(), "invalid dynamic import syntax")
		}
		r, e := p.unary()
		if e != nil {
			return nil, e
		}
		return &unaryExpr{base: base{Span{o.Span.Start, r.span().End}}, op: o.Type, right: r}, nil
	}
	construct := p.match(TokNew)
	var x expr
	var e error
	if construct && p.match(TokDot) {
		newToken := p.tokens[p.pos-2]
		target, err := p.need(TokIdent, "expected target after new.")
		if err != nil {
			return nil, err
		}
		if target.Literal != "target" {
			return nil, p.err(target, "expected target after new.")
		}
		x = &newTargetExpr{base: base{Span{newToken.Span.Start, target.Span.End}}}
		construct = false
	} else {
		x, e = p.primary()
		if e != nil {
			return nil, e
		}
	}
	for {
		if p.match(TokLParen) {
			isDynamicImport := isDynamicImportTarget(x)
			if isDynamicImport {
				if construct {
					return nil, p.err(p.prev(), "dynamic import is not a constructor")
				}
			}
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
			if isDynamicImport && (len(args) == 0 || len(args) > 2) {
				return nil, p.err(end, "dynamic import requires one or two arguments")
			}
			x = &callExpr{base: base{Span{x.span().Start, end.Span.End}}, callee: x, args: args, construct: construct}
			construct = false
		} else if p.match(TokDot) {
			if p.at(TokPrivateIdent) {
				n := p.take()
				if identifier, isSuper := x.(*identExpr); isSuper && identifier.name == "super" {
					return nil, p.err(n, "super cannot access a private class element")
				}
				x = &memberExpr{base: base{Span{x.span().Start, n.Span.End}}, object: x, property: &privateNameExpr{base: base{n.Span}, name: n.Literal}}
				continue
			}
			n, e := p.needIdentifierName("expected property name")
			if e != nil {
				return nil, e
			}
			if identifier, isImport := x.(*identExpr); isImport && identifier.name == "import" && n.Literal != "source" && n.Literal != "defer" {
				return nil, p.err(n, "invalid dynamic import phase")
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

func isDynamicImportTarget(expression expr) bool {
	if identifier, ok := expression.(*identExpr); ok {
		return identifier.name == "import"
	}
	member, ok := expression.(*memberExpr)
	if !ok {
		return false
	}
	identifier, ok := member.object.(*identExpr)
	if !ok || identifier.name != "import" {
		return false
	}
	property, ok := member.property.(*literalExpr)
	return ok && property.value.k == KindString && (jsString(property.value.s).equal(jsString(String("source").s)) || jsString(property.value.s).equal(jsString(String("defer").s)))
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
	case TokLet, TokVar, TokConst, TokFunction, TokClass, TokReturn, TokIf, TokElse, TokWhile,
		TokFor, TokBreak, TokContinue, TokTrue, TokFalse, TokNull, TokUndefined, TokNew, TokTypeof, TokDelete:
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
			return &literalExpr{base: base{t.Span}, value: Number(*t.numberValue), sourceLiteral: t.Literal}, nil
		}
		n, _ := strconv.ParseFloat(t.Literal, 64)
		return &literalExpr{base: base{t.Span}, value: Number(n), sourceLiteral: t.Literal}, nil
	case TokString:
		if t.stringValue != nil {
			return &literalExpr{base: base{t.Span}, value: StringUTF16(*t.stringValue), directiveEligible: true, directiveEscaped: t.hasEscape, sourceLiteral: t.Literal, legacyOctalEscape: t.legacyOctalEscape}, nil
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
		if t.Literal == "async" && p.at(TokFunction) && p.peek().Span.Start.Line == t.Span.End.Line {
			return nil, p.err(p.peek(), "async functions are not supported")
		}
		return &identExpr{base: base{t.Span}, name: t.Literal}, nil
	case TokLParen:
		x, e := p.expression()
		if e != nil {
			return nil, e
		}
		_, e = p.need(TokRParen, "expected ')'")
		if literal, ok := x.(*literalExpr); ok {
			literal.directiveEligible = false
		}
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
	case TokClass:
		name := ""
		if p.at(TokIdent) && p.peek().Literal != "extends" {
			name = p.take().Literal
		}
		return p.classTail(t, name)
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
func (p *parser) peekN(offset int) Token {
	index := p.pos + offset
	if index >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[index]
}
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
func (p *parser) matchToken(tokenType TokenType) (Token, bool) {
	if !p.at(tokenType) {
		return Token{}, false
	}
	return p.take(), true
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
