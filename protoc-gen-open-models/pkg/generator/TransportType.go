package generator

import (
	"bytes"
	"text/template"

	"github.com/iancoleman/strcase"
	"google.golang.org/protobuf/compiler/protogen"
)

type TransportType struct {
	Name            string
	Fields          []TransportFields
	LeadingComments []string
}

type TransportFields struct {
	LeadingComments []string
	TrailingComment string
	FieldName       string // name of the field in interface types and models
	FieldProtoName  string // name of the field in transport types
	Type            string
}

func (r *TransportType) Render() string {

	t, err := template.New("TransportType").Parse(TransportTypeTemplate)
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

var TransportTypeTemplate = `/**
 * @interface T{{.Name}} {{if .LeadingComments}}{{range $i, $commentLine := .LeadingComments}}
 * {{$commentLine}}{{end}}{{end}}
 */
export interface T{{.Name}} {
{{- range .Fields}}{{if .LeadingComments}}
/**{{range $i, $commentLine := .LeadingComments}}
 * {{$commentLine}} {{end}}
 */{{end}}
    {{.FieldProtoName}}?: {{.Type}};{{if .TrailingComment}} // {{.TrailingComment}}{{end}}{{end}}
}
`

func prepareTransportType(msg *protogen.Message, imports ImportMap) TransportType {
	name := messageName(msg)
	transportType := TransportType{
		Name:            PrefixReservedWords(strcase.ToCamel(name)),
		Fields:          nil,
		LeadingComments: multilineComment(string(msg.Comments.Leading)),
	}
	for _, field := range msg.Fields {
		transportType.Fields = append(transportType.Fields, TransportFields{
			LeadingComments: multilineComment(string(field.Comments.Leading)),
			TrailingComment: string(field.Comments.Trailing),
			FieldName:       field.Desc.JSONName(),
			FieldProtoName:  string(field.Desc.Name()),
			Type:            resolveInterfaceType(imports, field, "T"),
		})
	}
	return transportType
}
