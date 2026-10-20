package tsgen

// The JSON Schema fixture, type-checked from THIS file's source by
// TestJSONSchemaMapsEncodingJSON; it imports nothing.

// colour is an enum: its constants become the schema's enum.
type colour string

const (
	colourRed  colour = "red"
	colourBlue colour = "blue"
)

type base struct {
	// Shared is promoted into every embedding struct.
	Shared string `json:"shared"`
}

// card exercises the encoding/json rules the generator follows.
type card struct {
	base
	Colour  colour           `json:"colour"`
	Tags    []string         `json:"tags"`
	Maybe   *int32           `json:"maybe"`
	Omitted *int32           `json:"omitted,omitempty"`
	Counts  map[string]uint8 `json:"counts,omitempty"`
	Nested  base             `json:"nested,omitempty"`
	Hidden  string           `json:"-"`
	private string
}

// selfMarshal encodes itself, so its schema must be stated by an Override.
type selfMarshal struct{ X int }

func (selfMarshal) MarshalJSON() ([]byte, error) { return []byte(`"x"`), nil }
