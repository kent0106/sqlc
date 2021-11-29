package golang

import (
	"fmt"
	"strings"

	"github.com/xiazemin/sqlc/internal/metadata"
)

type QueryValue struct {
	Emit      bool
	Name      string
	Struct    *Struct
	Typ       string
	IsSlice   bool
	Slice     []*QueryValue
	NameSpace string
}

func (v QueryValue) EmitStruct() bool {
	return v.Emit
}

func (v QueryValue) IsStruct() bool {
	return v.Struct != nil
}

func (v QueryValue) isEmpty() bool {
	return v.Typ == "" && v.Name == "" && v.Struct == nil
}

func (v QueryValue) IsSliceType() bool {
	return v.IsSlice
}

func (v QueryValue) GetDefaultValueByType() string {

	if v.Struct != nil {
		return v.Struct.Name + "{}"
	}

	if v.Typ != "" {

		switch v.Typ {
		case "int", "uint", "int8", "uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64":
			return "0"
		case "float32", "float64":
			return "0"
		case "string":
			return "\"\""
		case "bool":
			return "false"
		case "interface{}":
			return "nil"
		}
		if strings.HasPrefix(v.Typ, "sql.Null") {
			return v.Typ + "{}"
		}
		if strings.HasPrefix(v.Typ, "*") || strings.HasPrefix(v.Typ, "[]") {
			return "nil"
		}
	}
	return "nil"
}

func (v QueryValue) ContainSlice() bool {
	if v.Struct != nil {
		for _, f := range v.Struct.Fields {
			if f.IsSlice {
				return true
			}
		}
	}
	if v.IsSlice {
		return true
	}
	return false
}

type genFunctions struct {
	functions                map[string]string
	shouldGenFunctions       map[string]bool
	shouldGenFunctionsImport map[string]bool
}

var nameSpaceToGenFunctions map[string]*genFunctions = make(map[string]*genFunctions)

func getGenFunctionsByNs(namespace string) *genFunctions {
	genFunctionsInNS, ok := nameSpaceToGenFunctions[namespace]
	if !ok {
		genFunctionsInNS = &genFunctions{
			functions:                make(map[string]string),
			shouldGenFunctions:       make(map[string]bool),
			shouldGenFunctionsImport: make(map[string]bool),
		}
	}
	return genFunctionsInNS
}

func (v QueryValue) ShouldGenFunctionsImport() bool {
	result := false
	if v.ContainSlice() {

		if v.Struct != nil {
			for _, f := range v.Struct.Fields {
				if f.IsSlice {
					functionName := formatType(f.Type) + "Slice2interface"
					genFunctionsByNs := getGenFunctionsByNs(v.NameSpace)
					if _, ok := genFunctionsByNs.shouldGenFunctionsImport[functionName]; ok {
						continue
					}
					genFunctionsByNs.shouldGenFunctionsImport[functionName] = true
					nameSpaceToGenFunctions[v.NameSpace] = genFunctionsByNs
					result = true
				}
			}
		}
		if v.IsSlice {
			functionName := formatType(v.Typ) + "Slice2interface"
			genFunctionsByNs := getGenFunctionsByNs(v.NameSpace)
			if _, ok := genFunctionsByNs.shouldGenFunctionsImport[functionName]; ok {
				return false
			}
			genFunctionsByNs.shouldGenFunctionsImport[functionName] = true
			nameSpaceToGenFunctions[v.NameSpace] = genFunctionsByNs
			result = true
		}

	}
	return result
}

func (v QueryValue) ShouldGenFunctions() bool {
	result := false
	if v.ContainSlice() {

		if v.Struct != nil {
			for _, f := range v.Struct.Fields {
				if f.IsSlice {
					functionName := formatType(f.Type) + "Slice2interface"
					genFunctionsByNs := getGenFunctionsByNs(v.NameSpace)
					if _, ok := genFunctionsByNs.shouldGenFunctions[functionName]; ok {
						continue
					}
					genFunctionsByNs.shouldGenFunctions[functionName] = true
					nameSpaceToGenFunctions[v.NameSpace] = genFunctionsByNs
					result = true
				}
			}
		}
		if v.IsSlice {
			functionName := formatType(v.Typ) + "Slice2interface"
			genFunctionsByNs := getGenFunctionsByNs(v.NameSpace)
			if _, ok := genFunctionsByNs.shouldGenFunctions[functionName]; ok {
				return false
			}
			genFunctionsByNs.shouldGenFunctions[functionName] = true
			nameSpaceToGenFunctions[v.NameSpace] = genFunctionsByNs
			result = true
		}

	}
	return result
}

