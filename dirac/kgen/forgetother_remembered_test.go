package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestForgetOtherRememberedLowers: the hand move and the Dig read
// ForgetOtherRemembered$ True. The hand move carries kop.SrForgetOther (the
// kernel clears after the picks and before the moves); the Dig emits
// OpClearRemembered immediately before its DigEx, whose DigRemember re-adds
// the taken cards. Any other value refuses by name.
func TestForgetOtherRememberedLowers(t *testing.T) {
	const probe = `Name:Forget Other Probe
ManaCost:1 W
Types:Sorcery
A:AB$ ChangeZone | Cost$ 1 | Origin$ Hand | Destination$ Exile | ChangeType$ Card | ChangeNum$ 1 | Mandatory$ True | ForgetOtherRemembered$ False
`
	if rf := lowerScript(t, "Forget Other Probe", probe); !anyRefusal(rf, "ForgetOtherRemembered$") {
		t.Fatalf("ForgetOtherRemembered$ False: refusals %v, want one naming the param", rf)
	}

	cardsDir := ratchetCards(t)
	p, err := loadCardNames([]string{"Cultivator Colossus", "Primal Surge"}, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	all, bs, _, err := buildPool(lowerPool(p))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]int{}
	for i, c := range all {
		byName[c.Name] = i
	}
	for _, n := range []string{"Cultivator Colossus", "Primal Surge"} {
		if _, ok := byName[n]; !ok {
			t.Errorf("%s: not lowered", n)
		}
	}
	if t.Failed() {
		return
	}

	for _, c := range []struct {
		card string
		dest kop.Zone
	}{
		{"Cultivator Colossus", kop.ZoneBattlefield},
	} {
		i, ok := byName[c.card]
		if !ok {
			continue
		}
		if r := lowerRefusals(all[i]); len(r) > 0 {
			t.Errorf("%s: refused: %v", c.card, r)
			continue
		}
		found := false
		for _, a := range bs[i].def.Abilities {
			for _, in := range a.Prog {
				if in.Op == kop.OpChooseMove && kop.Zone(in.C&7) == c.dest && in.C>>3&kop.SrForgetOther != 0 {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("%s: no built OpChooseMove from the hand to %v with SrForgetOther", c.card, c.dest)
		}
	}

	// A hand move into exile: Tidehollow Sculler carried it until upstream
	// Forge rewrote its return to Defined$ ExiledWith, so a synthetic probe
	// keeps the shape.
	const hand = `Name:Hand Exile Probe
ManaCost:W B
Types:Creature Zombie
PT:2/2
T:Mode$ ChangesZone | Destination$ Battlefield | Origin$ Any | ValidCard$ Card.Self | Execute$ PrbTake | TriggerDescription$ probe enters
SVar:PrbTake:DB$ ChangeZone | ValidTgts$ Opponent | DefinedPlayer$ Targeted | Origin$ Hand | Destination$ Exile | Chooser$ You | ChangeType$ Card.nonLand | ChangeNum$ 1 | Mandatory$ True | IsCurse$ True | ForgetOtherRemembered$ True | RememberChanged$ True
T:Mode$ ChangesZone | Destination$ Any | Origin$ Battlefield | ValidCard$ Card.Self | Execute$ PrbGive | TriggerDescription$ probe leaves
SVar:PrbGive:DB$ ChangeZone | Destination$ Hand | Origin$ Exile | Defined$ Remembered | SubAbility$ PrbClear
SVar:PrbClear:DB$ Cleanup | ClearRemembered$ True
`
	hc := lowerScriptCard(t, "Hand Exile Probe", hand)
	if r := lowerRefusals(hc); len(r) > 0 {
		t.Errorf("Hand Exile Probe: refused: %v", r)
	} else if _, hb, _, err := buildPool(&lowered{cards: []*lCard{hc}}); err != nil {
		t.Errorf("Hand Exile Probe: build: %v", err)
	} else {
		found := false
		for _, a := range hb[0].def.Abilities {
			for _, in := range a.Prog {
				if in.Op == kop.OpChooseMove && kop.Zone(in.C&7) == kop.ZoneExile && in.C>>3&kop.SrForgetOther != 0 {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("Hand Exile Probe: no built OpChooseMove from the hand to %v with SrForgetOther", kop.ZoneExile)
		}
	}

	i, ok := byName["Primal Surge"]
	if !ok {
		return
	}
	if r := lowerRefusals(all[i]); len(r) > 0 {
		t.Fatalf("Primal Surge: refused: %v", r)
	}
	found := false
	for _, a := range bs[i].def.Abilities {
		for j := 0; j+1 < len(a.Prog); j++ {
			clear, dig := a.Prog[j], a.Prog[j+1]
			if clear.Op == kop.OpClearRemembered && dig.Op == kop.OpDigEx && kop.Zone(dig.C&7) == kop.ZoneExile &&
				dig.C>>3&kop.DigRemember != 0 {
				found = true
			}
		}
	}
	if !found {
		t.Error("Primal Surge: no OpClearRemembered immediately before an OpDigEx with DigRemember")
	}
}
