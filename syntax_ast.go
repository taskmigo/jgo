package gots

type node interface{ span() Span }
type expr interface {
	node
	exprNode()
}
type stmt interface {
	node
	stmtNode()
}
type base struct{ S Span }

func (b base) span() Span { return b.S }

type literalExpr struct {
	base
	value             Value
	directiveEligible bool
	directiveEscaped  bool
	sourceLiteral     string
	legacyOctalEscape bool
}

func (*literalExpr) exprNode() {}

type identExpr struct {
	base
	name string
}

func (*identExpr) exprNode() {}

type privateNameExpr struct {
	base
	name string
}

func (*privateNameExpr) exprNode() {}

type unaryExpr struct {
	base
	op    TokenType
	right expr
}

func (*unaryExpr) exprNode() {}

type updateExpr struct {
	base
	op     TokenType
	target expr
}

func (*updateExpr) exprNode() {}

type binaryExpr struct {
	base
	left  expr
	op    TokenType
	right expr
}

func (*binaryExpr) exprNode() {}

type sequenceExpr struct {
	base
	values []expr
}

func (*sequenceExpr) exprNode() {}

type assignExpr struct {
	base
	target expr
	value  expr
}

func (*assignExpr) exprNode() {}

type callExpr struct {
	base
	callee    expr
	args      []expr
	construct bool
}

func (*callExpr) exprNode() {}

type memberExpr struct {
	base
	object   expr
	property expr
}

func (*memberExpr) exprNode() {}

type arrayExpr struct {
	base
	values []expr
}

func (*arrayExpr) exprNode() {}

type objectEntry struct {
	name  string
	value expr
}
type objectExpr struct {
	base
	entries []objectEntry
}

func (*objectExpr) exprNode() {}

type functionExpr struct {
	base
	name   string
	params []string
	body   []stmt
	strict bool
}

func (*functionExpr) exprNode() {}

type newTargetExpr struct{ base }

func (*newTargetExpr) exprNode() {}

type classElement struct {
	key         expr
	method      *functionExpr
	initializer expr
	static      bool
	accessor    string
	field       bool
	generator   bool
	staticBlock []stmt
}

type classExpr struct {
	base
	name     string
	heritage expr
	elements []classElement
}

func (*classExpr) exprNode() {}

type classStmt struct {
	base
	name       string
	definition *classExpr
}

func (*classStmt) stmtNode() {}

type emptyStmt struct{ base }

func (*emptyStmt) stmtNode() {}

type exprStmt struct {
	base
	e expr
}

func (*exprStmt) stmtNode() {}

type varStmt struct {
	base
	name           string
	value          expr
	declaration    TokenType
	hasInitializer bool
}

func (*varStmt) stmtNode() {}

type varsStmt struct {
	base
	declarations []*varStmt
}

func (*varsStmt) stmtNode() {}

type blockStmt struct {
	base
	body []stmt
}

func (*blockStmt) stmtNode() {}

type ifStmt struct {
	base
	test            expr
	then, otherwise stmt
}

func (*ifStmt) stmtNode() {}

type whileStmt struct {
	base
	test expr
	body stmt
}

func (*whileStmt) stmtNode() {}

type withStmt struct {
	base
	object expr
	body   stmt
}

func (*withStmt) stmtNode() {}

type forStmt struct {
	base
	init         stmt
	test, update expr
	body         stmt
	lexical      bool
}

func (*forStmt) stmtNode() {}

type breakStmt struct {
	base
	target string
}

func (*breakStmt) stmtNode() {}

type continueStmt struct {
	base
	target string
}

func (*continueStmt) stmtNode() {}

type labelledStmt struct {
	base
	label string
	body  stmt
}

func (*labelledStmt) stmtNode() {}

type returnStmt struct {
	base
	value expr
}

func (*returnStmt) stmtNode() {}

type functionStmt struct {
	base
	name string
	fn   *functionExpr
}

func (*functionStmt) stmtNode() {}
