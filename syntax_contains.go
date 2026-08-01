package gots

func statementsContainDirectCall(statements []stmt, name string) bool {
	for _, statement := range statements {
		switch node := statement.(type) {
		case *exprStmt:
			if expressionContainsDirectCall(node.e, name) {
				return true
			}
		case *varStmt:
			if expressionContainsDirectCall(node.value, name) {
				return true
			}
		case *varsStmt:
			for _, declaration := range node.declarations {
				if expressionContainsDirectCall(declaration.value, name) {
					return true
				}
			}
		case *blockStmt:
			if statementsContainDirectCall(node.body, name) {
				return true
			}
		case *ifStmt:
			if expressionContainsDirectCall(node.test, name) || statementsContainDirectCall([]stmt{node.then}, name) || node.otherwise != nil && statementsContainDirectCall([]stmt{node.otherwise}, name) {
				return true
			}
		case *whileStmt:
			if expressionContainsDirectCall(node.test, name) || statementsContainDirectCall([]stmt{node.body}, name) {
				return true
			}
		case *withStmt:
			if expressionContainsDirectCall(node.object, name) || statementsContainDirectCall([]stmt{node.body}, name) {
				return true
			}
		case *forStmt:
			if statementsContainDirectCall([]stmt{node.init, node.body}, name) || expressionContainsDirectCall(node.test, name) || expressionContainsDirectCall(node.update, name) {
				return true
			}
		case *returnStmt:
			if expressionContainsDirectCall(node.value, name) {
				return true
			}
		case *labelledStmt:
			if statementsContainDirectCall([]stmt{node.body}, name) {
				return true
			}
		}
	}
	return false
}

func expressionContainsDirectCall(expression expr, name string) bool {
	switch node := expression.(type) {
	case *unaryExpr:
		return expressionContainsDirectCall(node.right, name)
	case *updateExpr:
		return expressionContainsDirectCall(node.target, name)
	case *binaryExpr:
		return expressionContainsDirectCall(node.left, name) || expressionContainsDirectCall(node.right, name)
	case *sequenceExpr:
		for _, value := range node.values {
			if expressionContainsDirectCall(value, name) {
				return true
			}
		}
	case *assignExpr:
		return expressionContainsDirectCall(node.target, name) || expressionContainsDirectCall(node.value, name)
	case *callExpr:
		if identifier, ok := node.callee.(*identExpr); ok && identifier.name == name {
			return true
		}
		if expressionContainsDirectCall(node.callee, name) {
			return true
		}
		for _, argument := range node.args {
			if expressionContainsDirectCall(argument, name) {
				return true
			}
		}
	case *memberExpr:
		return expressionContainsDirectCall(node.object, name) || expressionContainsDirectCall(node.property, name)
	case *arrayExpr:
		for _, value := range node.values {
			if expressionContainsDirectCall(value, name) {
				return true
			}
		}
	case *objectExpr:
		for _, entry := range node.entries {
			if expressionContainsDirectCall(entry.value, name) {
				return true
			}
		}
	}
	return false
}
