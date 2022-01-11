package ast

type CreateEnumStmt struct {
	TypeName  *TypeName
	Vals      *List
	IsNotNull bool
}

func (n *CreateEnumStmt) Pos() int {
	return 0
}
