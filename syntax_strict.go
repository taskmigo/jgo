package gots

import "strings"

var strictReservedWords = map[string]struct{}{
	"implements": {}, "interface": {}, "package": {}, "private": {},
	"protected": {}, "public": {}, "static": {}, "yield": {},
}

func validateStrictStatements(statements []stmt, strict bool) error {
	for _, statement := range statements {
		if err := validateStrictStatement(statement, strict); err != nil {
			return err
		}
	}
	return nil
}

func validateStrictStatement(statement stmt, strict bool) error {
	switch node := statement.(type) {
	case *exprStmt:
		return validateStrictExpression(node.e, strict)
	case *varStmt:
		if strict && restrictedStrictBinding(node.name) {
			return strictSyntaxError(node.name, node.span(), "invalid binding identifier in strict mode")
		}
		return validateStrictExpression(node.value, strict)
	case *varsStmt:
		for _, declaration := range node.declarations {
			if err := validateStrictStatement(declaration, strict); err != nil {
				return err
			}
		}
	case *blockStmt:
		if strict {
			lexicalNames := make(map[string]struct{})
			for _, child := range node.body {
				switch declaration := child.(type) {
				case *functionStmt:
					if _, duplicate := lexicalNames[declaration.name]; duplicate {
						return strictSyntaxError(declaration.name, declaration.span(), "duplicate lexical declaration")
					}
					lexicalNames[declaration.name] = struct{}{}
				case *varStmt:
					if declaration.declaration != TokVar {
						if _, duplicate := lexicalNames[declaration.name]; duplicate {
							return strictSyntaxError(declaration.name, declaration.span(), "duplicate lexical declaration")
						}
						lexicalNames[declaration.name] = struct{}{}
					}
				}
			}
		}
		return validateStrictStatements(node.body, strict)
	case *ifStmt:
		if err := validateStrictExpression(node.test, strict); err != nil {
			return err
		}
		if strict {
			if _, declaration := node.then.(*functionStmt); declaration {
				return strictSyntaxError("function", node.then.span(), "function declaration is not allowed in statement position")
			}
			if node.otherwise != nil {
				if _, declaration := node.otherwise.(*functionStmt); declaration {
					return strictSyntaxError("function", node.otherwise.span(), "function declaration is not allowed in statement position")
				}
			}
		}
		if err := validateStrictStatement(node.then, strict); err != nil {
			return err
		}
		if node.otherwise != nil {
			return validateStrictStatement(node.otherwise, strict)
		}
	case *whileStmt:
		if err := validateStrictExpression(node.test, strict); err != nil {
			return err
		}
		return validateStrictStatement(node.body, strict)
	case *withStmt:
		if strict {
			return strictSyntaxError("with", node.span(), "with statement is not allowed in strict mode")
		}
		if err := validateStrictExpression(node.object, strict); err != nil {
			return err
		}
		return validateStrictStatement(node.body, strict)
	case *forStmt:
		if err := validateStrictStatement(node.init, strict); err != nil {
			return err
		}
		if err := validateStrictExpression(node.test, strict); err != nil {
			return err
		}
		if err := validateStrictExpression(node.update, strict); err != nil {
			return err
		}
		return validateStrictStatement(node.body, strict)
	case *returnStmt:
		return validateStrictExpression(node.value, strict)
	case *functionStmt:
		if strict && restrictedStrictBinding(node.name) {
			return strictSyntaxError(node.name, node.span(), "invalid function name in strict mode")
		}
		return validateStrictFunction(node.fn)
	case *classStmt:
		if restrictedStrictBinding(node.name) {
			return strictSyntaxError(node.name, node.span(), "invalid class name in strict mode")
		}
		return validateStrictClass(node.definition)
	case *labelledStmt:
		if strict {
			if _, reserved := strictReservedWords[node.label]; reserved {
				return strictSyntaxError(node.label, node.span(), "reserved word cannot be used as a label in strict mode")
			}
		}
		return validateStrictStatement(node.body, strict)
	}
	return nil
}

func validateStrictFunction(function *functionExpr) error {
	if function.strict {
		seen := make(map[string]struct{}, len(function.params))
		for _, parameter := range function.params {
			if restrictedStrictBinding(parameter) {
				return strictSyntaxError(parameter, function.span(), "invalid parameter name in strict mode")
			}
			if _, duplicate := seen[parameter]; duplicate {
				return strictSyntaxError(parameter, function.span(), "duplicate parameter name not allowed in strict mode")
			}
			seen[parameter] = struct{}{}
		}
		if function.name != "" && restrictedStrictBinding(function.name) {
			return strictSyntaxError(function.name, function.span(), "invalid function name in strict mode")
		}
	}
	return validateStrictStatements(function.body, function.strict)
}

