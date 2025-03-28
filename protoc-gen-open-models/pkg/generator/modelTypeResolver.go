package generator

import (
	"github.com/eclipse-furo/eclipsefuro/protoc-gen-open-models/pkg/sourceinfo"
	"google.golang.org/protobuf/types/descriptorpb"
	"path/filepath"
	"strings"
)

var WellKnownTypesMap = map[string]string{
	"StringValue": "string",
	"BytesValue":  "string",
	"BoolValue":   "boolean",
	"Int32Value":  "number",
	"Int64Value":  "number",
	"FloatValue":  "number",
	"DoubleValue": "number",
	"UInt32Value": "number",
	"UInt64Value": "number",
	"Timestamp":   "string",
	"Duration":    "string",
	"Struct":      "JSONObject",
	"Empty":       "Record<string, never>",
	"FieldMask":   "string[]",
}

var ModelTypesMap = map[string]string{
	"TYPE_STRING":   "STRING",
	"TYPE_BYTES":    "BYTES",
	"TYPE_BOOL":     "BOOLEAN",
	"TYPE_INT32":    "INT32",
	"TYPE_INT64":    "INT64",
	"TYPE_DOUBLE":   "DOUBLE",
	"TYPE_FLOAT":    "FLOAT",
	"TYPE_UINT32":   "UINT32",
	"TYPE_UINT64":   "UINT64",
	"TYPE_FIXED32":  "FIXED32",
	"TYPE_FIXED64":  "FIXED64",
	"TYPE_SFIXED32": "SFIXED32",
	"TYPE_SFIXED64": "SFIXED64",
	"TYPE_SINT32":   "SINT32",
	"TYPE_SINT64":   "SINT64",
}

