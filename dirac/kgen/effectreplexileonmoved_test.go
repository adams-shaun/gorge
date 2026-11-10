package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// stackExileOps returns the C of each StackExileMark a lowered card carries.
func stackExileOps(lc *lCard) []int {
	var cs []int
	for _, ab := range lc.Abilities {
		for _, in := range ab.Prog {
			if in.Op == "StackExileMark" {
				cs = append(cs, in.C)
			}
		}
	}
	return cs
}

// TestLowerEffectReplExileonmoved: the Effect that exiles a named spell
// instead of putting it into a graveyard as it leaves the stack lowers to
// OpExt ExtStackExileMark. Lilah, Undefeated Slickshot (trigger form, resolved
// exit only, owner gate, plot rider) and Nita, Forum Conciliator (grant form)
// lower whole; Goliath Daydreamer's Effect lowers (its dream counter stalls in
// the kernel) and the card refuses only on its Play. Each out-of-shape variant
// still refuses by name.
func TestLowerEffectReplExileonmoved(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	lowerNamed := func(name string) *lCard {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		return lowerCard(p, card.Faces[0].Name, card, false)
	}

	lilah := lowerNamed("Lilah, Undefeated Slickshot")
	if got := keclReason(lilah.Refusals); got != "" {
		t.Errorf("Lilah: refusals %q, want none", got)
	}
	wantLilah := kop.StackExileOwn | kop.StackExilePlot | kop.StackExileResolved
	if got := stackExileOps(lilah); len(got) != 1 || got[0] != wantLilah {
		t.Errorf("Lilah: StackExileMark C %v, want [%d]", got, wantLilah)
	}

	nita := lowerNamed("Nita, Forum Conciliator")
	if got := keclReason(nita.Refusals); got != "" {
		t.Errorf("Nita: refusals %q, want none", got)
	}
	if got := stackExileOps(nita); len(got) != 1 || got[0] != kop.StackExileGrant {
		t.Errorf("Nita: StackExileMark C %v, want [%d]", got, kop.StackExileGrant)
	}

	goliath := lowerNamed("Goliath Daydreamer")
	got := keclReason(goliath.Refusals)
	if strings.Contains(got, "Effect replacement") || strings.Contains(got, "stack-exile") {
		t.Errorf("Goliath: still refuses on the Effect: %q", got)
	}
	if !strings.Contains(got, "Play ValidZone$ Exile without IsRemembered") {
		t.Errorf("Goliath: refusals %q, want its Play", got)
	}
	if ops := stackExileOps(goliath); len(ops) != 1 || ops[0]&kop.StackExileDream == 0 {
		t.Errorf("Goliath: StackExileMark C %v, want the dream bit", ops)
	}

	// Out-of-shape variants of Lilah's script refuse by name.
	const base = "Name:Exile Bf Probe\nManaCost:1 U R\nTypes:Legendary Creature Human Rogue\nPT:3/3\n" +
		"T:Mode$ SpellCast | ValidCard$ Instant.MultiColor | Execute$ TrigEffect | TriggerZones$ Battlefield | TriggerDescription$ probe\n" +
		"SVar:TrigEffect:DB$ Effect | ReplacementEffects$ MoveToExileReplace | RememberObjects$ TriggeredCard | ConditionDefined$ TriggeredCard | ConditionPresent$ Card | ConditionZone$ Stack | ExileOnMoved$ Stack\n" +
		"SVar:MoveToExileReplace:Event$ Moved | ValidCard$ Card.IsRemembered+YouOwn | Origin$ Stack | Destination$ Graveyard | Fizzle$ False | ReplaceWith$ ReplaceExile | Description$ probe\n" +
		"SVar:ReplaceExile:DB$ ChangeZone | Defined$ ReplacedCard | Origin$ Stack | Destination$ Exile\n"
	if r := exileBfSynth(t, base); len(r) != 0 {
		t.Fatalf("the probe script itself refuses: %q", keclReason(r))
	}
	for _, c := range []struct{ name, from, to, want string }{
		{"Fizzle$ True", "Fizzle$ False", "Fizzle$ True", "Effect stack-exile replacement Fizzle$"},
		{"any owner", "Card.IsRemembered+YouOwn", "Card.IsRemembered", "Effect stack-exile replacement ValidCard$"},
		{"origin", "Origin$ Stack | Destination$ Graveyard", "Origin$ Hand | Destination$ Graveyard", "Effect stack-exile replacement Origin$"},
		{"body to hand", "Origin$ Stack | Destination$ Exile", "Origin$ Stack | Destination$ Hand", "Effect stack-exile replacement body"},
		{"body extra key", "Destination$ Exile\n", "Destination$ Exile | Tapped$ True | Foo$ Bar\n", "Effect stack-exile replacement body"},
		{"no condition", " | ConditionDefined$ TriggeredCard | ConditionPresent$ Card | ConditionZone$ Stack", "", "Effect stack-exile condition"},
	} {
		src := strings.Replace(base, c.from, c.to, 1)
		if src == base {
			t.Fatalf("%s: variant did not change the script", c.name)
		}
		if got := keclReason(exileBfSynth(t, src)); !strings.Contains(got, c.want) {
			t.Errorf("%s: refusals %q, want %q", c.name, got, c.want)
		}
	}
}
