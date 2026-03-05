package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/pluginpb"
)

// =============================================================================
// Unit Tests for functions that can be tested without protogen mocks
// =============================================================================

func TestProtoKindToJSONType(t *testing.T) {
	tests := []struct {
		kind     protoreflect.Kind
		expected string
	}{
		{protoreflect.BoolKind, "boolean"},
		{protoreflect.Int32Kind, "integer"},
		{protoreflect.Int64Kind, "integer"},
		{protoreflect.Sint32Kind, "integer"},
		{protoreflect.Sint64Kind, "integer"},
		{protoreflect.Uint32Kind, "integer"},
		{protoreflect.Uint64Kind, "integer"},
		{protoreflect.Fixed32Kind, "integer"},
		{protoreflect.Fixed64Kind, "integer"},
		{protoreflect.Sfixed32Kind, "integer"},
		{protoreflect.Sfixed64Kind, "integer"},
		{protoreflect.FloatKind, "number"},
		{protoreflect.DoubleKind, "number"},
		{protoreflect.StringKind, "string"},
		{protoreflect.BytesKind, "string"},
		{protoreflect.MessageKind, "string"}, // default case
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			result := protoKindToJSONType(tt.kind)
			if result != tt.expected {
				t.Errorf("protoKindToJSONType(%v) = %q, want %q", tt.kind, result, tt.expected)
			}
		})
	}
}

func TestIsUnsignedKind(t *testing.T) {
	tests := []struct {
		kind     protoreflect.Kind
		expected bool
	}{
		{protoreflect.Uint32Kind, true},
		{protoreflect.Uint64Kind, true},
		{protoreflect.Fixed32Kind, true},
		{protoreflect.Fixed64Kind, true},
		{protoreflect.Int32Kind, false},
		{protoreflect.Int64Kind, false},
		{protoreflect.Sint32Kind, false},
		{protoreflect.Sint64Kind, false},
		{protoreflect.Sfixed32Kind, false},
		{protoreflect.Sfixed64Kind, false},
		{protoreflect.FloatKind, false},
		{protoreflect.DoubleKind, false},
		{protoreflect.StringKind, false},
		{protoreflect.BoolKind, false},
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			result := isUnsignedKind(tt.kind)
			if result != tt.expected {
				t.Errorf("isUnsignedKind(%v) = %v, want %v", tt.kind, result, tt.expected)
			}
		})
	}
}

func TestExtractDescription(t *testing.T) {
	tests := []struct {
		name     string
		comments protogen.CommentSet
		expected string
	}{
		{
			name:     "leading comment",
			comments: protogen.CommentSet{Leading: " This is a leading comment "},
			expected: "This is a leading comment",
		},
		{
			name:     "trailing comment",
			comments: protogen.CommentSet{Trailing: " This is a trailing comment "},
			expected: "This is a trailing comment",
		},
		{
			name:     "both comments prefers leading",
			comments: protogen.CommentSet{Leading: "Leading", Trailing: "Trailing"},
			expected: "Leading",
		},
		{
			name:     "empty comments",
			comments: protogen.CommentSet{},
			expected: "",
		},
		{
			name:     "whitespace only",
			comments: protogen.CommentSet{Leading: "   "},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDescription(tt.comments)
			if result != tt.expected {
				t.Errorf("extractDescription() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDecodeVarint(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected uint64
		length   int
	}{
		{"single byte 0", []byte{0x00}, 0, 1},
		{"single byte 1", []byte{0x01}, 1, 1},
		{"single byte 127", []byte{0x7f}, 127, 1},
		{"two bytes 128", []byte{0x80, 0x01}, 128, 2},
		{"two bytes 300", []byte{0xac, 0x02}, 300, 2},
		{"three bytes", []byte{0x80, 0x80, 0x01}, 16384, 3},
		{"empty", []byte{}, 0, 0},
		{"overflow", []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x01}, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, length := decodeVarint(tt.data)
			if value != tt.expected || length != tt.length {
				t.Errorf("decodeVarint(%v) = (%d, %d), want (%d, %d)", tt.data, value, length, tt.expected, tt.length)
			}
		})
	}
}

func TestDecodeFixed64(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected float64
	}{
		{"zero", []byte{0, 0, 0, 0, 0, 0, 0, 0}, 0.0},
		{"one", []byte{0, 0, 0, 0, 0, 0, 0xf0, 0x3f}, 1.0},
		{"too short", []byte{0, 0, 0, 0}, 0},
		{"empty", []byte{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := decodeFixed64(tt.data)
			if result != tt.expected {
				t.Errorf("decodeFixed64(%v) = %f, want %f", tt.data, result, tt.expected)
			}
		})
	}
}

func TestSkipField(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		pos      int
		wireType uint64
		expected int
	}{
		{"varint single byte", []byte{0x01}, 0, 0, 1},
		{"varint multi byte", []byte{0x80, 0x01}, 0, 0, 2},
		{"fixed64", []byte{0, 0, 0, 0, 0, 0, 0, 0}, 0, 1, 8},
		{"fixed32", []byte{0, 0, 0, 0}, 0, 5, 4},
		{"length-delimited", []byte{0x03, 'a', 'b', 'c'}, 0, 2, 4},
		{"unknown wire type", []byte{0x00}, 0, 99, -1},
		{"varint incomplete", []byte{0x80}, 0, 0, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := skipField(tt.data, tt.pos, tt.wireType)
			if result != tt.expected {
				t.Errorf("skipField(%v, %d, %d) = %d, want %d", tt.data, tt.pos, tt.wireType, result, tt.expected)
			}
		})
	}
}

