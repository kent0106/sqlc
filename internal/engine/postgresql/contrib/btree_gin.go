// Code generated by sqlc-pg-gen. DO NOT EDIT.

package contrib

import (
	"github.com/xiazemin/sqlc/internal/sql/ast"
	"github.com/xiazemin/sqlc/internal/sql/catalog"
)

func BtreeGin() *catalog.Schema {
	s := &catalog.Schema{Name: "pg_catalog"}
	s.Funcs = []*catalog.Function{
		{
			Name: "gin_enum_cmp",
			Args: []*catalog.Argument{
				{
					Type: &ast.TypeName{Name: "anyenum"},
				},
				{
					Type: &ast.TypeName{Name: "anyenum"},
				},
			},
			ReturnType: &ast.TypeName{Name: "integer"},
		},
		{
			Name: "gin_numeric_cmp",
			Args: []*catalog.Argument{
				{
					Type: &ast.TypeName{Name: "numeric"},
				},
				{
					Type: &ast.TypeName{Name: "numeric"},
				},
			},
			ReturnType: &ast.TypeName{Name: "integer"},
		},
	}
	return s
}
