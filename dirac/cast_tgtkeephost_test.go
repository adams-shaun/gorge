package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// castOfferedFor reports whether the seat's decision offers a cast of o.
func castOfferedFor(d Decision, o Obj) bool {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == AkCast && d.Options[i].Source() == TCode(o) {
			return true
		}
	}
	return false
}

// callousHostScen is the tail-45-c001 pair 6 board (Callous Dismissal and
// Curator's Ward there; Blink of an Eye and Footfall Crater are their mbc-set
// stand-ins: the same {1}{U} bounce and a mana-neutral Enchant-land Aura): an Island and a Sandstone
// Needle (its counters set to ctrs) beside Blink of an Eye ({1}{U}, return
// target nonland permanent), the only nonland permanent being Footfall Crater
// attached to the Needle.
func callousHostScen(t *testing.T, ctrs int) (*scen, Obj) {
	t.Helper()
	return callousHostScenWith(t, ctrs, false)
}

// callousHostScenWith is callousHostScen; equip swaps Footfall Crater for a
// bare Equipment attached to the Needle instead.
func callousHostScenWith(t *testing.T, ctrs int, equip bool) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Sandstone Needle", Count: 1}, {Name: "Blink of an Eye", Count: 1},
		{Name: "Footfall Crater", Count: 1}, {Name: "Island", Count: 17}}
	tbl, _, _, err := cardsettest.Set(t, "mbc").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	if equip {
		tbl.Cards = append(append([]kop.CardDef(nil), tbl.Cards...),
			kop.CardDef{Name: "Test Sword", Types: kop.TArtifact | kop.TEquipment})
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	attachee := "Footfall Crater"
	if equip {
		attachee = "Test Sword"
	}
	for _, n := range []string{"Island", "Sandstone Needle", "Blink of an Eye", attachee} {
		if _, ok := id[n]; !ok {
			t.Fatalf("precondition: %s is not in the table", n)
		}
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true, GorgeLowering: true})
	sc.perm(id["Island"], 0)
	needle := sc.perm(id["Sandstone Needle"], 0)
	ward := sc.perm(id[attachee], 0)
	nsl := sc.slot(needle)
	a, ok := sc.s.riderAbilityOf(sc.k, nsl)
	if !ok {
		t.Fatal("precondition: Sandstone Needle is not the depletion-rider shape")
	}
	sc.s.perms[nsl].flags &^= pfTapped // it entered tapped; it has untapped since
	for n := sc.s.perms[nsl].ctrN(sc.s, int(a.ManaRiderCtr)); n > ctrs; n-- {
		sc.s.perms[nsl].subCtrN(sc.s, int(a.ManaRiderCtr), 1)
	}
	if n := sc.s.perms[nsl].ctrN(sc.s, int(a.ManaRiderCtr)); n != ctrs {
		t.Fatalf("precondition: the Needle holds %d depletion counters, want %d", n, ctrs)
	}
	wsl := sc.slot(ward)
	if sc.s.perms[wsl].attached != nsl {
		sc.s.attachTo(sc.k, wsl, nsl)
	}
	if sc.s.perms[wsl].attached != nsl {
		t.Fatal("precondition: Footfall Crater is not attached to the Needle")
	}
	return sc, sc.hand(id["Blink of an Eye"], 0)
}

// CR 704.5m, 605.3b: the Needle's {R}{R} on its last counter sacrifices it,
// and the Aura on it falls off; the Island alone cannot pay {1}{U}, so every
// plan strands the spell's only target and gorge's search offers no cast
// (potential_script.go targetsGone).
func TestCallousDismissalWardOnLastCounterNeedleNotOffered(t *testing.T) {
	sc, spell := callousHostScen(t, 1)
	if d := sc.s.Step(); castOfferedFor(d, spell) {
		t.Fatalf("Blink of an Eye is offered although paying it sacrifices the Needle under its only target; opts %v", d.Options[:d.N])
	}
}

// Control: a Needle with a counter to spare survives the payment, so the Aura
// stays a legal target and the cast is offered.
func TestCallousDismissalWardOnTwoCounterNeedleOffered(t *testing.T) {
	sc, spell := callousHostScen(t, 2)
	if d := sc.s.Step(); !castOfferedFor(d, spell) {
		t.Fatalf("Blink of an Eye is not offered with the Needle on two counters; opts %v", d.Options[:d.N])
	}
}

// Control (CR 704.5n): an Equipment on the Needle only unattaches when paying
// sacrifices the Needle, so it is never stranded and holds no host out; only
// an Aura is. The census read is called directly (a land bearer would have the
// Equipment unattached by state-based actions at the first Step), with the
// Aura on the same board as the positive control.
func TestConsumedAuraHostIgnoresEquipment(t *testing.T) {
	for _, tc := range []struct {
		equip bool
		want  bool
	}{{false, true}, {true, false}} {
		sc, _ := callousHostScenWith(t, 1, tc.equip)
		var att Slot = NoSlot
		for m := sc.s.occ; m.any(); m = m.rest() {
			if q := m.first(); sc.s.perms[q].attached != NoSlot && sc.s.perms[q].attached != q {
				att = q
			}
		}
		if att == NoSlot {
			t.Fatalf("equip=%v: precondition: nothing is attached to the Needle", tc.equip)
		}
		if isAura := sc.s.typesGen(sc.k, att)&kop.TAura != 0; isAura == tc.equip {
			t.Fatalf("equip=%v: precondition: attachment Aura=%v", tc.equip, isAura)
		}
		if tc.equip && sc.s.typesGen(sc.k, att)&kop.TEquipment == 0 {
			t.Fatal("precondition: the attachment is an Equipment")
		}
		var obj Obj
		for o := range sc.s.objs {
			if sc.s.occ.has(sc.s.objs[o].slot) && sc.s.objs[o].slot == att && sc.s.objs[o].zone == kop.ZoneBattlefield {
				obj = Obj(o)
			}
		}
		if _, got := sc.s.consumedAuraHost(sc.k, 0, obj); got != tc.want {
			t.Fatalf("equip=%v: consumedAuraHost = %v, want %v", tc.equip, got, tc.want)
		}
	}
}