func validateStrictExpression(expression expr, strict bool) error {
	switch node := expression.(type) {
	case *literalExpr:
		if strict && (node.legacyOctalEscape || isLegacyOctalLiteral(node.sourceLiteral)) {
			return strictSyntaxError(node.sourceLiteral, node.span(), "legacy octal syntax is not allowed in strict mode")
		}
	case *identExpr:
		if strict {
			if _, reserved := strictReservedWords[node.name]; reserved {
				return strictSyntaxError(node.name, node.span(), "reserved word not allowed in strict mode")
			}
		}
	case *unaryExpr:
		if node.op == TokDelete {
			if member, ok := node.right.(*memberExpr); ok {
				if _, private := member.property.(*privateNameExpr); private {
					return strictSyntaxError("delete", node.span(), "private class elements cannot be deleted")
				}
			}
		}
		if strict && node.op == TokDelete {
			if identifier, ok := node.right.(*identExpr); ok {
				return strictSyntaxError(identifier.name, identifier.span(), "deleting an unqualified identifier is not allowed in strict mode")
			}
		}
		return validateStrictExpression(node.right, strict)
	case *updateExpr:
		if strict {
			if identifier, ok := node.target.(*identExpr); ok && (identifier.name == "eval" || identifier.name == "arguments") {
				return strictSyntaxError(identifier.name, identifier.span(), "invalid update target in strict mode")
			}
		}
		return validateStrictExpression(node.target, strict)
	case *binaryExpr:
		if err := validateStrictExpression(node.left, strict); err != nil {
			return err
		}
		return validateStrictExpression(node.right, strict)
	case *sequenceExpr:
		for _, value := range node.values {
			if err := validateStrictExpression(value, strict); err != nil {
				return err
			}
		}
	case *assignExpr:
		if strict {
			if identifier, ok := node.target.(*identExpr); ok && (identifier.name == "eval" || identifier.name == "arguments") {
				return strictSyntaxError(identifier.name, identifier.span(), "invalid assignment target in strict mode")
			}
		}
		if err := validateStrictExpression(node.target, strict); err != nil {
			return err
		}
		return validateStrictExpression(node.value, strict)
	case *callExpr:
		if err := validateStrictExpression(node.callee, strict); err != nil {
			return err
		}
		for _, argument := range node.args {
			if err := validateStrictExpression(argument, strict); err != nil {
				return err
			}
		}
	case *memberExpr:
		if err := validateStrictExpression(node.object, strict); err != nil {
			return err
		}
		return validateStrictExpression(node.property, strict)
	case *arrayExpr:
		for _, value := range node.values {
			if err := validateStrictExpression(value, strict); err != nil {
				return err
			}
		}
	case *objectExpr:
		for _, entry := range node.entries {
			if err := validateStrictExpression(entry.value, strict); err != nil {
				return err
			}
		}
	case *functionExpr:
		return validateStrictFunction(node)
	case *classExpr:
		return validateStrictClass(node)
	}
	return nil
}

func validateStrictClass(class *classExpr) error {
	if class.name != "" && restrictedStrictBinding(class.name) {
		return strictSyntaxError(class.name, class.span(), "invalid class name in strict mode")
	}
	if class.heritage != nil {
		if err := validateStrictExpression(class.heritage, true); err != nil {
			return err
		}
	}
	for _, element := range class.elements {
		if element.staticBlock != nil {
			if err := validateStrictStatements(element.staticBlock, true); err != nil {
				return err
			}
			continue
		}
		if err := validateStrictExpression(element.key, true); err != nil {
			return err
		}
		if element.field {
			if element.initializer != nil {
				if err := validateStrictExpression(element.initializer, true); err != nil {
					return err
				}
			}
		} else {
			if err := validateStrictFunction(element.method); err != nil {
				return err
			}
		}
	}
	return nil
}

func restrictedStrictBinding(name string) bool {
	if name == "eval" || name == "arguments" {
		return true
	}
	_, reserved := strictReservedWords[name]
	return reserved
}

func isLegacyOctalLiteral(literal string) bool {
	if len(literal) < 2 || literal[0] != '0' || strings.ContainsAny(literal, ".eExXoObB") {
		return false
	}
	for _, character := range literal[1:] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func strictSyntaxError(literal string, span Span, message string) error {
	token := Token{Type: TokIdent, Literal: literal, Span: span}
	return &SyntaxError{Span: span, Token: token, Message: message}
}