func TestParseSchemaForMinMax(t *testing.T) {
	// Build test data with field 11 (maximum) and field 13 (minimum) as fixed64
	// Field tag = (fieldNum << 3) | wireType
	// Field 11, wireType 1 (fixed64): tag = (11 << 3) | 1 = 89 = 0x59
	// Field 13, wireType 1 (fixed64): tag = (13 << 3) | 1 = 105 = 0x69

	tests := []struct {
		name           string
		data           []byte
		expectedHasMax bool
		expectedHasMin bool
	}{
		{
			name:           "empty",
			data:           []byte{},
			expectedHasMax: false,
			expectedHasMin: false,
		},
		{
			name:           "has maximum only",
			data:           append([]byte{0x59}, make([]byte, 8)...), // field 11, wireType 1, 8 bytes
			expectedHasMax: true,
			expectedHasMin: false,
		},
		{
			name:           "has minimum only",
			data:           append([]byte{0x69}, make([]byte, 8)...), // field 13, wireType 1, 8 bytes
			expectedHasMax: false,
			expectedHasMin: true,
		},
		{
			name: "has both",
			data: func() []byte {
				d := append([]byte{0x59}, make([]byte, 8)...) // max
				d = append(d, 0x69)                           // min tag
				d = append(d, make([]byte, 8)...)             // min value
				return d
			}(),
			expectedHasMax: true,
			expectedHasMin: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasMax, hasMin := parseSchemaForMinMax(tt.data)
			if hasMax != tt.expectedHasMax || hasMin != tt.expectedHasMin {
				t.Errorf("parseSchemaForMinMax() = (%v, %v), want (%v, %v)", hasMax, hasMin, tt.expectedHasMax, tt.expectedHasMin)
			}
		})
	}
}

func TestDetectMinMaxFromRaw(t *testing.T) {
	// Extension field number 1143, wireType 2 (length-delimited)
	// Tag = (1143 << 3) | 2 = 9146 = 0x23BA (as varint: 0xBA 0x47)

	tests := []struct {
		name           string
		data           []byte
		expectedHasMax bool
		expectedHasMin bool
	}{
		{
			name:           "empty",
			data:           []byte{},
			expectedHasMax: false,
			expectedHasMin: false,
		},
		{
			name:           "no extension",
			data:           []byte{0x08, 0x01}, // some other field
			expectedHasMax: false,
			expectedHasMin: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasMax, hasMin := detectMinMaxFromRaw(tt.data)
			if hasMax != tt.expectedHasMax || hasMin != tt.expectedHasMin {
				t.Errorf("detectMinMaxFromRaw() = (%v, %v), want (%v, %v)", hasMax, hasMin, tt.expectedHasMax, tt.expectedHasMin)
			}
		})
	}
}

func TestJSONSchemaMarshaling(t *testing.T) {
	maxVal := float64(100)
	minVal := float64(0)
	readOnly := true
	additionalProps := true

	schema := &JSONSchema{
		Schema:               "https://json-schema.org/draft/2020-12/schema",
		ID:                   "test.Message",
		Type:                 "object",
		Description:          "Test message",
		Properties:           map[string]*JSONSchema{"field": {Type: "string"}},
		Required:             []string{"field"},
		Maximum:              &maxVal,
		Minimum:              &minVal,
		Default:              "default",
		ReadOnly:             &readOnly,
		AdditionalProperties: &additionalProps,
	}

	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Failed to marshal JSONSchema: %v", err)
	}

	var unmarshaled JSONSchema
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal JSONSchema: %v", err)
	}

	if unmarshaled.Schema != schema.Schema {
		t.Errorf("Schema mismatch: got %q, want %q", unmarshaled.Schema, schema.Schema)
	}
	if unmarshaled.ID != schema.ID {
		t.Errorf("ID mismatch: got %q, want %q", unmarshaled.ID, schema.ID)
	}
	if unmarshaled.Type != schema.Type {
		t.Errorf("Type mismatch: got %q, want %q", unmarshaled.Type, schema.Type)
	}
	if *unmarshaled.Maximum != maxVal {
		t.Errorf("Maximum mismatch: got %f, want %f", *unmarshaled.Maximum, maxVal)
	}
	if *unmarshaled.Minimum != minVal {
		t.Errorf("Minimum mismatch: got %f, want %f", *unmarshaled.Minimum, minVal)
	}
}

func TestJSONSchemaOmitEmpty(t *testing.T) {
	// Test that empty/nil fields are omitted
	schema := &JSONSchema{
		Type: "string",
	}

	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Should only have "type"
	if len(m) != 1 {
		t.Errorf("Expected 1 field, got %d: %v", len(m), m)
	}
	if m["type"] != "string" {
		t.Errorf("Expected type=string, got %v", m["type"])
	}
}

func TestDecodeFixed64Values(t *testing.T) {
	// Test specific float64 values
	tests := []struct {
		name  string
		value float64
	}{
		{"zero", 0.0},
		{"one", 1.0},
		{"negative", -1.0},
		{"pi", math.Pi},
		{"max", math.MaxFloat64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			bits := math.Float64bits(tt.value)
			data := make([]byte, 8)
			for i := 0; i < 8; i++ {
				data[i] = byte(bits >> (i * 8))
			}

			// Decode
			result := decodeFixed64(data)
			if result != tt.value {
				t.Errorf("decodeFixed64 roundtrip failed: got %f, want %f", result, tt.value)
			}
		})
	}
}

// =============================================================================
// Integration Tests (using protoc subprocess)
// =============================================================================

// TestGeneratedSchemas compares generated schema files against expected test files
func TestGeneratedSchemas(t *testing.T) {
	testFiles := []string{
		"furo.cube.Colour",
		"furo.cube.CubeDefinition",
		"furo.cube.CubeEntity",
		"furo.cube.CubeServiceGetListRequest",
		"furo.cube.CubeServiceGetRequest",
		"furo.cube.Materials",
		"google.protobuf.Value",
		"furo.type.Identifier",
		"blueberry.llm.FunctionParameters",
	}

	for _, name := range testFiles {
		t.Run(name, func(t *testing.T) {
			expectedPath := filepath.Join("testfiles", name)
			generatedPath := filepath.Join("open-models", "schema", name)

			expectedData, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("failed to read expected file %s: %v", expectedPath, err)
			}

			generatedData, err := os.ReadFile(generatedPath)
			if err != nil {
				t.Fatalf("failed to read generated file %s: %v", generatedPath, err)
			}

			// Parse both as JSON to compare semantically (ignoring whitespace and key order)
			var expected, generated map[string]interface{}

			if err := json.Unmarshal(expectedData, &expected); err != nil {
				t.Fatalf("failed to parse expected JSON %s: %v", expectedPath, err)
			}

			if err := json.Unmarshal(generatedData, &generated); err != nil {
				t.Fatalf("failed to parse generated JSON %s: %v", generatedPath, err)
			}

			// Compare each top-level field
			compareSchemas(t, name, expected, generated)
		})
	}
}

