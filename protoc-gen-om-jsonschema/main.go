// Package main implements a Protocol Buffers compiler plugin (protoc-gen-om-jsonschema)
// that generates JSON Schema files from protobuf message definitions.
//
// This plugin is designed to produce AI-friendly JSON Schemas that closely match
// the actual JSON wire format of protobuf messages. It prioritizes readability
// and usability over strict JSON Schema validation.
//
// Usage:
//
//	protoc --om-jsonschema_out=./output your_proto_files.proto
//
// Plugin Options:
//   - strict_any:   Use standard typeUrl/value format for google.protobuf.Any
//     instead of the AI-friendly @type format
//   - strict_map:   Use Entry types for maps instead of additionalProperties
//   - strict_oneof: Use description hints for oneof instead of x-oneof extension
//   - exclude_packages: Semicolon-separated list of packages to exclude (e.g., "google.api;openapi.v3")
//   - exclude_messages: Semicolon-separated list of messages to exclude (e.g., "mypackage.Internal")
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	openapi "github.com/google/gnostic/openapiv3"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// Plugin options control the output format of the generated schemas.
// These are set via protoc command line options like --om-jsonschema_opt=strict_any
var (
	// strictAny controls how google.protobuf.Any is represented in the schema.
	// When false (default): Uses @type field with additionalProperties (AI-friendly).
	// When true: Uses standard typeUrl/value fields from the proto definition.
	strictAny = false

	// strictMap controls how protobuf map fields are represented.
	// When false (default): Uses {"type": "object", "additionalProperties": {...}}
	// which matches the actual JSON wire format like {"key1": "value1"}.
	// When true: References separate *Entry message types with key/value properties.
	strictMap = false

	// strictOneof controls how protobuf oneof groups are indicated in the schema.
	// When false (default): Adds x-oneof extension array plus IMPORTANT hint in description.
	// When true: Only adds a concise ONEOF[name] hint in the description field.
	strictOneof = false

	// excludePackages is a semicolon-separated list of package names to exclude from generation.
	// Example: "google.api;openapi.v3" will skip all types in those packages.
	excludePackages = ""

	// excludeMessages is a semicolon-separated list of fully qualified message names to exclude.
	// Example: "mypackage.InternalMessage;mypackage.DebugInfo"
	excludeMessages = ""

	// Parsed exclusion lists (populated from the string options above)
	excludedPackageSet map[string]bool
	excludedMessageSet map[string]bool
)

// JSONSchema represents a JSON Schema document following the 2020-12 draft specification.
// This struct contains the subset of JSON Schema keywords needed to represent protobuf types.
// Each protobuf message and enum generates a separate JSON Schema file with this structure.
type JSONSchema struct {
	// Schema is the JSON Schema dialect identifier (always "https://json-schema.org/draft/2020-12/schema")
	Schema string `json:"$schema,omitempty"`

	// ID is the unique identifier for this schema, using the fully qualified protobuf name
	// (e.g., "google.protobuf.Any", "mypackage.MyMessage")
	ID string `json:"$id,omitempty"`

	// Ref is a reference to another schema by its $id, used for message and enum field types
	Ref string `json:"$ref,omitempty"`

	// Type is the JSON Schema type: "object" for messages, "string" for enums,
	// "array" for repeated fields, or primitive types for scalar fields
	Type string `json:"type,omitempty"`

	// Description contains the proto comment text, plus any oneof hints appended after " | "
	Description string `json:"description,omitempty"`

	// Properties maps JSON field names to their schemas (for type: "object")
	Properties map[string]*JSONSchema `json:"properties,omitempty"`

	// Items defines the element schema for array types (for type: "array")
	Items *JSONSchema `json:"items,omitempty"`

	// Required lists field names that must be present (populated from openapi.v3.schema annotation)
	Required []string `json:"required,omitempty"`

	// Enum lists allowed string values for enum types
	Enum []string `json:"enum,omitempty"`

	// Maximum is the maximum allowed numeric value (from openapi.v3.property annotation)
	Maximum *float64 `json:"maximum,omitempty"`

	// Minimum is the minimum allowed numeric value (from openapi.v3.property annotation,
	// or automatically set to 0 for unsigned integer types)
	Minimum *float64 `json:"minimum,omitempty"`

	// Default is the default value for this field (from openapi.v3.property annotation)
	Default interface{} `json:"default,omitempty"`

	// ReadOnly indicates the field is output-only (from openapi.v3.property annotation)
	ReadOnly *bool `json:"readOnly,omitempty"`

	// AdditionalProperties defines the schema for map values when using AI-friendly map format.
	// Can be bool (true to allow any additional properties) or *JSONSchema for typed values.
	// Used for: map<K,V> fields (default mode) and google.protobuf.Any (default mode).
	AdditionalProperties interface{} `json:"additionalProperties,omitempty"`

	// XOneof is a custom extension to indicate mutually exclusive field groups.
	// Each entry contains a group name and the list of field names in that group.
	// AI systems should set exactly ONE field from each group.
	// Only populated when strictOneof is false (default).
	XOneof []OneofGroup `json:"x-oneof,omitempty"`
}

