package generator

var ReservedWords = map[string]struct{}{
	"JSONObject": {},
	"Object":     {},
	"Any":        {},
	"String":     {},
	"Number":     {},
	"Date":       {},
}

func PrefixReservedWords(className string) string {
	if _, ok := ReservedWords[className]; ok {
		return "X" + className
	}
	return className
}
