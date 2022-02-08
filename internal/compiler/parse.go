package compiler

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/xiazemin/sqlc/internal/debug"
	"github.com/xiazemin/sqlc/internal/metadata"
	"github.com/xiazemin/sqlc/internal/opts"
	"github.com/xiazemin/sqlc/internal/source"
	"github.com/xiazemin/sqlc/internal/sql/ast"
	"github.com/xiazemin/sqlc/internal/sql/astutils"
	"github.com/xiazemin/sqlc/internal/sql/rewrite"
	"github.com/xiazemin/sqlc/internal/sql/sqlerr"
	"github.com/xiazemin/sqlc/internal/sql/validate"
	"github.com/xiazemin/sqlc/internal/util"
)

var ErrUnsupportedStatementType = errors.New("parseQuery: unsupported statement type")

func rewriteNumberedParameters(refs []paramRef, raw *ast.RawStmt, sql string) ([]source.Edit, error) {
	edits := make([]source.Edit, len(refs))
	for i, ref := range refs {
		edits[i] = source.Edit{
			Location: ref.ref.Location - raw.StmtLocation,
			Old:      fmt.Sprintf("$%d", ref.ref.Number),
			New:      "?",
		}
	}
	return edits, nil
}

func (c *Compiler) parseQuery(stmt ast.Node, src string, o opts.Parser) (*Query, error) {
	if o.Debug.DumpAST {
		debug.Dump(stmt)
	}
	if err := validate.ParamStyle(stmt); err != nil {
		return nil, err
	}
	if err := validate.ParamRef(stmt); err != nil {
		return nil, err
	}
	raw, ok := stmt.(*ast.RawStmt)
	if !ok {
		return nil, errors.New("node is not a statement")
	}
	switch n := raw.Stmt.(type) {
	case *ast.SelectStmt:
	case *ast.DeleteStmt:
	case *ast.InsertStmt:
		util.Xiazeminlog("ast.InsertStmt", n, false)
		if err := validate.InsertStmt(n); err != nil {
			return nil, err
		}
	case *ast.TruncateStmt:
	case *ast.UpdateStmt:
	default:
		return nil, ErrUnsupportedStatementType
	}

	rawSQL, err := source.Pluck(src, raw.StmtLocation, raw.StmtLen)
	if err != nil {
		return nil, err
	}
	if rawSQL == "" {
		return nil, errors.New("missing semicolon at end of file")
	}
	if err := validate.FuncCall(c.catalog, raw); err != nil {
		return nil, err
	}

	//每一个sql 语句的解析
	name, cmd, omits, err := metadata.Parse(strings.TrimSpace(rawSQL), c.parser.CommentSyntax())
	//解析出每个语句的函数名和后面的返回
	util.Xiazeminlog("omits", omits, false)
	if err != nil {
		return nil, err
	}
	if err := validate.Cmd(raw.Stmt, name, cmd); err != nil {
		return nil, err
	}

	//嵌套函数
	raw, namedParams, edits := rewrite.NamedParameters(c.conf.Engine, raw)

	//获取参数名
	rvs := rangeVars(raw.Stmt)
	util.Xiazeminlog("params", rvs, false)
	//获取参数的 占位符号 位置 ？
	refs := findParameters(raw.Stmt)
	util.Xiazeminlog("params refs", refs, false)

	if o.UsePositionalParameters {
		edits, err = rewriteNumberedParameters(refs, raw, rawSQL)
		if err != nil {
			return nil, err
		}
	} else {
		refs = uniqueParamRefs(refs)
		sort.Slice(refs, func(i, j int) bool { return refs[i].ref.Number < refs[j].ref.Number })
	}
	//解析参数,这里是真正解析参数的地方 @xiazemin
	params, err := resolveCatalogRefs(c.catalog, rvs, refs, namedParams)

	util.Xiazeminlog("resolveCatalogRefs", params, false)
	if err != nil {
		return nil, err
	}

	if !validateOmits(omits, params) {
		return nil, sqlerr.ColumnNotFound("", strings.Join(omits, ","))
	}
	valuesParams, length, err := resolveCatalogValuesRefs(c.catalog, rvs, refs, namedParams)

	util.Xiazeminlog("resolveCatalogRefs", valuesParams, false)
	if err != nil {
		return nil, err
	}

	qc, err := buildQueryCatalog(c.catalog, raw.Stmt)
	util.Xiazeminlog("buildQueryCatalog", params, false)
	if err != nil {
		return nil, err
	}
	cols, err := outputColumns(qc, raw.Stmt)
	util.Xiazeminlog("outputColumns", cols, false)

	if err != nil {
		return nil, err
	}

	expandEdits, err := c.expand(qc, raw)
	if err != nil {
		return nil, err
	}
	edits = append(edits, expandEdits...)

	expanded, err := source.Mutate(rawSQL, edits)
	if err != nil {
		return nil, err
	}

	// If the query string was edited, make sure the syntax is valid
	if expanded != rawSQL {
		if _, err := c.parser.Parse(strings.NewReader(expanded)); err != nil {
			return nil, fmt.Errorf("edited query syntax is invalid: %w", err)
		}
	}

	trimmed, comments, err := source.StripComments(expanded)
	if err != nil {
		return nil, err
	}

	return &Query{
		Cmd:                   cmd,
		Comments:              comments,
		Name:                  name,
		Params:                params,
		Columns:               cols,
		SQL:                   trimmed,
		InsertValuesLen:       length,
		InsertValuesParameter: valuesParams,
		Omits:                 omits,
	}, nil
}

func rangeVars(root ast.Node) []*ast.RangeVar {
	var vars []*ast.RangeVar
	find := astutils.VisitorFunc(func(node ast.Node) {
		switch n := node.(type) {
		case *ast.RangeVar:
			vars = append(vars, n)
		case *ast.In:
			util.Xiazeminlog("range var in", n, false)
			if n.Sel == nil {
				/*	name := "inxiazemin"
					vars = append(vars, &ast.RangeVar{
						Catalogname: &name,
						Schemaname:  &name,
						Relname:     &name,
						Inh:         false,
						//Relpersistence byte
						//Alias          *Alias
						Location: n.Pos(),
						TypeIn:   true,
					})
				*/
			} else {

			}
		}
	})
	astutils.Walk(find, root)
	return vars
}

func uniqueParamRefs(in []paramRef) []paramRef {
	m := make(map[int]struct{}, len(in))
	o := make([]paramRef, 0, len(in))
	for _, v := range in {
		if _, ok := m[v.ref.Number]; !ok {
			m[v.ref.Number] = struct{}{}
			o = append(o, v)
		}
	}
	return o
}

func validateOmits(omits []string, params []Parameter) bool {
	column := make(map[string]struct{}, len(params))
	for _, p := range params {
		column[p.Column.Name] = struct{}{}
	}
	for _, o := range omits {
		if _, ok := column[o]; !ok {
			return false
		}
	}
	return true
}