// OneofGroup represents a protobuf oneof group with its name and member fields.
// This provides a clear, unambiguous structure for AI systems to understand
// which fields are mutually exclusive.
type OneofGroup struct {
	// Name is the oneof group name as defined in the proto file.
	Name string `json:"name"`

	// Fields lists the JSON field names that belong to this oneof group.
	// Exactly one of these fields should be set at a time.
	Fields []string `json:"fields"`
}

// main is the entry point for the protoc plugin.
// It sets up command-line flag parsing and runs the protobuf code generator.
//
// The plugin receives protobuf file descriptors from protoc via stdin and writes
// generated JSON Schema files to the output directory specified by --om-jsonschema_out.
//
// Debug mode: Use --replay-request=<path> to read a previously captured request file
// (from protoc-gen-debugfile) instead of stdin, enabling standalone debugging with IDE debuggers.
func main() {
	// Check for standalone debug mode (--replay-request flag)
	// This allows running the plugin directly without protoc for debugging
	replayRequest := ""
	for i, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--replay-request=") {
			replayRequest = strings.TrimPrefix(arg, "--replay-request=")
			// Remove this arg so it doesn't confuse the flag parser
			os.Args = append(os.Args[:i+1], os.Args[i+2:]...)
			break
		}
	}

	if replayRequest != "" {
		// Debug mode: read request from file instead of stdin
		if err := runFromFile(replayRequest); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Normal mode: run as protoc plugin
	runAsPlugin()
}

// runFromFile reads a previously dumped CodeGeneratorRequest from a file
// and processes it. This enables debugging the plugin with an IDE debugger.
func runFromFile(path string) error {
	return runFromFileWithOutput(path, os.Stdout)
}

// runFromFileWithOutput is the testable version of runFromFile that accepts
// a custom output writer for the CodeGeneratorResponse.
func runFromFileWithOutput(path string, output io.Writer) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read request file: %w", err)
	}

	return processRequest(data, output)
}

// processRequest processes a CodeGeneratorRequest and writes the response to output.
// This is the core processing function used by both runFromFile and runAsPlugin.
func processRequest(data []byte, output io.Writer) error {
	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return fmt.Errorf("failed to unmarshal request: %w", err)
	}

	// Reset global state for testing
	strictAny = false
	strictMap = false
	strictOneof = false
	excludePackages = ""
	excludeMessages = ""
	excludedPackageSet = nil
	excludedMessageSet = nil

	// Parse options from the request's parameter field
	if req.Parameter != nil {
		for _, param := range strings.Split(*req.Parameter, ",") {
			parts := strings.SplitN(param, "=", 2)
			name := parts[0]
			value := ""
			if len(parts) > 1 {
				value = parts[1]
			}
			applyOption(name, value)
		}
	}

	// Initialize exclusion sets
	excludedPackageSet = parseExclusionList(excludePackages)
	excludedMessageSet = parseExclusionList(excludeMessages)

	// Create plugin from the request
	opts := protogen.Options{}
	plugin, err := opts.New(req)
	if err != nil {
		return fmt.Errorf("failed to create plugin: %w", err)
	}

	plugin.SupportedFeatures = uint64(1) // FEATURE_PROTO3_OPTIONAL

	// Process all files
	for _, f := range plugin.Files {
		if err := generateFile(plugin, f); err != nil {
			plugin.Error(err)
		}
	}

	// Write response to output
	resp := plugin.Response()
	out, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	output.Write(out)

	return nil
}

