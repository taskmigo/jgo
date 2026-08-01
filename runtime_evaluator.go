package gots

func (runtime *Runtime) checkpoint(span Span) error {
	execution := runtime.exec
	execution.steps++
	if execution.ctx.Err() != nil {
		return fmtExecutionError(ErrCancelled, span)
	}
	if runtime.config.MaxSteps > 0 && execution.steps > runtime.config.MaxSteps {
		return fmtExecutionError(ErrStepLimit, span)
	}
	return nil
}

type executionError struct {
	cause error
	span  Span
}

func (failure *executionError) Error() string {
	if failure.span.Start.Line > 0 {
		return failure.cause.Error() + " at " + positionString(failure.span.Start)
	}
	return failure.cause.Error()
}
func (failure *executionError) Unwrap() error { return failure.cause }
func fmtExecutionError(cause error, span Span) error {
	return &executionError{cause: cause, span: span}
}
func positionString(position Position) string {
	return fmtInt(position.Line) + ":" + fmtInt(position.Column)
}

func (runtime *Runtime) instantiateDeclarations(statements []stmt, environment *environment) error {
	if err := runtime.instantiateVarDeclarations(statements, environment.variable); err != nil {
		return err
	}
	for _, statement := range statements {
		switch declaration := statement.(type) {
		case *varStmt:
			if declaration.declaration != TokVar {
				if err := environment.createUninitializedBinding(declaration.name, declaration.declaration != TokConst); err != nil {
					return err
				}
			}
		case *varsStmt:
			for _, item := range declaration.declarations {
				if item.declaration != TokVar {
					if err := environment.createUninitializedBinding(item.name, item.declaration != TokConst); err != nil {
						return err
					}
				}
			}
		case *classStmt:
			if err := environment.createUninitializedBinding(declaration.name, true); err != nil {
				return err
			}
		case *functionStmt:
			functionValue := runtime.makeFunction(declaration.fn, environment)
			declarationEnvironment := environment.variable
			if declarationEnvironment.hasOwnBinding(declaration.name) {
				binding := declarationEnvironment.bindings[declaration.name]
				if !binding.mutable || binding.silentReadOnly {
					return typeError("cannot declare global function " + declaration.name)
				}
				binding.value = functionValue
				binding.initialized = true
				declarationEnvironment.bindings[declaration.name] = binding
			} else {
				declarationEnvironment.createMutableBinding(declaration.name, functionValue)
			}
		}
	}
	return nil
}

func (runtime *Runtime) instantiateVarDeclarations(statements []stmt, variableEnvironment *environment) error {
	for _, statement := range statements {
		switch node := statement.(type) {
		case *varStmt:
			if node.declaration == TokVar && !variableEnvironment.hasOwnBinding(node.name) {
				variableEnvironment.createMutableBinding(node.name, Undefined())
			}
		case *varsStmt:
			for _, declaration := range node.declarations {
				if declaration.declaration == TokVar && !variableEnvironment.hasOwnBinding(declaration.name) {
					variableEnvironment.createMutableBinding(declaration.name, Undefined())
				}
			}
		case *blockStmt:
			if err := runtime.instantiateVarDeclarations(node.body, variableEnvironment); err != nil {
				return err
			}
		case *ifStmt:
			if err := runtime.instantiateVarDeclarations([]stmt{node.then}, variableEnvironment); err != nil {
				return err
			}
			if node.otherwise != nil {
				if err := runtime.instantiateVarDeclarations([]stmt{node.otherwise}, variableEnvironment); err != nil {
					return err
				}
			}
		case *whileStmt:
			if err := runtime.instantiateVarDeclarations([]stmt{node.body}, variableEnvironment); err != nil {
				return err
			}
		case *withStmt:
			if err := runtime.instantiateVarDeclarations([]stmt{node.body}, variableEnvironment); err != nil {
				return err
			}
		case *forStmt:
			if err := runtime.instantiateVarDeclarations([]stmt{node.init, node.body}, variableEnvironment); err != nil {
				return err
			}
		case *labelledStmt:
			if err := runtime.instantiateVarDeclarations([]stmt{node.body}, variableEnvironment); err != nil {
				return err
			}
		}
	}
	return nil
}

