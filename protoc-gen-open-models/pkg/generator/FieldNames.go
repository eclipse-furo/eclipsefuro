package generator

import "github.com/iancoleman/strcase"

func cleanFieldName(body string) string {
	// todo: convert _var_name to XvarName
	return strcase.ToLowerCamel(body)
}