func resolveModelType(imports ImportMap, field sourceinfo.FieldInfo) (
	ModelType string, SetterCommand string, SetterType string, GetterType string, MapValueConstructor string, FieldConstructor string) {
	tn := field.Field.GetTypeName()

	fieldType := field.Field.Type.String()

	if t, ok := ModelTypesMap[fieldType]; ok {
		primitiveType := PrimitivesMap[fieldType]
		if field.Field.Label.String() == "LABEL_REPEATED" {
			imports.AddImport("@furo/open-models/dist/index", "ARRAY", "")
			imports.AddImport("@furo/open-models/dist/index", ModelTypesMap[fieldType], "")
			return "ARRAY<" + t + ", " + primitiveType + ">",
				"__TypeSetter",
				primitiveType + "[]",
				"ARRAY<" + t + ", " + primitiveType + ">",
				"", // ARRAY is uses a typesetter
				t
		}
		imports.AddImport("@furo/open-models/dist/index", ModelTypesMap[fieldType], "")
		return t, "__PrimitivesSetter", primitiveType, t, "", t
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
							className := allTypes[m].Name
							maptype = m[1:len(m)]
							// WELL KNOWN

							if isWellKnownType(tn) {
								ts := strings.Split(tn, ".")
								typeName := ts[len(ts)-1]

								// ANY
								if typeName == "Any" {
									imports.AddImport("@furo/open-models/dist/index", "type IAny", "")
									imports.AddImport("@furo/open-models/dist/index", "ANY", "")
									return "ANY", "__TypeSetter", "IAny", "ANY", "", "ANY"
								}

								primitiveMapType := WellKnownTypesMap[typeName]

								if typeName == "Empty" {
									imports.AddImport("@furo/open-models/dist/index", "EMPTY", "")
									return "EMPTY", "__TypeSetter", primitiveMapType, "EMPTY", "", "EMPTY"
								}

								// for model types return "MAP<string, STRING, string>;"
								imports.AddImport("@furo/open-models/dist/index", "MAP", "")
								imports.AddImport("@furo/open-models/dist/index", ModelTypesMap[primitiveMapType], "")
								return "MAP<string," + ModelTypesMap[primitiveMapType] + "," + PrimitivesMap[primitiveMapType] + ">",
									"__TypeSetter",
									"{ [key: string]: " + PrimitivesMap[primitiveMapType] + " }",
									"MAP<string," + ModelTypesMap[primitiveMapType] + "," + PrimitivesMap[primitiveMapType] + ">",
									ModelTypesMap[primitiveMapType],
									"MAP<string," + ModelTypesMap[primitiveMapType] + "," + PrimitivesMap[primitiveMapType] + ">"
							}

							fieldPackage := strings.Split("."+field.Package, ".")
							rel, _ := filepath.Rel(strings.Join(fieldPackage, "/"), "/"+typenameToPath(m))
							if !strings.HasPrefix(rel, "..") {
								rel = "./" + rel
							}
							imports.AddImport(rel, PrefixReservedWords(className), fullQualifiedName(maptype, ""))
							imports.AddImport("@furo/open-models/dist/index", "MAP", "")

							return "MAP<string," + fullQualifiedName(maptype, "") + "," + fullQualifiedName(maptype, "") + ">",
								"__TypeSetter",
								"{ [key: string]: " + fullQualifiedName(maptype, "") + " }",
								"MAP<string," + fullQualifiedName(maptype, "") + "," + fullQualifiedName(maptype, "") + ">",
								fullQualifiedName(maptype, ""),
								"MAP<string," + fullQualifiedName(maptype, "") + "," + fullQualifiedName(maptype, "") + ">"
						}
					}
					// for model types return "MAP<string, STRING, string>;"
					imports.AddImport("@furo/open-models/dist/index", "MAP", "")
					imports.AddImport("@furo/open-models/dist/index", ModelTypesMap[maptype], "")
					return "MAP<string," + ModelTypesMap[maptype] + "," + PrimitivesMap[maptype] + ">",
						"__TypeSetter",
						"{ [key: string]: " + PrimitivesMap[maptype] + " }",
						"MAP<string," + ModelTypesMap[maptype] + "," + PrimitivesMap[maptype] + ">",
						ModelTypesMap[maptype],
						"MAP<string," + ModelTypesMap[maptype] + "," + PrimitivesMap[maptype] + ">"

				}
			}
		}
	}

	if fieldType == "TYPE_MESSAGE" {
		// WELL KNOWN

		if isWellKnownType(tn) {
			ts := strings.Split(tn, ".")
			typeName := ts[len(ts)-1]

			// ANY
			if typeName == "Any" {
				imports.AddImport("@furo/open-models/dist/index", "type IAny", "")
				imports.AddImport("@furo/open-models/dist/index", "ANY", "")
				return "ANY", "__TypeSetter", "IAny", "ANY", "", "ANY"
			}

			primitiveType := WellKnownTypesMap[typeName]
			if primitiveType == "JSONObject" {
				imports.AddImport("@furo/open-models/dist/index", "JSONObject", "")
			}
			if typeName == "Empty" {
				imports.AddImport("@furo/open-models/dist/index", "EMPTY", "")
				return "EMPTY", "__TypeSetter", primitiveType, "EMPTY", "", "EMPTY"
			}

			imports.AddImport("@furo/open-models/dist/index", typeName, "")
			return typeName, "__TypeSetter", primitiveType + "| null", typeName, "", typeName
		}

		// MESSAGE
		t := field.Field.GetTypeName()
		className := dotToCamel(allTypes[t].Name)
		if strings.HasPrefix(t, ".") {
			t = t[1:]
		}
		if allTypes[tn].Package == field.Package {
			// we are in the same package
			// import is just ./[TypeName]
			importFile := t[len(field.Package)+1:]

			// do not add import for the same file (direct recursion types)
			t = fullQualifiedName(t, "")
			if field.Message.GetName() != importFile {
				imports.AddImport("./"+importFile, PrefixReservedWords(className), t)
			}

			if field.Field.Label.String() == "LABEL_REPEATED" {
				imports.AddImport("@furo/open-models/dist/index", "ARRAY", "")

				// if we are in the same package, we use the classNames
				if field.Field.GetTypeName() == "."+field.Package+"."+field.Message.GetName() {
					return "ARRAY<" + className + ", I" + className + ">",
						"__TypeSetter",
						"I" + className + "[]",
						"ARRAY<" + className + ", I" + className + ">",
						"",
						className
				}
				return "ARRAY<" + t + ", I" + t + ">",
					"__TypeSetter",
					"I" + t + "[]",
					"ARRAY<" + t + ", I" + t + ">",
					"",
					t
			}
			// if a field type equals the package name + message type we have a direct recusrion
			if field.Field.GetTypeName() == "."+field.Package+"."+field.Message.GetName() {
				imports.AddImport("@furo/open-models/dist/index", "RECURSION", "")
				return "RECURSION<" + className + ", I" + className + ">",
					"__TypeSetter",
					"I" + className,
					"RECURSION<" + className + ", I" + className + ">",
					"",
					className
			}
			// deep recursion
			if deepRecursionCheck(field.Field.GetTypeName()) {
				imports.AddImport("@furo/open-models/dist/index", "RECURSION", "")
				return "RECURSION<" + t + ", I" + t + ">",
					"__TypeSetter",
					"I" + t,
					"RECURSION<" + t + ", I" + t + ">",
					"",
					t
			}

			return t, "__TypeSetter", "I" + t, t, "", t
		}

		// find relative path to import target,

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
				imports.AddImport(rel, PrefixReservedWords(className), t)
			}
			if field.Field.Label.String() == "LABEL_REPEATED" {
				imports.AddImport("@furo/open-models/dist/index", "ARRAY", "")

				return "ARRAY<" + t + ", I" + t + ">",
					"__TypeSetter",
					"I" + t + "[]",
					"ARRAY<" + t + ", I" + t + ">",
					"",
					t
			}
			return t, "__TypeSetter", "I" + t, t, "", t
		}

		return field.Field.GetTypeName(), "__TypeSetter", "todo:resolve dependency", "???", "", field.Field.GetTypeName()
	}
	if fieldType == "TYPE_ENUM" {
		t := field.Field.GetTypeName()
		className := dotToCamel(allEnums[t].Name)
		if strings.HasPrefix(t, ".") {
			t = t[1:]
		}
		if allEnums[tn].Package == field.Package {
			// we are in the same package
			// import is just ./[TypeName.Nested]
			importFile := t[len(field.Package)+1:]
			fqn := fullQualifiedName(t, "")

			imports.AddImport("@furo/open-models/dist/index", "ENUM", "")
			imports.AddImport("./"+importFile, PrefixReservedWords(className), fqn)
			// create correct importFile for nested types

			return "ENUM<" + fqn + ">", "__TypeSetter", fqn, "ENUM<" + fqn + ">", "", "ENUM<" + fqn + ">"
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
				return "ENUM<" + fqn + ">", "__TypeSetter", fqn, "ENUM<" + fqn + ">", "", "ENUM<" + fqn + ">"
			}
			return "ENUM<" + fqn + ">", "__TypeSetter", fqn, "ENUM<" + fqn + ">", "", "ENUM<" + fqn + ">"
		}
		return "ENUM:UNRECOGNIZED", "__TypeSetter", "???", "???", "", "ENUM<" + t + ">"
	}

	return "UNRECOGNIZED", "UNRECOGNIZED", "UNRECOGNIZED", "UNRECOGNIZED", "", "UNRECOGNIZED"
}