func (runtime *Runtime) evalStatements(statements []stmt, environment *environment) completion {
	result := emptyCompletion()
	for _, statement := range statements {
		if err := runtime.checkpoint(statement.span()); err != nil {
			return operationalCompletion(err, statement.span())
		}
		statementResult := runtime.evalStatement(statement, environment)
		if statementResult.abrupt() {
			if statementResult.empty {
				statementResult.value = result.value
				statementResult.empty = result.empty
			}
			return statementResult
		}
		if !statementResult.empty {
			result = statementResult
		}
	}
	return result
}

func (runtime *Runtime) evalStatement(statement stmt, environment *environment) completion {
	switch node := statement.(type) {
	case *emptyStmt:
		return emptyCompletion()
	case *exprStmt:
		value, err := runtime.eval(node.e, environment)
		if err != nil {
			return throwCompletion(err, node.span())
		}
		return normalCompletion(value)
	case *varStmt:
		if node.declaration == TokVar && !node.hasInitializer {
			return emptyCompletion()
		}
		value, err := runtime.eval(node.value, environment)
		if err == nil {
			declarationEnvironment := environment
			if node.declaration == TokVar {
				declarationEnvironment = environment.variable
			}
			if node.declaration == TokVar {
				err = declarationEnvironment.putMutableBinding(node.name, value, runtime.exec.strict)
			} else {
				err = declarationEnvironment.initializeBinding(node.name, value)
			}
		}
		if err != nil {
			return throwCompletion(err, node.span())
		}
		return emptyCompletion()
	case *varsStmt:
		result := emptyCompletion()
		for _, declaration := range node.declarations {
			result = runtime.evalStatement(declaration, environment)
			if result.abrupt() {
				return result
			}
		}
		return result
	case *functionStmt:
		return emptyCompletion()
	case *classStmt:
		value, err := runtime.evalClass(node.definition, environment)
		if err == nil {
			err = environment.initializeBinding(node.name, value)
		}
		if err != nil {
			return throwCompletion(err, node.span())
		}
		return emptyCompletion()
	case *returnStmt:
		value, err := runtime.eval(node.value, environment)
		if err != nil {
			return throwCompletion(err, node.span())
		}
		return returnCompletion(value)
	case *blockStmt:
		blockEnvironment := newEnvironment(environment)
		if err := runtime.instantiateDeclarations(node.body, blockEnvironment); err != nil {
			return throwCompletion(&Exception{Name: "SyntaxError", Message: err.Error(), Span: node.span()}, node.span())
		}
		return runtime.evalStatements(node.body, blockEnvironment)
	case *ifStmt:
		test, err := runtime.eval(node.test, environment)
		if err != nil {
			return throwCompletion(err, node.test.span())
		}
		if toBoolean(test) {
			return runtime.evalStatement(node.then, environment)
		}
		if node.otherwise != nil {
			return runtime.evalStatement(node.otherwise, environment)
		}
		return normalCompletion(Undefined())
	case *whileStmt:
		return runtime.evalWhile(node, environment, nil)
	case *withStmt:
		objectValue, err := runtime.eval(node.object, environment)
		if err != nil {
			return throwCompletion(err, node.object.span())
		}
		objectValue, err = runtime.toObjectValue(objectValue)
		if err != nil {
			return throwCompletion(err, node.object.span())
		}
		return runtime.evalStatement(node.body, newObjectEnvironment(runtime, objectValue, environment))
	case *forStmt:
		return runtime.evalFor(node, environment, nil)
	case *breakStmt:
		return breakCompletion(node.target, node.span())
	case *continueStmt:
		return continueCompletion(node.target, node.span())
	case *labelledStmt:
		return runtime.evalLabelled(node, environment, nil)
	default:
		return throwCompletion(typeError("unsupported statement"), statement.span())
	}
}

func (runtime *Runtime) evalWhile(statement *whileStmt, environment *environment, labelSet []string) completion {
	result := normalCompletion(Undefined())
	for {
		if err := runtime.checkpoint(statement.span()); err != nil {
			return operationalCompletion(err, statement.span())
		}
		test, err := runtime.eval(statement.test, environment)
		if err != nil {
			return throwCompletion(err, statement.test.span())
		}
		if !toBoolean(test) {
			return result
		}
		bodyResult := runtime.evalStatement(statement.body, environment)
		switch bodyResult.kind {
		case completionBreak:
			if bodyResult.target == "" {
				return result
			}
			return bodyResult
		case completionContinue:
			if bodyResult.target != "" && !containsLabel(labelSet, bodyResult.target) {
				return bodyResult
			}
		case completionNormal:
			result = bodyResult
		default:
			return bodyResult
		}
	}
}

