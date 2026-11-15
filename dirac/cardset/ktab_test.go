package cardset

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// sampleData exercises the shapes the codec must keep apart.
func sampleData() *PoolData {
	return &PoolData{
		Name:     "t",
		NumCards: 2, NumTokens: 1, NumIDs: 4,
		Names:         []string{"A Card", "B Card", "c_a_food"},
		PoolID:        []uint16{0, 2, 3},
		Absent:        []AbsentID{{ID: 1, Name: "Gone", Token: true}},
		Status:        []Status{StatusSupported, StatusNotLowered, StatusUnverified},
		Reason:        []string{"", "not lowered: no", "unverified: in no evidence list"},
		Defs:          []kop.CardDef{{Power: -3, Toughness: 7}, {}, {Power: 1}},
		CardSubs:      []PoolSubMask{{5, 0, 0, 0}, {0, 1 << 63, 0, 0}, PoolSubMask{}.With(191).With(253)}, // trailing-zero, leading-zero and fourth-word arrays
		CardTokens:    [][]uint16{nil, {2}, {}},                                                           // nil and empty in one slice
		CardRefSubs:   [][]uint8{nil, nil, {}},
		CardPrinted:   [][]uint8{nil, {3, 250}, nil},
		AbilityOrigin: [][]uint8{{0xFF, 0x80}, nil, nil},
		StaticSubs:    [][]AbSubs{nil, {{Ab: 1, Subs: []uint8{4}}}, nil},
		OverrideSubs:  []OvSubs{{Set: []uint8{1}}, {}},
		Filters:       []kop.Filter{}, // empty, not nil
		Exprs:         []kop.Expr{},
		Conds:         []kop.Cond{},
		Overrides:     []kop.Override{},
		Emblems:       []kop.Emblem{},
		Subtypes:      []string{"", "Goblin"}, // an empty string is a real element
		LayerCls:      []LayerEntry{{I: 1, C: LayerClasses{Any: LcRecPT, Strips: true}}},
		RoomCards:     []int{-1, 0, 1 << 40},
		// CreatureTypes, LayerConflicts, NameReaders stay nil.
	}
}

func TestTableRoundTrip(t *testing.T) {
	want := sampleData()
	if !want.CardSubs[2].Has(191) || !want.CardSubs[2].Has(253) {
		t.Fatal("precondition: the sample card holds subtypes 191 and 253, beyond the old 128-bit mask and one table's 192")
	}
	b := EncodeTable(want, "somekey")
	if !bytes.Equal(b, EncodeTable(sampleData(), "somekey")) {
		t.Error("encoding the same data twice gave different bytes")
	}
	got, key, err := DecodeTable(b)
	if err != nil {
		t.Fatal(err)
	}
	if key != "somekey" {
		t.Errorf("key = %q", key)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("round trip differs:\nwant %+v\ngot  %+v", want, got)
	}
	// reflect.DeepEqual already separates these; say so where it matters.
	if got.CardTokens[0] != nil || got.CardTokens[2] == nil {
		t.Error("nil and empty slices were not kept apart")
	}
	if got.Filters == nil || got.CreatureTypes != nil {
		t.Error("nil and empty top-level slices were not kept apart")
	}
	if got.Defs[0].Power != -3 || got.RoomCards[0] != -1 || got.RoomCards[2] != 1<<40 {
		t.Error("ints did not survive")
	}
	if got.CardSubs[1][1] != 1<<63 || got.CardSubs[0] != (PoolSubMask{5, 0, 0, 0}) || !got.CardSubs[2].Has(191) || !got.CardSubs[2].Has(253) {
		t.Error("arrays did not survive")
	}
}

// The trailing-zero rule holds on the wire, not only after a decode.
func TestArrayDropsTrailingZeros(t *testing.T) {
	long := encodeValue(nil, reflect.ValueOf([4]uint8{1, 0, 2, 0}))
	if want := []byte{3, 1, 0, 2}; !bytes.Equal(long, want) {
		t.Errorf("got %v, want %v", long, want)
	}
	zero := encodeValue(nil, reflect.ValueOf([4]uint8{}))
	if want := []byte{0}; !bytes.Equal(zero, want) {
		t.Errorf("got %v, want %v", zero, want)
	}
}

