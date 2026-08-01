package gots

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"time"
)

var (
	ErrNilProgram = errors.New("gots: nil program")
	ErrRuntime    = errors.New("gots: runtime error")
	ErrStepLimit  = errors.New("gots: step limit exceeded")
	ErrCallDepth  = errors.New("gots: call depth exceeded")
	ErrCancelled  = errors.New("gots: execution cancelled")
)

type RuntimeError struct {
	Message string
	Span    Span
	Cause   error
}

func (e *RuntimeError) Error() string {
	if e.Span.Start.Line > 0 {
		return fmt.Sprintf("runtime error at %d:%d: %s", e.Span.Start.Line, e.Span.Start.Column, e.Message)
	}
	return "runtime error: " + e.Message
}
func (e *RuntimeError) Unwrap() error {
	if e.Cause != nil {
		return e.Cause
	}
	return ErrRuntime
}

type Stats struct {
	Steps        uint64
	MaxCallDepth int
	Duration     time.Duration
}
type Option func(*Runtime)

func WithMaxSteps(n uint64) Option  { return func(r *Runtime) { r.maxSteps = n } }
func WithMaxCallDepth(n int) Option { return func(r *Runtime) { r.maxDepth = n } }

type Runtime struct {
	global   *environment
	maxSteps uint64
	maxDepth int
	exec     *execution
	last     Stats
}
type Program struct {
	source string
	body   []stmt
}

func (p *Program) Source() string {
	if p == nil {
		return ""
	}
	return p.source
}

type execution struct {
	ctx            context.Context
	steps          uint64
	depth, maxSeen int
	started        time.Time
}

func New(options ...Option) *Runtime {
	r := &Runtime{global: newEnvironment(nil), maxDepth: 256}
	for _, o := range options {
		o(r)
	}
	weakMapConstructor := nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
		m := newWeakMapValue()
		if len(args) > 0 && !args[0].IsUndefined() && args[0].Kind() != KindNull {
			it := args[0]
			if it.k != KindObject || !it.o.array {
				return Undefined(), &RuntimeError{Message: "WeakMap iterable must be an array"}
			}
			l := int(number(it.o.props["length"]))
			for i := 0; i < l; i++ {
				entry, _ := property(it, fmt.Sprint(i))
				if entry.k != KindObject || !entry.o.array || int(number(entry.o.props["length"])) < 2 {
					return Undefined(), &RuntimeError{Message: "WeakMap entry must be a key-value pair"}
				}
				if _, e := m.o.weakmap.set(entry.o.props["0"], entry.o.props["1"]); e != nil {
					return Undefined(), e
				}
			}
		}
		return m, nil
	})
	weakMapConstructor.f.constructOnly = true
	r.global.define("WeakMap", weakMapConstructor, false)
	symbolConstructor := nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
		description := ""
		if len(args) > 0 && !args[0].IsUndefined() {
			description = args[0].String()
		}
		return Value{k: KindSymbol, sy: &symbolValue{identity: newIdentity(), description: description}}, nil
	})
	symbolConstructor.f.noConstruct = true
	symbolFor := nativeValue(func(_ *Runtime, _ Value, args []Value) (Value, error) {
		description := "undefined"
		if len(args) > 0 {
			description = args[0].String()
		}
		return Value{k: KindSymbol, sy: &symbolValue{identity: newIdentity(), description: description, registered: true}}, nil
	})
	symbolConstructor.f.props["for"] = symbolFor
	r.global.define("Symbol", symbolConstructor, false)
	r.global.define("NaN", Number(math.NaN()), true)
	r.global.define("Infinity", Number(math.Inf(1)), true)
	return r
}
func (r *Runtime) Compile(s string) (*Program, error) {
	b, e := parse(s)
	if e != nil {
		return nil, e
	}
	return &Program{source: s, body: b}, nil
}
func (r *Runtime) Run(p *Program) (Value, error) { return r.RunContext(context.Background(), p) }
func (r *Runtime) RunString(s string) (Value, error) {
	return r.RunStringContext(context.Background(), s)
}
func (r *Runtime) RunStringContext(ctx context.Context, s string) (Value, error) {
	p, e := r.Compile(s)
	if e != nil {
		return Undefined(), e
	}
	return r.RunContext(ctx, p)
}
func (r *Runtime) RunContext(ctx context.Context, p *Program) (result Value, err error) {
	if p == nil {
		return Undefined(), ErrNilProgram
	}
	if ctx == nil {
		ctx = context.Background()
	}
	x := &execution{ctx: ctx, started: time.Now()}
	r.exec = x
	defer func() {
		r.last = Stats{x.steps, x.maxSeen, time.Since(x.started)}
		r.exec = nil
		if v := recover(); v != nil {
			err = &RuntimeError{Message: fmt.Sprintf("host panic: %v", v)}
			result = Undefined()
		}
	}()
	result, _, err = r.evalStatements(p.body, r.global)
	return
}
func (r *Runtime) LastStats() Stats { return r.last }
func (r *Runtime) Set(name string, x any) error {
	v, e := r.fromGo(x)
	if e != nil {
		return e
	}
	if _, ok := r.global.values[name]; ok {
		return r.global.set(name, v)
	}
	r.global.define(name, v, false)
	return nil
}
func (r *Runtime) Get(name string) Value { v, _ := r.global.get(name); return v }

