package gots

func validatePrivateNames(statements []stmt, names map[string]struct{}) error {
	for _, statement := range statements {
		if err := validatePrivateNamesInStatement(statement, names); err != nil {
			return err
		}
	}
	return nil
}

func validatePrivateNamesInStatement(statement stmt, names map[string]struct{}) error {
	switch node := statement.(type) {
	case *exprStmt:
		return validatePrivateNamesInExpression(node.e, names)
	case *varStmt:
		return validatePrivateNamesInExpression(node.value, names)
	case *varsStmt:
		for _, declaration := range node.declarations {
			if err := validatePrivateNamesInExpression(declaration.value, names); err != nil {
				return err
			}
		}
	case *blockStmt:
		return validatePrivateNames(node.body, names)
	case *ifStmt:
		if err := validatePrivateNamesInExpression(node.test, names); err != nil {
			return err
		}
		if err := validatePrivateNamesInStatement(node.then, names); err != nil {
			return err
		}
		if node.otherwise != nil {
			return validatePrivateNamesInStatement(node.otherwise, names)
		}
	case *whileStmt:
		if err := validatePrivateNamesInExpression(node.test, names); err != nil {
			return err
		}
		return validatePrivateNamesInStatement(node.body, names)
	case *withStmt:
		if err := validatePrivateNamesInExpression(node.object, names); err != nil {
			return err
		}
		return validatePrivateNamesInStatement(node.body, names)
	case *forStmt:
		if err := validatePrivateNamesInStatement(node.init, names); err != nil {
			return err
		}
		if err := validatePrivateNamesInExpression(node.test, names); err != nil {
			return err
		}
		if err := validatePrivateNamesInExpression(node.update, names); err != nil {
			return err
		}
		return validatePrivateNamesInStatement(node.body, names)
	case *returnStmt:
		return validatePrivateNamesInExpression(node.value, names)
	case *labelledStmt:
		return validatePrivateNamesInStatement(node.body, names)
	case *functionStmt:
		return validatePrivateNames(node.fn.body, names)
	case *classStmt:
		return validatePrivateNamesInClass(node.definition, names)
	}
	return nil
}

func validatePrivateNamesInClass(class *classExpr, outer map[string]struct{}) error {
	names := clonePrivateNameSet(outer)
	for _, element := range class.elements {
		if privateName, ok := element.key.(*privateNameExpr); ok {
			names[privateName.name] = struct{}{}
		}
	}
	if class.heritage != nil {
		if err := validatePrivateNamesInExpression(class.heritage, outer); err != nil {
			return err
		}
	}
	for _, element := range class.elements {
		if element.staticBlock != nil {
			if err := validatePrivateNames(element.staticBlock, names); err != nil {
				return err
			}
			continue
		}
		if _, private := element.key.(*privateNameExpr); !private {
			if err := validatePrivateNamesInExpression(element.key, names); err != nil {
				return err
			}
		}
		if element.field && element.initializer != nil {
			if err := validatePrivateNamesInExpression(element.initializer, names); err != nil {
				return err
			}
		} else if element.method != nil {
			if err := validatePrivateNames(element.method.body, names); err != nil {
				return err
			}
		}
	}
	return nil
}

func validatePrivateNamesInExpression(expression expr, names map[string]struct{}) error {
	if expression == nil {
		return nil
	}
	switch node := expression.(type) {
	case *privateNameExpr:
		if _, found := names[node.name]; !found {
			return controlFlowSyntaxError(node.span(), "private name "+node.name+" is not declared")
		}
	case *functionExpr:
		return validatePrivateNames(node.body, names)
	case *classExpr:
		return validatePrivateNamesInClass(node, names)
	case *unaryExpr:
		return validatePrivateNamesInExpression(node.right, names)
	case *updateExpr:
		return validatePrivateNamesInExpression(node.target, names)
	case *binaryExpr:
		if err := validatePrivateNamesInExpression(node.left, names); err != nil {
			return err
		}
		return validatePrivateNamesInExpression(node.right, names)
	case *sequenceExpr:
		for _, value := range node.values {
			if err := validatePrivateNamesInExpression(value, names); err != nil {
				return err
			}
		}
	case *assignExpr:
		if err := validatePrivateNamesInExpression(node.target, names); err != nil {
			return err
		}
		return validatePrivateNamesInExpression(node.value, names)
	case *callExpr:
		if err := validatePrivateNamesInExpression(node.callee, names); err != nil {
			return err
		}
		for _, argument := range node.args {
			if err := validatePrivateNamesInExpression(argument, names); err != nil {
				return err
			}
		}
	case *memberExpr:
		if err := validatePrivateNamesInExpression(node.object, names); err != nil {
			return err
		}
		return validatePrivateNamesInExpression(node.property, names)
	case *arrayExpr:
		for _, value := range node.values {
			if err := validatePrivateNamesInExpression(value, names); err != nil {
				return err
			}
		}
	case *objectExpr:
		for _, entry := range node.entries {
			if err := validatePrivateNamesInExpression(entry.value, names); err != nil {
				return err
			}
		}
	}
	return nil
}

func clonePrivateNameSet(source map[string]struct{}) map[string]struct{} {
	clone := make(map[string]struct{}, len(source))
	for name := range source {
		clone[name] = struct{}{}
	}
	return clone
}
