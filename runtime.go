package gots

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNilProgram  = errors.New("gots: nil program")
	ErrStepLimit   = errors.New("gots: step limit exceeded")
	ErrCallDepth   = errors.New("gots: call depth exceeded")
	ErrCancelled   = errors.New("gots: execution cancelled")
	ErrRuntimeBusy = errors.New("gots: runtime is already executing")
)

// Exception is an uncaught ECMAScript exception crossing the host boundary.
type Exception struct {
	Name    string
	Message string
	Value   Value
	Span    Span
}

func (exception *Exception) Error() string {
	message := exception.Name
	if exception.Message != "" {
		message += ": " + exception.Message
	}
	if exception.Span.Start.Line > 0 {
		return fmt.Sprintf("%s at %d:%d", message, exception.Span.Start.Line, exception.Span.Start.Column)
	}
	return message
}

func typeError(message string) error { return &Exception{Name: "TypeError", Message: message} }
func referenceError(message string) error {
	return &Exception{Name: "ReferenceError", Message: message}
}

type Stats struct {
	Steps        uint64
	MaxCallDepth int
	Duration     time.Duration
}

type Result struct {
	Value Value
	Stats Stats
}

type Config struct {
	MaxSteps     uint64
	MaxCallDepth int
}

type Runtime struct {
	global            *environment
	config            Config
	exec              *execution
	symbols           map[string]*symbolValue
	iteratorSymbol    *symbolValue
	hasInstanceSymbol *symbolValue
	intrinsics        intrinsics
}

type Program struct {
	source string
	body   []stmt
	strict bool
}

func (program *Program) Source() string {
	if program == nil {
		return ""
	}
	return program.source
}

type execution struct {
	ctx                   context.Context
	steps                 uint64
	depth, maxSeen        int
	started               time.Time
	strict                bool
	activeFunction        Value
	newTarget             Value
	classFieldInitializer bool
}

func New(config Config) *Runtime {
	if config.MaxCallDepth == 0 {
		config.MaxCallDepth = 256
	}
	runtime := &Runtime{
		global:  newEnvironment(nil),
		config:  config,
		symbols: make(map[string]*symbolValue),
	}
	runtime.installGlobalBuiltins()
	return runtime
}

func (runtime *Runtime) Compile(source string) (*Program, error) {
	parsed, err := parse(source)
	if err != nil {
		return nil, err
	}
	return &Program{source: source, body: parsed.body, strict: parsed.strict}, nil
}

func (runtime *Runtime) EvaluateString(ctx context.Context, source string) (Result, error) {
	program, err := runtime.Compile(source)
	if err != nil {
		return Result{}, err
	}
	return runtime.Evaluate(ctx, program)
}

func (runtime *Runtime) Evaluate(ctx context.Context, program *Program) (result Result, err error) {
	if program == nil {
		return Result{}, ErrNilProgram
	}
	if runtime.exec != nil {
		return Result{}, ErrRuntimeBusy
	}
	if ctx == nil {
		ctx = context.Background()
	}
	execution := &execution{ctx: ctx, started: time.Now(), strict: program.strict}
	runtime.exec = execution
	defer func() {
		result.Stats = Stats{Steps: execution.steps, MaxCallDepth: execution.maxSeen, Duration: time.Since(execution.started)}
		runtime.exec = nil
		if recovered := recover(); recovered != nil {
			result.Value = Undefined()
			err = fmt.Errorf("host panic: %v", recovered)
		}
	}()

	if declarationError := runtime.instantiateDeclarations(program.body, runtime.global); declarationError != nil {
		return result, &Exception{Name: "SyntaxError", Message: declarationError.Error()}
	}
	completion := runtime.evalStatements(program.body, runtime.global)
	if completion.err != nil {
		return result, completion.err
	}
	if completion.kind == completionThrow {
		return result, completion.exception()
	}
	result.Value = completion.value
	return result, nil
}

func (runtime *Runtime) Define(name string, value any) error {
	runtimeValue, err := runtime.fromGo(value)
	if err != nil {
		return err
	}
	if runtime.global.hasOwnBinding(name) {
		return runtime.global.setMutableBinding(name, runtimeValue)
	}
	runtime.global.createMutableBinding(name, runtimeValue)
	return nil
}

func (runtime *Runtime) Lookup(name string) (Value, bool) {
	value, err := runtime.global.getBindingValue(name)
	return value, err == nil
}

func (runtime *Runtime) Call(ctx context.Context, callable, this Value, arguments ...Value) (result Result, err error) {
	if callable.k != KindFunction || callable.f.call == nil {
		return Result{}, typeError("value is not callable")
	}
	if runtime.exec != nil {
		value, err := runtime.call(callable, this, arguments, Span{})
		return Result{Value: value}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	execution := &execution{ctx: ctx, started: time.Now()}
	runtime.exec = execution
	defer func() {
		result.Stats = Stats{Steps: execution.steps, MaxCallDepth: execution.maxSeen, Duration: time.Since(execution.started)}
		runtime.exec = nil
		if recovered := recover(); recovered != nil {
			result.Value = Undefined()
			err = fmt.Errorf("host panic: %v", recovered)
		}
	}()
	result.Value, err = runtime.call(callable, this, arguments, Span{})
	return result, err
}
