package generator

import (
	"fmt"

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
