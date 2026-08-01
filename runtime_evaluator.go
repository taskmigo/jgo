package gots

func (r *Runtime) checkpoint(span Span) error {
	execution := r.exec
	execution.steps++
	if err := execution.ctx.Err(); err != nil {
		return &RuntimeError{Message: err.Error(), Span: span, Cause: ErrCancelled}
	}
	if r.maxSteps > 0 && execution.steps > r.maxSteps {
		return &RuntimeError{Message: ErrStepLimit.Error(), Span: span, Cause: ErrStepLimit}
	}
	return nil
}

func (r *Runtime) evalStatements(statements []stmt, env *environment) (Value, bool, error) {
	result := Undefined()
	for _, statement := range statements {
		if err := r.checkpoint(statement.span()); err != nil {
			return Undefined(), false, err
		}
		value, returned, err := r.evalStmt(statement, env)
		if err != nil || returned {
			return value, returned, err
		}
		result = value
	}
	return result, false, nil
}
func (r *Runtime) evalStmt(s stmt, e *environment) (Value, bool, error) {
	switch n := s.(type) {
	case *exprStmt:
		v, x := r.eval(n.e, e)
		return v, false, x
	case *varStmt:
		v, x := r.eval(n.value, e)
		if x == nil {
			e.define(n.name, v, n.constant)
		}
		return v, false, x
	case *varsStmt:
		out := Undefined()
		for _, declaration := range n.declarations {
			v, _, x := r.evalStmt(declaration, e)
			if x != nil {
				return Undefined(), false, x
			}
			out = v
		}
		return out, false, nil
	case *functionStmt:
		v := r.makeFunction(n.fn, e)
		e.define(n.name, v, true)
		return v, false, nil
	case *returnStmt:
		v, x := r.eval(n.value, e)
		return v, true, x
	case *blockStmt:
		return r.evalStatements(n.body, newEnvironment(e))
	case *ifStmt:
		v, x := r.eval(n.test, e)
		if x != nil {
			return Undefined(), false, x
		}
		if truthy(v) {
			return r.evalStmt(n.then, e)
		}
		if n.otherwise != nil {
			return r.evalStmt(n.otherwise, e)
		}
		return Undefined(), false, nil
	case *whileStmt:
		out := Undefined()
		for {
			if x := r.checkpoint(n.span()); x != nil {
				return Undefined(), false, x
			}
			v, x := r.eval(n.test, e)
			if x != nil {
				return Undefined(), false, x
			}
			if !truthy(v) {
				return out, false, nil
			}
			v, ret, x := r.evalStmt(n.body, e)
			if x != nil || ret {
				return v, ret, x
			}
			out = v
		}
	case *forStmt:
		loopEnv := e
		if n.lexical {
			loopEnv = newEnvironment(e)
		}
		if _, _, x := r.evalStmt(n.init, loopEnv); x != nil {
			return Undefined(), false, x
		}
		out := Undefined()
		for {
			if x := r.checkpoint(n.span()); x != nil {
				return Undefined(), false, x
			}
			test, x := r.eval(n.test, loopEnv)
			if x != nil {
				return Undefined(), false, x
			}
			if !truthy(test) {
				return out, false, nil
			}
			v, ret, x := r.evalStmt(n.body, loopEnv)
			if x != nil || ret {
				return v, ret, x
			}
			out = v
			if n.lexical {
				loopEnv = loopEnv.cloneLocal()
			}
			if _, x = r.eval(n.update, loopEnv); x != nil {
				return Undefined(), false, x
			}
		}
	}
	return Undefined(), false, nil
}
func (r *Runtime) eval(x expr, e *environment) (Value, error) {
	if z := r.checkpoint(x.span()); z != nil {
		return Undefined(), z
	}
	switch n := x.(type) {
	case *literalExpr:
		return n.value, nil
	case *identExpr:
		v, ok := e.get(n.name)
		if !ok {
			return Undefined(), r.err(n.span(), n.name+" is not defined", nil)
		}
		return v, nil
	case *arrayExpr:
		a := make([]Value, len(n.values))
		for i, q := range n.values {
			v, x := r.eval(q, e)
			if x != nil {
				return Undefined(), x
			}
			a[i] = v
		}
		return NewArray(a...), nil
	case *objectExpr:
		o := NewObject()
		for _, q := range n.entries {
			v, x := r.eval(q.value, e)
			if x != nil {
				return Undefined(), x
			}
			o.o.props[q.name] = v
		}
		return o, nil
	case *functionExpr:
		return r.makeFunction(n, e), nil
	case *unaryExpr:
		return r.evalUnary(n, e)
	case *binaryExpr:
		return r.evalBinary(n, e)
	case *sequenceExpr:
		out := Undefined()
		for _, item := range n.values {
			v, x := r.eval(item, e)
			if x != nil {
				return Undefined(), x
			}
			out = v
		}
		return out, nil
	case *memberExpr:
		return r.evalMember(n, e)
	case *assignExpr:
		return r.evalAssignment(n, e)
	case *callExpr:
		return r.evalCall(n, e)
	}
	return Undefined(), r.err(x.span(), "unsupported feature", nil)
}

