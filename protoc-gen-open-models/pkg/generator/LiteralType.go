package generator

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
	"google.golang.org/protobuf/compiler/protogen"
)

type LiteralType struct {
	Name            string
	Fields          []LiteralFields
	LeadingComments []string
}

type LiteralFields struct {
	LeadingComments []string
	TrailingComment string
	FieldName       string // name of the field in interface types and models
	FieldProtoName  string // name of the field in transport types
	Type            string
}

var LiteralTypeTemplate = `/**
 * @interface I{{.Name}} {{if .LeadingComments}}{{range $i, $commentLine := .LeadingComments}}
 * {{$commentLine}}{{end}}{{end}}
 */
export interface I{{.Name}} {
{{- range .Fields}}{{if .LeadingComments}}
    /**{{range $i, $commentLine := .LeadingComments}}
     * {{$commentLine}} {{end}}
     */{{end}}
    {{.FieldName}}?: {{.Type}};{{if .TrailingComment}} // {{.TrailingComment}}{{end}}{{end}}
}
`

func (r *LiteralType) Render() string {

	t, err := template.New("LiteralType").Parse(LiteralTypeTemplate)
	if err != nil {
		panic(err)
	}

	var res bytes.Buffer
	err = t.Execute(&res, r)
	if err != nil {
		panic(err)
	}
	return res.String()
}

func prepareLiteralType(msg *protogen.Message, imports ImportMap) LiteralType {
	name := messageName(msg)
	literalType := LiteralType{
		Name:            PrefixReservedWords(strcase.ToCamel(name)),
		Fields:          nil,
		LeadingComments: multilineComment(string(msg.Comments.Leading)),
	}
	for _, field := range msg.Fields {
		literalType.Fields = append(literalType.Fields, LiteralFields{
			LeadingComments: multilineComment(string(field.Comments.Leading)),
			TrailingComment: string(field.Comments.Trailing),
			FieldName:       field.Desc.JSONName(),
			FieldProtoName:  string(field.Desc.Name()),
			Type:            resolveInterfaceType(imports, field, "I"),
		})
	}
	return literalType
}

func fullQualifiedName(pkg string, name string) string {
	p := []string{}
	for _, s := range strings.Split(pkg, ".") {
		p = append(p, strings.ToUpper(s[:1])+s[1:])
	}

	return strcase.ToCamel(strings.Join(p, "")) + name
}

func dotToCamel(name string) string {
	p := []string{}
	for _, s := range strings.Split(name, ".") {
		p = append(p, strings.ToUpper(s[:1])+s[1:])
	}
	return strcase.ToCamel(strings.Join(p, ""))
}
