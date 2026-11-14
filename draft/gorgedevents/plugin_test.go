package gorgedevents

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
)

// prepared parses args on the plugin's own flags and returns the
// Commander value hook Prepare's wiring hands the event host.
func prepared(t *testing.T, args ...string) (*wiring, error) {
	t.Helper()
	p := &Plugin{}
	fs := flag.NewFlagSet("gorged", flag.ContinueOnError)
	p.Flags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	w, _, err := p.Prepare("", nil, nil)
	if err != nil {
		return nil, err
	}
	return w.(*wiring), nil
}

// TestCommanderRatingsFlag: -commander-ratings loads a draftsim-format value
// file (read at -commander-ratings-field, default blended) into the hook the
// event host's Commander persona values cards by; no flag keeps Heuristic
// (a nil hook), and an unreadable or malformed file fails Prepare, so
// gorged refuses to start.
func TestCommanderRatingsFlag(t *testing.T) {
	dir := t.TempDir()
	stub := []byte(`{"Card A": {"blended": 0.9, "prior": 0.1}, "Card B": {"blended": 0.1, "prior": 0.9}, "Card C": {"prior": 0.5}}`)
	path := filepath.Join(dir, "ratings.json")
	if err := os.WriteFile(path, stub, 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := prepared(t)
	if err != nil || w.commanderValue != nil {
		t.Fatalf("no flag: hook set %v, err %v; want Heuristic (nil)", w != nil && w.commanderValue != nil, err)
	}

	for _, field := range []string{"blended", "prior"} {
		args := []string{"-commander-ratings", path}
		if field != "blended" {
			args = append(args, "-commander-ratings-field", field)
		}
		w, err := prepared(t, args...)
		if err != nil || w.commanderValue == nil {
			t.Fatalf("field %s: hook set %v, err %v", field, w != nil && w.commanderValue != nil, err)
		}
		ov, err := pickbot.ParseRatingsOverride(stub, path, field, commanderRatingsSet)
		if err != nil {
			t.Fatal(err)
		}
		want := ov.CommanderValue(pickbot.DefaultTuning)
		for _, n := range []string{"Card A", "Card B", "Card C", "Not In File"} {
			ref := limited.CardRef{Name: n}
			if got, w := w.commanderValue(nil, ref), want(nil, ref); got != w {
				t.Errorf("field %s: value(%s) = %v, want %v", field, n, got, w)
			}
		}
		a, b := w.commanderValue(nil, limited.CardRef{Name: "Card A"}), w.commanderValue(nil, limited.CardRef{Name: "Card B"})
		if (field == "blended") != (a > b) {
			t.Errorf("field %s: Card A %v, Card B %v: the order does not follow the field", field, a, b)
		}
	}

	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"Card A": 1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{filepath.Join(dir, "missing.json"), bad} {
		if _, err := prepared(t, "-commander-ratings", f); err == nil || !strings.Contains(err.Error(), "-commander-ratings") {
			t.Errorf("%s: err %v, want a -commander-ratings error", f, err)
		}
	}
}
