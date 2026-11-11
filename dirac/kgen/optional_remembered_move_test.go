package kgen

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// optRemLowered lowers a synthetic sorcery whose root is `root` with the
// SubAbility `sub`, and returns its refusal reasons and the "Op:C" of every
// instruction it emitted.
func optRemLowered(t *testing.T, root, sub string) (refusals, ops []string) {
	t.Helper()
	svars := map[string]string{"DBX": sub}
	r := cards.ResolveSVar(map[string]string{"R": root}, "R")
	if r == nil {
		t.Fatalf("root %q did not resolve", root)
	}
	r.Params["SubAbility"] = "DBX"
	r.Sub = cards.ResolveSVar(svars, "DBX")
	if r.Sub == nil {
		t.Fatalf("sub %q did not resolve", sub)
	}
	face := &cards.Face{Name: "Synthetic", ManaCost: "B", Types: []string{"Sorcery"}, SVars: svars, Abilities: []*cards.SA{r}}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc := lowerCard(p, "Synthetic", card, false)
	for _, rf := range lc.Refusals {
		refusals = append(refusals, rf.Reason)
	}
	for _, a := range lc.Abilities {
		for _, in := range a.Prog {
			ops = append(ops, in.Op+":"+strconv.Itoa(in.C))
		}
	}
	return
}

// TestOptionalRememberedLibraryMove: an Optional$ True move of a looking
// Peek's remembered card from the library to the battlefield lowers behind
// kop.MayGuardMoveRemembered (Into the Wilds); the same move with no Peek
// before it, a Peek of two cards, or to another zone stays refused by name.
func TestOptionalRememberedLibraryMove(t *testing.T) {
	const move = "DB$ ChangeZone | Optional$ True | Origin$ Library | Destination$ Battlefield | Defined$ Remembered"
	look := "SP$ PeekAndReveal | Cost$ B | PeekAmount$ 1 | NoReveal$ True | RememberPeeked$ True"

	got, ops := optRemLowered(t, look, move)
	if len(got) != 0 {
		t.Fatalf("refused: %q", got)
	}
	want := "MayAsk:" + strconv.Itoa(kop.MayGuardMoveRemembered) + " Return:1"
	if !strings.Contains(strings.Join(ops, " "), want) {
		t.Errorf("ops %q lack %q", ops, want)
	}

	for _, c := range []struct{ name, root, sub string }{
		{"no Peek before it", "SP$ Draw | Cost$ B | NumCards$ 1", move},
		{"two cards peeked", "SP$ PeekAndReveal | Cost$ B | PeekAmount$ 2 | NoReveal$ True | RememberPeeked$ True", move},
		{"to the hand", look, "DB$ ChangeZone | Optional$ True | Origin$ Library | Destination$ Hand | Defined$ Remembered"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := optRemLowered(t, c.root, c.sub)
			kdefWant(t, got, "ChangeZone Optional$")
		})
	}
}
