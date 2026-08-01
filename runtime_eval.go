package gots

import "strings"

func (runtime *Runtime) installEvalAndFunctionBuiltins() {
	evalFunction := nativeValue(func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		return runtime.performEval(argument(arguments, 0), runtime.global, false, false)
	})
	evalFunction.f.object.prototype = runtime.intrinsics.functionPrototype
	runtime.intrinsics.evalFunction = defineFunctionMetadata(evalFunction, "eval", 1)
	runtime.global.createMutableBinding("eval", runtime.intrinsics.evalFunction)

	functionConstructor := nativeValue(func(runtime *Runtime, _ Value, arguments []Value) (Value, error) {
		return runtime.createDynamicFunction(arguments)
	})
	functionConstructor.f.construct = functionConstructor.f.call
	functionConstructor.f.object.prototype = runtime.intrinsics.functionPrototype
	linkConstructor(functionConstructor, runtime.intrinsics.functionPrototype)
	runtime.global.createMutableBinding("Function", defineFunctionMetadata(functionConstructor, "Function", 1))
}

func (runtime *Runtime) performEval(source Value, callerEnvironment *environment, strictCaller, direct bool) (Value, error) {
	if source.k != KindString {
		return source, nil
	}
	parsed, err := parse(jsString(source.s).goString())
	if err != nil {
		return Undefined(), err
	}
	if direct && runtime.exec.classFieldInitializer && statementsContainIdentifier(parsed.body, "arguments") {
		return Undefined(), &Exception{Name: "SyntaxError", Message: "arguments is not allowed in eval inside a class field initializer"}
	}
	if statementsContainIdentifier(parsed.body, "super") {
		return Undefined(), &Exception{Name: "SyntaxError", Message: "super is not valid in this eval context"}
	}

	strictEval := strictCaller || parsed.strict
	if strictCaller {
		annotateFunctionStrictness(parsed.body, true)
		if err := validateStrictStatements(parsed.body, true); err != nil {
			return Undefined(), err
		}
	}

	baseEnvironment := runtime.global
	variableEnvironment := runtime.global
	if direct {
		baseEnvironment = callerEnvironment
		variableEnvironment = callerEnvironment.variable
	}
	lexicalEnvironment := newEnvironment(baseEnvironment)
	if strictEval {
		lexicalEnvironment.variable = lexicalEnvironment
	} else {
		lexicalEnvironment.variable = variableEnvironment
		if err := validateEvalVarConflicts(parsed.body, lexicalEnvironment, variableEnvironment); err != nil {
			return Undefined(), err
		}
	}

	if err := runtime.instantiateDeclarations(parsed.body, lexicalEnvironment); err != nil {
		return Undefined(), &Exception{Name: "SyntaxError", Message: err.Error()}
	}
	previousStrict := runtime.exec.strict
	runtime.exec.strict = strictEval
	defer func() { runtime.exec.strict = previousStrict }()
	result := runtime.evalStatements(parsed.body, lexicalEnvironment)
	if result.err != nil {
		return Undefined(), result.err
	}
	if result.kind == completionThrow {
		return Undefined(), result.exception()
	}
	if result.empty {
		return Undefined(), nil
	}
	return result.value, nil
}

func statementsContainIdentifier(statements []stmt, name string) bool {
	for _, statement := range statements {
		switch node := statement.(type) {
		case *exprStmt:
			if expressionContainsIdentifier(node.e, name) {
				return true
			}
		case *varStmt:
			if expressionContainsIdentifier(node.value, name) {
				return true
			}
		case *varsStmt:
			for _, declaration := range node.declarations {
				if expressionContainsIdentifier(declaration.value, name) {
					return true
				}
			}
		case *blockStmt:
			if statementsContainIdentifier(node.body, name) {
				return true
			}
		case *ifStmt:
			if expressionContainsIdentifier(node.test, name) || statementsContainIdentifier([]stmt{node.then}, name) || node.otherwise != nil && statementsContainIdentifier([]stmt{node.otherwise}, name) {
				return true
			}
		case *whileStmt:
			if expressionContainsIdentifier(node.test, name) || statementsContainIdentifier([]stmt{node.body}, name) {
				return true
			}
		case *withStmt:
			if expressionContainsIdentifier(node.object, name) || statementsContainIdentifier([]stmt{node.body}, name) {
				return true
			}
		case *forStmt:
			if statementsContainIdentifier([]stmt{node.init, node.body}, name) || expressionContainsIdentifier(node.test, name) || expressionContainsIdentifier(node.update, name) {
				return true
			}
		case *returnStmt:
			if expressionContainsIdentifier(node.value, name) {
				return true
			}
		case *labelledStmt:
			if statementsContainIdentifier([]stmt{node.body}, name) {
				return true
			}
		}
	}
	return false
}

