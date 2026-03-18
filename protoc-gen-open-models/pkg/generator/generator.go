package generator

import (
	"path"
	"path/filepath"
	"strings"

	openapi_v3 "github.com/google/gnostic/openapiv3"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var projectFiles = map[string]string{}

var allTypes = map[string]*protogen.Message{}
var allEnums = map[string]*protogen.Enum{}

// messageName returns the local name of a message relative to its file's package.
// e.g., "MyMessage" for top-level, "Outer.Inner" for nested.
func messageName(msg *protogen.Message) string {
	fullName := string(msg.Desc.FullName())
	pkg := string(msg.Desc.ParentFile().Package())
	if pkg == "" {
		return fullName
	}
	return fullName[len(pkg)+1:]
}

// enumName returns the local name of an enum relative to its file's package.
func enumName(enum *protogen.Enum) string {
	fullName := string(enum.Desc.FullName())
	pkg := string(enum.Desc.ParentFile().Package())
	if pkg == "" {
		return fullName
	}
	return fullName[len(pkg)+1:]
}

// fieldTypeName returns the fully-qualified type name with leading dot.
func fieldTypeName(field *protogen.Field) string {
	switch field.Desc.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return "." + string(field.Message.Desc.FullName())
	case protoreflect.EnumKind:
		return "." + string(field.Enum.Desc.FullName())
	default:
		return ""
	}
}

// kindToTypeString maps protoreflect.Kind to the TYPE_* string format
// used by the PrimitivesMap/ModelTypesMap/ModelPrimitivesMap lookup tables.
var kindToTypeString = map[protoreflect.Kind]string{
	protoreflect.StringKind:   "TYPE_STRING",
	protoreflect.BytesKind:    "TYPE_BYTES",
	protoreflect.BoolKind:     "TYPE_BOOL",
	protoreflect.Int32Kind:    "TYPE_INT32",
	protoreflect.Int64Kind:    "TYPE_INT64",
	protoreflect.DoubleKind:   "TYPE_DOUBLE",
	protoreflect.FloatKind:    "TYPE_FLOAT",
	protoreflect.Uint32Kind:   "TYPE_UINT32",
	protoreflect.Uint64Kind:   "TYPE_UINT64",
	protoreflect.Fixed32Kind:  "TYPE_FIXED32",
	protoreflect.Fixed64Kind:  "TYPE_FIXED64",
	protoreflect.Sfixed32Kind: "TYPE_SFIXED32",
	protoreflect.Sfixed64Kind: "TYPE_SFIXED64",
	protoreflect.Sint32Kind:   "TYPE_SINT32",
	protoreflect.Sint64Kind:   "TYPE_SINT64",
	protoreflect.MessageKind:  "TYPE_MESSAGE",
	protoreflect.EnumKind:     "TYPE_ENUM",
	protoreflect.GroupKind:    "TYPE_GROUP",
}

func GenerateAll(plugin *protogen.Plugin) {
	// Pass 1: build global registries
	for _, file := range plugin.Files {
		pkg := string(file.Desc.Package())
		pkgPath := strings.Replace(pkg, ".", "/", -1)

		// Collect top-level enums
		for _, enum := range file.Enums {
			name := enumName(enum)
			projectFiles[pkgPath+"/"+name] = "ENUM"
			allEnums["."+pkg+"."+name] = enum
		}

		// Collect top-level messages and their nested types
		for _, msg := range file.Messages {
			name := messageName(msg)
			projectFiles[pkgPath+"/"+name] = "MESSAGE"
			allTypes["."+pkg+"."+name] = msg

			// Inline enums inside this message
			for _, nestedEnum := range msg.Enums {
				eName := enumName(nestedEnum)
				projectFiles[pkgPath+"/"+eName] = "ENUM"
				allEnums["."+pkg+"."+eName] = nestedEnum
			}

			// Nested messages (1 level)
			for _, nestedMsg := range msg.Messages {
				nName := messageName(nestedMsg)
				projectFiles[pkgPath+"/"+nName] = "MESSAGE"
				allTypes["."+pkg+"."+nName] = nestedMsg

				// Nested messages (2 levels deep)
				for _, nestedNestedMsg := range nestedMsg.Messages {
					nnName := messageName(nestedNestedMsg)
					projectFiles[pkgPath+"/"+nnName] = "MESSAGE"
					allTypes["."+pkg+"."+nnName] = nestedNestedMsg
				}

				// Inline enums inside nested messages
				for _, nestedEnum := range nestedMsg.Enums {
					eName := enumName(nestedEnum)
					projectFiles[pkgPath+"/"+eName] = "ENUM"
					allEnums["."+pkg+"."+eName] = nestedEnum
				}
			}
		}

		// Collect services
		for _, service := range file.Services {
			projectFiles[pkgPath+"/"+string(service.Desc.Name())] = "SERVICE"
		}
	}

	// Pass 2: generate output files
	for _, file := range plugin.Files {
		generate(plugin, file)
	}
}