// runAsPlugin runs the normal protoc plugin mode, reading from stdin.
func runAsPlugin() {
	// Check if debug dump is requested by scanning stdin first
	// We need to read stdin, optionally save it, then process it
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read stdin: %v\n", err)
		os.Exit(1)
	}

	// Parse the request to check for debug option
	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(input, req); err != nil {
		fmt.Fprintf(os.Stderr, "failed to unmarshal request: %v\n", err)
		os.Exit(1)
	}

	// Set up command-line flags for plugin options.
	// These are passed via --om-jsonschema_opt=option1,option2
	var flags flag.FlagSet
	flags.BoolVar(&strictAny, "strict_any", false, "Use strict google.protobuf.Any schema with typeUrl/value fields instead of AI-friendly @type format")
	flags.BoolVar(&strictMap, "strict_map", false, "Use strict map schema with Entry types instead of AI-friendly additionalProperties format")
	flags.BoolVar(&strictOneof, "strict_oneof", false, "Use description hint for oneof instead of x-oneof extension")
	flags.StringVar(&excludePackages, "exclude_packages", "", "Semicolon-separated list of packages to exclude from generation")
	flags.StringVar(&excludeMessages, "exclude_messages", "", "Semicolon-separated list of messages to exclude from generation")

	// Configure and run the protoc plugin using the already-parsed request
	opts := protogen.Options{ParamFunc: func(name, value string) error {
		// Handle boolean flags without values (e.g., "strict_any" instead of "strict_any=true").
		// protoc passes options without "=true" suffix, so we need to handle this case.
		if value == "" {
			value = "true"
		}
		return flags.Set(name, value)
	}}

	plugin, err := opts.New(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create plugin: %v\n", err)
		os.Exit(1)
	}

	// Parse exclusion lists into sets for fast lookup
	excludedPackageSet = parseExclusionList(excludePackages)
	excludedMessageSet = parseExclusionList(excludeMessages)

	// Enable support for proto3 optional fields (explicit presence tracking).
	// This allows the plugin to distinguish between "field not set" and "field set to default".
	plugin.SupportedFeatures = uint64(1) // FEATURE_PROTO3_OPTIONAL

	// Process all files in the compilation, including dependencies.
	// gen.Files contains both directly requested files and their imports.
	// We generate schemas for all files to ensure types like google.protobuf.Any
	// have schema files even when they're only used as dependencies.
	for _, f := range plugin.Files {
		if err := generateFile(plugin, f); err != nil {
			plugin.Error(err)
		}
	}

	// Write response to stdout
	resp := plugin.Response()
	out, err := proto.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal response: %v\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(out)
}

// applyOption applies a single option by name and value.
// Used when parsing options from a dumped request file.
func applyOption(name, value string) {
	switch name {
	case "strict_any":
		strictAny = value != "false"
	case "strict_map":
		strictMap = value != "false"
	case "strict_oneof":
		strictOneof = value != "false"
	case "exclude_packages":
		excludePackages = value
	case "exclude_messages":
		excludeMessages = value
	}
}

// parseExclusionList converts a semicolon-separated string into a set (map[string]bool).
// Empty strings result in an empty set.
func parseExclusionList(list string) map[string]bool {
	result := make(map[string]bool)
	if list == "" {
		return result
	}
	for _, item := range strings.Split(list, ";") {
		item = strings.TrimSpace(item)
		if item != "" {
			result[item] = true
		}
	}
	return result
}

// isPackageExcluded checks if a package should be excluded from generation.
func isPackageExcluded(packageName string) bool {
	return excludedPackageSet[packageName]
}

// isMessageExcluded checks if a specific message should be excluded from generation.
func isMessageExcluded(fullName string) bool {
	return excludedMessageSet[fullName]
}

// isEnumExcluded checks if a specific enum should be excluded from generation.
// Uses the same exclusion list as messages.
func isEnumExcluded(fullName string) bool {
	return excludedMessageSet[fullName]
}

