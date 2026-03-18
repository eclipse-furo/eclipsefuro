package generator

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/compiler/protogen"
)

type ServiceType struct {
	Name            string
	Methods         []ServiceMethods
	LeadingComments []string
	Package         string
}

type ServiceMethods struct {
	Name                string // GetList
	RequestTypeLiteral  string // IFuroCubeCubeServiceGetRequest
	ResponseTypeLiteral string // IFuroCubeCubeServiceGetResponse
	RequestType         string // FuroCubeCubeServiceGetRequest
	ResponseType        string // FuroCubeCubeServiceGetResponse
	Verb                string // GET
	Path                string // /v1/cubes/{cube_id}
	Body                string // "" | "*" | "data"
	LeadingComments     []string
	TrailingComment     string
	CleintStreaming     bool
	ServerStreaming     bool
}

var ServiceTemplate = `/**{{if .LeadingComments}}{{range $i, $commentLine := .LeadingComments}}
 *{{$commentLine}}{{end}}{{end}}
 **/
export class {{.Name}} {
{{range $i, $method := .Methods}}
  /**{{if .LeadingComments}}{{range $i, $commentLine := .LeadingComments}}
   * {{$commentLine}}{{end}}{{end}}
   */
  public {{.Name}}: StrictFetcher<{{.RequestTypeLiteral}},{{if .ServerStreaming}}AsyncIterable<{{.ResponseTypeLiteral}}> | {{end}}{{.ResponseTypeLiteral}}> = new StrictFetcher<{{.RequestTypeLiteral}},{{if .ServerStreaming}}AsyncIterable<{{.ResponseTypeLiteral}}> | {{end}}{{.ResponseTypeLiteral}}>(
    API_OPTIONS,
    '{{.Verb}}',
    '{{.Path}}',
     {{.RequestType}},
     {{.ResponseType}}{{if .Body}},
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

func prepareServiceType(service *protogen.Service, imports ImportMap) ServiceType {

	imports.AddImport("@furo/open-models/dist/StrictFetcher", "StrictFetcher", "")

	pkg := string(service.Desc.ParentFile().Package())

	// todo: implement fallback, when package is not set
	pathSegments := strings.Split(pkg, ".")
	for i := range pathSegments {
		pathSegments[i] = ".."
	}

	imports.AddImport(strings.Join(pathSegments, "/")+"/API_OPTIONS", "API_OPTIONS", "")

	serviceType := ServiceType{
		Name:            strcase.ToCamel(string(service.Desc.Name())),
		Methods:         make([]ServiceMethods, 0, len(service.Methods)),
		LeadingComments: multilineComment(string(service.Comments.Leading)),
		Package:         pkg,
	}

	for _, method := range service.Methods {

		apiOptions, _ := ExtractAPIOptions(method)
		verb, path, err := extractPathAndPattern(apiOptions)
		// on err, we have no REST endpoints
		if err == nil {
			body := ""
			if apiOptions != nil {
				body = cleanFieldName(apiOptions.GetBody())
			}
			serviceMethods := ServiceMethods{
				Name:                PrefixReservedWords(string(method.Desc.Name())),
				RequestTypeLiteral:  resolveServiceTypeLiteral("."+string(method.Input.Desc.FullName()), service, imports),
				ResponseTypeLiteral: resolveServiceTypeLiteral("."+string(method.Output.Desc.FullName()), service, imports),
				RequestType:         resolveServiceType("."+string(method.Input.Desc.FullName()), service, imports),
				ResponseType:        resolveServiceType("."+string(method.Output.Desc.FullName()), service, imports),
				Verb:                verb,
				Path:                path,
				Body:                body,
				LeadingComments:     multilineComment(string(method.Comments.Leading)),
				TrailingComment:     string(method.Comments.Trailing),
				CleintStreaming:     method.Desc.IsStreamingClient(),
				ServerStreaming:     method.Desc.IsStreamingServer(),
			}

			serviceType.Methods = append(serviceType.Methods, serviceMethods)
		}
	}

	return serviceType
}

func resolveServiceTypeLiteral(typeName string, service *protogen.Service, imports ImportMap) string {
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
		return primitiveType
	}

	pkg := string(service.Desc.ParentFile().Package())

	// regular message type
	classNameIn := messageName(allTypes[typeName])
	fieldPackage := strings.Split("."+pkg, ".")
	rel, _ := filepath.Rel(strings.Join(fieldPackage, "/"), "/"+typenameToPath(typeName))
	if !strings.HasPrefix(rel, "..") {
		rel = "./" + rel
	}
	imports.AddImport(rel, "type I"+PrefixReservedWords(classNameIn), "I"+fullQualifiedTypeName(typeName))
	return "I" + fullQualifiedTypeName(typeName)
}

func resolveServiceType(typeName string, service *protogen.Service, imports ImportMap) string {
	// WELL KNOWN

	if isWellKnownType(typeName) {
		ts := strings.Split(typeName, ".")
		name := ts[len(ts)-1]

		// ANY
		if name == "Any" {
			imports.AddImport("@furo/open-models/dist/index", "ANY", "")
			return "ANY"
		}

		// Empty
		if name == "Empty" {
			return "Record<string, never>"
		}

		primitiveType := WellKnownTypesMap[name]
		return primitiveType
	}

	pkg := string(service.Desc.ParentFile().Package())

	// regular message type
	classNameIn := messageName(allTypes[typeName])
	fieldPackage := strings.Split("."+pkg, ".")
	rel, _ := filepath.Rel(strings.Join(fieldPackage, "/"), "/"+typenameToPath(typeName))
	if !strings.HasPrefix(rel, "..") {
		rel = "./" + rel
	}
	imports.AddImport(rel, PrefixReservedWords(classNameIn), fullQualifiedTypeName(typeName))
	return fullQualifiedTypeName(typeName)
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
