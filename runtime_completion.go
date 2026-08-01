package gots

type completionKind uint8

const (
	completionNormal completionKind = iota
	completionBreak
	completionContinue
	completionReturn
	completionThrow
)

type completion struct {
	kind   completionKind
	value  Value
	err    error
	span   Span
	target string
	empty  bool
}

func normalCompletion(value Value) completion {
	return completion{kind: completionNormal, value: value}
}
func emptyCompletion() completion {
	return completion{kind: completionNormal, value: Undefined(), empty: true}
}
func returnCompletion(value Value) completion {
	return completion{kind: completionReturn, value: value}
}
func breakCompletion(target string, span Span) completion {
	return completion{kind: completionBreak, value: Undefined(), target: target, span: span}
}
func continueCompletion(target string, span Span) completion {
	return completion{kind: completionContinue, value: Undefined(), target: target, span: span}
}
func throwCompletion(err error, span Span) completion {
	if exception, ok := err.(*Exception); ok && exception.Span.Start.Line == 0 {
		exception.Span = span
	}
	return completion{kind: completionThrow, value: Undefined(), err: err, span: span}
}
func operationalCompletion(err error, span Span) completion {
	return completion{kind: completionThrow, value: Undefined(), err: err, span: span}
}
func (result completion) abrupt() bool { return result.kind != completionNormal || result.err != nil }
func (result completion) exception() error {
	if result.err != nil {
		return result.err
	}
	return &Exception{Name: "Error", Value: result.value, Span: result.span}
}
