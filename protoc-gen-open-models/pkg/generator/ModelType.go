package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"text/template"

	openapi_v3 "github.com/google/gnostic/openapiv3"
	"github.com/iancoleman/strcase"
	"google.golang.org/protobuf/compiler/protogen"
)

type ModelType struct {
	Name            string
	Fields          []ModelFields
	LeadingComments []string
	MetaTypeName    string
	RequiredFields  string
	ReadonlyFields  string
	DefaultValues   *map[string]string // deviated from openapi_v3.DefaultType
}

type ModelFields struct {
	LeadingComments     []string
	TrailingComment     string
	FieldName           string // name of the field in interface types and models
	FieldProtoName      string // name of the field in transport types
	ModelType           string // like STRING, ENUM<SomeTHING>, Int32Value, FuroTypeMessage
	SetterCommand       string // for primitive or typesetter
	SetterType          string // string or LFuroTypeMessage
	GetterType          string // string or FuroTypeMessage
	Kind                string // proto field type like TYPE_MESSAGE
	EnumDefault         string // Name of the first enum option
	MAPValueConstructor string // value constructor for MAP
	FieldConstructor    string // constructor for the field / usually it is the same as ModelType
	Constraints         string // Openapi Constraints as json literal

}

type FieldConstraints struct {
	Nullable         bool    `json:"nullable,omitempty"`
	ReadOnly         bool    `json:"read_only,omitempty"`
	WriteOnly        bool    `json:"write_only,omitempty"`
	Deprecated       bool    `json:"deprecated,omitempty"`
	Title            string  `json:"title,omitempty"`
	Maximum          float64 `json:"maximum,omitempty"`
	Minimum          float64 `json:"minimum,omitempty"`
	ExclusiveMaximum bool    `json:"exclusive_maximum,omitempty"`
	ExclusiveMinimum bool    `json:"exclusive_minimum,omitempty"`
	MultipleOf       float64 `json:"multiple_of,omitempty"`
	MaxLength        int64   `json:"max_length,omitempty"`
	MinLength        int64   `json:"min_length,omitempty"`
	Pattern          string  `json:"pattern,omitempty"`
	MaxItems         int64   `json:"max_items,omitempty"`
	MinItems         int64   `json:"min_items,omitempty"`
	UniqueItems      bool    `json:"unique_items,omitempty"`
	MaxProperties    int64   `json:"max_properties,omitempty"`
	MinProperties    int64   `json:"min_properties,omitempty"`
	Type             string  `json:"type,omitempty"`
	Description      string  `json:"description,omitempty"`
	Format           string  `json:"format,omitempty"`
	Required         bool    `json:"required,omitempty"`
}

func buildDescriptionField(lines []string) string {
	s := strings.Join(lines, "\n")

	// Normalize newlines first
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "")

	// Escape backslash first to avoid double-escaping
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, "\n", `\n`)

	return strings.TrimSpace(s)
}

var ModelTypeTemplate = `/**
 * {{.Name}} {{if .LeadingComments}}{{range $i, $commentLine := .LeadingComments}}
 * {{$commentLine}}{{end}}{{end}}
 */
export class {{.Name}} extends FieldNode {
{{range .Fields}}
  /**{{if .LeadingComments}}{{range $i, $commentLine := .LeadingComments}}
   *{{$commentLine}}{{end}}{{end}}
   **/
  private _{{.FieldName}}: {{.ModelType}};{{if .TrailingComment}} // {{.TrailingComment}}{{end}}
{{end}}
 public __defaultValues: I{{.Name}};

  constructor(
    initData?: I{{.Name}},
    parent?: FieldNode,
    parentAttributeName?: string,
  ) {
    super(undefined, parent, parentAttributeName);
    this.__meta.typeName = '{{.MetaTypeName}}';
    this.__meta.description = '{{.Name}}{{if .LeadingComments}} {{desc .LeadingComments}}{{end}}';

    this.__meta.nodeFields = [
{{- $first := true }}{{range .Fields}}{{if not $first}}, {{else}}{{$first = false}}{{end}}
      {
        fieldName: '{{.FieldName}}',
        protoName: '{{.FieldProtoName}}',
        FieldConstructor: {{.FieldConstructor}},
        {{- if ne .MAPValueConstructor ""}}
        ValueConstructor: {{.MAPValueConstructor}},{{end}}{{if .Constraints}}
        constraints: {{.Constraints}},{{end}}
        description: '{{desc .LeadingComments}}'
      }{{end}}
    ];

    // Initialize the fields
    // ---------------------
{{ range .Fields}}
   /**{{if .LeadingComments}}{{range $i, $commentLine := .LeadingComments}}
    * {{$commentLine}}{{end}}{{end}}
    **/
    this._{{.FieldName}} = new {{.ModelType}}(undefined,{{if eq .Kind "TYPE_ENUM"}}{{.SetterType}}, {{.SetterType}}.{{.EnumDefault}}, {{end}} this, '{{.FieldName}}');
{{end}}


    // Set required fields
    [{{.RequiredFields}}].forEach(fieldName => {
      (
        this[fieldName as keyof {{.Name}}] as FieldNode
      ).__meta.required = true;
    });


    // Default values from openAPI annotations
    this.__defaultValues = {
	{{- if .DefaultValues}}{{range $fn, $value := .DefaultValues}}
      {{$fn}}:{{$value}},{{end}}{{end}}
    };

    // Initialize the fields with init data
    if (initData !== undefined) {
      this.__fromLiteral({ ...this.__defaultValues, ...initData });
    } else {
      this.__fromLiteral(this.__defaultValues);
    }

    // Set readonly fields after the init, so child nodes are readonly too
    [{{.ReadonlyFields}}].forEach(fieldName => {
      (
        this[fieldName as keyof {{.Name}}] as FieldNode
      ).__readonly = true;
    });

    this.__meta.isPristine = true;
  }
{{range .Fields}}
  /**{{if .LeadingComments}}{{range $i, $commentLine := .LeadingComments}}
   * {{$commentLine}}{{end}}{{end}}
   * The getter receives the FieldNode
   **/
  public get {{.FieldName}}(): {{.GetterType}} {
    return this._{{.FieldName}};
  }

  /**
   * The setter receives {{.SetterType | bt}}
   **/
  public set {{.FieldName}}(v: {{.SetterType}}) {
    this.{{.SetterCommand}}(this._{{.FieldName}}, v);
  }
{{end}}

  fromLiteral(data: I{{.Name}}): void {
    super.__fromLiteral(data);
  }

  toLiteral(): I{{.Name}} {
    return super.__toLiteral() as I{{.Name}};
  }
}

Registry.register('{{.MetaTypeName}}', {{.Name}});
`

