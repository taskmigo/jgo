package gots

func validateNewTargetStatements(statements []stmt, allowed bool) error {
	for _, statement := range statements {
		switch node := statement.(type) {
		case *exprStmt:
			if err := validateNewTargetExpression(node.e, allowed); err != nil {
				return err
			}
		case *varStmt:
			if err := validateNewTargetExpression(node.value, allowed); err != nil {
				return err
			}
		case *varsStmt:
			for _, declaration := range node.declarations {
				if err := validateNewTargetExpression(declaration.value, allowed); err != nil {
					return err
				}
			}
		case *blockStmt:
			if err := validateNewTargetStatements(node.body, allowed); err != nil {
				return err
			}
		case *ifStmt:
			if err := validateNewTargetExpression(node.test, allowed); err != nil {
				return err
			}
			if err := validateNewTargetStatements([]stmt{node.then}, allowed); err != nil {
				return err
			}
			if node.otherwise != nil {
				if err := validateNewTargetStatements([]stmt{node.otherwise}, allowed); err != nil {
					return err
				}
			}
		case *whileStmt:
			if err := validateNewTargetExpression(node.test, allowed); err != nil {
				return err
			}
			if err := validateNewTargetStatements([]stmt{node.body}, allowed); err != nil {
				return err
			}
		case *withStmt:
			if err := validateNewTargetExpression(node.object, allowed); err != nil {
				return err
			}
			if err := validateNewTargetStatements([]stmt{node.body}, allowed); err != nil {
				return err
			}
		case *forStmt:
			if err := validateNewTargetStatements([]stmt{node.init, node.body}, allowed); err != nil {
				return err
			}
			if err := validateNewTargetExpression(node.test, allowed); err != nil {
				return err
			}
			if err := validateNewTargetExpression(node.update, allowed); err != nil {
				return err
			}
		case *returnStmt:
			if err := validateNewTargetExpression(node.value, allowed); err != nil {
				return err
			}
		case *labelledStmt:
			if err := validateNewTargetStatements([]stmt{node.body}, allowed); err != nil {
				return err
			}
		case *functionStmt:
			if err := validateNewTargetStatements(node.fn.body, true); err != nil {
				return err
			}
		case *classStmt:
			if err := validateNewTargetClass(node.definition, allowed); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateNewTargetClass(class *classExpr, outerAllowed bool) error {
	if class.heritage != nil {
		if err := validateNewTargetExpression(class.heritage, outerAllowed); err != nil {
			return err
		}
	}
	for _, element := range class.elements {
		if element.staticBlock != nil {
			if err := validateNewTargetStatements(element.staticBlock, true); err != nil {
				return err
			}
			continue
		}
		if err := validateNewTargetExpression(element.key, outerAllowed); err != nil {
			return err
		}
		if element.field {
			if element.initializer != nil {
				if err := validateNewTargetExpression(element.initializer, true); err != nil {
					return err
				}
			}
		} else if err := validateNewTargetStatements(element.method.body, true); err != nil {
			return err
		}
	}
	return nil
}

func validateNewTargetExpression(expression expr, allowed bool) error {
	switch node := expression.(type) {
	case *newTargetExpr:
		if !allowed {
			return controlFlowSyntaxError(node.span(), "new.target is not allowed outside a function")
		}
	case *functionExpr:
		return validateNewTargetStatements(node.body, true)
	case *classExpr:
		return validateNewTargetClass(node, allowed)
	case *unaryExpr:
		return validateNewTargetExpression(node.right, allowed)
	case *updateExpr:
		return validateNewTargetExpression(node.target, allowed)
	case *binaryExpr:
		if err := validateNewTargetExpression(node.left, allowed); err != nil {
			return err
		}
		return validateNewTargetExpression(node.right, allowed)
	case *sequenceExpr:
		for _, value := range node.values {
			if err := validateNewTargetExpression(value, allowed); err != nil {
				return err
			}
		}
	case *assignExpr:
		if err := validateNewTargetExpression(node.target, allowed); err != nil {
			return err
		}
		return validateNewTargetExpression(node.value, allowed)
	case *callExpr:
		if err := validateNewTargetExpression(node.callee, allowed); err != nil {
			return err
		}
		for _, argument := range node.args {
			if err := validateNewTargetExpression(argument, allowed); err != nil {
				return err
			}
		}
	case *memberExpr:
		if err := validateNewTargetExpression(node.object, allowed); err != nil {
			return err
		}
		return validateNewTargetExpression(node.property, allowed)
	case *arrayExpr:
		for _, value := range node.values {
			if err := validateNewTargetExpression(value, allowed); err != nil {
				return err
			}
		}
	case *objectExpr:
		for _, entry := range node.entries {
			if err := validateNewTargetExpression(entry.value, allowed); err != nil {
				return err
			}
		}
	}
	return nil
}