// TestGeneratedSchemasStrict compares generated strict schema files against expected test files
func TestGeneratedSchemasStrict(t *testing.T) {
	testFiles := []string{
		"google.protobuf.Value",
		"furo.type.Identifier",
		"blueberry.llm.FunctionParameters",
		"google.protobuf.Any",
	}

	for _, name := range testFiles {
		t.Run(name, func(t *testing.T) {
			expectedPath := filepath.Join("testfiles-strict", name)
			generatedPath := filepath.Join("open-models", "schema-strict", name)

			expectedData, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("failed to read expected file %s: %v", expectedPath, err)
			}

			generatedData, err := os.ReadFile(generatedPath)
			if err != nil {
				t.Fatalf("failed to read generated file %s: %v", generatedPath, err)
			}

			// Parse both as JSON to compare semantically (ignoring whitespace and key order)
			var expected, generated map[string]interface{}

			if err := json.Unmarshal(expectedData, &expected); err != nil {
				t.Fatalf("failed to parse expected JSON %s: %v", expectedPath, err)
			}

			if err := json.Unmarshal(generatedData, &generated); err != nil {
				t.Fatalf("failed to parse generated JSON %s: %v", generatedPath, err)
			}

			// Compare each top-level field
			compareSchemas(t, name, expected, generated)
		})
	}
}

func compareSchemas(t *testing.T, name string, expected, generated map[string]interface{}) {
	t.Helper()

	// Check $schema
	if exp, gen := expected["$schema"], generated["$schema"]; exp != gen {
		t.Errorf("%s: $schema mismatch: expected %q, got %q", name, exp, gen)
	}

	// Check $id
	if exp, gen := expected["$id"], generated["$id"]; exp != gen {
		t.Errorf("%s: $id mismatch: expected %q, got %q", name, exp, gen)
	}

	// Check type
	if exp, gen := expected["type"], generated["type"]; exp != gen {
		t.Errorf("%s: type mismatch: expected %q, got %q", name, exp, gen)
	}

	// Check properties exist in both
	expProps, expHasProps := expected["properties"].(map[string]interface{})
	genProps, genHasProps := generated["properties"].(map[string]interface{})

	if expHasProps != genHasProps {
		t.Errorf("%s: properties presence mismatch: expected has=%v, generated has=%v", name, expHasProps, genHasProps)
		return
	}

	if expHasProps {
		compareProperties(t, name, expProps, genProps)
	}

	// Check enum values for enum types
	if expEnum, ok := expected["enum"].([]interface{}); ok {
		genEnum, genOk := generated["enum"].([]interface{})
		if !genOk {
			t.Errorf("%s: expected enum but generated has none", name)
		} else {
			compareEnumValues(t, name, expEnum, genEnum)
		}
	}
}

func compareProperties(t *testing.T, schemaName string, expected, generated map[string]interface{}) {
	t.Helper()

	// Check that all expected properties exist in generated
	for propName, expProp := range expected {
		genProp, exists := generated[propName]
		if !exists {
			t.Errorf("%s: missing property %q in generated schema", schemaName, propName)
			continue
		}

		expPropMap, expOk := expProp.(map[string]interface{})
		genPropMap, genOk := genProp.(map[string]interface{})

		if !expOk || !genOk {
			t.Errorf("%s.%s: property is not an object", schemaName, propName)
			continue
		}

		compareProperty(t, schemaName, propName, expPropMap, genPropMap)
	}

	// Check for extra properties in generated (informational, not an error)
	for propName := range generated {
		if _, exists := expected[propName]; !exists {
			t.Logf("%s: extra property %q in generated schema (not in expected)", schemaName, propName)
		}
	}
}

func compareProperty(t *testing.T, schemaName, propName string, expected, generated map[string]interface{}) {
	t.Helper()

	// Check type
	if exp, gen := expected["type"], generated["type"]; exp != nil && exp != gen {
		t.Errorf("%s.%s: type mismatch: expected %q, got %q", schemaName, propName, exp, gen)
	}

	// Check $ref
	if exp, gen := expected["$ref"], generated["$ref"]; exp != nil && exp != gen {
		t.Errorf("%s.%s: $ref mismatch: expected %q, got %q", schemaName, propName, exp, gen)
	}

	// Check maximum
	if exp := expected["maximum"]; exp != nil {
		gen := generated["maximum"]
		if !numbersEqual(exp, gen) {
			t.Errorf("%s.%s: maximum mismatch: expected %v, got %v", schemaName, propName, exp, gen)
		}
	}

	// Check minimum
	if exp := expected["minimum"]; exp != nil {
		gen := generated["minimum"]
		if !numbersEqual(exp, gen) {
			t.Errorf("%s.%s: minimum mismatch: expected %v, got %v", schemaName, propName, exp, gen)
		}
	}

	// Check readOnly
	if exp := expected["readOnly"]; exp != nil {
		gen := generated["readOnly"]
		if exp != gen {
			t.Errorf("%s.%s: readOnly mismatch: expected %v, got %v", schemaName, propName, exp, gen)
		}
	}

	// Check default (informational - some test files don't include defaults)
	if exp := expected["default"]; exp != nil {
		gen := generated["default"]
		if gen == nil {
			t.Logf("%s.%s: expected has default %v but generated has none", schemaName, propName, exp)
		}
	}

	// Note: description is intentionally not compared strictly as it may vary
	// based on comment source location
}

