package limited

import (
	"encoding/json"
	"testing"
)

func TestRarityJSONRoundtrip(t *testing.T) {
	for _, r := range []Rarity{Common, Uncommon, Rare, Mythic, Land, Special} {
		b, err := r.MarshalJSON()
		if err != nil {
			t.Fatalf("marshal %c: %v", r, err)
		}
		if string(b) != `"`+r.String()+`"` {
			t.Fatalf("marshal %c produced %s", r, b)
		}
		var back Rarity
		if err := back.UnmarshalJSON(b); err != nil {
			t.Fatalf("unmarshal %s: %v", b, err)
		}
		if back != r {
			t.Fatalf("roundtrip of %c came back %c", r, back)
		}
	}
}

func TestRarityUnmarshalRejects(t *testing.T) {
	for _, bad := range []string{`""`, `"CC"`, `C`, `null`, `3`} {
		var r Rarity
		if err := r.UnmarshalJSON([]byte(bad)); err == nil {
			t.Errorf("UnmarshalJSON(%s) succeeded", bad)
		}
	}
}

func TestCardRefJSONTags(t *testing.T) {
	// The ref's JSON shape is part of the event-log and wire contract: the
	// snake_case tags must spell the field names exactly.
	ref := CardRef{ID: "c117", Name: "Serra Angel", Set: "FDN", Number: "14", Rarity: Uncommon}
	want := `{"id":"c117","name":"Serra Angel","set":"FDN","number":"14","rarity":"U"}`
	b, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != want {
		t.Fatalf("CardRef JSON = %s, want %s", b, want)
	}
}