// Call invokes a JavaScript or native function from host bridge code.
func (r *Runtime) Call(fn, this Value, args ...Value) (Value, error) {
	if fn.k != KindFunction {
		return Undefined(), &RuntimeError{Message: "value is not callable"}
	}
	if r.exec != nil {
		return r.call(fn, this, args, Span{})
	}
	x := &execution{ctx: context.Background(), started: time.Now()}
	r.exec = x
	defer func() {
		r.last = Stats{x.steps, x.maxSeen, time.Since(x.started)}
		r.exec = nil
	}()
	return r.call(fn, this, args, Span{})
}
func (r *Runtime) checkpoint(s Span) error {
	x := r.exec
	x.steps++
	if e := x.ctx.Err(); e != nil {
		return &RuntimeError{Message: e.Error(), Span: s, Cause: ErrCancelled}
	}
	if r.maxSteps > 0 && x.steps > r.maxSteps {
		return &RuntimeError{Message: ErrStepLimit.Error(), Span: s, Cause: ErrStepLimit}
	}
	return nil
}
func (r *Runtime) evalStatements(ss []stmt, e *environment) (Value, bool, error) {
	out := Undefined()
	for _, s := range ss {
		if x := r.checkpoint(s.span()); x != nil {
			return Undefined(), false, x
		}
		v, ret, x := r.evalStmt(s, e)
		if x != nil || ret {
			return v, ret, x
		}
		out = v
	}
	return out, false, nil
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
		if n.op == TokTypeof {
			if id, ok := n.right.(*identExpr); ok {
				if _, found := e.get(id.name); !found {
					return String("undefined"), nil
				}
			}
			v, x := r.eval(n.right, e)
			if x != nil {
				return Undefined(), x
			}
			switch v.k {
			case KindUndefined:
				return String("undefined"), nil
			case KindBoolean:
				return String("boolean"), nil
			case KindNumber:
				return String("number"), nil
			case KindString:
				return String("string"), nil
			case KindFunction:
				return String("function"), nil
			case KindSymbol:
				return String("symbol"), nil
			default:
				return String("object"), nil
			}
		}
		v, x := r.eval(n.right, e)
		if x != nil {
			return Undefined(), x
		}
		switch n.op {
		case TokBang:
			return Boolean(!truthy(v)), nil
		case TokMinus:
			return Number(-number(v)), nil
		default:
			return Number(number(v)), nil
		}
	case *binaryExpr:
		return r.evalBinary(n, e)
	case *memberExpr:
		o, x := r.eval(n.object, e)
		if x != nil {
			return Undefined(), x
		}
		k, x := r.eval(n.property, e)
		if x != nil {
			return Undefined(), x
		}
		v, _ := property(o, k.String())
		return v, nil
	case *assignExpr:
		v, x := r.eval(n.value, e)
		if x != nil {
			return Undefined(), x
		}
		switch t := n.target.(type) {
		case *identExpr:
			x = e.set(t.name, v)
		case *memberExpr:
			o, z := r.eval(t.object, e)
			if z != nil {
				return Undefined(), z
			}
			k, z := r.eval(t.property, e)
			if z != nil {
				return Undefined(), z
			}
			x = setProperty(o, k.String(), v)
		}
		if x != nil {
			return Undefined(), r.err(n.span(), x.Error(), x)
		}
		return v, nil
	case *callExpr:
		return r.evalCall(n, e)
	}
	return Undefined(), r.err(x.span(), "unsupported feature", nil)
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
		return Boolean(equal(l, q)), nil
	case TokNE, TokStrictNE:
		return Boolean(!equal(l, q)), nil
	}
	return q, nil
}
func equal(a, b Value) bool {
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
func (r *Runtime) makeFunction(n *functionExpr, e *environment) Value {
	return Value{k: KindFunction, f: &function{identity: newIdentity(), props: map[string]Value{}, params: n.params, body: n.body, closure: e, name: n.name}}
}
func (r *Runtime) evalCall(n *callExpr, e *environment) (Value, error) {
	var this = Undefined()
	callee, x := r.eval(n.callee, e)
	if x != nil {
		return Undefined(), x
	}
	if m, ok := n.callee.(*memberExpr); ok {
		this, x = r.eval(m.object, e)
		if x != nil {
			return Undefined(), x
		}
	}
	if callee.k != KindFunction {
		return Undefined(), r.err(n.span(), "value is not callable", nil)
	}
	if callee.f.constructOnly && !n.construct {
		return Undefined(), r.err(n.span(), "constructor requires new", nil)
	}
	if callee.f.noConstruct && n.construct {
		return Undefined(), r.err(n.span(), "function is not a constructor", nil)
	}
	args := make([]Value, len(n.args))
	for i, a := range n.args {
		args[i], x = r.eval(a, e)
		if x != nil {
			return Undefined(), x
		}
	}
	return r.call(callee, this, args, n.span())
}
func (r *Runtime) call(v, this Value, args []Value, s Span) (Value, error) {
	f := v.f
	r.exec.depth++
	defer func() { r.exec.depth-- }()
	if r.exec.depth > r.exec.maxSeen {
		r.exec.maxSeen = r.exec.depth
	}
	if r.maxDepth > 0 && r.exec.depth > r.maxDepth {
		return Undefined(), r.err(s, ErrCallDepth.Error(), ErrCallDepth)
	}
	if x := r.checkpoint(s); x != nil {
		return Undefined(), x
	}
	if f.native != nil {
		return f.native(r, this, args)
	}
	env := newEnvironment(f.closure)
	for i, p := range f.params {
		v := Undefined()
		if i < len(args) {
			v = args[i]
		}
		env.define(p, v, false)
	}
	if f.name != "" {
		env.define(f.name, Value{k: KindFunction, f: f}, true)
	}
	v, _, x := r.evalStatements(f.body, env)
	runtime.KeepAlive(f.identity)
	return v, x
}
func (r *Runtime) err(s Span, m string, c error) error {
	return &RuntimeError{Message: m, Span: s, Cause: c}
}
func (r *Runtime) fromGo(x any) (Value, error) {
	if x == nil {
		return Null(), nil
	}
	if v, ok := x.(Value); ok {
		return v, nil
	}
	if f, ok := x.(NativeFunction); ok {
		return nativeValue(f), nil
	}
	v := reflect.ValueOf(x)
	if v.Kind() == reflect.Func {
		return nativeValue(reflectFunction(v)), nil
	}
	switch q := x.(type) {
	case string:
		return String(q), nil
	case bool:
		return Boolean(q), nil
	case int:
		return Number(float64(q)), nil
	case int64:
		return Number(float64(q)), nil
	case float64:
		return Number(q), nil
	}
	return Undefined(), fmt.Errorf("unsupported Go value %T", x)
}
func reflectFunction(fn reflect.Value) NativeFunction {
	return func(_ *Runtime, _ Value, args []Value) (Value, error) {
		t := fn.Type()
		if (!t.IsVariadic() && len(args) != t.NumIn()) || (t.IsVariadic() && len(args) < t.NumIn()-1) {
			return Undefined(), &RuntimeError{Message: "invalid host function argument count"}
		}
		in := make([]reflect.Value, len(args))
		for i, a := range args {
			typ := t.In(i)
			if t.IsVariadic() && i >= t.NumIn()-1 {
				typ = t.In(t.NumIn() - 1).Elem()
			}
			switch typ.Kind() {
			case reflect.String:
				in[i] = reflect.ValueOf(a.String()).Convert(typ)
			case reflect.Float64:
				in[i] = reflect.ValueOf(number(a)).Convert(typ)
			case reflect.Int:
				in[i] = reflect.ValueOf(int(number(a))).Convert(typ)
			case reflect.Bool:
				in[i] = reflect.ValueOf(truthy(a)).Convert(typ)
			default:
				return Undefined(), &RuntimeError{Message: "unsupported host argument type"}
			}
		}
		out := fn.Call(in)
		if len(out) == 0 {
			return Undefined(), nil
		}
		switch v := out[0].Interface().(type) {
		case string:
			return String(v), nil
		case int:
			return Number(float64(v)), nil
		case float64:
			return Number(v), nil
		case bool:
			return Boolean(v), nil
		case Value:
			return v, nil
		}
		return Undefined(), &RuntimeError{Message: "unsupported host return type"}
	}
}