func compareEnumValues(t *testing.T, schemaName string, expected, generated []interface{}) {
	t.Helper()

	if len(expected) != len(generated) {
		t.Errorf("%s: enum length mismatch: expected %d values, got %d", schemaName, len(expected), len(generated))
		return
	}

	// Create a set of expected values
	expSet := make(map[string]bool)
	for _, v := range expected {
		if s, ok := v.(string); ok {
			expSet[s] = true
		}
	}

	// Check all generated values exist in expected
	for _, v := range generated {
		if s, ok := v.(string); ok {
			if !expSet[s] {
				t.Errorf("%s: unexpected enum value %q", schemaName, s)
			}
		}
	}

	// Check order matches
	for i, exp := range expected {
		if exp != generated[i] {
			t.Errorf("%s: enum value order mismatch at index %d: expected %q, got %q", schemaName, i, exp, generated[i])
		}
	}
}

func numbersEqual(a, b interface{}) bool {
	// Handle JSON number comparisons (may be float64 or int)
	af, aOk := toFloat64(a)
	bf, bOk := toFloat64(b)
	if aOk && bOk {
		return af == bf
	}
	return a == b
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// TestGeneration runs the full generation pipeline and verifies output
func TestGeneration(t *testing.T) {
	// Skip if protoc is not available
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not found in PATH, skipping generation test")
	}

	// Create a temporary output directory
	tmpDir, err := os.MkdirTemp("", "jsonschema-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build the plugin
	pluginPath := filepath.Join(tmpDir, "protoc-gen-om-jsonschema")
	buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build plugin: %v\n%s", err, output)
	}

	// Run protoc with only the cube proto files
	schemaOutDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaOutDir, 0755); err != nil {
		t.Fatalf("failed to create schema output dir: %v", err)
	}

	protocCmd := exec.Command("protoc",
		"--plugin=protoc-gen-om-jsonschema="+pluginPath,
		"-I./proto_dependencies",
		"-I./proto",
		"--om-jsonschema_out="+schemaOutDir,
		"./proto/furo/cube/cube.proto",
		"./proto/furo/cube/cubeservice.proto",
		"./proto/furo/cube/colours.proto",
		"./proto/furo/cube/enums.proto",
	)
	if output, err := protocCmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, output)
	}

	// Verify expected files were generated
	expectedFiles := []string{
		"furo.cube.Colour",
		"furo.cube.CubeDefinition",
		"furo.cube.CubeEntity",
		"furo.cube.CubeServiceGetListRequest",
		"furo.cube.CubeServiceGetRequest",
		"furo.cube.Materials",
	}

	for _, name := range expectedFiles {
		path := filepath.Join(schemaOutDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s was not generated", name)
			continue
		}

		// Verify it's valid JSON
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("failed to read %s: %v", name, err)
			continue
		}

		var schema map[string]interface{}
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Errorf("%s is not valid JSON: %v", name, err)
			continue
		}

		// Verify required fields exist
		if schema["$schema"] == nil {
			t.Errorf("%s missing $schema", name)
		}
		if schema["$id"] == nil {
			t.Errorf("%s missing $id", name)
		}
		if schema["type"] == nil {
			t.Errorf("%s missing type", name)
		}
	}
}

// TestStrictAnyOption tests the strict_any option generates standard protobuf Any schema
func TestStrictAnyOption(t *testing.T) {
	// Skip if protoc is not available
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not found in PATH, skipping strict_any test")
	}

	// Create a temporary output directory
	tmpDir, err := os.MkdirTemp("", "jsonschema-strict-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build the plugin
	pluginPath := filepath.Join(tmpDir, "protoc-gen-om-jsonschema")
	buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build plugin: %v\n%s", err, output)
	}

	// Run protoc with strict_any option
	schemaOutDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaOutDir, 0755); err != nil {
		t.Fatalf("failed to create schema output dir: %v", err)
	}

	protocCmd := exec.Command("protoc",
		"--plugin=protoc-gen-om-jsonschema="+pluginPath,
		"-I./proto_dependencies",
		"-I./proto",
		"--om-jsonschema_out="+schemaOutDir,
		"--om-jsonschema_opt=strict_any",
		"./proto/furo/cube/cube.proto",
	)
	if output, err := protocCmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, output)
	}

	// Check that google.protobuf.Any was generated with strict format
	anyPath := filepath.Join(schemaOutDir, "google.protobuf.Any")
	data, err := os.ReadFile(anyPath)
	if err != nil {
		t.Fatalf("failed to read google.protobuf.Any: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("google.protobuf.Any is not valid JSON: %v", err)
	}

	// Verify strict format: should have typeUrl and value, NOT @type
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("google.protobuf.Any missing properties")
	}

	if _, hasTypeUrl := props["typeUrl"]; !hasTypeUrl {
		t.Error("strict_any: expected typeUrl property")
	}

	if _, hasValue := props["value"]; !hasValue {
		t.Error("strict_any: expected value property")
	}

	if _, hasAtType := props["@type"]; hasAtType {
		t.Error("strict_any: should NOT have @type property")
	}

	// Verify no additionalProperties (or it's not true)
	if additionalProps, exists := schema["additionalProperties"]; exists {
		if additionalProps == true {
			t.Error("strict_any: should NOT have additionalProperties: true")
		}
	}
}

