package gots

import (
	"context"
	"errors"
	"fmt"
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
	instance := &Runtime{global: newEnvironment(nil), maxDepth: 256}
	for _, option := range options {
		option(instance)
	}
	instance.installGlobalBuiltins()
	return instance
}

func (r *Runtime) Compile(source string) (*Program, error) {
	body, err := parse(source)
	if err != nil {
		return nil, err
	}
	return &Program{source: source, body: body}, nil
}

func (r *Runtime) Run(program *Program) (Value, error) {
	return r.RunContext(context.Background(), program)
}

func (r *Runtime) RunString(source string) (Value, error) {
	return r.RunStringContext(context.Background(), source)
}

func (r *Runtime) RunStringContext(ctx context.Context, source string) (Value, error) {
	program, err := r.Compile(source)
	if err != nil {
		return Undefined(), err
	}
	return r.RunContext(ctx, program)
}

func (r *Runtime) RunContext(ctx context.Context, program *Program) (result Value, err error) {
	if program == nil {
		return Undefined(), ErrNilProgram
	}
	if ctx == nil {
		ctx = context.Background()
	}
	execution := &execution{ctx: ctx, started: time.Now()}
	r.exec = execution
	defer func() {
		r.last = Stats{execution.steps, execution.maxSeen, time.Since(execution.started)}
		r.exec = nil
		if recovered := recover(); recovered != nil {
			err = &RuntimeError{Message: fmt.Sprintf("host panic: %v", recovered)}
			result = Undefined()
		}
	}()
	result, _, err = r.evalStatements(program.body, r.global)
	return result, err
}

func (r *Runtime) LastStats() Stats { return r.last }

func (r *Runtime) Set(name string, value any) error {
	runtimeValue, err := r.fromGo(value)
	if err != nil {
		return err
	}
	if _, exists := r.global.values[name]; exists {
		return r.global.set(name, runtimeValue)
	}
	r.global.define(name, runtimeValue, false)
	return nil
}

func (r *Runtime) Get(name string) Value {
	value, _ := r.global.get(name)
	return value
}

// Call invokes a JavaScript or native function from host bridge code.
func (r *Runtime) Call(function, this Value, args ...Value) (Value, error) {
	if function.k != KindFunction {
		return Undefined(), &RuntimeError{Message: "value is not callable"}
	}
	if r.exec != nil {
		return r.call(function, this, args, Span{})
	}
	execution := &execution{ctx: context.Background(), started: time.Now()}
	r.exec = execution
	defer func() {
		r.last = Stats{execution.steps, execution.maxSeen, time.Since(execution.started)}
		r.exec = nil
	}()
	return r.call(function, this, args, Span{})
}
