package host

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestFeedbackMatchMarshalsTheSidecarsKeys pins the structural guarantee the
// feedback snapshot leans on: match.json claims to be "the sidecar plus deck
// contents", so every key the persisted sidecar writes must appear with the
// same name in FeedbackMatch's own JSON — a field added to sidecar without
// being mirrored into feedbackMatch (host/feedback.go) fails here, and the
// snapshot cannot silently drift from the persisted shape.
func TestFeedbackMatchMarshalsTheSidecarsKeys(t *testing.T) {
	winner := uint8(1)
	sc := sidecar{
		Table: "t1", Match: 2, Seed: 7,
		Seats:        nil,
		Names:        []string{"a", "b"},
		PlayerNames:  []string{"P1", "P2"},
		Decks:        []string{"a", "b"},
		Spectator:    "omniscient",
		State:        "live",
		Result:       "win",
		Winner:       &winner,
		Head:         "abcd",
		Events:       42,
		Turns:        3,
		Reason:       "",
		Mulligans:    1,
		Format:       FormatCommander,
		StartingLife: 40,
		Commanders:   [][]int{{0}},
	}
	sidecarKeys := func(t *testing.T, raw []byte) map[string]bool {
		t.Helper()
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		keys := map[string]bool{}
		for k := range m {
			keys[k] = true
		}
		return keys
	}
	scRaw, err := json.Marshal(sc)
	if err != nil {
		t.Fatal(err)
	}
	fm := feedbackMatch(sc, [][]string{{"x"}, {"y"}}, nil, nil)
	fmRaw, err := json.Marshal(fm)
	if err != nil {
		t.Fatal(err)
	}
	want := sidecarKeys(t, scRaw)
	got := sidecarKeys(t, fmRaw)
	for k := range want {
		if !got[k] {
			t.Errorf("sidecar key %q missing from FeedbackMatch's JSON", k)
		}
	}
	// A match whose cfg.Tokens is empty (a corpus with no tokens compiled,
	// or a registry built without one) carries no tokens key at all — the
	// omitempty keeps the plain sidecar-plus-deck shape byte-comparable.
	extra := len(got) - len(want)
	if extra != 1 {
		t.Errorf("FeedbackMatch carries %d keys beyond the sidecar's %d, want exactly 1 (deck_cards)", extra, len(want))
	}
	if !got["deck_cards"] {
		t.Error("FeedbackMatch's JSON is missing deck_cards")
	}
	// With tokens captured, both new keys appear: the scripts themselves
	// and, when any could not be read back, the reason list. These are the
	// only fields beyond the sidecar plus deck_cards by design — anything
	// else must be added here, so the snapshot shape cannot drift silently.
	fm.Tokens = map[string]string{"r_1_1_goblin": "Name:Goblin Token\n"}
	fm.TokensUnread = []string{"c_3_3_wurm: no source path"}
	fmRaw, err = json.Marshal(fm)
	if err != nil {
		t.Fatal(err)
	}
	got = sidecarKeys(t, fmRaw)
	extra = len(got) - len(want)
	if extra != 3 || !got["tokens"] || !got["tokens_unread"] {
		t.Errorf("FeedbackMatch with tokens carries %d keys beyond the sidecar (want 3: deck_cards, tokens, tokens_unread); keys deck_cards=%v tokens=%v tokens_unread=%v",
			extra, got["deck_cards"], got["tokens"], got["tokens_unread"])
	}
	// And the shared values must agree, not just the key names.
	var scBack, fmBack map[string]any
	_ = json.Unmarshal(scRaw, &scBack)
	_ = json.Unmarshal(fmRaw, &fmBack)
	for k := range want {
		if !reflect.DeepEqual(scBack[k], fmBack[k]) {
			t.Errorf("key %q: sidecar %v, FeedbackMatch %v", k, scBack[k], fmBack[k])
		}
	}
}
