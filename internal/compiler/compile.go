package compiler

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xiazemin/sqlc/internal/metadata"
	"github.com/xiazemin/sqlc/internal/migrations"
	"github.com/xiazemin/sqlc/internal/multierr"
	"github.com/xiazemin/sqlc/internal/opts"
	"github.com/xiazemin/sqlc/internal/sql/ast"
	"github.com/xiazemin/sqlc/internal/sql/catalog"
	"github.com/xiazemin/sqlc/internal/sql/sqlerr"
	"github.com/xiazemin/sqlc/internal/sql/sqlpath"
	"github.com/xiazemin/sqlc/internal/util"
)

// TODO: Rename this interface Engine
type Parser interface {
	Parse(io.Reader) ([]ast.Statement, error)
	CommentSyntax() metadata.CommentSyntax
	IsReservedKeyword(string) bool
}

// copied over from gen.go
func structName(name string) string {
	out := ""
	for _, p := range strings.Split(name, "_") {
		if p == "id" {
			out += "ID"
		} else {
			out += strings.Title(p)
		}
	}
	return out
}

var identPattern = regexp.MustCompile("[^a-zA-Z0-9_]+")

func enumValueName(value string) string {
	name := ""
	id := strings.Replace(value, "-", "_", -1)
	id = strings.Replace(id, ":", "_", -1)
	id = strings.Replace(id, "/", "_", -1)
	id = identPattern.ReplaceAllString(id, "")
	for _, part := range strings.Split(id, "_") {
		name += strings.Title(part)
	}
	return name
}

// end copypasta
func parseCatalog(p Parser, c *catalog.Catalog, schemas []string) error {
	files, err := sqlpath.Glob(schemas)
	if err != nil {
		return err
	}
	merr := multierr.New()
	for _, filename := range files {
		blob, err := ioutil.ReadFile(filename)
		if err != nil {
			merr.Add(filename, "", 0, err)
			continue
		}
		contents := migrations.RemoveRollbackStatements(string(blob))
		stmts, err := p.Parse(strings.NewReader(contents))
		if err != nil {
			merr.Add(filename, contents, 0, err)
			continue
		}
		for i := range stmts {
			if err := c.Update(stmts[i]); err != nil {
				merr.Add(filename, contents, stmts[i].Pos(), err)
				continue
			}
		}
	}
	if len(merr.Errs()) > 0 {
		return merr
	}
	return nil
}

func (c *Compiler) parseQueries(o opts.Parser) (*Result, error) {
	var q []*Query
	merr := multierr.New()
	set := map[string]struct{}{}
	files, err := sqlpath.Glob(c.conf.Queries)
	if err != nil {
		return nil, err
	}
	for _, filename := range files {
		blob, err := ioutil.ReadFile(filename)
		if err != nil {
			merr.Add(filename, "", 0, err)
			continue
		}
		src := string(blob)
		// 从原文件，得到一棵棵语法树 根节点
		stmts, err := c.parser.Parse(strings.NewReader(src))
		util.Xiazeminlog("query stmts", stmts, false)
		if err != nil {
			merr.Add(filename, src, 0, err)
			continue
		}
		for _, stmt := range stmts {
			//解析查询
			query, err := c.parseQuery(stmt.Raw, src, o)
			util.Xiazeminlog(stmt.GetOpName(), query, false)
			if err == ErrUnsupportedStatementType {
				continue
			}
			if err != nil {
				var e *sqlerr.Error
				loc := stmt.Raw.Pos()
				if errors.As(err, &e) && e.Location != 0 {
					loc = e.Location
				}
				merr.Add(filename, src, loc, err)
				continue
			}
			if query.Name != "" {
				if _, exists := set[query.Name]; exists {
					merr.Add(filename, src, stmt.Raw.Pos(), fmt.Errorf("duplicate query name: %s", query.Name))
					continue
				}
				set[query.Name] = struct{}{}
			}
			query.Filename = filepath.Base(filename)
			if query != nil {
				q = append(q, query)
			}
		}
	}
	if len(merr.Errs()) > 0 {
		return nil, merr
	}
	if len(q) == 0 {
		return nil, fmt.Errorf("no queries contained in paths %s", strings.Join(c.conf.Queries, ","))
	}
	return &Result{
		Catalog: c.catalog,
		Queries: q,
	}, nil
}
