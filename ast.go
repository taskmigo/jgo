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
	value Value
}

func (*literalExpr) exprNode() {}

type identExpr struct {
	base
	name string
}

func (*identExpr) exprNode() {}

type unaryExpr struct {
	base
	op    TokenType
	right expr
}

func (*unaryExpr) exprNode() {}

type binaryExpr struct {
	base
	left  expr
	op    TokenType
	right expr
}

func (*binaryExpr) exprNode() {}

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
}

func (*functionExpr) exprNode() {}

type exprStmt struct {
	base
	e expr
}

func (*exprStmt) stmtNode() {}

type varStmt struct {
	base
	name     string
	value    expr
	constant bool
}

func (*varStmt) stmtNode() {}

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