// TestDefaultAnyFormat tests the default AI-friendly Any schema
func TestDefaultAnyFormat(t *testing.T) {
	// Skip if protoc is not available
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not found in PATH, skipping default Any test")
	}

	// Create a temporary output directory
	tmpDir, err := os.MkdirTemp("", "jsonschema-default-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build the plugin
	pluginPath := filepath.Join(tmpDir, "protoc-gen-om-jsonschema")
	buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build plugin: %v\n%s", err, output)
	}

	// Run protoc WITHOUT strict_any option
	schemaOutDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaOutDir, 0755); err != nil {
		t.Fatalf("failed to create schema output dir: %v", err)
	}

	protocCmd := exec.Command("protoc",
		"--plugin=protoc-gen-om-jsonschema="+pluginPath,
		"-I./proto_dependencies",
		"-I./proto",
		"--om-jsonschema_out="+schemaOutDir,
		"./proto/furo/cube/cube.proto",
	)
	if output, err := protocCmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, output)
	}

	// Check that google.protobuf.Any was generated with AI-friendly format
	anyPath := filepath.Join(schemaOutDir, "google.protobuf.Any")
	data, err := os.ReadFile(anyPath)
	if err != nil {
		t.Fatalf("failed to read google.protobuf.Any: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("google.protobuf.Any is not valid JSON: %v", err)
	}

	// Verify AI-friendly format: should have @type, NOT typeUrl/value
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("google.protobuf.Any missing properties")
	}

	if _, hasAtType := props["@type"]; !hasAtType {
		t.Error("default: expected @type property")
	}

	if _, hasTypeUrl := props["typeUrl"]; hasTypeUrl {
		t.Error("default: should NOT have typeUrl property")
	}

	if _, hasValue := props["value"]; hasValue {
		t.Error("default: should NOT have value property")
	}

	// Verify additionalProperties: true
	if additionalProps, exists := schema["additionalProperties"]; !exists || additionalProps != true {
		t.Error("default: expected additionalProperties: true")
	}

	// Verify required: ["@type"]
	required, ok := schema["required"].([]interface{})
	if !ok || len(required) != 1 || required[0] != "@type" {
		t.Error("default: expected required: [\"@type\"]")
	}
}

// TestStrictMapOption tests the strict_map option generates Entry types for maps
func TestStrictMapOption(t *testing.T) {
	// Skip if protoc is not available
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not found in PATH, skipping strict_map test")
	}

	// Create a temporary output directory
	tmpDir, err := os.MkdirTemp("", "jsonschema-strict-map-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build the plugin
	pluginPath := filepath.Join(tmpDir, "protoc-gen-om-jsonschema")
	buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build plugin: %v\n%s", err, output)
	}

	// Run protoc with strict_map option
	schemaOutDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaOutDir, 0755); err != nil {
		t.Fatalf("failed to create schema output dir: %v", err)
	}

	protocCmd := exec.Command("protoc",
		"--plugin=protoc-gen-om-jsonschema="+pluginPath,
		"-I./proto_dependencies",
		"-I./proto",
		"--om-jsonschema_out="+schemaOutDir,
		"--om-jsonschema_opt=strict_map",
		"./proto/furo/type/identifier.proto",
	)
	if output, err := protocCmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, output)
	}

	// Check that the Entry type was generated
	entryPath := filepath.Join(schemaOutDir, "furo.type.Identifier.AttributesEntry")
	if _, err := os.Stat(entryPath); os.IsNotExist(err) {
		t.Error("strict_map: expected AttributesEntry file to be generated")
	}

	// Check that the Identifier schema uses $ref to Entry type
	identifierPath := filepath.Join(schemaOutDir, "furo.type.Identifier")
	data, err := os.ReadFile(identifierPath)
	if err != nil {
		t.Fatalf("failed to read furo.type.Identifier: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("furo.type.Identifier is not valid JSON: %v", err)
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("furo.type.Identifier missing properties")
	}

	attrProp, ok := props["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("furo.type.Identifier missing attributes property")
	}

	// In strict mode, should have $ref to Entry type, NOT additionalProperties
	if ref, hasRef := attrProp["$ref"]; !hasRef || ref != "furo.type.Identifier.AttributesEntry" {
		t.Errorf("strict_map: expected $ref to AttributesEntry, got %v", attrProp)
	}

	if _, hasAdditional := attrProp["additionalProperties"]; hasAdditional {
		t.Error("strict_map: should NOT have additionalProperties")
	}
}

// TestDefaultMapFormat tests the default AI-friendly map schema with additionalProperties
func TestDefaultMapFormat(t *testing.T) {
	// Skip if protoc is not available
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not found in PATH, skipping default map test")
	}

	// Create a temporary output directory
	tmpDir, err := os.MkdirTemp("", "jsonschema-default-map-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build the plugin
	pluginPath := filepath.Join(tmpDir, "protoc-gen-om-jsonschema")
	buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build plugin: %v\n%s", err, output)
	}

	// Run protoc WITHOUT strict_map option
	schemaOutDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaOutDir, 0755); err != nil {
		t.Fatalf("failed to create schema output dir: %v", err)
	}

	protocCmd := exec.Command("protoc",
		"--plugin=protoc-gen-om-jsonschema="+pluginPath,
		"-I./proto_dependencies",
		"-I./proto",
		"--om-jsonschema_out="+schemaOutDir,
		"./proto/furo/type/identifier.proto",
	)
	if output, err := protocCmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, output)
	}

	// Check that NO Entry type was generated
	entryPath := filepath.Join(schemaOutDir, "furo.type.Identifier.AttributesEntry")
	if _, err := os.Stat(entryPath); !os.IsNotExist(err) {
		t.Error("default: AttributesEntry file should NOT be generated")
	}

	// Check that the Identifier schema uses additionalProperties
	identifierPath := filepath.Join(schemaOutDir, "furo.type.Identifier")
	data, err := os.ReadFile(identifierPath)
	if err != nil {
		t.Fatalf("failed to read furo.type.Identifier: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("furo.type.Identifier is not valid JSON: %v", err)
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("furo.type.Identifier missing properties")
	}

	attrProp, ok := props["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("furo.type.Identifier missing attributes property")
	}

	// In default mode, should have type: object and additionalProperties
	if attrProp["type"] != "object" {
		t.Errorf("default: expected type: object, got %v", attrProp["type"])
	}

	additionalProps, hasAdditional := attrProp["additionalProperties"].(map[string]interface{})
	if !hasAdditional {
		t.Error("default: expected additionalProperties")
	} else if additionalProps["type"] != "string" {
		t.Errorf("default: expected additionalProperties.type: string, got %v", additionalProps["type"])
	}

	// Should NOT have $ref to Entry type
	if _, hasRef := attrProp["$ref"]; hasRef {
		t.Error("default: should NOT have $ref to Entry type")
	}
}