func (r *Runtime) evalUnary(expression *unaryExpr, env *environment) (Value, error) {
	if expression.op == TokTypeof {
		if identifier, ok := expression.right.(*identExpr); ok {
			if _, found := env.get(identifier.name); !found {
				return String("undefined"), nil
			}
		}
		value, err := r.eval(expression.right, env)
		if err != nil {
			return Undefined(), err
		}
		return String(typeOf(value)), nil
	}

	value, err := r.eval(expression.right, env)
	if err != nil {
		return Undefined(), err
	}
	switch expression.op {
	case TokBang:
		return Boolean(!truthy(value)), nil
	case TokMinus:
		return Number(-number(value)), nil
	default:
		return Number(number(value)), nil
	}
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

func (r *Runtime) evalMember(expression *memberExpr, env *environment) (Value, error) {
	object, err := r.eval(expression.object, env)
	if err != nil {
		return Undefined(), err
	}
	key, err := r.eval(expression.property, env)
	if err != nil {
		return Undefined(), err
	}
	value, _ := property(object, key.String())
	return value, nil
}

func (r *Runtime) evalAssignment(expression *assignExpr, env *environment) (Value, error) {
	value, err := r.eval(expression.value, env)
	if err != nil {
		return Undefined(), err
	}
	switch target := expression.target.(type) {
	case *identExpr:
		err = env.set(target.name, value)
	case *memberExpr:
		object, evalErr := r.eval(target.object, env)
		if evalErr != nil {
			return Undefined(), evalErr
		}
		key, evalErr := r.eval(target.property, env)
		if evalErr != nil {
			return Undefined(), evalErr
		}
		err = setProperty(object, key.String(), value)
	}
	if err != nil {
		return Undefined(), r.err(expression.span(), err.Error(), err)
	}
	return value, nil
}
func (r *Runtime) evalBinary(n *binaryExpr, e *environment) (Value, error) {
	l, x := r.eval(n.left, e)
	if x != nil {
		return Undefined(), x
	}
	if n.op == TokAnd && !truthy(l) {
		return l, nil
	}
	if n.op == TokOr && truthy(l) {
		return l, nil
	}
	q, x := r.eval(n.right, e)
	if x != nil {
		return Undefined(), x
	}
	switch n.op {
	case TokPlus:
		if l.k == KindString || q.k == KindString {
			return String(l.String() + q.String()), nil
		}
		return Number(number(l) + number(q)), nil
	case TokMinus:
		return Number(number(l) - number(q)), nil
	case TokStar:
		return Number(number(l) * number(q)), nil
	case TokSlash:
		return Number(number(l) / number(q)), nil
	case TokLT:
		return Boolean(number(l) < number(q)), nil
	case TokLE:
		return Boolean(number(l) <= number(q)), nil
	case TokGT:
		return Boolean(number(l) > number(q)), nil
	case TokGE:
		return Boolean(number(l) >= number(q)), nil
	case TokEQ, TokStrictEQ:
		return Boolean(strictlyEqual(l, q)), nil
	case TokNE, TokStrictNE:
		return Boolean(!strictlyEqual(l, q)), nil
	}
	return q, nil
}
func strictlyEqual(a, b Value) bool {
	if a.k != b.k {
		return false
	}
	switch a.k {
	case KindUndefined, KindNull:
		return true
	case KindBoolean:
		return a.b == b.b
	case KindNumber:
		return a.n == b.n
	case KindString:
		return a.s == b.s
	case KindObject:
		return a.o == b.o
	case KindFunction:
		return a.f == b.f
	case KindSymbol:
		return a.sy == b.sy
	}
	return false
}

func (r *Runtime) err(s Span, m string, c error) error {
	return &RuntimeError{Message: m, Span: s, Cause: c}
}