func (runtime *Runtime) evalFor(statement *forStmt, environment *environment, labelSet []string) completion {
	loopEnvironment := environment
	if statement.lexical {
		loopEnvironment = newEnvironment(environment)
	}
	if err := runtime.instantiateDeclarations([]stmt{statement.init}, loopEnvironment); err != nil {
		return throwCompletion(&Exception{Name: "SyntaxError", Message: err.Error()}, statement.span())
	}
	initialization := runtime.evalStatement(statement.init, loopEnvironment)
	if initialization.abrupt() {
		return initialization
	}
	result := normalCompletion(Undefined())
	for {
		if err := runtime.checkpoint(statement.span()); err != nil {
			return operationalCompletion(err, statement.span())
		}
		test, err := runtime.eval(statement.test, loopEnvironment)
		if err != nil {
			return throwCompletion(err, statement.test.span())
		}
		if !toBoolean(test) {
			return result
		}
		bodyResult := runtime.evalStatement(statement.body, loopEnvironment)
		switch bodyResult.kind {
		case completionBreak:
			if bodyResult.target == "" {
				return result
			}
			return bodyResult
		case completionContinue:
			if bodyResult.target != "" && !containsLabel(labelSet, bodyResult.target) {
				return bodyResult
			}
		case completionNormal:
			result = bodyResult
		default:
			return bodyResult
		}
		if statement.lexical {
			loopEnvironment = loopEnvironment.cloneLocal()
		}
		if _, err := runtime.eval(statement.update, loopEnvironment); err != nil {
			return throwCompletion(err, statement.update.span())
		}
	}
}

func (runtime *Runtime) evalLabelled(statement *labelledStmt, environment *environment, labelSet []string) completion {
	labelSet = append(labelSet, statement.label)
	var result completion
	switch body := statement.body.(type) {
	case *labelledStmt:
		result = runtime.evalLabelled(body, environment, labelSet)
	case *whileStmt:
		result = runtime.evalWhile(body, environment, labelSet)
	case *forStmt:
		result = runtime.evalFor(body, environment, labelSet)
	default:
		result = runtime.evalStatement(body, environment)
	}
	if result.kind == completionBreak && result.target == statement.label {
		return normalCompletion(result.value)
	}
	return result
}

func containsLabel(labels []string, target string) bool {
	for _, label := range labels {
		if label == target {
			return true
		}
	}
	return false
}

func (runtime *Runtime) eval(expression expr, environment *environment) (Value, error) {
	if err := runtime.checkpoint(expression.span()); err != nil {
		return Undefined(), err
	}
	switch node := expression.(type) {
	case *literalExpr:
		return node.value, nil
	case *identExpr, *memberExpr:
		reference, err := runtime.evalReference(expression, environment)
		if err != nil {
			return Undefined(), err
		}
		return reference.getValue()
	case *arrayExpr:
		values := make([]Value, len(node.values))
		for index, item := range node.values {
			value, err := runtime.eval(item, environment)
			if err != nil {
				return Undefined(), err
			}
			values[index] = value
		}
		return runtime.newArray(values...), nil
	case *objectExpr:
		object := runtime.newOrdinaryObject()
		for _, entry := range node.entries {
			value, err := runtime.eval(entry.value, environment)
			if err != nil {
				return Undefined(), err
			}
			_ = defineProperty(object, StringKey(entry.name), defaultProperty(value))
		}
		return object, nil
	case *functionExpr:
		return runtime.makeFunction(node, environment), nil
	case *classExpr:
		return runtime.evalClass(node, environment)
	case *newTargetExpr:
		return runtime.exec.newTarget, nil
	case *unaryExpr:
		return runtime.evalUnary(node, environment)
	case *updateExpr:
		return runtime.evalUpdate(node, environment)
	case *binaryExpr:
		return runtime.evalBinary(node, environment)
	case *sequenceExpr:
		result := Undefined()
		for _, item := range node.values {
			value, err := runtime.eval(item, environment)
			if err != nil {
				return Undefined(), err
			}
			result = value
		}
		return result, nil
	case *assignExpr:
		reference, err := runtime.evalReference(node.target, environment)
		if err != nil {
			return Undefined(), err
		}
		value, err := runtime.eval(node.value, environment)
		if err == nil {
			err = reference.putValue(value)
		}
		return value, err
	case *callExpr:
		return runtime.evalCall(node, environment)
	default:
		return Undefined(), typeError("unsupported expression")
	}
}

