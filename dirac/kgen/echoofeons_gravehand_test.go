package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// czallInstrs builds the named cards and returns each card's OpChangeZoneAllEx
// instructions (only the Graveyard/Hand two-zone sweeps carry MvGraveExile).
func czallInstrs(t *testing.T, names ...string) map[string][]kop.Instr {
	t.Helper()
	all, bs, _, err := buildPool(lowerNamedPool(t, names))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]kop.Instr{}
	for i, c := range all {
		if r := lowerRefusals(c); len(r) > 0 {
			t.Errorf("%s: refused: %v", c.Name, r)
		}
		for _, a := range bs[i].def.Abilities {
			for _, in := range a.Prog {
				if in.Op == kop.OpChangeZoneAllEx && in.C>>3&kop.MvGraveExile != 0 {
					out[c.Name] = append(out[c.Name], in)
				}
			}
		}
	}
	return out
}

// TestLowerEchoOfEonsGravehandAll: Echo of Eons' Hand,Graveyard sweep over
// every player lowers hand-first with the kernel shuffling both libraries
// (CzHandGrave|CzShuffleEach, no Shuffle op after it); Weftwalking's (the
// Midnight Clock shape) Graveyard,Hand YouOwn sweep keeps its plain ZoneHand code;
// every out-of-shape variant refuses by name.
func TestLowerEchoOfEonsGravehandAll(t *testing.T) {
	got := czallInstrs(t, "Echo of Eons", "Weftwalking")
	echo := got["Echo of Eons"]
	if len(echo) != 1 {
		t.Fatalf("Echo of Eons sweeps = %d, want 1", len(echo))
	}
	if want := kop.Const(kop.CzHandGrave | kop.CzShuffleEach); echo[0].B != want || kop.Zone(echo[0].C&7) != kop.ZoneLibrary {
		t.Errorf("Echo of Eons B = %+v C = %#x, want %+v into the library", echo[0].B, echo[0].C, want)
	}
	for _, name := range []string{"Weftwalking"} {
		if len(got[name]) != 1 {
			t.Fatalf("%s sweeps = %d, want 1", name, len(got[name]))
		}
		if want := kop.Const(int(kop.ZoneHand)); got[name][0].B != want {
			t.Errorf("%s B = %+v, want %+v (unchanged)", name, got[name][0].B, want)
		}
	}

	const ok = "SP$ ChangeZoneAll | ChangeType$ Card | Origin$ Hand,Graveyard | Destination$ Library | Shuffle$ True | UseAllOriginZones$ True"
	refusals, ops := czLowered(t, ok)
	if len(refusals) != 0 {
		t.Fatalf("the Echo shape refused: %q", refusals)
	}
	if len(ops) == 0 || ops[0] != "ChangeZoneAll:from=Hand,Graveyard|ShuffleEach" || strings.Contains(strings.Join(ops, " "), "Shuffle:") {
		t.Errorf("ops = %q, want one sweep with the ShuffleEach flag and no Shuffle", ops)
	}
	if _, ops = czLowered(t, strings.Replace(ok, "Hand,Graveyard", "Graveyard,Hand", 1)); len(ops) == 0 || ops[0] != "ChangeZoneAll:from=Graveyard,Hand|ShuffleEach" {
		t.Errorf("the reversed order lowers to %q", ops)
	}

	for _, c := range []struct{ name, root, want string }{
		{"no shuffle", strings.Replace(ok, " | Shuffle$ True", "", 1), "outside Destination$ Library, Shuffle$ True"},
		{"to the hand", strings.Replace(ok, "Destination$ Library", "Destination$ Hand", 1), "outside Destination$ Library, Shuffle$ True"},
		{"filtered cards", strings.Replace(ok, "ChangeType$ Card", "ChangeType$ Creature", 1), "outside ChangeType$ Card"},
		{"your cards only", strings.Replace(ok, "ChangeType$ Card", "ChangeType$ Card.YouOwn", 1), "outside ChangeType$ Card"},
		{"all-zones false", strings.Replace(ok, "UseAllOriginZones$ True", "UseAllOriginZones$ False", 1), "UseAllOriginZones$ \"False\""},
		{"hand first without UseAll", strings.Replace(ok, " | UseAllOriginZones$ True", " | ChangeType$ Card.YouOwn", 1), "outside Graveyard,Hand, ChangeType$ Card.YouOwn"},
		{"extra key", ok + " | RememberChanged$ True", "with RememberChanged$"},
		{"third zone", strings.Replace(ok, "Hand,Graveyard", "Hand,Graveyard,Exile", 1), "Origin"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := czLowered(t, c.root)
			kdefWant(t, got, c.want)
		})
	}
}