func expressionContainsIdentifier(expression expr, name string) bool {
	switch node := expression.(type) {
	case *identExpr:
		return node.name == name
	case *unaryExpr:
		return expressionContainsIdentifier(node.right, name)
	case *updateExpr:
		return expressionContainsIdentifier(node.target, name)
	case *binaryExpr:
		return expressionContainsIdentifier(node.left, name) || expressionContainsIdentifier(node.right, name)
	case *sequenceExpr:
		for _, value := range node.values {
			if expressionContainsIdentifier(value, name) {
				return true
			}
		}
	case *assignExpr:
		return expressionContainsIdentifier(node.target, name) || expressionContainsIdentifier(node.value, name)
	case *callExpr:
		if expressionContainsIdentifier(node.callee, name) {
			return true
		}
		for _, argument := range node.args {
			if expressionContainsIdentifier(argument, name) {
				return true
			}
		}
	case *memberExpr:
		return expressionContainsIdentifier(node.object, name) || expressionContainsIdentifier(node.property, name)
	case *arrayExpr:
		for _, value := range node.values {
			if expressionContainsIdentifier(value, name) {
				return true
			}
		}
	case *objectExpr:
		for _, entry := range node.entries {
			if expressionContainsIdentifier(entry.value, name) {
				return true
			}
		}
	}
	return false
}

func validateEvalVarConflicts(statements []stmt, lexicalEnvironment, variableEnvironment *environment) error {
	names := make(map[string]struct{})
	collectVarDeclaredNames(statements, names)
	for current := lexicalEnvironment; current != nil; current = current.outer {
		if !current.object {
			for name := range names {
				if existing, found := current.bindings[name]; found && existing.lexical {
					return &Exception{Name: "SyntaxError", Message: "eval variable conflicts with lexical binding " + name}
				}
			}
		}
		if current == variableEnvironment {
			break
		}
	}
	return nil
}

func collectVarDeclaredNames(statements []stmt, names map[string]struct{}) {
	for _, statement := range statements {
		switch node := statement.(type) {
		case *varStmt:
			if node.declaration == TokVar {
				names[node.name] = struct{}{}
			}
		case *varsStmt:
			for _, declaration := range node.declarations {
				if declaration.declaration == TokVar {
					names[declaration.name] = struct{}{}
				}
			}
		case *functionStmt:
			names[node.name] = struct{}{}
		case *blockStmt:
			collectVarDeclaredNames(node.body, names)
		case *ifStmt:
			collectVarDeclaredNames([]stmt{node.then}, names)
			if node.otherwise != nil {
				collectVarDeclaredNames([]stmt{node.otherwise}, names)
			}
		case *whileStmt:
			collectVarDeclaredNames([]stmt{node.body}, names)
		case *withStmt:
			collectVarDeclaredNames([]stmt{node.body}, names)
		case *forStmt:
			collectVarDeclaredNames([]stmt{node.init, node.body}, names)
		case *labelledStmt:
			collectVarDeclaredNames([]stmt{node.body}, names)
		}
	}
}

func (runtime *Runtime) createDynamicFunction(arguments []Value) (Value, error) {
	parameterStrings := make([]string, 0, max(0, len(arguments)-1))
	for index := 0; index+1 < len(arguments); index++ {
		parameter, err := runtime.toString(arguments[index])
		if err != nil {
			return Undefined(), err
		}
		parameterStrings = append(parameterStrings, parameter.goString())
	}
	body := ""
	if len(arguments) > 0 {
		bodyValue, err := runtime.toString(arguments[len(arguments)-1])
		if err != nil {
			return Undefined(), err
		}
		body = bodyValue.goString()
	}
	source := "function anonymous(" + strings.Join(parameterStrings, ",") + ") {\n" + body + "\n}"
	parsed, err := parse(source)
	if err != nil {
		return Undefined(), err
	}
	if len(parsed.body) != 1 {
		return Undefined(), &SyntaxError{Message: "invalid function constructor source"}
	}
	declaration, ok := parsed.body[0].(*functionStmt)
	if !ok {
		return Undefined(), &SyntaxError{Message: "invalid function constructor source"}
	}
	return runtime.makeFunction(declaration.fn, runtime.global), nil
}
