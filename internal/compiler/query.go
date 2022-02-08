package compiler

import (
	"github.com/xiazemin/sqlc/internal/sql/ast"
)

type Table struct {
	Rel     *ast.TableName
	Columns []*Column
}

type Column struct {
	Name     string
	DataType string
	NotNull  bool
	IsArray  bool
	IsSlice  bool
	Comment  string
	Length   *int

	// XXX: Figure out what PostgreSQL calls `foo.id`
	Scope string
	Table *ast.TableName
	Type  *ast.TypeName
}

type Query struct {
	SQL                   string
	Name                  string
	Cmd                   string // TODO: Pick a better name. One of: one, many, exec, execrows
	Columns               []*Column
	Params                []Parameter
	Comments              []string
	InsertValuesLen       int64
	InsertValuesParameter []Parameter
	Omits                 []string
	// XXX: Hack
	Filename string
}

//这里存的是参数，in 之所以有问题是因为没有解析出Parameter，name 是Colum的name
type Parameter struct {
	Number int
	Column *Column
}
