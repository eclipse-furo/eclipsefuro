package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/bufbuild/protoplugin"
	"github.com/eclipse-furo/eclipsefuro/protoc-gen-open-models/pkg/sourceinfo"
	openapi_v3 "github.com/google/gnostic/openapiv3"
	"github.com/iancoleman/strcase"
	"slices"
	"strings"
	"text/template"
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

var ModelTypeTemplate = `/**
 * {{.Name}} {{if .LeadingComments}}{{range $i, $commentLine := .LeadingComments}}
 * {{$commentLine}}{{end}}{{end}}
 */
export class {{.Name}} extends FieldNode {
{{range .Fields}}
  {{if .LeadingComments}}{{range $i, $commentLine := .LeadingComments}}
  // {{$commentLine}}{{end}}{{end}}
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

    this.__meta.nodeFields = [
{{- $first := true }}{{range .Fields}}{{if not $first}}, {{else}}{{$first = false}}{{end}}
      {
        fieldName: '{{.FieldName}}',
        protoName: '{{.FieldProtoName}}',
        FieldConstructor: {{.FieldConstructor}},
        {{- if ne .MAPValueConstructor ""}}
        ValueConstructor: {{.MAPValueConstructor}},{{end}}{{if .Constraints}}
        constraints: {{.Constraints}},{{end}}
      }{{end}}
    ];

    // Initialize the fields
{{- range .Fields}}{{if .LeadingComments}}{{range $i, $commentLine := .LeadingComments}}
    // {{$commentLine}}{{end}}{{end}}
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
{{if .LeadingComments}}{{range $i, $commentLine := .LeadingComments}}
  // {{$commentLine}}{{end}}{{end}}
  public get {{.FieldName}}(): {{.GetterType}} {
    return this._{{.FieldName}};
  }

  public set {{.FieldName}}(v: {{.SetterType}}) {
    this.{{.SetterCommand}}(this._{{.FieldName}}, v);
  }
{{end}}

  fromLiteral(data: I{{.Name}}) {
    super.__fromLiteral(data);
  }

  toLiteral(): I{{.Name}} {
    return super.__toLiteral();
  }
}

Registry.register('{{.MetaTypeName}}', {{.Name}});
`

func (r *ModelType) Render() string {

	t, err := template.New("ModelType").Parse(ModelTypeTemplate)
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

func prepareModelType(message *sourceinfo.MessageInfo, imports ImportMap, si sourceinfo.SourceInfo, request protoplugin.Request) ModelType {
	reqFields := []string{}

	readonlyFields := []string{}

	modelType := ModelType{
		Name:            PrefixReservedWords(strcase.ToCamel(message.Name)),
		Fields:          nil,
		LeadingComments: multilineComment(message.Info.GetLeadingComments()),
		MetaTypeName:    message.Package + "." + message.Name,
	}

	defaultValuesMap := map[string]string{}

	for _, field := range message.FieldInfos {
		// check if field is in required list
		if message.OpenApiSchema != nil {
			if slices.Contains(message.OpenApiSchema.Required, field.Name) {
				reqFields = append(reqFields, field.Field.GetJsonName())
			}
		}

		enumDefault := ""
		if field.Field.Type.String() == "TYPE_ENUM" {
			enumDefault = resolveFirstEnumOptionForField(field, si, request)
		}
		m, sc, st, gt, mapValueConstructor, fc := resolveModelType(imports, field)
		var constraints string
		fieldConstraints := FieldConstraints{}
		if field.OpenApiProperties != nil {
			// check for readonly fields
			if field.OpenApiProperties.ReadOnly {
				readonlyFields = append(readonlyFields, field.Field.GetJsonName())
			}
			if field.OpenApiProperties.Default != nil {
				// collect the defaults

				switch d := field.OpenApiProperties.Default.Oneof.(type) {
				case *openapi_v3.DefaultType_String_:
					// check for json object or array
					if (strings.Contains(d.String_, "[") || strings.Contains(d.String_, "{") || d.String_ == "null") && json.Valid([]byte(d.String_)) {
						defaultValuesMap[field.Field.GetJsonName()] = d.String_
					} else {
						defaultValuesMap[field.Field.GetJsonName()] = "\"" + d.String_ + "\""
					}

					break
				case *openapi_v3.DefaultType_Number:
					defaultValuesMap[field.Field.GetJsonName()] = fmt.Sprintf("%f", d.Number)
					break
				case *openapi_v3.DefaultType_Boolean:
					if d.Boolean {
						defaultValuesMap[field.Field.GetJsonName()] = "true"
					} else {
						defaultValuesMap[field.Field.GetJsonName()] = "false"
					}

				}
				// do not put the defaults in to the constraints
				field.OpenApiProperties.Default = nil
			}

			c, err := json.Marshal(field.OpenApiProperties)
			if err == nil {
				json.Unmarshal(c, &fieldConstraints)
				if message.OpenApiSchema != nil {
					if slices.Contains(message.OpenApiSchema.Required, field.Name) {
						fieldConstraints.Required = true
					}
				}
			}

		} else {
			// check for required constraints only, when no other constraints are given
			if message.OpenApiSchema != nil {
				if slices.Contains(message.OpenApiSchema.Required, field.Name) {
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
			LeadingComments:     multilineComment(field.Info.GetLeadingComments()),
			TrailingComment:     field.Info.GetTrailingComments(),
			FieldName:           field.Field.GetJsonName(), // todo: check preserve proto names
			FieldProtoName:      field.Field.GetName(),     // todo: check  preserve proto names
			ModelType:           m,
			SetterCommand:       sc,
			SetterType:          st,
			GetterType:          gt,
			Kind:                field.Field.Type.String(),
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

func resolveFirstEnumOptionForField(field sourceinfo.FieldInfo, si sourceinfo.SourceInfo, request protoplugin.Request) string {
	field.Field.Label.String()
	var enumSi []sourceinfo.EnumInfo
	// find the correct descriptor
Exit:
	for _, proto := range request.AllFileDescriptorProtos() {
		for _, descriptorProto := range proto.EnumType {
			if "."+proto.GetPackage()+"."+descriptorProto.GetName() == field.Field.GetTypeName() {
				enumSi = sourceinfo.GetSourceInfo(proto).Enums
				continue Exit
			}

		}
	}

	for _, info := range enumSi {
		if "."+field.Package+"."+info.Name == field.Field.GetTypeName() {
			return info.ValuesInfo[0].Name
		}
	}

	if field.Field.DefaultValue != nil {
		return *field.Field.DefaultValue
	}

	// field.Package           // google.protobuf
	// field.Message.GetName() // FieldDescriptorProto
	fqn := *field.Field.TypeName // .google.protobuf.FieldDescriptorProto.Type
	ss := strings.Split(fqn, ".")
	enumName := ss[len(ss)-1]

	for _, enumType := range field.Message.EnumType {
		if enumType.GetName() == enumName {
			return enumType.Value[0].GetName()
		}
	}

	if enum, ok := allEnums[fqn]; ok {
		return enum.Message.Value[0].GetName()
	}

	return "/ ModelType.go line 327"
}
