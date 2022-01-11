package golang

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xiazemin/sqlc/internal/codegen"
	"github.com/xiazemin/sqlc/internal/compiler"
	"github.com/xiazemin/sqlc/internal/config"
	"github.com/xiazemin/sqlc/internal/core"
	"github.com/xiazemin/sqlc/internal/inflection"
	"github.com/xiazemin/sqlc/internal/sql/catalog"
	"github.com/xiazemin/sqlc/internal/util"
)

func buildEnums(r *compiler.Result, settings config.CombinedSettings) []Enum {
	var enums []Enum
	for _, schema := range r.Catalog.Schemas {
		if schema.Name == "pg_catalog" {
			continue
		}
		for _, typ := range schema.Types {
			enum, ok := typ.(*catalog.Enum)
			if !ok {
				continue
			}
			var enumName string
			if schema.Name == r.Catalog.DefaultSchema {
				enumName = enum.Name
			} else {
				enumName = schema.Name + "_" + enum.Name
			}
			e := Enum{
				Name:      StructName(enumName, settings),
				Comment:   enum.Comment,
				IsNotNull: enum.IsNotNull,
			}
			for _, v := range enum.Vals {
				util.Xiazeminlog("enum.Vals ", v, false)
				util.Xiazeminlog("enum.Vals ", enumName, false)
				util.Xiazeminlog("enum.Vals ", EnumReplace(v), false)
				e.Constants = append(e.Constants, Constant{
					Name:  StructName(enumName+"_"+EnumReplace(v), settings),
					Value: v,
					Type:  e.Name,
				})
			}
			if !enum.IsNotNull {
				e.Constants = append(e.Constants, Constant{
					Name:  StructName(enumName+"_"+"NULL", settings),
					Value: "null",
					Type:  e.Name,
				})
			}
			enums = append(enums, e)
		}
	}
	if len(enums) > 0 {
		sort.Slice(enums, func(i, j int) bool { return enums[i].Name < enums[j].Name })
	}
	return enums
}

func buildStructs(r *compiler.Result, settings config.CombinedSettings) []Struct {
	var structs []Struct
	for _, schema := range r.Catalog.Schemas {
		if schema.Name == "pg_catalog" {
			continue
		}
		for _, table := range schema.Tables {
			var tableName string
			if schema.Name == r.Catalog.DefaultSchema {
				tableName = table.Rel.Name
			} else {
				tableName = schema.Name + "_" + table.Rel.Name
			}
			structName := tableName
			if !settings.Go.EmitExactTableNames {
				structName = inflection.Singular(structName)
			}
			s := Struct{
				Table:   core.FQN{Schema: schema.Name, Rel: table.Rel.Name},
				Name:    StructName(structName, settings),
				Comment: table.Comment,
			}
			for _, column := range table.Columns {
				tags := map[string]string{}
				if settings.Go.EmitDBTags {
					tags["db:"] = column.Name
				}
				if settings.Go.EmitJSONTags {
					tags["json:"] = column.Name
				}
				s.Fields = append(s.Fields, Field{
					Name:    StructName(column.Name, settings),
					Type:    goType(r, compiler.ConvertColumn(table.Rel, column), settings),
					Tags:    tags,
					Comment: column.Comment,
				})
			}
			structs = append(structs, s)
		}
	}
	if len(structs) > 0 {
		sort.Slice(structs, func(i, j int) bool { return structs[i].Name < structs[j].Name })
	}
	return structs
}

type goColumn struct {
	id int
	*compiler.Column
	IsSlice bool
}

func columnName(c *compiler.Column, pos int) string {
	if c.Name != "" {
		return c.Name
	}
	return fmt.Sprintf("column_%d", pos+1)
}

func paramName(p compiler.Parameter) string {
	if p.Column == nil {
		return fmt.Sprintf("dollar_Column_%d", p.Number)
	}
	if p.Column.Name != "" {
		return argName(p.Column.Name)
	}
	return fmt.Sprintf("dollar_%d", p.Number)
}

func argName(name string) string {
	out := ""
	for i, p := range strings.Split(name, "_") {
		if i == 0 {
			out += strings.ToLower(p)
		} else if p == "id" {
			out += "ID"
		} else {
			out += strings.Title(p)
		}
	}
	return out
}

