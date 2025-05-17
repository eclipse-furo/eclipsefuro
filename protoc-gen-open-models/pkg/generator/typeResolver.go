package generator

import (
	"github.com/eclipse-furo/eclipsefuro/protoc-gen-open-models/pkg/sourceinfo"
	"google.golang.org/protobuf/types/descriptorpb"
	"path/filepath"
	"strings"
)

// https://protobuf.dev/programming-guides/json/
var PrimitivesMap = map[string]string{
	"TYPE_STRING":   "string",
	"TYPE_BYTES":    "string",
	"TYPE_BOOL":     "boolean",
	"TYPE_INT32":    "number",
	"TYPE_INT64":    "string",
	"TYPE_DOUBLE":   "number",
	"TYPE_FLOAT":    "number",
	"TYPE_UINT32":   "number",
	"TYPE_UINT64":   "string",
	"TYPE_FIXED32":  "number",
	"TYPE_FIXED64":  "string",
	"TYPE_SFIXED32": "number",
	"TYPE_SFIXED64": "string",
	"TYPE_SINT32":   "number",
	"TYPE_SINT64":   "string",
}

func resolveInterfaceType(imports ImportMap, field sourceinfo.FieldInfo, kindPrefix string) string {
	tn := field.Field.GetTypeName()

	fieldType := field.Field.Type.String()

	if t, ok := PrimitivesMap[fieldType]; ok {
		if field.Field.Label.String() == "LABEL_REPEATED" {
			return t + "[]"
		}
		return t
	}

	// Maps
	for _, nested := range field.Message.NestedType {
		if nested.Options != nil {
			if *nested.Options.MapEntry {
				if strings.Title(field.Name)+"Entry" == *nested.Name {
					// this is a map
					maptype := "not_evaluated"
					if !(*nested.Field[1].Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE ||
						*nested.Field[1].Type == descriptorpb.FieldDescriptorProto_TYPE_ENUM ||
						*nested.Field[1].Type == descriptorpb.FieldDescriptorProto_TYPE_GROUP) {
						t := nested.Field[1].Type.String()
						maptype = t
					} else {
						// can be a message or a primitive
						if *nested.Field[1].Type == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
							// message
							m := *nested.Field[1].TypeName
							maptype = m[1:len(m)]
							className := allTypes[m].Name

							// WELL KNOWN
							if isWellKnownType(tn) {
								ts := strings.Split(tn, ".")
								typeName := ts[len(ts)-1]

								if typeName == "Any" {
									imports.AddImport("@furo/open-models/dist/index", "type IAny", "")
									return "IAny"
								}

								if typeName == "Empty" {
									return WellKnownTypesMap[typeName]
								}

								// well known types are using primitives
								primitiveMapType := WellKnownTypesMap[typeName]
								imports.AddImport("@furo/open-models/dist/index", typeName, "")
								return "{ [key: string]: " + PrimitivesMap[primitiveMapType] + " }"
							}
							fieldPackage := strings.Split("."+field.Package, ".")
							rel, _ := filepath.Rel(strings.Join(fieldPackage, "/"), "/"+typenameToPath(m))
							if !strings.HasPrefix(rel, "..") {
								rel = "./" + rel
							}
							imports.AddImport(rel, kindPrefix+PrefixReservedWords(className), kindPrefix+fullQualifiedName(maptype, ""))
							return "{ [key: string]: " + kindPrefix + fullQualifiedName(maptype, "") + " }"
						}
					}
					return "{ [key: string]: " + PrimitivesMap[maptype] + " }"
					// for model types return "MAP<string, STRING, string>;"
				}
			}

		}
	}

	if fieldType == "TYPE_MESSAGE" {
		// WELL KNOWN
		if isWellKnownType(tn) {
			ts := strings.Split(tn, ".")
			typeName := ts[len(ts)-1]

			if typeName == "Any" {
				imports.AddImport("@furo/open-models/dist/index", "type IAny", "")
				return "IAny"
			}

			if typeName == "Empty" {
				return WellKnownTypesMap[typeName]
			}

			// well known types are using primitives
			primitiveType := WellKnownTypesMap[typeName]
			imports.AddImport("@furo/open-models/dist/index", typeName, "")
			return primitiveType
		}

		// ANY

		// MESSAGE
		t := field.Field.GetTypeName()
		className := dotToCamel(allTypes[t].Name)
		if strings.HasPrefix(t, ".") {
			t = t[1:]
		}
		if allTypes[tn].Package == field.Package {
			// we are in the same package
			// import is just ./[TypeName.Nested]
			importFile := t[len(field.Package)+1:]

			t = fullQualifiedName(t, "")
			// add imports for Transport, Literal and Model
			// do not add import for the same file (direct recursion types)
			if field.Message.GetName() != importFile {
				imports.AddImport("./"+importFile, kindPrefix+PrefixReservedWords(className), kindPrefix+t)
			} else {
				// repeated recursion like
				if field.Field.Label.String() == "LABEL_REPEATED" {
					return kindPrefix + className + "[]"
				}
				return kindPrefix + className
			}
			if field.Field.Label.String() == "LABEL_REPEATED" {
				return kindPrefix + t + "[]"
			}
			return kindPrefix + t
		}

		if _, ok := projectFiles[typenameToPath(tn)]; ok {
			// definition is in project root
			ss := strings.Split(field.Field.GetTypeName(), ".")
			importFile := ss[len(ss)-1]
			fieldPackage := strings.Split("."+field.Package, ".")
			rel, _ := filepath.Rel(strings.Join(fieldPackage, "/"), "/"+typenameToPath(tn))
			if !strings.HasPrefix(rel, "..") {
				rel = "./" + rel
			}
			// do not add import for the same file (direct recursion types)
			t = fullQualifiedName(t, "")
			if field.Message.GetName() != importFile {
				imports.AddImport(rel, kindPrefix+PrefixReservedWords(className), kindPrefix+t)
			}
			if field.Field.Label.String() == "LABEL_REPEATED" {
				return kindPrefix + t + "[]"
			}
			return kindPrefix + t
		}

		return field.Field.GetTypeName()
	}
	if fieldType == "TYPE_ENUM" {
		t := field.Field.GetTypeName()
		className := dotToCamel(allEnums[t].Name)
		if strings.HasPrefix(t, ".") {
			t = t[1:]
		}
		if allEnums[tn].Package == field.Package {
			// we are in the same package
			// import is just ./[TypeName]
			importFile := t[len(field.Package)+1:]
			fqn := fullQualifiedName(t, "")

			// enum are without prefix
			imports.AddImport("./"+importFile, PrefixReservedWords(className), fqn)
			if field.Field.Label.String() == "LABEL_REPEATED" {
				return fqn + "[]"
			}
			return fqn + " | string"
		}
		if _, ok := projectFiles[typenameToPath(tn)]; ok {

			fieldPackage := strings.Split("."+field.Package, ".")
			rel, _ := filepath.Rel(strings.Join(fieldPackage, "/"), "/"+typenameToPath(tn))
			if !strings.HasPrefix(rel, "..") {
				rel = "./" + rel
			}

			fqn := fullQualifiedName(t, "")

			// enum are without prefix
			imports.AddImport("@furo/open-models/dist/index", "ENUM", "")
			imports.AddImport(rel, PrefixReservedWords(className), fqn)
			if field.Field.Label.String() == "LABEL_REPEATED" {
				return fqn + "[]"
			}
			return fqn + " | string"
		}
		return "ENUM:UNRECOGNIZED"
	}

	return "UNRECOGNIZED"
}
