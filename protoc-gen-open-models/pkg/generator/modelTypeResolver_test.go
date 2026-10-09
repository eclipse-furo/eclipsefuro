package generator

import "testing"

func TestIsWellKnownType(t *testing.T) {
	tests := []struct {
		typeName string
		want     bool
	}{
		// mapped onto @furo/open-models types
		{".google.protobuf.Any", true},
		{".google.protobuf.Empty", true},
		{".google.protobuf.StringValue", true},
		{".google.protobuf.Int64Value", true},
		{".google.protobuf.Timestamp", true},
		{".google.protobuf.Duration", true},
		{".google.protobuf.FieldMask", true},
		{".google.protobuf.Struct", true},
		{".google.protobuf.Value", true},
		{".google.protobuf.ListValue", true},

		// descriptor and other google.protobuf messages are generated as regular messages
		{".google.protobuf.FieldOptions", false},
		{".google.protobuf.DescriptorProto.ExtensionRange", false},
		{".google.protobuf.Struct.FieldsEntry", false},
		{".google.protobuf.Api", false},
		// added in newer descriptor.proto versions (protoc >= 27)
		{".google.protobuf.FieldOptions.FeatureSupport", false},
		{".google.protobuf.FeatureSet.VisibilityFeature", false},
		// anything a future protoc adds
		{".google.protobuf.SomethingNew", false},
		{".google.protobuf.SomethingNew.Nested", false},

		// other packages
		{".my.pkg.Timestamp", false},
		{".google.type.Date", false},
		{".google.protobuf", false},
	}
	for _, tt := range tests {
		if got := isWellKnownType(tt.typeName); got != tt.want {
			t.Errorf("isWellKnownType(%q) = %v, want %v", tt.typeName, got, tt.want)
		}
	}
}