func generate(plugin *protogen.Plugin, file *protogen.File) {
	filePath := filepath.Dir(file.Desc.Path())
	pkg := string(file.Desc.Package())

	// Build enum types
	for _, enum := range file.Enums {
		name := enumName(enum)
		f := plugin.NewGeneratedFile(path.Join(filePath, name+".ts"), "")
		f.P(createEnum(enum, pkg))
	}

	// Build services
	for _, service := range file.Services {
		f := plugin.NewGeneratedFile(path.Join(filePath, string(service.Desc.Name())+".ts"), "")
		f.P(createOpenModelService(service))
	}

	// Build model files for top-level messages
	for _, msg := range file.Messages {
		name := messageName(msg)
		f := plugin.NewGeneratedFile(path.Join(filePath, name+".ts"), "")
		f.P(createOpenModel(msg, pkg))

		// Inline enums inside this message
		for _, nestedEnum := range msg.Enums {
			eName := enumName(nestedEnum)
			ef := plugin.NewGeneratedFile(path.Join(filePath, eName+".ts"), "")
			ef.P(createEnum(nestedEnum, pkg))
		}

		// Nested messages
		for _, nestedMsg := range msg.Messages {
			nName := messageName(nestedMsg)
			nf := plugin.NewGeneratedFile(path.Join(filePath, nName+".ts"), "")
			nf.P(createOpenModel(nestedMsg, pkg))

			// Inline enums inside nested messages
			for _, nestedEnum := range nestedMsg.Enums {
				eName := enumName(nestedEnum)
				ef := plugin.NewGeneratedFile(path.Join(filePath, eName+".ts"), "")
				ef.P(createEnum(nestedEnum, pkg))
			}

			// Nested-nested messages
			for _, nestedNestedMsg := range nestedMsg.Messages {
				nnName := messageName(nestedNestedMsg)
				nnf := plugin.NewGeneratedFile(path.Join(filePath, nnName+".ts"), "")
				nnf.P(createOpenModel(nestedNestedMsg, pkg))
			}
		}
	}
}

func createEnum(enum *protogen.Enum, pkg string) string {
	name := enumName(enum)
	options := []string{}

	for _, val := range enum.Values {
		leading := string(val.Comments.Leading)
		if leading != "" {
			options = append(options, multilineCommentString(leading))
		}
		valName := string(val.Desc.Name())
		o := "  " + valName + " = \"" + valName + "\","
		trailing := string(val.Comments.Trailing)
		if trailing != "" {
			o = o + " //" + trailing
			if strings.HasSuffix(o, "\n") {
				o = o[:len(o)-1]
			}
		}
		options = append(options, o)
	}

	leading := string(enum.Comments.Leading)
	content := "export enum " + dotToCamel(PrefixReservedWords(name)) + " {\n" + strings.Join(options, "\n") + "\n}"
	parts := []string{
		"// Code generated by furo protoc-gen-open-models. DO NOT EDIT.",
		"// protoc-gen-open-models version: ????",
		"",
		multilineCommentString(leading),
		content,
	}
	return strings.Join(parts, "\n")
}

func createOpenModel(msg *protogen.Message, pkg string) string {
	imports := ImportMap{Imports: make(map[string]map[string]string)}
	imports.AddImport("@furo/open-models/dist/index", "FieldNode", "")
	imports.AddImport("@furo/open-models/dist/index", "Registry", "")

	openApiSchema, _ := ExtractOpenApiMessageOptions(msg)

	literalType := prepareLiteralType(msg, imports)
	transportType := prepareTransportType(msg, imports)
	modelType := prepareModelType(msg, imports, openApiSchema)

	parts := []string{
		"// Code generated by furo protoc-gen-open-models. DO NOT EDIT.",
		"// protoc-gen-open-models version: ????",
		imports.Render(),
		literalType.Render(),
		transportType.Render(),
		modelType.Render(),
	}
	return strings.Join(parts, "\n")
}

func createOpenModelService(service *protogen.Service) string {
	imports := ImportMap{Imports: make(map[string]map[string]string)}
	serviceType := prepareServiceType(service, imports)
	parts := []string{
		"// Code generated by furo protoc-gen-open-models. DO NOT EDIT.",
		"// protoc-gen-open-models version: ????",
		imports.Render(),
		serviceType.Render(),
	}
	return strings.Join(parts, "\n")
}

// openApiSchemaForMessage extracts the OpenAPI schema for a message, or returns nil.
func openApiSchemaForMessage(msg *protogen.Message) *openapi_v3.Schema {
	schema, _ := ExtractOpenApiMessageOptions(msg)
	return schema
}