func TestTableRejectsDamage(t *testing.T) {
	good := EncodeTable(sampleData(), "somekey")
	if _, _, err := DecodeTable(good); err != nil {
		t.Fatalf("the undamaged control fails: %v", err)
	}
	fp := poolSchema()
	fpAt := bytes.Index(good, fp[:])
	if fpAt < 0 {
		t.Fatal("no fingerprint in the table")
	}
	damage := func(at int) []byte {
		b := bytes.Clone(good)
		b[at] ^= 0x40
		return b
	}
	cases := []struct {
		name string
		b    []byte
		want string
	}{
		{"bad magic", damage(0), "bad magic"},
		{"future format", damage(len(tableMagic)), "table format"},
		{"wrong schema fingerprint", damage(fpAt + 5), "schema fingerprint"},
		{"flipped payload byte", damage(len(good) - 3), "checksum"},
		{"flipped checksum byte", damage(fpAt + 32 + 1 + len("somekey") + 1), "checksum"},
		{"truncated payload", good[:len(good)-1], "checksum"},
		{"extended payload", append(bytes.Clone(good), 0), "checksum"},
		{"header cut", good[:fpAt+10], "truncated"},
		{"empty", nil, "bad magic"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := DecodeTable(c.b)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want an error with %q", err, c.want)
			}
		})
	}
}

// A payload whose checksum is right but whose content is wrong is an error, not a
// crash: the decoder bounds every length.
func TestTableRejectsMalformedPayload(t *testing.T) {
	if _, err := decodePayload([]byte{0}); err != nil {
		t.Fatalf("the empty PoolData, the control: %v", err)
	}
	for name, payload := range map[string][]byte{
		"no such field":      {0x7F},
		"huge slice":         {5, 0xFF, 0xFF, 0xFF, 0xFF, 0x0F},
		"truncated value":    {1, 9},
		"no terminator":      {},
		"bytes past the end": {0, 0},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodePayload(payload); err == nil || !strings.Contains(err.Error(), "malformed payload") {
				t.Errorf("%v: %v", payload, err)
			}
		})
	}
}

func TestSchemaFingerprintStable(t *testing.T) {
	if schemaFingerprint(reflect.TypeFor[PoolData]()) != poolSchema() {
		t.Error("fingerprint is not stable")
	}
	type a struct{ X int8 }
	type b struct{ Y int8 }
	type c struct{ X [2]int8 }
	type d struct{ X [3]int8 }
	if schemaFingerprint(reflect.TypeFor[a]()) == schemaFingerprint(reflect.TypeFor[b]()) {
		t.Error("a field rename did not change the fingerprint")
	}
	if schemaFingerprint(reflect.TypeFor[c]()) == schemaFingerprint(reflect.TypeFor[d]()) {
		t.Error("an array length did not change the fingerprint")
	}
}

func TestSchemaFingerprintRejectsUnsupportedKinds(t *testing.T) {
	type inner struct{ P *int }
	cases := map[string]reflect.Type{
		"map":        reflect.TypeFor[struct{ M map[string]int }](),
		"func":       reflect.TypeFor[struct{ F func() }](),
		"pointer":    reflect.TypeFor[struct{ P *int }](),
		"interface":  reflect.TypeFor[struct{ I any }](),
		"float":      reflect.TypeFor[struct{ F float64 }](),
		"nested":     reflect.TypeFor[[]inner](),
		"unexported": reflect.TypeFor[struct{ x int }](),
	}
	for name, typ := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("%s did not panic", typ)
				}
			}()
			schemaFingerprint(typ)
		})
	}
	// Encode refuses them too, even where the schema walk was skipped.
	one := 1
	for name, v := range map[string]any{
		"map":     struct{ M map[string]int }{map[string]int{"a": 1}},
		"func":    struct{ F func() }{func() {}},
		"pointer": struct{ P *int }{&one},
	} {
		t.Run("encode "+name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("encoding %T did not panic", v)
				}
			}()
			encodeValue(nil, reflect.ValueOf(v))
		})
	}
}
