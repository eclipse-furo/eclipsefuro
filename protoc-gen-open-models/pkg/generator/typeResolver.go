package generator

import (
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
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

func resolveInterfaceType(imports ImportMap, field *protogen.Field, kindPrefix string) string {
	tn := fieldTypeName(field)
	fieldType := kindToTypeString[field.Desc.Kind()]
	fieldPkg := string(field.Parent.Desc.ParentFile().Package())
	parentName := string(field.Parent.Desc.Name())
	isRepeated := field.Desc.IsList()

	if t, ok := PrimitivesMap[fieldType]; ok {
		if isRepeated {
			return t + "[]"
		}
		return t
	}

	// Maps
	if field.Desc.IsMap() {
		valueField := field.Message.Fields[1] // map value is always index 1
		valueKind := valueField.Desc.Kind()

		if valueKind != protoreflect.MessageKind &&
			valueKind != protoreflect.EnumKind &&
			valueKind != protoreflect.GroupKind {
			t := kindToTypeString[valueKind]
			return "{ [key: string]: " + PrimitivesMap[t] + " }"
		}

		if valueKind == protoreflect.MessageKind {
			m := "." + string(valueField.Message.Desc.FullName())
			className := allTypes[m].Desc.Name()

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

				primitiveMapType := WellKnownTypesMap[typeName]
				imports.AddImport("@furo/open-models/dist/index", typeName, "")
				return "{ [key: string]: " + PrimitivesMap[primitiveMapType] + " }"
			}

			fieldPackage := strings.Split("."+fieldPkg, ".")
			rel, _ := filepath.Rel(strings.Join(fieldPackage, "/"), "/"+typenameToPath(m))
			if !strings.HasPrefix(rel, "..") {
				rel = "./" + rel
			}
			maptype := string(m[1:])
			imports.AddImport(rel, "type "+kindPrefix+PrefixReservedWords(string(className)), kindPrefix+fullQualifiedName(maptype, ""))
			return "{ [key: string]: " + kindPrefix + fullQualifiedName(maptype, "") + " }"
		}

		return "{ [key: string]: unknown }"
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

			primitiveType := WellKnownTypesMap[typeName]
			imports.AddImport("@furo/open-models/dist/index", typeName, "")
			return primitiveType
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
				imports.AddImport("./"+importFile, "type "+kindPrefix+PrefixReservedWords(className), kindPrefix+t)
			} else {
				if isRepeated {
					return kindPrefix + className + "[]"
				}
				return kindPrefix + className
			}
			if isRepeated {
				return kindPrefix + t + "[]"
			}
			return kindPrefix + t
		}

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
				imports.AddImport(rel, "type "+kindPrefix+PrefixReservedWords(className), kindPrefix+t)
			}
			if isRepeated {
				return kindPrefix + t + "[]"
			}
			return kindPrefix + t
		}

		return tn
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

			imports.AddImport("./"+importFile, PrefixReservedWords(className), fqn)
			if isRepeated {
				return fqn + "[]"
			}
			return fqn + " | string"
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
				return fqn + "[]"
			}
			return fqn + " | string"
		}
		return "ENUM:UNRECOGNIZED"
	}

	return "UNRECOGNIZED"
}