// TestDefaultOneofFormat tests the default x-oneof extension for oneof groups
func TestDefaultOneofFormat(t *testing.T) {
	// Skip if protoc is not available
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not found in PATH, skipping default oneof test")
	}

	// Create a temporary output directory
	tmpDir, err := os.MkdirTemp("", "jsonschema-default-oneof-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build the plugin
	pluginPath := filepath.Join(tmpDir, "protoc-gen-om-jsonschema")
	buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build plugin: %v\n%s", err, output)
	}

	// Run protoc WITHOUT strict_oneof option (uses google.protobuf.Value which has oneof)
	schemaOutDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaOutDir, 0755); err != nil {
		t.Fatalf("failed to create schema output dir: %v", err)
	}

	protocCmd := exec.Command("protoc",
		"--plugin=protoc-gen-om-jsonschema="+pluginPath,
		"-I./proto_dependencies",
		"-I./proto",
		"--om-jsonschema_out="+schemaOutDir,
		"./proto_dependencies/google/protobuf/struct.proto",
	)
	if output, err := protocCmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, output)
	}

	// Check that google.protobuf.Value has x-oneof
	valuePath := filepath.Join(schemaOutDir, "google.protobuf.Value")
	data, err := os.ReadFile(valuePath)
	if err != nil {
		t.Fatalf("failed to read google.protobuf.Value: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("google.protobuf.Value is not valid JSON: %v", err)
	}

	// Verify x-oneof exists
	xOneof, hasXOneof := schema["x-oneof"].([]interface{})
	if !hasXOneof {
		t.Fatal("default: expected x-oneof extension")
	}

	if len(xOneof) == 0 {
		t.Error("default: x-oneof should have at least one group")
	}

	// Check first group has expected structure {name: "groupName", fields: [...]}
	if group, ok := xOneof[0].(map[string]interface{}); ok {
		name, hasName := group["name"].(string)
		fields, hasFields := group["fields"].([]interface{})
		if !hasName || !hasFields {
			t.Error("default: x-oneof group should have 'name' and 'fields' properties")
		}
		if name != "kind" {
			t.Errorf("default: expected oneof group name 'kind', got %v", name)
		}
		if len(fields) < 2 {
			t.Error("default: x-oneof group should have at least two fields")
		}
	} else {
		t.Error("default: x-oneof group should be an object with 'name' and 'fields'")
	}

	// Should have IMPORTANT hint in description (not the strict ONEOF[] format)
	desc, _ := schema["description"].(string)
	if !strings.Contains(desc, "IMPORTANT:") {
		t.Error("default: description should contain IMPORTANT hint for x-oneof")
	}
	if strings.Contains(desc, "ONEOF[") {
		t.Error("default: description should NOT contain strict ONEOF[] format")
	}
}

// TestStrictOneofOption tests the strict_oneof option with description hint
func TestStrictOneofOption(t *testing.T) {
	// Skip if protoc is not available
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not found in PATH, skipping strict_oneof test")
	}

	// Create a temporary output directory
	tmpDir, err := os.MkdirTemp("", "jsonschema-strict-oneof-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build the plugin
	pluginPath := filepath.Join(tmpDir, "protoc-gen-om-jsonschema")
	buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build plugin: %v\n%s", err, output)
	}

	// Run protoc WITH strict_oneof option
	schemaOutDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaOutDir, 0755); err != nil {
		t.Fatalf("failed to create schema output dir: %v", err)
	}

	protocCmd := exec.Command("protoc",
		"--plugin=protoc-gen-om-jsonschema="+pluginPath,
		"-I./proto_dependencies",
		"-I./proto",
		"--om-jsonschema_out="+schemaOutDir,
		"--om-jsonschema_opt=strict_oneof",
		"./proto_dependencies/google/protobuf/struct.proto",
	)
	if output, err := protocCmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, output)
	}

	// Check that google.protobuf.Value has description hint
	valuePath := filepath.Join(schemaOutDir, "google.protobuf.Value")
	data, err := os.ReadFile(valuePath)
	if err != nil {
		t.Fatalf("failed to read google.protobuf.Value: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("google.protobuf.Value is not valid JSON: %v", err)
	}

	// Verify description contains ONEOF hint
	desc, hasDesc := schema["description"].(string)
	if !hasDesc {
		t.Fatal("strict_oneof: expected description")
	}

	if !strings.Contains(desc, "ONEOF[kind]") {
		t.Error("strict_oneof: description should contain ONEOF[kind] hint")
	}

	if !strings.Contains(desc, "Set exactly ONE of") {
		t.Error("strict_oneof: description should contain 'Set exactly ONE of'")
	}

	// Should NOT have x-oneof
	if _, hasXOneof := schema["x-oneof"]; hasXOneof {
		t.Error("strict_oneof: should NOT have x-oneof extension")
	}
}

// TestExcludePackages tests the exclude_packages option
func TestExcludePackages(t *testing.T) {
	// Skip if protoc is not available
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not found in PATH, skipping exclude packages test")
	}

	// Create a temporary output directory
	tmpDir, err := os.MkdirTemp("", "jsonschema-exclude-packages-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build the plugin
	pluginPath := filepath.Join(tmpDir, "protoc-gen-om-jsonschema")
	buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build plugin: %v\n%s", err, output)
	}

	// Run protoc with exclude_packages option
	schemaOutDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaOutDir, 0755); err != nil {
		t.Fatalf("failed to create schema output dir: %v", err)
	}

	protocCmd := exec.Command("protoc",
		"--plugin=protoc-gen-om-jsonschema="+pluginPath,
		"-I./proto_dependencies",
		"-I./proto",
		"--om-jsonschema_out="+schemaOutDir,
		"--om-jsonschema_opt=exclude_packages=google.api;openapi.v3",
		"./proto_dependencies/google/protobuf/struct.proto",
		"./proto_dependencies/google/api/http.proto",
		"./proto_dependencies/openapiv3/openapiv3.proto",
	)
	if output, err := protocCmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, output)
	}

	// Verify google.api types were NOT generated
	files, _ := filepath.Glob(filepath.Join(schemaOutDir, "google.api.*"))
	if len(files) > 0 {
		t.Errorf("exclude_packages: google.api.* files should not exist, found: %v", files)
	}

	// Verify openapi.v3 types were NOT generated
	files, _ = filepath.Glob(filepath.Join(schemaOutDir, "openapi.v3.*"))
	if len(files) > 0 {
		t.Errorf("exclude_packages: openapi.v3.* files should not exist, found: %v", files)
	}

	// Verify google.protobuf types WERE generated (not excluded)
	valuePath := filepath.Join(schemaOutDir, "google.protobuf.Value")
	if _, err := os.Stat(valuePath); os.IsNotExist(err) {
		t.Error("exclude_packages: google.protobuf.Value should exist (not excluded)")
	}
}

