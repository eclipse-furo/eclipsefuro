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
	"ListValue":   "JSONValue[]",
	"Value":       "JSONValue",
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
	"ListValue":   "JSONValue[]",
	"Value":       "JSONValue",
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

		// Enum-valued map. MAP builds its values with `new Constructor()` via the
		// ValueConstructor descriptor slot, so it needs the bound <Name>ENUM subclass.
		if valueKind == protoreflect.EnumKind {
			e := "." + string(valueField.Enum.Desc.FullName())
			if fqn, ok := resolveEnumImport(imports, e, fieldPkg, true); ok {
				imports.AddImport("@furo/open-models/dist/index", "MAP", "")
				m := "MAP<string," + fqn + "ENUM," + fqn + ">"
				return m,
					"__TypeSetter",
					"{ [key: string]: " + fqn + " }",
					m,
					fqn + "ENUM",
					m
			}
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
				return "ANY", "__TypeSetter", "IAny | null", "ANY", "", "ANY"
			}

			primitiveType := ModelWellKnownTypesMap[typeName]
			if primitiveType == "JSONObject" {
				imports.AddImport("@furo/open-models/dist/index", "type JSONObject", "")
			}
			if primitiveType == "JSONValue" || primitiveType == "JSONValue[]" {
				imports.AddImport("@furo/open-models/dist/index", "type JSONValue", "")
			}
			if typeName == "Empty" {
				imports.AddImport("@furo/open-models/dist/index", "EMPTY", "")
				return "EMPTY", "__TypeSetter", primitiveType, "EMPTY", "", "EMPTY"
			}

			imports.AddImport("@furo/open-models/dist/index", typeName, "")
			return typeName, "__TypeSetter", primitiveType + " | null", typeName, "", typeName
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
						"I" + className + "[] | null",
						"ARRAY<" + className + ", I" + className + ">",
						"",
						className
				}
				return "ARRAY<" + t + ", I" + t + ">",
					"__TypeSetter",
					"I" + t + "[] | null",
					"ARRAY<" + t + ", I" + t + ">",
					"",
					t
			}
			// if a field type equals the package name + message type we have a direct recursion
			if tn == "."+fieldPkg+"."+parentName {
				imports.AddImport("@furo/open-models/dist/index", "RECURSION", "")
				return "RECURSION<" + className + ", I" + className + ">",
					"__TypeSetter",
					"I" + className + " | null",
					"RECURSION<" + className + ", I" + className + ">",
					"",
					className
			}
			// deep recursion
			if deepRecursionCheck(tn) {
				imports.AddImport("@furo/open-models/dist/index", "RECURSION", "")
				return "RECURSION<" + t + ", I" + t + ">",
					"__TypeSetter",
					"I" + t + " | null",
					"RECURSION<" + t + ", I" + t + ">",
					"",
					t
			}

			return t, "__TypeSetter", "I" + t + " | null", t, "", t
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
					"I" + t + "[] | null",
					"ARRAY<" + t + ", I" + t + ">",
					"",
					t
			}
			return t, "__TypeSetter", "I" + t + " | null", t, "", t
		}

		return tn, "__TypeSetter", "todo:resolve dependency", "???", "", tn
	}
	if fieldType == "TYPE_ENUM" {
		fqn, ok := resolveEnumImport(imports, tn, fieldPkg, isRepeated)
		if !ok {
			return "ENUM:UNRECOGNIZED", "__TypeSetter", "???", "???", "", "ENUM<" + strings.TrimPrefix(tn, ".") + ">"
		}

		// A repeated enum is an ARRAY of the generated <Name>ENUM subclass. ENUM itself
		// cannot be the element constructor: ARRAY builds children with `new Constructor()`.
		if isRepeated {
			imports.AddImport("@furo/open-models/dist/index", "ARRAY", "")
			arr := "ARRAY<" + fqn + "ENUM, " + fqn + ">"
			return arr, "__TypeSetter", fqn + "[]", arr, "", fqn + "ENUM"
		}

		imports.AddImport("@furo/open-models/dist/index", "ENUM", "")
		return "ENUM<" + fqn + ">", "__TypeSetter", fqn, "ENUM<" + fqn + ">", "", "ENUM<" + fqn + ">"
	}

	return "UNRECOGNIZED", "UNRECOGNIZED", "UNRECOGNIZED", "UNRECOGNIZED", "", "UNRECOGNIZED"
}

// isWellKnownType reports whether tn is a well-known type that maps onto an
// @furo/open-models type (ANY, EMPTY or a primitive wrapper) instead of a generated class.
// It is an allowlist: every other google.protobuf message, e.g. the descriptor types and
// the ones newer protoc versions add (FieldOptions.FeatureSupport, ...), is a regular message.
func isWellKnownType(tn string) bool {
	name, found := strings.CutPrefix(tn, ".google.protobuf.")
	if !found || strings.Contains(name, ".") {
		return false
	}
	if name == "Any" {
		return true
	}
	_, ok := ModelWellKnownTypesMap[name]
	return ok
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
