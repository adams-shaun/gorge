package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Mental Note mills two, then draws: the card it mills can be a Dredge card
// whose election replaces that draw (CR 702.52a), though the graveyard holds
// none when the resolution starts. Forge's own resolution asks the player
// (Mental Note over Golgari Thug), so the ask-free predicate must open the
// chain's dredge-fill gate while a Dredge card sits in a library, and may stay
// closed when none is anywhere a fill could take it from.
func TestMillThenDrawChainOpensDredgeFillGate(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	note, ok := reg.Lookup("Mental Note")
	if !ok {
		t.Fatal("Mental Note missing from corpus")
	}
	thug, ok := reg.Lookup("Golgari Thug")
	if !ok {
		t.Fatal("Golgari Thug missing from corpus")
	}
	sa := note.Faces[0].Abilities[0]
	may, cond := cards.SAChainMayAskCond(sa, note.Faces[0].SVars, note.Faces[0], true)
	if may || cond&cards.MayAskCondDredgeFill == 0 {
		t.Fatalf("Mill then Draw chain: may=%v cond=%b, want the dredge-fill reason only", may, cond)
	}
	// The resolving spell's whole predicate, with Mental Note on the stack.
	onStack := func(seed uint64, extra ...*cards.Card) bool {
		e, _ := tokenReplGame(t, seed, append([]*cards.Card{note}, extra...)...)
		moveSeededCard(t, e, 0, note, state.ZHand)
		addMana(t, e, 0, "U")
		castSpellOption(t, e, "Mental Note")
		if len(e.G.Stack) == 0 {
			t.Fatal("Mental Note is not on the stack")
		}
		return tapeMayAsk(e)
	}
	if onStack(5101) {
		t.Fatal("no Dredge card anywhere: the chain must stay ask-free")
	}
	if !onStack(5102, thug) {
		t.Fatal("a Dredge card in a library must make Mental Note's resolution may-ask")
	}
}

// TestDredgeFillChainCensus walks real cards of each shape: an allowlisted
// graveyard-filling link before a Draw takes the dredge-fill reason; a
// non-allowlisted filler (Surveil, Discard, Sacrifice, Dig) is already "may
// ask" by its API, so the tape run is taken whatever the board holds; a Draw
// before the fill is neither (Careful Study, Faithless Looting draw first).
func TestDredgeFillChainCensus(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	rows := []struct {
		name      string
		fill, may bool
	}{
		{"Mental Note", true, false},
		{"Exclude", true, false},
		{"Aura Blast", true, false},
		{"Discovery // Dispersal", false, true},
		{"Dredge", false, true},
		{"Rowan's Grim Search", false, true},
		{"Brainbite", false, true},
		{"Careful Study", false, true},
		{"Faithless Looting", false, true},
	}
	for _, r := range rows {
		c, ok := reg.Lookup(r.name)
		if !ok {
			t.Errorf("%s missing from corpus", r.name)
			continue
		}
		f := c.Faces[0]
		if len(f.Abilities) == 0 {
			t.Errorf("%s has no spell ability", r.name)
			continue
		}
		may, cond := cards.SAChainMayAskCond(f.Abilities[0], f.SVars, f, true)
		if got := cond&cards.MayAskCondDredgeFill != 0; got != r.fill || may != r.may {
			t.Errorf("%s: fill=%v may=%v, want fill=%v may=%v", r.name, got, may, r.fill, r.may)
		}
	}
}