// generateFile processes a single .proto file and generates JSON Schema files
// for all messages and enums defined in that file.
//
// Parameters:
//   - gen: The protogen plugin instance, used to create output files
//   - file: The protobuf file descriptor containing messages and enums to process
//
// Returns an error if schema generation fails for any type.
func generateFile(gen *protogen.Plugin, file *protogen.File) error {
	// Check if this file's package is excluded
	packageName := string(file.Desc.Package())
	if isPackageExcluded(packageName) {
		return nil
	}

	// Generate a separate JSON Schema file for each top-level message.
	// Nested messages are handled recursively within generateMessageSchema.
	for _, msg := range file.Messages {
		if err := generateMessageSchema(gen, file, msg); err != nil {
			return err
		}
	}

	// Generate a separate JSON Schema file for each top-level enum.
	// Enums defined within messages are handled in generateMessageSchema.
	for _, enum := range file.Enums {
		if err := generateEnumSchema(gen, file, enum); err != nil {
			return err
		}
	}

	return nil
}

// generateMessageSchema creates a JSON Schema file for a protobuf message type.
// The output file is named after the fully qualified message name (e.g., "package.MessageName").
//
// This function handles:
//   - Standard message types (converted to JSON Schema objects)
//   - Map entry types (skipped in default mode, as maps use additionalProperties)
//   - google.protobuf.Any (special AI-friendly format in default mode)
//   - Oneof groups (indicated via x-oneof extension or description hints)
//   - Nested messages and enums (processed recursively)
//
// Parameters:
//   - gen: The protogen plugin instance for creating output files
//   - file: The parent file (unused but kept for potential future use)
//   - msg: The message descriptor to convert to JSON Schema
//
// Returns an error if schema creation or JSON marshaling fails.
func generateMessageSchema(gen *protogen.Plugin, file *protogen.File, msg *protogen.Message) error {
	// Get the fully qualified message name (e.g., "google.protobuf.Any").
	// This becomes both the output filename and the schema's $id.
	fullName := string(msg.Desc.FullName())

	// Check if this specific message is excluded
	if isMessageExcluded(fullName) {
		return nil
	}

	// Skip map entry types when using AI-friendly map format.
	// In protobuf, map<K,V> fields are internally represented as repeated MapEntry messages.
	// In default mode, we represent maps using additionalProperties instead of referencing
	// the Entry type, so we don't need to generate schema files for Entry messages.
	if !strictMap && msg.Desc.IsMapEntry() {
		return nil
	}

	// Special handling for google.protobuf.Any to make it more AI-friendly.
	// The standard Any has typeUrl (a URL) and value (base64-encoded bytes), which is
	// difficult for AI to work with. Instead, we use @type (the schema $id) with
	// additionalProperties, matching how Any is serialized to JSON.
	if fullName == "google.protobuf.Any" && !strictAny {
		return generateAnySchema(gen, fullName, msg)
	}

	// Create the base schema structure for this message.
	// All messages are represented as JSON Schema "object" types.
	schema := &JSONSchema{
		Schema:     "https://json-schema.org/draft/2020-12/schema",
		ID:         fullName,
		Type:       "object",
		Properties: make(map[string]*JSONSchema),
	}

	// Extract the message description from proto comments.
	// Leading comments (above the message) are preferred over trailing comments.
	// The message name is appended to help identify the type in the description.
	msgDesc := extractDescription(msg.Comments)
	if msgDesc != "" {
		schema.Description = msgDesc + string(msg.Desc.Name())
	}

	// Check for required fields specified via openapi.v3.schema annotation.
	// This allows proto authors to mark certain fields as required in the JSON Schema.
	schema.Required = getRequiredFields(msg)

	// Convert each field in the message to a JSON Schema property.
	// Field names are converted to their JSON representation (typically camelCase).
	for _, field := range msg.Fields {
		fieldSchema := convertField(field)
		jsonName := field.Desc.JSONName()
		schema.Properties[jsonName] = fieldSchema
	}

	// Handle oneof groups - mutually exclusive field sets.
	// In protobuf, setting one field in a oneof automatically clears the others.
	// We need to communicate this constraint to AI systems to prevent data loss.
	oneofGroups := getOneofGroups(msg)
	if len(oneofGroups) > 0 {
		if strictOneof {
			// Strict mode: Add only a concise description hint.
			// Format: "ONEOF[groupName]: Set exactly ONE of [field1, field2, field3]"
			// This is more compact but less machine-readable than x-oneof.
			var oneofHints []string
			for _, group := range oneofGroups {
				oneofHints = append(oneofHints, fmt.Sprintf("ONEOF[%s]: Set exactly ONE of [%s]", group.Name, strings.Join(group.Fields, ", ")))
			}
			hint := strings.Join(oneofHints, ". ")
			if schema.Description != "" {
				schema.Description = schema.Description + " | " + hint
			} else {
				schema.Description = hint
			}
		} else {
			// Default mode: Use x-oneof extension with a detailed explanation.
			// The x-oneof array provides machine-readable information about oneof groups.
			// Each entry has "name" (the oneof group name) and "fields" (the field names).
			// The IMPORTANT hint explains how to use x-oneof for AI systems.
			schema.XOneof = oneofGroups

			oneofHint := "IMPORTANT: This message contains mutually exclusive field groups defined in x-oneof. Each x-oneof entry has a 'name' (the group name) and 'fields' (the field names). You must set exactly ONE field from each group, not multiple. Setting multiple fields from the same group will cause data loss."
			if schema.Description != "" {
				schema.Description = schema.Description + " | " + oneofHint
			} else {
				schema.Description = oneofHint
			}
		}
	}

	// Create the output file with the fully qualified message name.
	// The file has no extension - just the message name like "google.protobuf.Any".
	filename := fullName
	outputFile := gen.NewGeneratedFile(filename, "")

	// Marshal the schema to pretty-printed JSON.
	// We use 2-space indentation for readability.
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema for %s: %w", fullName, err)
	}

	outputFile.Write(data)

	// Recursively process nested message types.
	// In protobuf, messages can contain other message definitions.
	// Each nested message gets its own schema file with a qualified name like "Parent.Nested".
	for _, nested := range msg.Messages {
		if err := generateMessageSchema(gen, file, nested); err != nil {
			return err
		}
	}

	// Recursively process nested enum types defined within this message.
	for _, enum := range msg.Enums {
		if err := generateEnumSchema(gen, file, enum); err != nil {
			return err
		}
	}

	return nil
}

