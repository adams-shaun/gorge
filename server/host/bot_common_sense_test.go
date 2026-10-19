package host

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestBotCommonSensePersistsAndRestoreAdoptsDefault pins TableConfig.
// BotCommonSense as persisted per-table configuration (the guard changes the
// intents a seat answers, so a resumed table must rebuild the same seats):
// an explicit setting — true or false — survives a restart verbatim, a
// pre-field tables.json adopts the server's DefaultBotCommonSense exactly the
// way bot_auto_pay_mana does.
func TestBotCommonSensePersistsAndRestoreAdoptsDefault(t *testing.T) {
	dir := t.TempDir()
	o := diskOptions(t, dir)
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.AddTable(TableConfig{ID: "t1", Seats: 2, Decks: []string{"a", "b"}, Spectator: view.Public, BotCommonSense: true}); err != nil {
		r.Close()
		t.Fatal(err)
	}
	if err := r.AddTable(TableConfig{ID: "t2", Seats: 2, Decks: []string{"a", "b"}, Spectator: view.Public, BotCommonSense: false}); err != nil {
		r.Close()
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "tables.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Precondition: both explicit settings reached the persisted file, so the
	// restart assertions below compare real values, not absences.
	if !strings.Contains(string(raw), `"bot_common_sense": true`) || !strings.Contains(string(raw), `"bot_common_sense": false`) {
		t.Fatalf("an explicit bot_common_sense was omitted from tables.json:\n%s", raw)
	}

	// A current explicit setting survives a restart whose startup default
	// differs from it: true with the flag off, false with the flag on.
	restored := diskOptions(t, dir)
	restored.DefaultBotCommonSense = true
	r, err = New(restored)
	if err != nil {
		t.Fatal(err)
	}
	r.mu.RLock()
	got1 := r.tables["t1"].cfg.BotCommonSense
	got2 := r.tables["t2"].cfg.BotCommonSense
	r.mu.RUnlock()
	if !got1 || got2 {
		r.Close()
		t.Fatalf("restart replaced an explicit setting: t1 %v (want true) t2 %v (want false)", got1, got2)
	}
	r.Close()

	// Remove the key from every record to model tables.json written before
	// the field existed: both adopt the restore default.
	var legacy struct {
		Tables []struct {
			Config map[string]json.RawMessage `json:"config"`
			Match  int                        `json:"match"`
		} `json:"tables"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		t.Fatal(err)
	}
	for i := range legacy.Tables {
		delete(legacy.Tables[i].Config, "bot_common_sense")
	}
	raw, err = json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err = New(restored)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.mu.RLock()
	migrated1 := r.tables["t1"].cfg.BotCommonSense
	migrated2 := r.tables["t2"].cfg.BotCommonSense
	r.mu.RUnlock()
	if !migrated1 || !migrated2 {
		t.Fatalf("legacy tables did not adopt DefaultBotCommonSense on restore: t1 %v t2 %v", migrated1, migrated2)
	}
}

// wrappedBot asserts s carries the bots/sanity guard (the UnwrapSeat
// decorator chain of sanity.Wrap and its TargetHint opt-in) and returns the
// plain seat underneath.
func wrappedBot(t *testing.T, s seat.Seat) seat.Seat {
	t.Helper()
	u, ok := s.(interface{ UnwrapSeat() seat.Seat })
	if !ok {
		t.Fatalf("common-sense seat = %T, want the sanity guard's UnwrapSeat decorator", s)
	}
	if th, ok := s.(seat.TargetHintConsumer); !ok || !th.WantsTargetHints() {
		t.Fatalf("common-sense seat = %T, want the guard's TargetHint opt-in", s)
	}
	return u.UnwrapSeat()
}

// TestBotCommonSenseBuildsWrappedSeats pins Done means 2: with the setting,
// a table's `bot` seats (and its human-seat caretakers) are built wrapped by
// sanity.Wrap; without it, they are exactly today's plain policy seat.
func TestBotCommonSenseBuildsWrappedSeats(t *testing.T) {
	names := []string{"a", "b"}
	for _, tc := range []struct{ commonSense, wantWrapped bool }{{false, false}, {true, true}} {
		seats := defaultSeatsWithAutoPayMana(BotPolicy, false, tc.commonSense, names, 7, 0, bots.Deps{}, hostedDecisionDeadlineMS, nil)
		for i, s := range seats {
			if !tc.wantWrapped {
				// Precondition: the flag-off seat really is the plain
				// *heuristic.Bot the served tables have always built.
				if _, ok := s.(*heuristic.Bot); !ok {
					t.Fatalf("commonSense=false seat %d = %T, want the unwrapped plain bot", i, s)
				}
				continue
			}
			if inner := wrappedBot(t, s); func() bool {
				_, ok := inner.(*heuristic.Bot)
				return !ok
			}() {
				t.Fatalf("commonSense=true seat %d unwrapped to %T, want the built *heuristic.Bot", i, inner)
			}
		}
		caretaker, err := newCaretakerSeat(BotPolicy, 7, false, tc.commonSense, bots.Deps{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !tc.wantWrapped {
			if _, ok := caretaker.(*heuristic.Bot); !ok {
				t.Fatalf("commonSense=false caretaker = %T, want the unwrapped plain bot", caretaker)
			}
			continue
		}
		if inner := wrappedBot(t, caretaker); func() bool {
			_, ok := inner.(*heuristic.Bot)
			return !ok
		}() {
			t.Fatalf("commonSense=true caretaker unwrapped to %T, want the built *heuristic.Bot", inner)
		}
	}
}
