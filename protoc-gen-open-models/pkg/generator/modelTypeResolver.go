package generator

import (
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var WellKnownTypesMap = map[string]string{
	"StringValue": "string",
	"BytesValue":  "string",
	"BoolValue":   "boolean",
	"Int32Value":  "number",
	"Int64Value":  "string",
	"FloatValue":  "number",
	"DoubleValue": "number",
	"UInt32Value": "number",
	"UInt64Value": "string",
	"Timestamp":   "string",
	"Duration":    "string",
	"Struct":      "JSONObject",
	"Empty":       "Record<string, never>",
	"FieldMask":   "string[]",
}

var ModelWellKnownTypesMap = map[string]string{
	"StringValue": "string",
	"BytesValue":  "string",
	"BoolValue":   "boolean",
	"Int32Value":  "number",
	"Int64Value":  "bigint",
	"FloatValue":  "number",
	"DoubleValue": "number",
	"UInt32Value": "number",
	"UInt64Value": "bigint",
	"Timestamp":   "string",
	"Duration":    "string",
	"Struct":      "JSONObject",
	"Empty":       "Record<string, never>",
	"FieldMask":   "string[]",
}

// https://protobuf.dev/programming-guides/json/
var ModelPrimitivesMap = map[string]string{
	"TYPE_STRING":   "string",
	"TYPE_BYTES":    "string",
	"TYPE_BOOL":     "boolean",
	"TYPE_INT32":    "number",
	"TYPE_INT64":    "bigint",
	"TYPE_DOUBLE":   "number",
	"TYPE_FLOAT":    "number",
	"TYPE_UINT32":   "number",
	"TYPE_UINT64":   "bigint",
	"TYPE_FIXED32":  "number",
	"TYPE_FIXED64":  "bigint",
	"TYPE_SFIXED32": "number",
	"TYPE_SFIXED64": "bigint",
	"TYPE_SINT32":   "number",
	"TYPE_SINT64":   "bigint",
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

func resolveModelType(imports ImportMap, field *protogen.Field) (
	ModelType string, SetterCommand string, SetterType string, GetterType string, MapValueConstructor string, FieldConstructor string) {
	tn := fieldTypeName(field)
	fieldType := kindToTypeString[field.Desc.Kind()]
	fieldPkg := string(field.Parent.Desc.ParentFile().Package())
	parentName := string(field.Parent.Desc.Name())
	isRepeated := field.Desc.IsList()

	if t, ok := ModelTypesMap[fieldType]; ok {
		primitiveType := ModelPrimitivesMap[fieldType]
		if isRepeated {
			imports.AddImport("@furo/open-models/dist/index", "ARRAY", "")
			imports.AddImport("@furo/open-models/dist/index", ModelTypesMap[fieldType], "")
			return "ARRAY<" + t + ", " + primitiveType + ">",
				"__TypeSetter",
				primitiveType + "[]",
				"ARRAY<" + t + ", " + primitiveType + ">",
				"",
				t
		}
		imports.AddImport("@furo/open-models/dist/index", ModelTypesMap[fieldType], "")
		return t, "__PrimitivesSetter", primitiveType, t, "", t
	}

	// Maps
	if field.Desc.IsMap() {
		valueField := field.Message.Fields[1] // map value is always index 1
		valueKind := valueField.Desc.Kind()

		if valueKind != protoreflect.MessageKind &&
			valueKind != protoreflect.EnumKind &&
			valueKind != protoreflect.GroupKind {
			maptype := kindToTypeString[valueKind]
			imports.AddImport("@furo/open-models/dist/index", "MAP", "")
			imports.AddImport("@furo/open-models/dist/index", ModelTypesMap[maptype], "")
			return "MAP<string," + ModelTypesMap[maptype] + "," + ModelPrimitivesMap[maptype] + ">",
				"__TypeSetter",
				"{ [key: string]: " + ModelPrimitivesMap[maptype] + " }",
				"MAP<string," + ModelTypesMap[maptype] + "," + ModelPrimitivesMap[maptype] + ">",
				ModelTypesMap[maptype],
				"MAP<string," + ModelTypesMap[maptype] + "," + ModelPrimitivesMap[maptype] + ">"
		}

		if valueKind == protoreflect.MessageKind {
			m := "." + string(valueField.Message.Desc.FullName())
			className := messageName(allTypes[m])
			maptype := string(m[1:])

			// WELL KNOWN
			if isWellKnownType(tn) {
				ts := strings.Split(tn, ".")
				typeName := ts[len(ts)-1]

				if typeName == "Any" {
					imports.AddImport("@furo/open-models/dist/index", "type IAny", "")
					imports.AddImport("@furo/open-models/dist/index", "ANY", "")
					return "ANY", "__TypeSetter", "IAny", "ANY", "", "ANY"
				}

				primitiveMapType := ModelWellKnownTypesMap[typeName]

				if typeName == "Empty" {
					imports.AddImport("@furo/open-models/dist/index", "EMPTY", "")
					return "EMPTY", "__TypeSetter", primitiveMapType, "EMPTY", "", "EMPTY"
				}

				imports.AddImport("@furo/open-models/dist/index", "MAP", "")
				imports.AddImport("@furo/open-models/dist/index", ModelTypesMap[primitiveMapType], "")
				return "MAP<string," + ModelTypesMap[primitiveMapType] + "," + ModelPrimitivesMap[primitiveMapType] + ">",
					"__TypeSetter",
					"{ [key: string]: " + ModelPrimitivesMap[primitiveMapType] + " }",
					"MAP<string," + ModelTypesMap[primitiveMapType] + "," + ModelPrimitivesMap[primitiveMapType] + ">",
					ModelTypesMap[primitiveMapType],
					"MAP<string," + ModelTypesMap[primitiveMapType] + "," + ModelPrimitivesMap[primitiveMapType] + ">"
			}

			fieldPackage := strings.Split("."+fieldPkg, ".")
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

		return "MAP<string,unknown,unknown>", "__TypeSetter", "unknown", "unknown", "", "unknown"
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

			primitiveType := ModelWellKnownTypesMap[typeName]
			if primitiveType == "JSONObject" {
				imports.AddImport("@furo/open-models/dist/index", "type JSONObject", "")
			}
			if typeName == "Empty" {
				imports.AddImport("@furo/open-models/dist/index", "EMPTY", "")
				return "EMPTY", "__TypeSetter", primitiveType, "EMPTY", "", "EMPTY"
			}

			imports.AddImport("@furo/open-models/dist/index", typeName, "")
			return typeName, "__TypeSetter", primitiveType + "| null", typeName, "", typeName
		}

		// MESSAGE
		t := tn
		className := dotToCamel(messageName(allTypes[t]))
		if strings.HasPrefix(t, ".") {
			t = t[1:]
		}
		refPkg := string(allTypes[tn].Desc.ParentFile().Package())
		if refPkg == fieldPkg {
			importFile := t[len(fieldPkg)+1:]

			t = fullQualifiedName(t, "")
			if parentName != importFile {
				imports.AddImport("./"+importFile, PrefixReservedWords(className), t)
			}

			if isRepeated {
				imports.AddImport("@furo/open-models/dist/index", "ARRAY", "")

				if tn == "."+fieldPkg+"."+parentName {
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
			// if a field type equals the package name + message type we have a direct recursion
			if tn == "."+fieldPkg+"."+parentName {
				imports.AddImport("@furo/open-models/dist/index", "RECURSION", "")
				return "RECURSION<" + className + ", I" + className + ">",
					"__TypeSetter",
					"I" + className,
					"RECURSION<" + className + ", I" + className + ">",
					"",
					className
			}
			// deep recursion
			if deepRecursionCheck(tn) {
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

		// find relative path to import target

		if _, ok := projectFiles[typenameToPath(tn)]; ok {
			ss := strings.Split(tn, ".")
			importFile := ss[len(ss)-1]
			fieldPackage := strings.Split("."+fieldPkg, ".")
			rel, _ := filepath.Rel(strings.Join(fieldPackage, "/"), "/"+typenameToPath(tn))
			if !strings.HasPrefix(rel, "..") {
				rel = "./" + rel
			}

			t = fullQualifiedName(t, "")
			if parentName != importFile {
				imports.AddImport(rel, PrefixReservedWords(className), t)
			}
			if isRepeated {
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

		return tn, "__TypeSetter", "todo:resolve dependency", "???", "", tn
	}
	if fieldType == "TYPE_ENUM" {
		t := tn
		className := dotToCamel(enumName(allEnums[t]))
		if strings.HasPrefix(t, ".") {
			t = t[1:]
		}
		enumPkg := string(allEnums[tn].Desc.ParentFile().Package())
		if enumPkg == fieldPkg {
			importFile := t[len(fieldPkg)+1:]
			fqn := fullQualifiedName(t, "")

			imports.AddImport("@furo/open-models/dist/index", "ENUM", "")
			imports.AddImport("./"+importFile, PrefixReservedWords(className), fqn)

			return "ENUM<" + fqn + ">", "__TypeSetter", fqn, "ENUM<" + fqn + ">", "", "ENUM<" + fqn + ">"
		}
		if _, ok := projectFiles[typenameToPath(tn)]; ok {
			fieldPackage := strings.Split("."+fieldPkg, ".")
			rel, _ := filepath.Rel(strings.Join(fieldPackage, "/"), "/"+typenameToPath(tn))
			if !strings.HasPrefix(rel, "..") {
				rel = "./" + rel
			}
			fqn := fullQualifiedName(t, "")

			imports.AddImport("@furo/open-models/dist/index", "ENUM", "")
			imports.AddImport(rel, PrefixReservedWords(className), fqn)
			if isRepeated {
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
	return deepRecursionCheckRecursion(typename, typename, map[string]bool{})
}
func deepRecursionCheckRecursion(startAt string, lookFor string, visited map[string]bool) bool {

	if startAt == "" || lookFor == "" {
		return false
	}

	if visited[startAt] {
		return false
	}
	visited[startAt] = true

	msg := allTypes[startAt]
	if msg == nil {
		return false
	}

	for _, f := range msg.Fields {
		if f.Desc.Kind() == protoreflect.MessageKind {
			fqn := "." + string(f.Message.Desc.FullName())
			if fqn == lookFor {
				return true
			}
			if !f.Desc.IsList() && !f.Desc.IsMap() {
				if deepRecursionCheckRecursion(fqn, lookFor, visited) {
					return true
				}
			}
		}
	}
	return false
}

func typenameToPath(tn string) string {
	msg := allTypes[tn]
	if msg != nil {
		pkg := string(msg.Desc.ParentFile().Package())
		name := messageName(msg)
		return strings.Replace(pkg, ".", "/", -1) + "/" + name
	}
	return strings.Replace(tn[1:], ".", "/", -1)
}
