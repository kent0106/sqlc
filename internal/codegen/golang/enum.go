package golang

import (
	"regexp"
	"strings"

	pinyin "github.com/mozillazg/go-pinyin"
)

var IdentPattern = regexp.MustCompile("[^a-zA-Z0-9_]+")

type Constant struct {
	Name  string
	Type  string
	Value string
}

type Enum struct {
	Name      string
	Comment   string
	Constants []Constant
}

var reHan = regexp.MustCompile("[\u4E00-\u9FFF]+")

func EnumReplace(value string) string {
	id := strings.Replace(value, "-", "_", -1)
	id = strings.Replace(id, ":", "_", -1)
	id = strings.Replace(id, "/", "_", -1)
	if reHan.Match([]byte(value)) {
		matches := reHan.FindAllStringIndex(value, -1)
		val1 := ""
		last := 0
		for _, m := range matches {
			results := pinyin.LazyConvert(value[m[0]:m[1]], nil)
			val1 = val1 + value[last:m[0]] + strings.Join(results, "_")
			last = m[1]
		}
		id = val1
	}
	return IdentPattern.ReplaceAllString(id, "")
}

func EnumValueName(value string) string {
	name := ""
	id := strings.Replace(value, "-", "_", -1)
	id = strings.Replace(id, ":", "_", -1)
	id = strings.Replace(id, "/", "_", -1)
	id = IdentPattern.ReplaceAllString(id, "")
	for _, part := range strings.Split(id, "_") {
		name += strings.Title(part)
	}
	return name
}