// getOneofGroups extracts oneof field groups from a message.
// Each group contains the group name and the JSON names of fields in that group.
//
// For example, a proto oneof like:
//
//	oneof content {
//	  string text = 1;
//	  bytes binary = 2;
//	}
//
// Returns: [{Name: "content", Fields: ["text", "binary"]}]
//
// Note: Synthetic oneofs (created for proto3 optional fields) are excluded
// since they don't represent actual mutual exclusion constraints.
func getOneofGroups(msg *protogen.Message) []OneofGroup {
	// Map to collect fields by their oneof group name.
	oneofMap := make(map[string][]string)

	// Iterate through all fields and group them by oneof.
	for _, field := range msg.Fields {
		// Check if this field belongs to a oneof group.
		// IsSynthetic() returns true for proto3 optional fields which create
		// "fake" oneofs for presence tracking - we skip those.
		if field.Oneof != nil && !field.Oneof.Desc.IsSynthetic() {
			oneofName := string(field.Oneof.Desc.Name())
			jsonName := field.Desc.JSONName()
			oneofMap[oneofName] = append(oneofMap[oneofName], jsonName)
		}
	}

	// Convert the map to the output format: [{Name: "groupName", Fields: [...]}, ...]
	var result []OneofGroup
	for name, fields := range oneofMap {
		result = append(result, OneofGroup{
			Name:   name,
			Fields: fields,
		})
	}

	return result
}

// generateAnySchema creates a special AI-friendly JSON Schema for google.protobuf.Any.
//
// The standard protobuf Any type has:
//   - typeUrl: A URL like "type.googleapis.com/package.MessageName"
//   - value: Base64-encoded serialized protobuf bytes
//
// This is difficult for AI systems to work with because:
//   - The value is opaque binary data
//   - The typeUrl format is verbose
//
// Instead, when serialized to JSON, Any uses a more friendly format:
//
//	{
//	  "@type": "package.MessageName",
//	  "field1": "value1",
//	  "field2": "value2"
//	}
//
// This function generates a schema that matches this JSON format:
//   - @type field (required) contains the schema $id of the embedded message
//   - additionalProperties: true allows any other fields from the embedded message
//
// Parameters:
//   - gen: The protogen plugin instance for creating output files
//   - fullName: The fully qualified name ("google.protobuf.Any")
//   - msg: The Any message descriptor (unused but kept for consistency)
func generateAnySchema(gen *protogen.Plugin, fullName string, msg *protogen.Message) error {
	// Allow any additional properties since Any can contain any message type.
	additionalProps := true

	schema := &JSONSchema{
		Schema:               "https://json-schema.org/draft/2020-12/schema",
		ID:                   fullName,
		Type:                 "object",
		Description:          "A container for any message type. The @type field identifies which message type is contained, and the remaining fields are the actual message data. Set @type to the $id of the target schema and include all fields from that schema.",
		Properties:           make(map[string]*JSONSchema),
		Required:             []string{"@type"},
		AdditionalProperties: &additionalProps,
	}

	// The @type property tells AI systems which message type is embedded.
	// It should be set to the $id of another schema file (e.g., "mypackage.MyMessage").
	schema.Properties["@type"] = &JSONSchema{
		Type:        "string",
		Description: "The $id of the schema that describes the additional fields in this object.",
	}

	// Write the schema file
	outputFile := gen.NewGeneratedFile(fullName, "")

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema for %s: %w", fullName, err)
	}

	outputFile.Write(data)

	return nil
}

