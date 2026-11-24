package ir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// The JSON codec. Each IR struct carries its own lower-camel `json` tags in
// declaration order, with omitempty on every field, so nil and empty collapse
// and the encoding is deterministic (encoding/json writes struct fields in
// declaration order and never ranges over a map; the IR holds none). Enums
// encode by name through their own MarshalJSON, ColorSet as a letter string
// and ZoneSet as a list of names, so appending an enum const or a new field
// needs no edit here. Src is encoded like any other field.

// EncodeCard returns the canonical JSON of c.
func EncodeCard(c *Card) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(c); err != nil {
		// Every IR value is encodable (unnamed enum values encode as a
		// placeholder name that decoding rejects), so this is unreachable.
		panic(fmt.Sprintf("ir: EncodeCard: %v", err))
	}
	return bytes.TrimRight(buf.Bytes(), "\n")
}

// DecodeCard parses the output of EncodeCard. It rejects unknown enum names,
// unknown fields and trailing data.
func DecodeCard(b []byte) (*Card, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var c Card
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("ir: decode card: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("ir: decode card: trailing data after the card")
	}
	return &c, nil
}
