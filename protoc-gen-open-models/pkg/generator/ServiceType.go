package generator

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/eclipse-furo/eclipsefuro/protoc-gen-open-models/pkg/sourceinfo"
	"github.com/iancoleman/strcase"
	"google.golang.org/genproto/googleapis/api/annotations"
)

type ServiceType struct {
	Name            string
	Methods         []ServiceMethods
	LeadingComments []string
	Package         string
}

type ServiceMethods struct {
	Name                string // GetList
	RequestTypeLiteral  string //LFuroCubeCubeServiceGetRequest
	ResponseTypeLiteral string // LFuroCubeCubeServiceGetResponse
	Verb                string // GET
	Path                string // /v1/cubes/{cube_id}
	Body                string // "" | "*" | "data"
	LeadingComments     []string
	TrailingComment     string
	CleintStreaming     bool
	ServerStreaming     bool
}

var ServiceTemplate = `{{if .LeadingComments}}{{range $i, $commentLine := .LeadingComments}}
// {{$commentLine}}{{end}}{{end}}

export class {{.Name}} {

{{range $i, $method := .Methods}}{{if .LeadingComments}}{{range $i, $commentLine := .LeadingComments}}
  // {{$commentLine}}{{end}}{{end}}
  public {{.Name}}: Fetcher<{{.RequestTypeLiteral}},{{if .ServerStreaming}}AsyncIterable<{{.ResponseTypeLiteral}}> | {{end}}{{.ResponseTypeLiteral}}> = new Fetcher<{{.RequestTypeLiteral}},{{if .ServerStreaming}}AsyncIterable<{{.ResponseTypeLiteral}}> | {{end}}{{.ResponseTypeLiteral}}>(
    API_OPTIONS,
    '{{.Verb}}',
    '{{.Path}}',{{if .Body}}
    '{{.Body}}'{{end}}
  ); {{if .TrailingComment}}// {{.TrailingComment}}{{end}}
{{end}}

}
`

func (r *ServiceType) Render() string {

	t, err := template.New("ServiceType").Parse(ServiceTemplate)
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

func prepareServiceType(service sourceinfo.ServiceInfo, imports ImportMap) ServiceType {

	imports.AddImport("@furo/open-models/dist/Fetcher", "Fetcher", "")

	// todo: implement fallback, when package is not set
	pathSegments := strings.Split(service.Package, ".")
	for i, _ := range pathSegments {
		pathSegments[i] = ".."
	}

	imports.AddImport(strings.Join(pathSegments, "/")+"/API_OPTIONS", "API_OPTIONS", "")

	serviceType := ServiceType{
		Name:            strcase.ToCamel(service.Name),
		Methods:         make([]ServiceMethods, 0, len(service.Methods)),
		LeadingComments: multilineComment(service.Info.GetLeadingComments()),
		Package:         service.Package,
	}

	for _, method := range service.Methods {

		verb, path, err := extractPathAndPattern(method.HttpRule.ApiOptions)
		// on err, we have no REST endpoints
		if err == nil {
			serviceMethods := ServiceMethods{
				Name:                PrefixReservedWords(method.Name),
				RequestTypeLiteral:  resolveServiceType(method.Method.GetInputType(), service, imports),
				ResponseTypeLiteral: resolveServiceType(method.Method.GetOutputType(), service, imports),
				Verb:                verb,
				Path:                path,
				Body:                cleanFieldName(method.HttpRule.ApiOptions.GetBody()),
				LeadingComments:     multilineComment(method.Info.GetLeadingComments()),
				TrailingComment:     method.Info.GetTrailingComments(),
				CleintStreaming:     method.Method.ClientStreaming != nil,
				ServerStreaming:     method.Method.ServerStreaming != nil,
			}

			serviceType.Methods = append(serviceType.Methods, serviceMethods)
		}
	}

	return serviceType
}

func resolveServiceType(typeName string, service sourceinfo.ServiceInfo, imports ImportMap) string {
	// WELL KNOWN

	if isWellKnownType(typeName) {
		ts := strings.Split(typeName, ".")
		name := ts[len(ts)-1]

		// ANY
		if name == "Any" {
			imports.AddImport("@furo/open-models/dist/index", "type IAny", "")
			return "IAny"
		}

		// Empty
		if name == "Empty" {
			return "Record<string, never>"
		}

		primitiveType := WellKnownTypesMap[name]
		// imports.AddImport("@furo/open-models/dist/index", name, "")
		return primitiveType
	}

	// Any
	// Empty

	// regular message type
	classNameIn := allTypes[typeName].Name
	fieldPackage := strings.Split("."+service.Package, ".")
	rel, _ := filepath.Rel(strings.Join(fieldPackage, "/"), "/"+typenameToPath(typeName))
	if !strings.HasPrefix(rel, "..") {
		rel = "./" + rel
	}
	imports.AddImport(rel, "I"+PrefixReservedWords(classNameIn), "I"+fullQualifiedTypeName(typeName))
	return "I" + fullQualifiedTypeName(typeName)
}

func baseTypeName(typeName string) string {
	ts := strings.Split(typeName, ".")
	return ts[len(ts)-1]
}

func fullQualifiedTypeName(typeName string) string {
	p := []string{}
	for _, s := range strings.Split(typeName[1:], ".") {
		p = append(p, strings.ToUpper(s[:1])+s[1:])
	}
	return strings.Join(p, "")
}

func extractPathAndPattern(rule *annotations.HttpRule) (path string, pattern string, err error) {
	if rule == nil {
		return "", "", errors.New("No REST endpoint available")
	}
	switch r := rule.Pattern.(type) {
	case *annotations.HttpRule_Get:
		return "GET", r.Get, nil

	case *annotations.HttpRule_Put:
		return "PUT", r.Put, nil

	case *annotations.HttpRule_Post:
		return "POST", r.Post, nil

	case *annotations.HttpRule_Patch:
		return "PATCH", r.Patch, nil

	case *annotations.HttpRule_Delete:
		return "DELETE", r.Delete, nil

	case *annotations.HttpRule_Custom:
		return r.Custom.Kind, r.Custom.Path, nil

	}
	// should not happen
	return "", "", errors.New("No match")
}