// generateEnumSchema creates a JSON Schema file for a protobuf enum type.
// Enums are represented as JSON Schema string types with an enum constraint
// listing all valid values.
//
// For example, a proto enum like:
//
//	enum Status {
//	  UNKNOWN = 0;
//	  ACTIVE = 1;
//	  INACTIVE = 2;
//	}
//
// Generates:
//
//	{
//	  "$id": "package.Status",
//	  "type": "string",
//	  "enum": ["UNKNOWN", "ACTIVE", "INACTIVE"]
//	}
//
// Parameters:
//   - gen: The protogen plugin instance for creating output files
//   - file: The parent file (unused but kept for consistency)
//   - enum: The enum descriptor to convert to JSON Schema
func generateEnumSchema(gen *protogen.Plugin, file *protogen.File, enum *protogen.Enum) error {
	fullName := string(enum.Desc.FullName())

	// Check if this specific enum is excluded
	if isEnumExcluded(fullName) {
		return nil
	}

	schema := &JSONSchema{
		Schema: "https://json-schema.org/draft/2020-12/schema",
		ID:     fullName,
		Type:   "string",
	}

	// Extract the enum description from proto comments.
	enumDesc := extractDescription(enum.Comments)
	if enumDesc != "" {
		schema.Description = enumDesc
	}

	// Add all enum value names to the allowed values list.
	// We use the proto name (e.g., "ACTIVE") not the numeric value.
	for _, value := range enum.Values {
		schema.Enum = append(schema.Enum, string(value.Desc.Name()))
	}

	// Write the schema file
	filename := fullName
	outputFile := gen.NewGeneratedFile(filename, "")

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema for %s: %w", fullName, err)
	}

	outputFile.Write(data)

	return nil
}

