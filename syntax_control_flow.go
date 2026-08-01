package gots

type controlFlowContext struct {
	breakableDepth int
	iterationDepth int
	labels         map[string]bool
	allowReturn    bool
}

func validateControlFlow(statements []stmt, allowReturn bool) error {
	context := controlFlowContext{labels: make(map[string]bool), allowReturn: allowReturn}
	for _, statement := range statements {
		if err := validateStatementControlFlow(statement, context); err != nil {
			return err
		}
	}
	return nil
}

func validateStatementControlFlow(statement stmt, context controlFlowContext) error {
	switch node := statement.(type) {
	case *exprStmt:
		return validateExpressionControlFlow(node.e)
	case *varStmt:
		return validateExpressionControlFlow(node.value)
	case *varsStmt:
		for _, declaration := range node.declarations {
			if err := validateExpressionControlFlow(declaration.value); err != nil {
				return err
			}
		}
	case *breakStmt:
		if node.target == "" {
			if context.breakableDepth == 0 {
				return controlFlowSyntaxError(node.span(), "break statement is not inside a loop or switch")
			}
			return nil
		}
		if _, found := context.labels[node.target]; !found {
			return controlFlowSyntaxError(node.span(), "undefined break target "+node.target)
		}
	case *continueStmt:
		if node.target == "" {
			if context.iterationDepth == 0 {
				return controlFlowSyntaxError(node.span(), "continue statement is not inside a loop")
			}
			return nil
		}
		iterationTarget, found := context.labels[node.target]
		if !found || !iterationTarget {
			return controlFlowSyntaxError(node.span(), "undefined continue target "+node.target)
		}
	case *returnStmt:
		if !context.allowReturn {
			return controlFlowSyntaxError(node.span(), "return statement is not inside a function")
		}
		return validateExpressionControlFlow(node.value)
	case *blockStmt:
		for _, child := range node.body {
			if err := validateStatementControlFlow(child, context); err != nil {
				return err
			}
		}
	case *ifStmt:
		if err := validateExpressionControlFlow(node.test); err != nil {
			return err
		}
		if err := validateStatementControlFlow(node.then, context); err != nil {
			return err
		}
		if node.otherwise != nil {
			return validateStatementControlFlow(node.otherwise, context)
		}
	case *whileStmt:
		if err := validateExpressionControlFlow(node.test); err != nil {
			return err
		}
		loopContext := context
		loopContext.breakableDepth++
		loopContext.iterationDepth++
		return validateStatementControlFlow(node.body, loopContext)
	case *withStmt:
		if err := validateExpressionControlFlow(node.object); err != nil {
			return err
		}
		return validateStatementControlFlow(node.body, context)
	case *forStmt:
		if err := validateStatementControlFlow(node.init, context); err != nil {
			return err
		}
		if err := validateExpressionControlFlow(node.test); err != nil {
			return err
		}
		if err := validateExpressionControlFlow(node.update); err != nil {
			return err
		}
		loopContext := context
		loopContext.breakableDepth++
		loopContext.iterationDepth++
		return validateStatementControlFlow(node.body, loopContext)
	case *labelledStmt:
		if _, duplicate := context.labels[node.label]; duplicate {
			return controlFlowSyntaxError(node.span(), "duplicate label "+node.label)
		}
		if _, labelledFunction := node.body.(*functionStmt); labelledFunction {
			return controlFlowSyntaxError(node.span(), "labelled function declarations require Annex B")
		}
		if _, labelledClass := node.body.(*classStmt); labelledClass {
			return controlFlowSyntaxError(node.span(), "class declaration is not allowed as a labelled item")
		}
		if declaration, lexicalDeclaration := node.body.(*varStmt); lexicalDeclaration && declaration.declaration != TokVar {
			return controlFlowSyntaxError(node.span(), "lexical declaration is not allowed as a labelled item")
		}
		if declarations, lexicalDeclarations := node.body.(*varsStmt); lexicalDeclarations && len(declarations.declarations) > 0 && declarations.declarations[0].declaration != TokVar {
			return controlFlowSyntaxError(node.span(), "lexical declaration is not allowed as a labelled item")
		}
		labelContext := context
		labelContext.labels = cloneLabels(context.labels)
		labelContext.labels[node.label] = labelsIterationStatement(node.body)
		return validateStatementControlFlow(node.body, labelContext)
	case *functionStmt:
		return validateControlFlow(node.fn.body, true)
	case *classStmt:
		return validateClassControlFlow(node.definition)
	}
	return nil
}

func validateExpressionControlFlow(expression expr) error {
	switch node := expression.(type) {
	case *functionExpr:
		return validateControlFlow(node.body, true)
	case *classExpr:
		return validateClassControlFlow(node)
	case *unaryExpr:
		return validateExpressionControlFlow(node.right)
	case *updateExpr:
		return validateExpressionControlFlow(node.target)
	case *binaryExpr:
		if err := validateExpressionControlFlow(node.left); err != nil {
			return err
		}
		return validateExpressionControlFlow(node.right)
	case *sequenceExpr:
		for _, value := range node.values {
			if err := validateExpressionControlFlow(value); err != nil {
				return err
			}
		}
	case *assignExpr:
		if err := validateExpressionControlFlow(node.target); err != nil {
			return err
		}
		return validateExpressionControlFlow(node.value)
	case *callExpr:
		if err := validateExpressionControlFlow(node.callee); err != nil {
			return err
		}
		for _, argument := range node.args {
			if err := validateExpressionControlFlow(argument); err != nil {
				return err
			}
		}
	case *memberExpr:
		if err := validateExpressionControlFlow(node.object); err != nil {
			return err
		}
		return validateExpressionControlFlow(node.property)
	case *arrayExpr:
		for _, value := range node.values {
			if err := validateExpressionControlFlow(value); err != nil {
				return err
			}
		}
	case *objectExpr:
		for _, entry := range node.entries {
			if err := validateExpressionControlFlow(entry.value); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateClassControlFlow(class *classExpr) error {
	if class.heritage != nil {
		if err := validateExpressionControlFlow(class.heritage); err != nil {
			return err
		}
	}
	for _, element := range class.elements {
		if element.staticBlock != nil {
			if err := validateControlFlow(element.staticBlock, false); err != nil {
				return err
			}
			continue
		}
		if err := validateExpressionControlFlow(element.key); err != nil {
			return err
		}
		if element.field {
			if element.initializer != nil {
				if err := validateExpressionControlFlow(element.initializer); err != nil {
					return err
				}
			}
		} else {
			if err := validateControlFlow(element.method.body, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func labelsIterationStatement(statement stmt) bool {
	switch node := statement.(type) {
	case *whileStmt, *forStmt:
		return true
	case *labelledStmt:
		return labelsIterationStatement(node.body)
	default:
		return false
	}
}

func cloneLabels(labels map[string]bool) map[string]bool {
	clone := make(map[string]bool, len(labels)+1)
	for label, iteration := range labels {
		clone[label] = iteration
	}
	return clone
}

func controlFlowSyntaxError(span Span, message string) error {
	token := Token{Span: span}
	return &SyntaxError{Span: span, Token: token, Message: message}
}