func isWellKnownType(tn string) bool {

	return strings.HasPrefix(tn, ".google.protobuf.") &&
		tn != ".google.protobuf.Api" &&
		tn != ".google.protobuf.ListValue" &&
		tn != ".google.protobuf.Value" &&
		tn != ".google.protobuf.Type" &&
		tn != ".google.protobuf.Descriptor" &&
		tn != ".google.protobuf.Enum" &&
		tn != ".google.protobuf.ExtensionRangeOptions" &&
		tn != ".google.protobuf.ExtensionRangeOptions.Declaration" &&
		tn != ".google.protobuf.EnumValue" &&
		tn != ".google.protobuf.EnumValueOptions" &&
		tn != ".google.protobuf.UninterpretedOption" &&
		tn != ".google.protobuf.FeatureSet" &&
		tn != ".google.protobuf.EnumOptions" &&
		tn != ".google.protobuf.EnumReservedRange" &&
		tn != ".google.protobuf.FieldOptions" &&
		tn != ".google.protobuf.FieldOptions.EditionDefault" &&
		tn != ".google.protobuf.EnumValueDescriptorProto" &&
		tn != ".google.protobuf.EnumDescriptorProto.EnumReservedRange" &&
		tn != ".google.protobuf.Mixin" &&
		tn != ".google.protobuf.SourceCodeInfo.Location" &&
		tn != ".google.protobuf.Method" &&
		tn != ".google.protobuf.Option" &&
		tn != ".google.protobuf.MethodOptions" &&
		tn != ".google.protobuf.FileOptions" &&
		tn != ".google.protobuf.Field" &&
		tn != ".google.protobuf.FeatureSetDefaults.FeatureSetEditionDefault" &&
		tn != ".google.protobuf.Struct.FieldsEntry" &&
		tn != ".google.protobuf.ServiceDescriptorProto" &&
		tn != ".google.protobuf.SourceCodeInfo" &&
		tn != ".google.protobuf.SourceContext" &&
		tn != ".google.protobuf.FileDescriptorProto" &&
		tn != ".google.protobuf.DescriptorProto" &&
		tn != ".google.protobuf.GeneratedCodeInfo.Annotation" &&
		tn != ".google.protobuf.EnumDescriptorProto" &&
		tn != ".google.protobuf.DescriptorProto.ExtensionRange" &&
		tn != ".google.protobuf.FieldDescriptorProto" &&
		tn != ".google.protobuf.ExtensionRange" &&
		tn != ".google.protobuf.UninterpretedOption.NamePart" &&
		tn != ".google.protobuf.MethodDescriptorProto" &&
		tn != ".google.protobuf.ServiceOptions" &&
		tn != ".google.protobuf.OneofOptions" &&
		tn != ".google.protobuf.MessageOptions" &&
		tn != ".google.protobuf.OneofDescriptorProto" &&
		tn != ".google.protobuf.DescriptorProto.ReservedRange" &&
		tn != ".google.protobuf.Syntax"
}

func deepRecursionCheck(typename string) bool {
	return deepRecursionCheckRecursion(typename, typename)
}
func deepRecursionCheckRecursion(startAt string, lookFor string) bool {

	for _, info := range allTypes[startAt].FieldInfos {
		if info.Field.GetTypeName() == lookFor {
			return true
		}

		if info.Field.Type.String() == "TYPE_MESSAGE" && info.Field.Label.String() != "LABEL_REPEATED" {
			return deepRecursionCheckRecursion(info.Field.GetTypeName(), lookFor)
		}

	}
	return false
}

func typenameToPath(tn string) string {
	info := allTypes[tn]
	if info.ParentOfNested != nil {
		ret := strings.Replace(info.ParentOfNested.Package, ".", "/", -1)
		return ret + "/" + info.Name
	}
	return strings.Replace(tn[1:], ".", "/", -1)
}