// convertField transforms a protobuf field into a JSON Schema definition.
// This is the core type-mapping logic that handles all protobuf field types:
//
//   - Scalar types (int32, string, bool, etc.) → JSON primitives
//   - Message types → $ref to another schema
//   - Enum types → $ref to the enum schema
//   - Repeated fields → array with items schema
//   - Map fields → object with additionalProperties (default) or $ref to Entry type (strict)
//
// The function also applies any openapi.v3.property annotations (maximum, minimum, etc.)
// and automatically sets minimum: 0 for unsigned integer types.
//
// Parameters:
//   - field: The protobuf field descriptor to convert
//
// Returns the JSON Schema representation of this field.
func convertField(field *protogen.Field) *JSONSchema {
	schema := &JSONSchema{}

	// Extract the field description from proto comments.
	desc := extractDescription(field.Comments)
	if desc != "" {
		schema.Description = desc
	}

	// Apply any OpenAPI v3 property annotations (maximum, minimum, default, readOnly).
	// These are specified in proto files using: [(openapi.v3.property) = {...}]
	applyOpenAPIPropertyOptions(field, schema)

	// Handle map<K,V> fields first (before checking IsList).
	// In protobuf's internal representation, maps are actually repeated MapEntry messages,
	// so field.Desc.IsList() would return true for maps. We check IsMap() first to
	// handle them with the AI-friendly additionalProperties format.
	//
	// Default output for map<string, MyMessage>:
	//   {"type": "object", "additionalProperties": {"$ref": "MyMessage"}}
	//
	// This matches the JSON wire format: {"key1": {...}, "key2": {...}}
	if field.Desc.IsMap() && !strictMap {
		schema.Type = "object"

		// Get the value type of the map. Map entries always have two fields:
		// index 0 is the key (always string in JSON), index 1 is the value.
		valueField := field.Message.Fields[1]

		// Set additionalProperties based on the value type
		if valueField.Message != nil {
			// Map value is a message type - reference it
			schema.AdditionalProperties = &JSONSchema{
				Ref: string(valueField.Message.Desc.FullName()),
			}
		} else if valueField.Enum != nil {
			// Map value is an enum type - reference it
			schema.AdditionalProperties = &JSONSchema{
				Ref: string(valueField.Enum.Desc.FullName()),
			}
		} else {
			// Map value is a scalar type - inline it
			schema.AdditionalProperties = &JSONSchema{
				Type: protoKindToJSONType(valueField.Desc.Kind()),
			}
		}
		return schema
	}

	// Handle repeated fields (arrays).
	// These are represented as JSON Schema arrays with an items schema.
	//
	// Output for repeated MyMessage:
	//   {"type": "array", "items": {"$ref": "MyMessage"}}
	//
	// Output for repeated int32:
	//   {"type": "array", "items": {"type": "integer"}}
	if field.Desc.IsList() {
		schema.Type = "array"
		itemSchema := &JSONSchema{}

		// Set the items schema based on the element type
		if field.Message != nil {
			itemSchema.Ref = string(field.Message.Desc.FullName())
		} else if field.Enum != nil {
			itemSchema.Ref = string(field.Enum.Desc.FullName())
		} else {
			itemSchema.Type = protoKindToJSONType(field.Desc.Kind())
		}

		schema.Items = itemSchema
		return schema
	}

	// Handle singular message-typed fields.
	// These reference another schema by its $id.
	//
	// Output for MyMessage field:
	//   {"$ref": "package.MyMessage"}
	if field.Message != nil {
		schema.Ref = string(field.Message.Desc.FullName())
		return schema
	}

	// Handle singular enum-typed fields.
	// These reference the enum schema by its $id.
	if field.Enum != nil {
		schema.Ref = string(field.Enum.Desc.FullName())
		return schema
	}

	// Handle scalar types (int32, string, bool, float, bytes, etc.).
	// Convert the protobuf kind to the appropriate JSON Schema type.
	schema.Type = protoKindToJSONType(field.Desc.Kind())

	// Automatically set minimum: 0 for unsigned integer types.
	// This helps AI systems understand that these fields cannot be negative.
	// Only applied if minimum wasn't already set via openapi.v3.property annotation.
	if schema.Minimum == nil && isUnsignedKind(field.Desc.Kind()) {
		zero := float64(0)
		schema.Minimum = &zero
	}

	return schema
}

// protoKindToJSONType converts a protobuf field kind to the equivalent JSON Schema type.
//
// Mapping:
//   - bool → "boolean"
//   - All integer types (int32, int64, uint32, uint64, sint*, sfixed*, fixed*) → "integer"
//   - float, double → "number"
//   - string → "string"
//   - bytes → "string" (base64 encoded in JSON)
//   - Unknown types → "string" (safe default)
func protoKindToJSONType(kind protoreflect.Kind) string {
	switch kind {
	case protoreflect.BoolKind:
		return "boolean"

	// All integer types map to JSON Schema "integer"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "integer"

	// Floating point types map to JSON Schema "number"
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return "number"

	case protoreflect.StringKind:
		return "string"

	// Bytes are base64-encoded in JSON, so they're strings in the schema.
	// Ideally we'd add format: "byte" but keeping it simple for now.
	case protoreflect.BytesKind:
		return "string"

	// Default to string for any unknown types (shouldn't happen with valid protos)
	default:
		return "string"
	}
}

// isUnsignedKind checks if a protobuf field kind represents an unsigned integer type.
// Used to automatically set minimum: 0 for fields that cannot be negative.
//
// Unsigned types: uint32, uint64, fixed32, fixed64
// Signed types (returns false): int32, int64, sint32, sint64, sfixed32, sfixed64
func isUnsignedKind(kind protoreflect.Kind) bool {
	switch kind {
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return true
	default:
		return false
	}
}