// TestExcludeMessages tests the exclude_messages option
func TestExcludeMessages(t *testing.T) {
	// Skip if protoc is not available
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not found in PATH, skipping exclude messages test")
	}

	// Create a temporary output directory
	tmpDir, err := os.MkdirTemp("", "jsonschema-exclude-messages-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build the plugin
	pluginPath := filepath.Join(tmpDir, "protoc-gen-om-jsonschema")
	buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build plugin: %v\n%s", err, output)
	}

	// Run protoc with exclude_messages option
	schemaOutDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaOutDir, 0755); err != nil {
		t.Fatalf("failed to create schema output dir: %v", err)
	}

	protocCmd := exec.Command("protoc",
		"--plugin=protoc-gen-om-jsonschema="+pluginPath,
		"-I./proto_dependencies",
		"-I./proto",
		"--om-jsonschema_out="+schemaOutDir,
		"--om-jsonschema_opt=exclude_messages=google.protobuf.Value;google.protobuf.ListValue",
		"./proto_dependencies/google/protobuf/struct.proto",
	)
	if output, err := protocCmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, output)
	}

	// Verify excluded messages were NOT generated
	valuePath := filepath.Join(schemaOutDir, "google.protobuf.Value")
	if _, err := os.Stat(valuePath); !os.IsNotExist(err) {
		t.Error("exclude_messages: google.protobuf.Value should NOT exist (excluded)")
	}

	listValuePath := filepath.Join(schemaOutDir, "google.protobuf.ListValue")
	if _, err := os.Stat(listValuePath); !os.IsNotExist(err) {
		t.Error("exclude_messages: google.protobuf.ListValue should NOT exist (excluded)")
	}

	// Verify other messages WERE generated (not excluded)
	structPath := filepath.Join(schemaOutDir, "google.protobuf.Struct")
	if _, err := os.Stat(structPath); os.IsNotExist(err) {
		t.Error("exclude_messages: google.protobuf.Struct should exist (not excluded)")
	}
}

// TestParseExclusionList tests the parseExclusionList helper function
func TestParseExclusionList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: map[string]bool{},
		},
		{
			name:     "single item",
			input:    "google.api",
			expected: map[string]bool{"google.api": true},
		},
		{
			name:     "multiple items",
			input:    "google.api;openapi.v3;mypackage",
			expected: map[string]bool{"google.api": true, "openapi.v3": true, "mypackage": true},
		},
		{
			name:     "with whitespace",
			input:    " google.api ; openapi.v3 ",
			expected: map[string]bool{"google.api": true, "openapi.v3": true},
		},
		{
			name:     "empty items ignored",
			input:    "google.api;;openapi.v3",
			expected: map[string]bool{"google.api": true, "openapi.v3": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseExclusionList(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d items, got %d", len(tt.expected), len(result))
			}
			for key := range tt.expected {
				if !result[key] {
					t.Errorf("expected key %q to be present", key)
				}
			}
		})
	}
}

// =============================================================================
// Coverage Tests using processRequest directly
// These tests use a captured CodeGeneratorRequest to test schema generation
// without running protoc as a subprocess, enabling coverage measurement.
// =============================================================================

