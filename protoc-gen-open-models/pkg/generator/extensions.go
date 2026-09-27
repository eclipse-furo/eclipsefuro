package generator

import (
	"fmt"
	"slices"

	openapi_v3 "github.com/google/gnostic/openapiv3"
	options "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func ExtractAPIOptions(method *protogen.Method) (*options.HttpRule, error) {
	opts, ok := method.Desc.Options().(*descriptorpb.MethodOptions)
	if !ok || opts == nil {
		return nil, nil
	}
	if !proto.HasExtension(opts, options.E_Http) {
		return nil, nil
	}
	ext := proto.GetExtension(opts, options.E_Http)
	rule, ok := ext.(*options.HttpRule)
	if !ok {
		return nil, fmt.Errorf("extension is %T; want an HttpRule", ext)
	}
	return rule, nil
}

func ExtractOpenApiFieldOptions(field *protogen.Field) (*openapi_v3.Schema, error) {
	opts, ok := field.Desc.Options().(*descriptorpb.FieldOptions)
	if !ok || opts == nil {
		return nil, nil
	}
	if !proto.HasExtension(opts, openapi_v3.E_Property) {
		return nil, nil
	}
	ext := proto.GetExtension(opts, openapi_v3.E_Property)
	schema, ok := ext.(*openapi_v3.Schema)
	if !ok {
		return nil, fmt.Errorf("extension is %T; want an OpenApi Property", ext)
	}
	return schema, nil
}

// ExtractFieldBehaviors returns the google.api.field_behavior values of a field, nil when it has none.
func ExtractFieldBehaviors(field *protogen.Field) []options.FieldBehavior {
	opts, ok := field.Desc.Options().(*descriptorpb.FieldOptions)
	if !ok || opts == nil || !proto.HasExtension(opts, options.E_FieldBehavior) {
		return nil
	}
	behaviors, _ := proto.GetExtension(opts, options.E_FieldBehavior).([]options.FieldBehavior)
	return behaviors
}

// isOutputOnlyField reports whether a client cannot set the field: openapi read_only, or
// google.api.field_behavior OUTPUT_ONLY or IDENTIFIER.
func isOutputOnlyField(field *protogen.Field) bool {
	if props, _ := ExtractOpenApiFieldOptions(field); props != nil && props.ReadOnly {
		return true
	}
	return slices.ContainsFunc(ExtractFieldBehaviors(field), func(b options.FieldBehavior) bool {
		return b == options.FieldBehavior_OUTPUT_ONLY || b == options.FieldBehavior_IDENTIFIER
	})
}

// isRequiredField reports whether the field is (google.api.field_behavior) = REQUIRED. A field of
// a real oneof never is: only one field of its group can be set.
func isRequiredField(field *protogen.Field) bool {
	if oneof := field.Desc.ContainingOneof(); oneof != nil && !oneof.IsSynthetic() {
		return false
	}
	return slices.Contains(ExtractFieldBehaviors(field), options.FieldBehavior_REQUIRED)
}

func ExtractOpenApiMessageOptions(msg *protogen.Message) (*openapi_v3.Schema, error) {
	opts, ok := msg.Desc.Options().(*descriptorpb.MessageOptions)
	if !ok || opts == nil {
		return nil, nil
	}
	if !proto.HasExtension(opts, openapi_v3.E_Schema) {
		return nil, nil
	}
	ext := proto.GetExtension(opts, openapi_v3.E_Schema)
	schema, ok := ext.(*openapi_v3.Schema)
	if !ok {
		return nil, fmt.Errorf("extension is %T; want an OpenApi Property", ext)
	}
	return schema, nil
}