func (r *ModelType) Render() string {

	t, err := template.New("ModelType").
		Funcs(template.FuncMap{
			"desc": buildDescriptionField,
			"bt":   func(s string) string { return "`" + s + "`" },
		}).
		Parse(ModelTypeTemplate)
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

func prepareModelType(msg *protogen.Message, imports ImportMap, openApiSchema *openapi_v3.Schema) ModelType {
	reqFields := []string{}
	readonlyFields := []string{}
	name := messageName(msg)
	pkg := string(msg.Desc.ParentFile().Package())

	modelType := ModelType{
		Name:            PrefixReservedWords(strcase.ToCamel(name)),
		Fields:          nil,
		LeadingComments: multilineComment(string(msg.Comments.Leading)),
		MetaTypeName:    pkg + "." + name,
	}

	defaultValuesMap := map[string]string{}

	for _, field := range msg.Fields {
		fieldName := string(field.Desc.Name())
		jsonName := field.Desc.JSONName()
		fieldType := kindToTypeString[field.Desc.Kind()]

		// check if field is in required list
		if openApiSchema != nil {
			if slices.Contains(openApiSchema.Required, fieldName) {
				reqFields = append(reqFields, jsonName)
			}
		}

		enumDefault := ""
		if fieldType == "TYPE_ENUM" {
			enumDefault = resolveFirstEnumOptionForField(field)
		}
		m, sc, st, gt, mapValueConstructor, fc := resolveModelType(imports, field)

		openApiProps, _ := ExtractOpenApiFieldOptions(field)

		var constraints string
		fieldConstraints := FieldConstraints{}
		if openApiProps != nil {
			// check for readonly fields
			if openApiProps.ReadOnly {
				readonlyFields = append(readonlyFields, jsonName)
			}
			if openApiProps.Default != nil {
				// collect the defaults
				switch d := openApiProps.Default.Oneof.(type) {
				case *openapi_v3.DefaultType_String_:
					if (strings.Contains(d.String_, "[") || strings.Contains(d.String_, "{") || d.String_ == "null") && json.Valid([]byte(d.String_)) {
						defaultValuesMap[jsonName] = d.String_
					} else {
						defaultValuesMap[jsonName] = "\"" + d.String_ + "\""
					}

					break
				case *openapi_v3.DefaultType_Number:
					defaultValuesMap[jsonName] = fmt.Sprintf("%f", d.Number)
					break
				case *openapi_v3.DefaultType_Boolean:
					if d.Boolean {
						defaultValuesMap[jsonName] = "true"
					} else {
						defaultValuesMap[jsonName] = "false"
					}

				}
				// do not put the defaults in to the constraints
				openApiProps.Default = nil
			}

			c, err := json.Marshal(openApiProps)
			if err == nil {
				json.Unmarshal(c, &fieldConstraints)
				if openApiSchema != nil {
					if slices.Contains(openApiSchema.Required, fieldName) {
						fieldConstraints.Required = true
					}
				}
			}

		} else {
			// check for required constraints only, when no other constraints are given
			if openApiSchema != nil {
				if slices.Contains(openApiSchema.Required, fieldName) {
					fieldConstraints.Required = true
				}
			}
		}
		fieldConstraintsJson, _ := json.Marshal(fieldConstraints)
		constraints = string(fieldConstraintsJson)

		if len(defaultValuesMap) > 0 {
			modelType.DefaultValues = &defaultValuesMap
		}

		modelType.Fields = append(modelType.Fields, ModelFields{
			LeadingComments:     multilineComment(string(field.Comments.Leading)),
			TrailingComment:     string(field.Comments.Trailing),
			FieldName:           jsonName,
			FieldProtoName:      fieldName,
			ModelType:           m,
			SetterCommand:       sc,
			SetterType:          st,
			GetterType:          gt,
			Kind:                fieldType,
			EnumDefault:         enumDefault,
			MAPValueConstructor: mapValueConstructor,
			FieldConstructor:    fc,
			Constraints:         constraints,
		})
	}

	if len(reqFields) > 0 {
		modelType.RequiredFields = "'" + strings.Join(reqFields, "', '") + "'"
	}

	if len(readonlyFields) > 0 {
		modelType.ReadonlyFields = "'" + strings.Join(readonlyFields, "', '") + "'"
	}

	return modelType
}

func resolveFirstEnumOptionForField(field *protogen.Field) string {
	if field.Enum != nil && len(field.Enum.Values) > 0 {
		return string(field.Enum.Values[0].Desc.Name())
	}
	return ""
}