// TestProcessRequestCoverage tests the processRequest function directly
// using a previously captured CodeGeneratorRequest file.
func TestProcessRequestCoverage(t *testing.T) {
	// Read the captured request file
	requestFile := "testfiles/request.bin"
	data, err := os.ReadFile(requestFile)
	if err != nil {
		t.Skipf("Skipping coverage test: %v (run './test-gen-open-models.sh' with debug option first)", err)
	}

	// Process the request
	var output bytes.Buffer
	err = processRequest(data, &output)
	if err != nil {
		t.Fatalf("processRequest failed: %v", err)
	}

	// Parse the response
	resp := &pluginpb.CodeGeneratorResponse{}
	if err := proto.Unmarshal(output.Bytes(), resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify we got some files
	if len(resp.File) == 0 {
		t.Fatal("Expected at least one generated file")
	}

	// Check for expected schema files
	expectedSchemas := []string{
		"google.protobuf.Any",
		"google.protobuf.Value",
		"google.protobuf.Struct",
	}

	fileNames := make(map[string]bool)
	for _, f := range resp.File {
		fileNames[f.GetName()] = true
	}

	for _, expected := range expectedSchemas {
		if !fileNames[expected] {
			t.Errorf("Expected schema %q not found in response", expected)
		}
	}

	// Verify the google.protobuf.Any schema content
	for _, f := range resp.File {
		if f.GetName() == "google.protobuf.Any" {
			var schema map[string]interface{}
			if err := json.Unmarshal([]byte(f.GetContent()), &schema); err != nil {
				t.Fatalf("Failed to parse google.protobuf.Any schema: %v", err)
			}
			// Default mode should have @type property
			props, ok := schema["properties"].(map[string]interface{})
			if !ok {
				t.Fatal("Expected properties in google.protobuf.Any schema")
			}
			if _, hasAtType := props["@type"]; !hasAtType {
				t.Error("Default mode should have @type property in google.protobuf.Any")
			}
			break
		}
	}
}

// TestProcessRequestWithStrictOptions tests processRequest with strict options
func TestProcessRequestWithStrictOptions(t *testing.T) {
	requestFile := "testfiles/request.bin"
	data, err := os.ReadFile(requestFile)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	// Modify the request to include strict options
	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	// Set strict options
	param := "strict_any,strict_map,strict_oneof"
	req.Parameter = &param

	// Re-marshal the modified request
	modifiedData, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal modified request: %v", err)
	}

	var output bytes.Buffer
	err = processRequest(modifiedData, &output)
	if err != nil {
		t.Fatalf("processRequest with strict options failed: %v", err)
	}

	// Parse and verify response
	resp := &pluginpb.CodeGeneratorResponse{}
	if err := proto.Unmarshal(output.Bytes(), resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify google.protobuf.Any has strict format
	for _, f := range resp.File {
		if f.GetName() == "google.protobuf.Any" {
			var schema map[string]interface{}
			if err := json.Unmarshal([]byte(f.GetContent()), &schema); err != nil {
				t.Fatalf("Failed to parse schema: %v", err)
			}
			props, ok := schema["properties"].(map[string]interface{})
			if !ok {
				t.Fatal("Expected properties in schema")
			}
			// Strict mode should have typeUrl, not @type
			if _, hasTypeUrl := props["typeUrl"]; !hasTypeUrl {
				t.Error("Strict mode should have typeUrl property")
			}
			if _, hasAtType := props["@type"]; hasAtType {
				t.Error("Strict mode should NOT have @type property")
			}
			break
		}
	}
}

// TestProcessRequestWithExclusions tests processRequest with exclusion options
func TestProcessRequestWithExclusions(t *testing.T) {
	requestFile := "testfiles/request.bin"
	data, err := os.ReadFile(requestFile)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	// Exclude google.api package and google.protobuf.FieldMask message
	param := "exclude_packages=google.api;openapi.v3,exclude_messages=google.protobuf.FieldMask"
	req.Parameter = &param

	modifiedData, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal modified request: %v", err)
	}

	var output bytes.Buffer
	err = processRequest(modifiedData, &output)
	if err != nil {
		t.Fatalf("processRequest with exclusions failed: %v", err)
	}

	resp := &pluginpb.CodeGeneratorResponse{}
	if err := proto.Unmarshal(output.Bytes(), resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Check that excluded files are not present
	for _, f := range resp.File {
		name := f.GetName()
		if strings.HasPrefix(name, "google.api.") {
			t.Errorf("google.api.* should be excluded, found: %s", name)
		}
		if strings.HasPrefix(name, "openapi.v3.") {
			t.Errorf("openapi.v3.* should be excluded, found: %s", name)
		}
		if name == "google.protobuf.FieldMask" {
			t.Error("google.protobuf.FieldMask should be excluded")
		}
	}

	// Verify some files were still generated
	fileNames := make(map[string]bool)
	for _, f := range resp.File {
		fileNames[f.GetName()] = true
	}
	if !fileNames["google.protobuf.Any"] {
		t.Error("google.protobuf.Any should still be generated")
	}
}

// TestApplyOption tests the applyOption helper function
func TestApplyOption(t *testing.T) {
	// Reset globals
	strictAny = false
	strictMap = false
	strictOneof = false
	excludePackages = ""
	excludeMessages = ""

	tests := []struct {
		name   string
		option string
		value  string
		check  func() bool
	}{
		{"strict_any true", "strict_any", "", func() bool { return strictAny }},
		{"strict_any false", "strict_any", "false", func() bool { return !strictAny }},
		{"strict_map", "strict_map", "", func() bool { return strictMap }},
		{"strict_oneof", "strict_oneof", "", func() bool { return strictOneof }},
		{"exclude_packages", "exclude_packages", "pkg1;pkg2", func() bool { return excludePackages == "pkg1;pkg2" }},
		{"exclude_messages", "exclude_messages", "msg1;msg2", func() bool { return excludeMessages == "msg1;msg2" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset before each test
			strictAny = false
			strictMap = false
			strictOneof = false
			excludePackages = ""
			excludeMessages = ""

			applyOption(tt.option, tt.value)
			if !tt.check() {
				t.Errorf("applyOption(%q, %q) did not set expected value", tt.option, tt.value)
			}
		})
	}
}

// TestIsPackageExcluded tests the isPackageExcluded function
func TestIsPackageExcluded(t *testing.T) {
	excludedPackageSet = map[string]bool{
		"google.api": true,
		"openapi.v3": true,
	}

	tests := []struct {
		pkg      string
		excluded bool
	}{
		{"google.api", true},
		{"openapi.v3", true},
		{"google.protobuf", false},
		{"mypackage", false},
	}

	for _, tt := range tests {
		t.Run(tt.pkg, func(t *testing.T) {
			if got := isPackageExcluded(tt.pkg); got != tt.excluded {
				t.Errorf("isPackageExcluded(%q) = %v, want %v", tt.pkg, got, tt.excluded)
			}
		})
	}
}

// TestIsMessageExcluded tests the isMessageExcluded function
func TestIsMessageExcluded(t *testing.T) {
	excludedMessageSet = map[string]bool{
		"google.protobuf.FieldMask": true,
		"mypackage.Internal":        true,
	}

	tests := []struct {
		msg      string
		excluded bool
	}{
		{"google.protobuf.FieldMask", true},
		{"mypackage.Internal", true},
		{"google.protobuf.Any", false},
		{"mypackage.Public", false},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			if got := isMessageExcluded(tt.msg); got != tt.excluded {
				t.Errorf("isMessageExcluded(%q) = %v, want %v", tt.msg, got, tt.excluded)
			}
		})
	}
}

// TestIsEnumExcluded tests the isEnumExcluded function
func TestIsEnumExcluded(t *testing.T) {
	excludedMessageSet = map[string]bool{
		"mypackage.Status": true,
	}

	tests := []struct {
		enum     string
		excluded bool
	}{
		{"mypackage.Status", true},
		{"mypackage.Type", false},
	}

	for _, tt := range tests {
		t.Run(tt.enum, func(t *testing.T) {
			if got := isEnumExcluded(tt.enum); got != tt.excluded {
				t.Errorf("isEnumExcluded(%q) = %v, want %v", tt.enum, got, tt.excluded)
			}
		})
	}
}