func (v QueryValue) GenerateFunctions() string {
	result := ""
	if v.ContainSlice() {
		template := `func %sSlice2interface(l []%s) []interface{} {
		   v := make([]interface{}, len(l))
		   for i, val := range l {
			   v[i] = val
	   
		   }
		   return v
	   }

	   `

		batchTemplate := `
		//if len(IDs)>65536,mysql will return error
		func BatchRun%s(batch int, IDs []%s, fn func([]%s) error) error {
		for i := 0; i <= len(IDs)/batch; i++ {
			l := i * batch
			r := (i + 1) * batch
			if r > len(IDs) {
				r = len(IDs)
			}
			if r > l {
				batchIDs := IDs[l:r]
				if err := fn(batchIDs); err != nil {
					return err
				}
			}
		}
		return nil
	}
	   `
		if v.Struct != nil {
			for _, f := range v.Struct.Fields {
				if f.IsSlice {
					functionName := formatType(f.Type) + "Slice2interface"

					genFunctionsByNs := getGenFunctionsByNs(v.NameSpace)
					if _, ok := genFunctionsByNs.functions[functionName]; ok {
						continue
					}
					result += fmt.Sprintf(template, formatType(f.Type), f.Type)
					result += fmt.Sprintf(batchTemplate, formatType(f.Type), f.Type, f.Type)
					genFunctionsByNs.functions[functionName] = result
					nameSpaceToGenFunctions[v.NameSpace] = genFunctionsByNs
				}
			}
		}
		if v.IsSlice {
			functionName := formatType(v.Typ) + "Slice2interface"
			genFunctionsByNs := getGenFunctionsByNs(v.NameSpace)
			if _, ok := genFunctionsByNs.functions[functionName]; ok {
				return result
			}
			result += fmt.Sprintf(template, formatType(v.Typ), v.Typ)
			result += fmt.Sprintf(batchTemplate, formatType(v.Typ), v.Typ, v.Typ)
			genFunctionsByNs.functions[functionName] = result
			nameSpaceToGenFunctions[v.NameSpace] = genFunctionsByNs
		}

	}
	return result
}

func (v QueryValue) Pair() string {
	if v.isEmpty() {
		return ""
	}
	if v.IsSlice {
		return v.Name + " []" + v.Type()
	}
	return v.Name + " " + v.Type()
}

func (v QueryValue) Type() string {
	if v.Typ != "" {
		return v.Typ
	}
	if v.Struct != nil {
		return v.Struct.Name
	}
	panic("no type for QueryValue: " + v.Name)
}

func formatType(typ string) string {
	if len(typ) > 4 && typ[:4] == "sql." {
		return typ[4:]
	}
	return typ
}

func (v QueryValue) Params() string {
	if v.isEmpty() {
		return ""
	}
	var out []string
	if v.Struct == nil {
		if strings.HasPrefix(v.Typ, "[]") && v.Typ != "[]byte" {
			out = append(out, "pq.Array("+v.Name+")")
		} else {
			if v.IsSlice {
				out = append(out, formatType(v.Typ)+"Slice2interface("+v.Name+")...")
			} else {
				out = append(out, v.Name)
			}
		}
	} else {
		if v.ContainSlice() {
			//append(append([]interface{}{arg.Bio}, int32Slice2interface(arg.ID)...), stringSlice2interface(arg.Name)...)...
			out := ""

			for _, f := range v.Struct.Fields {
				if strings.HasPrefix(f.Type, "[]") && f.Type != "[]byte" {
					out = fmt.Sprintf(out, "pq.Array("+v.Name+"."+f.Name+")")
				} else if f.IsSlice {
					sl := formatType(f.Type) + "Slice2interface(" + v.Name + "." + f.Name + ")"
					if out == "" {
						out = sl
					} else {
						out = "append(" + out + "," + sl + "...)"
					}

				} else {
					if out == "" {
						out = "[]interface{}{" + v.Name + "." + f.Name + "}"
					} else {
						out = "append(" + out + "," + v.Name + "." + f.Name + ")"
					}
				}
			}
			return out + "..."
		} else {
			for _, f := range v.Struct.Fields {
				if strings.HasPrefix(f.Type, "[]") && f.Type != "[]byte" {
					out = append(out, "pq.Array("+v.Name+"."+f.Name+")")
				} else {
					out = append(out, v.Name+"."+f.Name)
				}
			}
		}
	}
	if len(out) <= 3 {
		return strings.Join(out, ",")
	}
	out = append(out, "")
	return "\n" + strings.Join(out, ",\n")
}

func (v QueryValue) Scan() string {
	var out []string
	if v.Struct == nil {
		if strings.HasPrefix(v.Typ, "[]") && v.Typ != "[]byte" {
			out = append(out, "pq.Array(&"+v.Name+")")
		} else {
			out = append(out, "&"+v.Name)
		}
	} else {
		for _, f := range v.Struct.Fields {
			if strings.HasPrefix(f.Type, "[]") && f.Type != "[]byte" {
				out = append(out, "pq.Array(&"+v.Name+"."+f.Name+")")
			} else {
				out = append(out, "&"+v.Name+"."+f.Name)
			}
		}
	}
	if len(out) <= 3 {
		return strings.Join(out, ",")
	}
	out = append(out, "")
	return "\n" + strings.Join(out, ",\n")
}

// A struct used to generate methods and fields on the Queries struct
type Query struct {
	Cmd          string
	Comments     []string
	MethodName   string
	FieldName    string
	ConstantName string
	SQL          string
	SourceName   string
	Ret          QueryValue
	Arg          QueryValue
}

func (q Query) hasRetType() bool {
	scanned := q.Cmd == metadata.CmdOne || q.Cmd == metadata.CmdMany
	return scanned && !q.Ret.isEmpty()
}
