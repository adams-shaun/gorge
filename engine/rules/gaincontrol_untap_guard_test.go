package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGainControlUntapOnlyUntapsATappedPermanent pins Involuntary
// Employment's "Untap that creature" (GainControl Untap$ True): an untapped
// permanent cannot untap (CR 701.26b), so no Untap event -- and no "becomes
// untapped" trigger, Fishing Pole's -- follows for one; a tapped one untaps;
// a tapped one with a stun counter loses the counter instead (CR 122.1d).
func TestGainControlUntapOnlyUntapsATappedPermanent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		tapped    bool
		stun      bool
		wantUntap bool
	}{
		{"untapped", false, false, false},
		{"tapped", true, false, true},
		{"tapped-stunned", true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := New(Config{Seed: 1, Names: []string{"a", "b"}, Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
			spell := e.G.AddObject(choiceCorpusCard(t, "Involuntary Employment"), 0)
			bears := e.G.AddObject(choiceCorpusCard(t, "Grizzly Bears"), 1)
			e.emit(events.Event{Kind: events.MoveZone, Obj: bears.ID, From: state.ZLibrary, To: state.ZBattlefield})
			if tc.tapped {
				e.emit(events.Event{Kind: events.Tap, Obj: bears.ID})
			}
			if tc.stun {
				e.emit(events.Event{Kind: events.CounterChange, Obj: bears.ID, Counter: "STUN", Amount: 1})
			}
			sa := spell.Face().SpellAbility()
			mark := len(e.L.Events)
			effects.Resolve(e, &effects.Ctx{Source: spell.ID, Controller: 0, SVars: spell.Face().SVars,
				Targets: []state.Target{{Obj: bears.ID}}}, sa)
			if got := e.G.Obj(bears.ID).Controller; got != 0 {
				t.Fatalf("controller = %d, want 0", got)
			}
			untapped := false
			for _, ev := range e.L.Events[mark:] {
				if ev.Kind == events.Untap && ev.Obj == bears.ID {
					untapped = true
				}
			}
			if untapped != tc.wantUntap {
				t.Fatalf("Untap event emitted = %v, want %v", untapped, tc.wantUntap)
			}
			if tc.stun && (e.G.Obj(bears.ID).Counter("STUN") != 0 || !e.G.Obj(bears.ID).Tapped) {
				t.Fatalf("stunned: STUN %d tapped %v, want the counter removed instead of untapping", e.G.Obj(bears.ID).Counter("STUN"), e.G.Obj(bears.ID).Tapped)
			}
		})
	}
}