func (runtime *Runtime) evalUpdate(expression *updateExpr, environment *environment) (Value, error) {
	reference, err := runtime.evalReference(expression.target, environment)
	if err != nil {
		return Undefined(), err
	}
	oldValue, err := reference.getValue()
	if err != nil {
		return Undefined(), err
	}
	number, err := runtime.toNumber(oldValue)
	if err != nil {
		return Undefined(), err
	}
	if expression.op == TokPlusPlus {
		number++
	} else {
		number--
	}
	newValue := Number(number)
	return newValue, reference.putValue(newValue)
}

func (runtime *Runtime) evalReference(expression expr, environment *environment) (reference, error) {
	switch target := expression.(type) {
	case *identExpr:
		return bindingReference(runtime, environment, target.name), nil
	case *memberExpr:
		if identifier, ok := target.object.(*identExpr); ok && identifier.name == "super" {
			activeFunction := runtime.exec.activeFunction
			if activeFunction.k != KindFunction || activeFunction.f.homeObject == nil {
				return reference{}, referenceError("super property is not valid in this context")
			}
			receiver, err := environment.getBindingValue("this")
			if err != nil {
				return reference{}, err
			}
			property, err := runtime.eval(target.property, environment)
			if err != nil {
				return reference{}, err
			}
			key, err := runtime.toPropertyKey(property)
			if err != nil {
				return reference{}, err
			}
			superBase := activeFunction.f.homeObject.prototype
			if superBase == nil {
				return reference{}, typeError("super base is null")
			}
			return propertyReferenceWithReceiver(runtime, Value{k: KindObject, o: superBase}, key, receiver), nil
		}
		base, err := runtime.eval(target.object, environment)
		if err != nil {
			return reference{}, err
		}
		if privateName, ok := target.property.(*privateNameExpr); ok {
			identifier := environment.resolvePrivateName(privateName.name)
			if identifier == nil {
				return reference{}, &Exception{Name: "SyntaxError", Message: "private name " + privateName.name + " is not declared"}
			}
			return privateReference(runtime, base, identifier), nil
		}
		property, err := runtime.eval(target.property, environment)
		if err != nil {
			return reference{}, err
		}
		key, err := runtime.toPropertyKey(property)
		if err != nil {
			return reference{}, err
		}
		return propertyReference(runtime, base, key), nil
	default:
		return reference{}, referenceError("invalid assignment target")
	}
}

func (runtime *Runtime) evalUnary(expression *unaryExpr, environment *environment) (Value, error) {
	if expression.op == TokDelete {
		switch expression.right.(type) {
		case *identExpr, *memberExpr:
			reference, err := runtime.evalReference(expression.right, environment)
			if err != nil {
				return Undefined(), err
			}
			deleted, err := reference.delete()
			if err != nil {
				return Undefined(), err
			}
			if !deleted && reference.strict {
				return Undefined(), typeError("property cannot be deleted")
			}
			return Boolean(deleted), nil
		default:
			if _, err := runtime.eval(expression.right, environment); err != nil {
				return Undefined(), err
			}
			return Boolean(true), nil
		}
	}
	if expression.op == TokTypeof {
		if identifier, ok := expression.right.(*identExpr); ok && environment.resolveBinding(identifier.name) == nil {
			return String("undefined"), nil
		}
		value, err := runtime.eval(expression.right, environment)
		if err != nil {
			return Undefined(), err
		}
		return String(typeOf(value)), nil
	}
	value, err := runtime.eval(expression.right, environment)
	if err != nil {
		return Undefined(), err
	}
	if expression.op == TokBang {
		return Boolean(!toBoolean(value)), nil
	}
	number, err := runtime.toNumber(value)
	if err != nil {
		return Undefined(), err
	}
	if expression.op == TokMinus {
		number = -number
	}
	return Number(number), nil
}

func typeOf(value Value) string {
	switch value.k {
	case KindUndefined:
		return "undefined"
	case KindBoolean:
		return "boolean"
	case KindNumber:
		return "number"
	case KindString:
		return "string"
	case KindFunction:
		return "function"
	case KindSymbol:
		return "symbol"
	default:
		return "object"
	}
}