func buildQueries(r *compiler.Result, settings config.CombinedSettings, structs []Struct) []Query {
	qs := make([]Query, 0, len(r.Queries))
	for _, query := range r.Queries {
		if query.Name == "" {
			continue
		}
		if query.Cmd == "" {
			continue
		}

		gq := Query{
			Cmd:          query.Cmd,
			ConstantName: codegen.LowerTitle(query.Name),
			FieldName:    codegen.LowerTitle(query.Name) + "Stmt",
			MethodName:   query.Name,
			SourceName:   query.Filename,
			SQL:          query.SQL,
			Comments:     query.Comments,
		}

		util.Xiazeminlog("query", query, false)

		if len(query.Params) == 1 {
			p := query.Params[0]
			gq.Arg = QueryValue{
				Name:      paramName(p),
				Typ:       goType(r, p.Column, settings),
				IsSlice:   isSlice(p.Column),
				NameSpace: settings.Go.Package,
			}
		} else if len(query.Params) > 1 {
			var cols []goColumn
			for _, p := range query.Params {
				cols = append(cols, goColumn{
					id:      p.Number,
					Column:  p.Column,
					IsSlice: isSlice(p.Column),
				})
			}
			gq.Arg = QueryValue{
				Emit:      true,
				Name:      "arg",
				Struct:    columnsToStruct(r, gq.MethodName+"Params", cols, settings), //@TODO xiazemin 数组一会儿处理
				NameSpace: settings.Go.Package,
			}
		}

		if len(query.Columns) == 1 {
			c := query.Columns[0]
			gq.Ret = QueryValue{
				Name:      columnName(c, 0),
				Typ:       goType(r, c, settings), //获取类型从这里进入
				IsSlice:   isSlice(c),
				NameSpace: settings.Go.Package,
			}
		} else if len(query.Columns) > 1 {
			var gs *Struct
			var emit bool

			for _, s := range structs {
				if len(s.Fields) != len(query.Columns) {
					continue
				}
				same := true
				for i, f := range s.Fields {
					c := query.Columns[i]
					sameName := f.Name == StructName(columnName(c, i), settings)
					sameType := f.Type == goType(r, c, settings)
					sameTable := sameTableName(c.Table, s.Table, r.Catalog.DefaultSchema)
					if !sameName || !sameType || !sameTable {
						same = false
					}
				}
				if same {
					gs = &s
					break
				}
			}

			if gs == nil {
				var columns []goColumn
				for i, c := range query.Columns {
					columns = append(columns, goColumn{
						id:      i,
						Column:  c,
						IsSlice: isSlice(c),
					})
				}
				gs = columnsToStruct(r, gq.MethodName+"Row", columns, settings)
				emit = true
			}
			gq.Ret = QueryValue{
				Emit:      emit,
				Name:      "i",
				Struct:    gs,
				NameSpace: settings.Go.Package,
			}
		}
		util.Xiazeminlog(" result gq", gq, false)
		qs = append(qs, gq)
	}
	sort.Slice(qs, func(i, j int) bool { return qs[i].MethodName < qs[j].MethodName })
	return qs
}

func isSlice(col *compiler.Column) bool {
	if col == nil {
		return false
	}
	return col.IsSlice
}

// It's possible that this method will generate duplicate JSON tag values
//
//   Columns: count, count,   count_2
//    Fields: Count, Count_2, Count2
// JSON tags: count, count_2, count_2
//
// This is unlikely to happen, so don't fix it yet
func columnsToStruct(r *compiler.Result, name string, columns []goColumn, settings config.CombinedSettings) *Struct {
	gs := Struct{
		Name: name,
	}
	seen := map[string]int{}
	suffixes := map[int]int{}
	for i, c := range columns {
		colName := columnName(c.Column, i)
		tagName := colName
		fieldName := StructName(colName, settings)
		// Track suffixes by the ID of the column, so that columns referring to the same numbered parameter can be
		// reused.
		suffix := 0
		if o, ok := suffixes[c.id]; ok {
			suffix = o
		} else if v := seen[colName]; v > 0 {
			suffix = v + 1
		}
		suffixes[c.id] = suffix
		if suffix > 0 {
			tagName = fmt.Sprintf("%s_%d", tagName, suffix)
			fieldName = fmt.Sprintf("%s_%d", fieldName, suffix)
		}
		tags := map[string]string{}
		if settings.Go.EmitDBTags {
			tags["db:"] = tagName
		}
		if settings.Go.EmitJSONTags {
			tags["json:"] = tagName
		}
		gs.Fields = append(gs.Fields, Field{
			Name:    fieldName,
			Type:    goType(r, c.Column, settings),
			Tags:    tags,
			IsSlice: c.IsSlice,
		})
		seen[colName]++
	}
	return &gs
}
