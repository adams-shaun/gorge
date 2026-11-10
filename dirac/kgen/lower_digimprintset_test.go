package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// digImprintScript is an inline stand-in for the permanent Dig Imprint$ shape
// (the real scripts are Forge's, GPL, and never committed); the DIGPARAMS and
// TAIL holes are what the refusal variants change.
const digImprintScript = `Name:Imprint Probe
ManaCost:1 B
Types:Creature Human
PT:2/1
T:Mode$ DamageDone | ValidSource$ Card.Self | ValidTarget$ Player | CombatDamage$ True | TriggerZones$ Battlefield | Execute$ PrDig | TriggerDescription$ probe
SVar:PrDig:DB$ Dig | Defined$ TriggeredTarget | DigNum$ 2 | ChangeNum$ All | DestinationZone$ Exile | Imprint$ True DIGPARAMS| SubAbility$ PrEff
SVar:PrEff:DB$ Effect | StaticAbilities$ PrPlay | RememberObjects$ Imprinted | ForgetOnMoved$ Exile | SubAbility$ PrClean
SVar:PrPlay:Mode$ Continuous | MayPlay$ True | Affected$ Card.IsRemembered | AffectedZone$ Exile | Description$ probe play.
SVar:PrClean:DB$ Cleanup | ClearRemembered$ True TAIL
Oracle:probe
`

func digImprintProbe(t *testing.T, params, tail string) (refusals []string, lc *lCard) {
	t.Helper()
	src := strings.NewReplacer("DIGPARAMS", params, "TAIL", tail).Replace(digImprintScript)
	card, diags := cards.ParseBytes("imprint_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("script did not parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{byName: map[string]*cards.Card{"Imprint Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc = lowerCard(p, "Imprint Probe", card, false)
	for _, rf := range lc.Refusals {
		refusals = append(refusals, rf.Reason)
	}
	return refusals, lc
}

// progOps lists the op names of the card's first ability.
func progOps(lc *lCard) []string {
	var ops []string
	for _, in := range lc.Abilities[0].Prog {
		ops = append(ops, in.Op)
	}
	return ops
}

// TestDigImprintPermanent: Reno and Rude and Rotisserie Elemental lower with
// the dig followed by the imprint marker, the play grant over the imprinted
// set and the cleanup's clear; the shapes the kernel set does not hold stay
// refused by name.
func TestDigImprintPermanent(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Reno and Rude", "Rotisserie Elemental"} {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		for _, rf := range lc.Refusals {
			t.Errorf("%s refused: %s", name, rf.Reason)
		}
		ops := strings.Join(progOps(lc), " ")
		if !strings.Contains(ops, "Dig DigImprint") || !strings.Contains(ops, "MayPlay") || !strings.HasSuffix(ops, "ClearImprinted") {
			t.Errorf("%s lowered to %q, want Dig DigImprint ... MayPlay ... ClearImprinted", name, ops)
		}
		for _, in := range lc.Abilities[0].Prog {
			if in.Op == "MayPlay" && in.Obj.Kind != rImprinted {
				t.Errorf("%s: MayPlay reads %v, want the imprinted set", name, in.Obj)
			}
		}
	}

	rf, lc := digImprintProbe(t, "", "| ClearImprinted$ True")
	if len(rf) != 0 {
		t.Fatalf("the probe refused: %v", rf)
	}
	if ops := strings.Join(progOps(lc), " "); !strings.HasPrefix(ops, "Dig DigImprint MayPlay") {
		t.Fatalf("probe lowered to %q", ops)
	}

	for _, c := range []struct{ params, tail, want string }{
		{"| ChangeValid$ Creature ", "| ClearImprinted$ True", "Dig Imprint$ on a permanent beside ChangeValid$"},
		{"| Optional$ True ", "| ClearImprinted$ True", "Dig Imprint$ on a permanent beside Optional$"},
		{"| RememberChanged$ True ", "| ClearImprinted$ True", "Dig Imprint$ on a permanent beside RememberChanged$"},
	} {
		rf, _ := digImprintProbe(t, c.params, c.tail)
		if len(rf) == 0 || !strings.Contains(rf[0], c.want) {
			t.Errorf("%q: refusals %v, want %q", c.params, rf, c.want)
		}
	}
}
