package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Inline stand-ins for the two writer shapes (the real scripts are Forge's,
// GPL, and never committed); the holes are what the variants change.
const putChoicesProbe = `Name:Choice Probe
ManaCost:2 G
Types:Sorcery
A:SP$ PutCounter | Choices$ Creature.YouCtrl | CounterType$ P1P1 | CounterNum$ 1 PARAMS| SubAbility$ DBPump
SVar:DBPump:DB$ Pump | Defined$ Remembered | KW$ Trample | SubAbility$ DBEnd
SVar:DBEnd:DB$ Cleanup | ClearRemembered$ True
Oracle:probe
`

const searcherProbe = `Name:Searcher Probe
ManaCost:2 G
Types:Instant
A:SP$ ChangeZone | Origin$ Library | Destination$ Battlefield | ChangeType$ Land.Basic | ChangeNum$ 1 | Tapped$ True PARAMS| SubAbility$ DBImm
SVar:DBImm:DB$ ImmediateTrigger | Execute$ TrigCtr | ConditionCheckSVar$ Count$RememberedSize | SubAbility$ DBEnd
SVar:TrigCtr:DB$ PutCounter | ValidTgts$ Creature.YouCtrl | CounterType$ P1P1
SVar:DBEnd:DB$ Cleanup | ClearRemembered$ True
Oracle:probe
`

func lowerProbe(t *testing.T, script, name, params string) *lCard {
	t.Helper()
	card, diags := cards.ParseBytes("probe.txt", []byte(strings.Replace(script, "PARAMS", params, 1)))
	if card == nil {
		t.Fatalf("script did not parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, name, card, false)
}

func probeRefusals(lc *lCard) string {
	var rs []string
	for _, r := range lowerRefusals(lc) {
		rs = append(rs, r.Reason)
	}
	return strings.Join(rs, "; ")
}

// TestRememberChoiceWriters: PutCounter's Choices$ pick with RememberCards$
// and a library ChangeZone with RememberSearcher$ are remembered-set writers,
// so the same-chain readers (a Pump of Remembered, an ImmediateTrigger gated
// on the set's size) lower instead of looking like cross-ability reads.
func TestRememberChoiceWriters(t *testing.T) {
	// the writer: the recipient pick, then the counter with the B=2 arm
	lc := lowerProbe(t, putChoicesProbe, "Choice Probe", "| RememberCards$ True ")
	if r := probeRefusals(lc); r != "" {
		t.Fatalf("RememberCards$ on a Choices$ pick refused: %s", r)
	}
	var put *lInstr
	for i, in := range lc.Abilities[0].Prog {
		if in.Op == "PutCounter" {
			put = &lc.Abilities[0].Prog[i]
		}
	}
	if put == nil || put.Note != notePutRememberPut || !isConstN(put.B, 2) {
		t.Fatalf("PutCounter %+v, want the RememberPut arm (B=2)", put)
	}
	if got := probeRefusals(lowerProbe(t, putChoicesProbe, "Choice Probe", "")); !strings.Contains(got, "reads Remembered written by another ability") {
		t.Errorf("the same chain without a writer: refusals %q, want the cross-ability read", got)
	}
	if got := probeRefusals(lowerProbe(t, putChoicesProbe, "Choice Probe", "| RememberCards$ False ")); !strings.Contains(got, "RememberCards$") {
		t.Errorf("RememberCards$ False: refusals %q, want it named", got)
	}

	// the searcher: the marker sits right before the Search, the gate reads the count
	lc = lowerProbe(t, searcherProbe, "Searcher Probe", "| RememberSearcher$ True ")
	if r := probeRefusals(lc); r != "" {
		t.Fatalf("RememberSearcher$ refused: %s", r)
	}
	ops := strings.Join(progOps(lc), " ")
	if !strings.Contains(ops, "RememberSearcher Search") {
		t.Errorf("lowered to %q, want RememberSearcher right before Search", ops)
	}
	if got := probeRefusals(lowerProbe(t, searcherProbe, "Searcher Probe", "")); !strings.Contains(got, "reads Remembered written by another ability") {
		t.Errorf("the same chain without a writer: refusals %q, want the cross-ability read", got)
	}
	// shapes the count cannot carry stay refused by name
	for _, tc := range []struct{ params, want string }{
		{"| RememberSearcher$ True | Optional$ True ", "RememberSearcher$ beside Confirm"},
		{"| RememberSearcher$ False ", "RememberSearcher \"False\""},
		{"| RememberSearcher$ True | OriginAlternative$ Graveyard ", "RememberSearcher$ beside alt=Graveyard"},
	} {
		if got := probeRefusals(lowerProbe(t, searcherProbe, "Searcher Probe", tc.params)); !strings.Contains(got, tc.want) {
			t.Errorf("%s: refusals %q, want %q", tc.params, got, tc.want)
		}
	}
	// an object read of the set beside the searcher refuses: the kernel counts the player
	objRead := strings.Replace(searcherProbe, "SVar:DBEnd:", "SVar:DBPump:DB$ Pump | Defined$ Remembered | KW$ Trample | SubAbility$ DBEnd\nSVar:DBEnd:", 1)
	objRead = strings.Replace(objRead, "SubAbility$ DBImm", "SubAbility$ DBPump", 1)
	if got := probeRefusals(lowerProbe(t, objRead, "Searcher Probe", "| RememberSearcher$ True ")); !strings.Contains(got, "remembered-object read") {
		t.Errorf("an object read beside the searcher: refusals %q, want the object-read refusal", got)
	}

	// the two corpus cards lower clean, and the Charm's mode program carries the writer
	names := []string{"Season of Gathering", "Prishe's Wanderings"}
	lw := lowerNamedPool(t, names)
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]int{}
	for i, c := range all {
		if !c.Token {
			byName[c.Name] = i
		}
	}
	for _, name := range names {
		i, ok := byName[name]
		if !ok {
			t.Fatalf("%s: not lowered", name)
		}
		if r := lowerRefusals(all[i]); len(r) > 0 {
			t.Errorf("%s: refused: %v", name, r)
		}
		if name == "Prishe's Wanderings" {
			found := false
			for _, a := range bs[i].def.Abilities {
				for _, in := range a.Prog {
					found = found || in.Op == kop.OpExt && kop.ExtOp(in.A.N) == kop.ExtRememberSearcher
				}
			}
			if !found {
				t.Errorf("%s: no ExtRememberSearcher in the built program", name)
			}
		}
	}
}
