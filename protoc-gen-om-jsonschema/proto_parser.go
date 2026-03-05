package main

import "math"

// This file contains low-level protobuf wire format parsing functions.
// These are used to detect field presence in raw protobuf bytes, which is
// necessary because proto3 doesn't encode fields set to their default values.

// detectMinMaxFromRaw parses raw FieldOptions protobuf bytes to detect
// whether maximum and/or minimum fields are present in an openapi.v3.property extension.
//
// This function is necessary because proto3 doesn't encode fields set to their
// default value (e.g., 0 for numbers). So when we see minimum: 0 in a .proto file,
// the standard proto.GetExtension API returns 0, but we can't tell if it was
// explicitly set to 0 or just not set at all.
//
// By parsing the raw bytes, we can detect field presence regardless of value.
//
// Parameters:
//   - data: Raw marshaled FieldOptions bytes
//
// Returns:
//   - hasMax: true if maximum field is present (field number 11)
//   - hasMin: true if minimum field is present (field number 13)
func detectMinMaxFromRaw(data []byte) (hasMax, hasMin bool) {
	// The field number for the openapi.v3.property extension in FieldOptions.
	// This is defined in the openapiv3 proto file.
	const extPropertyFieldNum = 1143

	i := 0
	// Parse the FieldOptions message to find the property extension field
	for i < len(data) {
		// Decode the field tag (field number + wire type)
		fieldTag, n := decodeVarint(data[i:])
		if n == 0 {
			break
		}
		i += n

		// Extract field number (upper 29 bits) and wire type (lower 3 bits)
		fieldNum := fieldTag >> 3
		wireType := fieldTag & 0x7

		// Check if this is the openapi.v3.property extension field
		if fieldNum == extPropertyFieldNum && wireType == 2 { // wire type 2 = length-delimited
			// Found the extension - read its length and parse the Schema message
			length, n := decodeVarint(data[i:])
			if n == 0 {
				break
			}
			i += n

			// Extract the Schema message bytes and parse them for min/max
			schemaData := data[i : i+int(length)]
			return parseSchemaForMinMax(schemaData)
		} else {
			// Not the field we're looking for - skip it
			i = skipField(data, i, wireType)
			if i < 0 {
				break
			}
		}
	}

	return false, false
}

// parseSchemaForMinMax parses raw Schema message bytes to detect presence
// of maximum (field 11) and minimum (field 13) fields.
//
// In the openapiv3.Schema message:
//   - Field 11: maximum (double, wire type 1 = fixed64)
//   - Field 13: minimum (double, wire type 1 = fixed64)
//
// We don't care about the actual values here (those come from proto.GetExtension),
// we just need to know if the fields are present.
//
// Parameters:
//   - data: Raw bytes of a serialized openapiv3.Schema message
//
// Returns:
//   - hasMax: true if field 11 (maximum) is present
//   - hasMin: true if field 13 (minimum) is present
func parseSchemaForMinMax(data []byte) (hasMax, hasMin bool) {
	i := 0
	// Parse all fields in the Schema message
	for i < len(data) {
		// Decode the field tag
		fieldTag, n := decodeVarint(data[i:])
		if n == 0 {
			break
		}
		i += n

		fieldNum := fieldTag >> 3
		wireType := fieldTag & 0x7

		// Check for maximum and minimum fields
		switch fieldNum {
		case 11: // maximum field (double type = wire type 1 = fixed64)
			if wireType == 1 {
				hasMax = true
				i += 8 // Skip the 8-byte double value
				continue
			}
		case 13: // minimum field (double type = wire type 1 = fixed64)
			if wireType == 1 {
				hasMin = true
				i += 8 // Skip the 8-byte double value
				continue
			}
		}

		// Skip any other fields we don't care about
		i = skipField(data, i, wireType)
		if i < 0 {
			break
		}
	}

	return hasMax, hasMin
}

// decodeVarint decodes a variable-length integer from protobuf wire format.
//
// Varints use 7 bits per byte for data, with the high bit indicating continuation.
// For example:
//   - 0x01 → 1 (single byte, high bit = 0)
//   - 0x80 0x01 → 128 (two bytes, first has high bit = 1)
//
// This is used for field tags, lengths, and integer field values in protobuf encoding.
//
// Parameters:
//   - data: Byte slice starting at the varint to decode
//
// Returns:
//   - The decoded uint64 value
//   - Number of bytes consumed (0 if decoding failed or overflow)
func decodeVarint(data []byte) (uint64, int) {
	var x uint64
	var s uint // Bit shift amount

	for i, b := range data {
		// Prevent overflow - varints can be at most 10 bytes for uint64
		if i == 10 {
			return 0, 0
		}

		// Check if this is the last byte (high bit = 0)
		if b < 0x80 {
			return x | uint64(b)<<s, i + 1
		}

		// Not the last byte - add the lower 7 bits and continue
		x |= uint64(b&0x7f) << s
		s += 7
	}

	// Ran out of data before finding terminating byte
	return 0, 0
}

// decodeFixed64 decodes a 64-bit floating point number from protobuf wire format.
// Fixed64 values are stored as 8 bytes in little-endian order.
//
// Parameters:
//   - data: Byte slice containing at least 8 bytes
//
// Returns the decoded float64 value, or 0 if data is too short.
func decodeFixed64(data []byte) float64 {
	if len(data) < 8 {
		return 0
	}

	// Assemble the 8 bytes into a uint64 (little-endian order)
	bits := uint64(data[0]) | uint64(data[1])<<8 | uint64(data[2])<<16 | uint64(data[3])<<24 |
		uint64(data[4])<<32 | uint64(data[5])<<40 | uint64(data[6])<<48 | uint64(data[7])<<56

	// Convert the bits to a float64
	return math.Float64frombits(bits)
}

// skipField advances the position past a protobuf field value based on its wire type.
// This is used when parsing raw protobuf bytes and we want to skip over fields
// we don't care about.
//
// Wire types:
//   - 0: Varint (variable length, 1-10 bytes)
//   - 1: Fixed64 (always 8 bytes)
//   - 2: Length-delimited (length prefix + data)
//   - 5: Fixed32 (always 4 bytes)
//
// Note: Wire types 3 and 4 (start/end group) are deprecated and not supported.
//
// Parameters:
//   - data: The full byte slice being parsed
//   - pos: Current position after the field tag (at the start of the value)
//   - wireType: The wire type from the field tag
//
// Returns the new position after skipping the field, or -1 if parsing fails.
func skipField(data []byte, pos int, wireType uint64) int {
	switch wireType {
	case 0: // Varint - scan until we find a byte with high bit = 0
		for pos < len(data) {
			if data[pos]&0x80 == 0 {
				return pos + 1
			}
			pos++
		}
		return -1 // Ran out of data

	case 1: // Fixed64 - always 8 bytes
		return pos + 8

	case 2: // Length-delimited - read length prefix, then skip that many bytes
		length, n := decodeVarint(data[pos:])
		if n == 0 {
			return -1
		}
		return pos + n + int(length)

	case 5: // Fixed32 - always 4 bytes
		return pos + 4

	default: // Unknown wire type (including deprecated group types 3 and 4)
		return -1
	}
}
