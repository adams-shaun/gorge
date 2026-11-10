package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func lowerCorpusCard(t *testing.T, name string) *lCard {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup(name)
	if !ok || card == nil {
		t.Fatalf("%s not in the corpus", name)
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	return lowerCard(p, card.Faces[0].Name, card, false)
}

func lowerScript(t *testing.T, name, src string) []string {
	t.Helper()
	card, diags := cards.ParseBytes("probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("script did not parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: testutil.CorpusRegistry(t)}
	var out []string
	for _, rf := range lowerCard(p, name, card, false).Refusals {
		out = append(out, rf.Reason)
	}
	return out
}

// TestImprintZoneWriters: the Token that imprints what it creates lowers with
// the imprint flag, the read over the imprinted set and the clearing Cleanup
// (Iroh, Tea Master); an exile that imprints lowers the same way (Siren's
// Ruse); the riders that share the flag's bits refuse by name.
func TestImprintZoneWriters(t *testing.T) {
	iroh := lowerCorpusCard(t, "Iroh, Tea Master")
	for _, rf := range iroh.Refusals {
		t.Errorf("Iroh refused: %s", rf.Reason)
	}
	var flagged, read, cleared bool
	for _, a := range iroh.Abilities {
		for _, in := range a.Prog {
			flagged = flagged || in.Op == "CreateToken" && in.C&2048 != 0
			read = read || in.Op == "PutCounter" && in.Obj.Kind == rImprinted
			cleared = cleared || in.Op == "ClearImprinted"
		}
	}
	if !flagged || !read || !cleared {
		t.Errorf("Iroh: imprint flag %v, read of the imprinted set %v, clear %v; want all three", flagged, read, cleared)
	}

	ruse := lowerCorpusCard(t, "Siren's Ruse")
	for _, rf := range ruse.Refusals {
		t.Errorf("Siren's Ruse refused: %s", rf.Reason)
	}
	marked := false
	for _, a := range ruse.Abilities {
		for _, in := range a.Prog {
			marked = marked || in.Op == "Exile" && strings.Contains(in.CS, "ImprintMove")
		}
	}
	if !marked {
		t.Error("Siren's Ruse: the exile carries no imprint mark")
	}

	const src = `Name:Imprint Zone Probe
ManaCost:1 W
Types:Creature Human
PT:1/1
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ PrTok | TriggerDescription$ probe
SVar:PrTok:DB$ Token | TokenScript$ w_1_1_ally TOKPARAMS| ImprintTokens$ True | SubAbility$ PrPut
SVar:PrPut:DB$ PutCounter | Defined$ Imprinted | CounterType$ P1P1 | CounterNum$ 1 | SubAbility$ PrClean
SVar:PrClean:DB$ Cleanup | ClearImprinted$ True
Oracle:probe
`
	if rf := lowerScript(t, "Imprint Zone Probe", strings.Replace(src, "TOKPARAMS", "", 1)); len(rf) != 0 {
		t.Fatalf("the probe refused: %v", rf)
	}
	noClear := strings.Replace(strings.Replace(src, "TOKPARAMS", "", 1), " | SubAbility$ PrClean", "", 1)
	noClear = strings.Replace(noClear, "Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self", "Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You", 1)
	if !strings.Contains(noClear, "Phase$ Upkeep") || strings.Contains(noClear, "SubAbility$ PrClean") {
		t.Fatal("the no-Cleanup probe was not built")
	}
	if rf := lowerScript(t, "Imprint Zone Probe", noClear); !strings.Contains(strings.Join(rf, "\n"), "imprints without an unconditional Cleanup ClearImprinted$") {
		t.Errorf("a repeatable imprint without the Cleanup: refusals %v, want the uncleared-imprint refusal", rf)
	}
	for _, c := range []struct{ params, want string }{
		{"| AtEOT$ Destroy ", "Token ImprintTokens$ beside AttachedTo$ or an extended AtEOT$"},
		{"| AtEOT$ SacrificeCombat ", "Token AtEOT SacrificeCombat outside an Attacks trigger"},
	} {
		rf := lowerScript(t, "Imprint Zone Probe", strings.Replace(src, "TOKPARAMS", c.params, 1))
		if len(rf) == 0 || !strings.Contains(strings.Join(rf, "\n"), c.want) {
			t.Errorf("%q: refusals %v, want %q", c.params, rf, c.want)
		}
	}
}

// TestImprintCrossAbilityRefuses: gorge's imprint persists on the source, the
// kernel's set lasts one resolution, so a read in another ability than the
// writer's stays refused by name (Oblivious Bookworm, Superior Foes of
// Spider-Man, the probe below), and the card-level check refuses an ability
// that reads the set without writing it.
func TestImprintCrossAbilityRefuses(t *testing.T) {
	for _, c := range []struct{ name, want string }{
		{"Oblivious Bookworm", "Count$ImprintedSize"},
		{"Superior Foes of Spider-Man", "Defined$ Imprinted outside a DigUntil ImprintFound$"},
	} {
		var got []string
		for _, rf := range lowerCorpusCard(t, c.name).Refusals {
			got = append(got, rf.Reason)
		}
		if !strings.Contains(strings.Join(got, "\n"), c.want) {
			t.Errorf("%s: refusals %v, want %q", c.name, got, c.want)
		}
	}

	const src = `Name:Imprint Cross Probe
ManaCost:1 W
Types:Creature Human
PT:1/1
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ PrTok | TriggerDescription$ probe
SVar:PrTok:DB$ Token | TokenScript$ w_1_1_ally | ImprintTokens$ True
A:AB$ PutCounter | Cost$ T | Defined$ Imprinted | CounterType$ P1P1 | CounterNum$ 1 | SpellDescription$ probe
Oracle:probe
`
	rf := lowerScript(t, "Imprint Cross Probe", src)
	if len(rf) == 0 || !strings.Contains(strings.Join(rf, "\n"), "Defined$ Imprinted") {
		t.Errorf("the cross-ability read: refusals %v, want a Defined$ Imprinted refusal", rf)
	}

	lc := &lowerCtx{out: &lCard{Name: "x"}, name: "x"}
	lc.out.Abilities = []*lAbility{
		{Index: 0, Prog: []lInstr{{Op: "CreateToken", C: 2048}, {Op: "ClearImprinted"}}},
		{Index: 1, Prog: []lInstr{{Op: "PutCounter", Obj: lRef{Kind: rImprinted}}}},
	}
	lc.refuseCrossAbilityImprinted()
	if len(lc.out.Refusals) != 1 || lc.out.Refusals[0].Node != "ab[1]" {
		t.Errorf("card-level check: refusals %v, want only ab[1]", lc.out.Refusals)
	}
}