func (runtime *Runtime) evalBinary(expression *binaryExpr, environment *environment) (Value, error) {
	left, err := runtime.eval(expression.left, environment)
	if err != nil {
		return Undefined(), err
	}
	if expression.op == TokAnd && !toBoolean(left) {
		return left, nil
	}
	if expression.op == TokOr && toBoolean(left) {
		return left, nil
	}
	right, err := runtime.eval(expression.right, environment)
	if err != nil {
		return Undefined(), err
	}
	if expression.op == TokAnd || expression.op == TokOr {
		return right, nil
	}
	return runtime.applyBinaryOperator(expression.op, left, right)
}

func (runtime *Runtime) applyBinaryOperator(operator TokenType, left, right Value) (Value, error) {
	switch operator {
	case TokEQ, TokNE:
		equal, err := runtime.looselyEqual(left, right)
		if operator == TokNE {
			equal = !equal
		}
		return Boolean(equal), err
	case TokStrictEQ:
		return Boolean(strictlyEqual(left, right)), nil
	case TokStrictNE:
		return Boolean(!strictlyEqual(left, right)), nil
	case TokPlus:
		leftPrimitive, err := runtime.toPrimitive(left, false)
		if err != nil {
			return Undefined(), err
		}
		rightPrimitive, err := runtime.toPrimitive(right, false)
		if err != nil {
			return Undefined(), err
		}
		if leftPrimitive.k == KindString || rightPrimitive.k == KindString {
			leftString, err := runtime.toString(leftPrimitive)
			if err != nil {
				return Undefined(), err
			}
			rightString, err := runtime.toString(rightPrimitive)
			if err != nil {
				return Undefined(), err
			}
			return StringUTF16(append(leftString.clone(), rightString...)), nil
		}
		leftNumber, err := runtime.toNumber(leftPrimitive)
		if err != nil {
			return Undefined(), err
		}
		rightNumber, err := runtime.toNumber(rightPrimitive)
		return Number(leftNumber + rightNumber), err
	case TokIn:
		object := objectRecord(right)
		if object == nil {
			return Undefined(), typeError("right-hand side of 'in' is not an object")
		}
		key, err := runtime.toPropertyKey(left)
		if err != nil {
			return Undefined(), err
		}
		return Boolean(ordinaryHasProperty(object, key)), nil
	case TokLT, TokLE, TokGT, TokGE:
		leftPrimitive, err := runtime.toPrimitive(left, false)
		if err != nil {
			return Undefined(), err
		}
		rightPrimitive, err := runtime.toPrimitive(right, false)
		if err != nil {
			return Undefined(), err
		}
		if leftPrimitive.k == KindString && rightPrimitive.k == KindString {
			comparison := compareUTF16(leftPrimitive.s, rightPrimitive.s)
			switch operator {
			case TokLT:
				return Boolean(comparison < 0), nil
			case TokLE:
				return Boolean(comparison <= 0), nil
			case TokGT:
				return Boolean(comparison > 0), nil
			default:
				return Boolean(comparison >= 0), nil
			}
		}
		leftNumber, err := runtime.toNumber(leftPrimitive)
		if err != nil {
			return Undefined(), err
		}
		rightNumber, err := runtime.toNumber(rightPrimitive)
		if err != nil {
			return Undefined(), err
		}
		switch operator {
		case TokLT:
			return Boolean(leftNumber < rightNumber), nil
		case TokLE:
			return Boolean(leftNumber <= rightNumber), nil
		case TokGT:
			return Boolean(leftNumber > rightNumber), nil
		default:
			return Boolean(leftNumber >= rightNumber), nil
		}
	}
	leftNumber, err := runtime.toNumber(left)
	if err != nil {
		return Undefined(), err
	}
	rightNumber, err := runtime.toNumber(right)
	if err != nil {
		return Undefined(), err
	}
	switch operator {
	case TokMinus:
		return Number(leftNumber - rightNumber), nil
	case TokStar:
		return Number(leftNumber * rightNumber), nil
	case TokSlash:
		return Number(leftNumber / rightNumber), nil
	default:
		return right, nil
	}
}

func compareUTF16(left, right []uint16) int {
	length := min(len(left), len(right))
	for index := 0; index < length; index++ {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return 0
}

func fmtInt(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 12)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	for left, right := 0, len(digits)-1; left < right; left, right = left+1, right-1 {
		digits[left], digits[right] = digits[right], digits[left]
	}
	return string(digits)
}