// extractDescription extracts documentation text from proto comments.
// It prefers leading comments (above the definition) over trailing comments (same line).
//
// In proto files, comments look like:
//
//	// This is a leading comment
//	message Foo {
//	  string bar = 1; // This is a trailing comment
//	}
//
// Returns an empty string if no comments are present.
func extractDescription(comments protogen.CommentSet) string {
	// Leading comments (above the definition) are preferred as they're typically
	// more detailed documentation.
	if comments.Leading != "" {
		return strings.TrimSpace(string(comments.Leading))
	}

	// Fall back to trailing comments (on the same line) if no leading comment exists.
	if comments.Trailing != "" {
		return strings.TrimSpace(string(comments.Trailing))
	}

	return ""
}

// getRequiredFields extracts the list of required field names from a message's
// openapi.v3.schema annotation, if present.
//
// In proto files, this is specified as:
//
//	message Foo {
//	  option (openapi.v3.schema) = {required: ["bar", "baz"]};
//	  string bar = 1;
//	  string baz = 2;
//	}
//
// Returns nil if no required fields are specified or if the annotation is missing.
func getRequiredFields(msg *protogen.Message) []string {
	// Get the message-level options
	opts := msg.Desc.Options().(*descriptorpb.MessageOptions)
	if opts == nil {
		return nil
	}

	// Try to get the openapi.v3.schema extension
	schemaExt := proto.GetExtension(opts, openapi.E_Schema)
	if schemaExt == nil {
		return nil
	}

	// Type-assert to the Schema type and extract required fields
	schema, ok := schemaExt.(*openapi.Schema)
	if !ok || schema == nil {
		return nil
	}

	return schema.Required
}

// applyOpenAPIPropertyOptions reads openapi.v3.property annotations from a field
// and applies the constraints to the JSON Schema.
//
// Supported annotations:
//   - readOnly: true → schema.ReadOnly = true
//   - maximum: N → schema.Maximum = N
//   - minimum: N → schema.Minimum = N
//   - format: "positive" → schema.Minimum = 0 (workaround for proto3 zero value issue)
//   - default: {number: N} → schema.Default = N
//   - default: {boolean: B} → schema.Default = B
//   - default: {string: S} → schema.Default = S
//
// Example proto annotation:
//
//	int32 count = 1 [(openapi.v3.property) = {
//	  maximum: 100
//	  minimum: 10
//	  default: {number: 50}
//	}];
//
// Note: Due to proto3's zero-value encoding, minimum: 0 cannot be detected directly
// from the annotation. Use format: "positive" as a workaround to indicate minimum: 0.
func applyOpenAPIPropertyOptions(field *protogen.Field, schema *JSONSchema) {
	// Get the field-level options
	opts := field.Desc.Options().(*descriptorpb.FieldOptions)
	if opts == nil {
		return
	}

	// Marshal options to raw bytes for detecting presence of min/max fields.
	// This is necessary because proto3 doesn't encode zero values, making it
	// impossible to distinguish "minimum: 0" from "minimum not set" using
	// the standard proto.GetExtension API.
	optBytes, _ := proto.Marshal(opts)

	// Try to get the openapi.v3.property extension
	propExt := proto.GetExtension(opts, openapi.E_Property)
	if propExt == nil {
		return
	}

	prop, ok := propExt.(*openapi.Schema)
	if !ok || prop == nil {
		return
	}

	// Apply readOnly constraint
	if prop.ReadOnly {
		readOnly := true
		schema.ReadOnly = &readOnly
	}

	// Detect presence of maximum/minimum in the raw protobuf bytes.
	// We need this extra step because proto3 doesn't encode default values,
	// so a field with "minimum: 0" would appear the same as no minimum at all
	// when using the standard proto API.
	hasMax, hasMin := detectMinMaxFromRaw(optBytes)

	if hasMax {
		schema.Maximum = &prop.Maximum
	}

	if hasMin {
		schema.Minimum = &prop.Minimum
	}

	// Workaround for proto3 zero value limitation:
	// Since minimum: 0 cannot be detected directly, proto authors can use
	// format: "positive" to indicate that the field must be >= 0.
	// This is useful for fields like percentages, counts, etc.
	if prop.Format == "positive" && schema.Minimum == nil {
		zero := float64(0)
		schema.Minimum = &zero
	}

	// Apply default value if specified.
	// The default can be a number, boolean, or string.
	if prop.Default != nil {
		switch v := prop.Default.Oneof.(type) {
		case *openapi.DefaultType_Number:
			schema.Default = v.Number
		case *openapi.DefaultType_Boolean:
			schema.Default = v.Boolean
		case *openapi.DefaultType_String_:
			schema.Default = v.String_
		}
	}
}
