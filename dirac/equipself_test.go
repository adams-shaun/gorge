package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEquipNotOfferedItself: gorge never offers an attach ability its own
// source as the object (rules/cast_targets.go excludeSelf, CR 701.3a). An
// Equipment that an animator made a creature matches Creature.YouCtrl, so
// without kgen's FOther on the Equip's target filter the kernel offered Equip
// with itself as the target (and, as the controller's only creature, the
// activation at all). A non-Attach ability of the same permanent that targets
// itself keeps its source (Mother of Runes).
func TestEquipNotOfferedItself(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	fdnSet := cardsettest.Set(t, "fdn")
	tbl, _, _, err := fdnSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou},
		kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Flags: kop.FOther})
	f, fOther := kop.FilterID(len(tbl.Filters)-2), kop.FilterID(len(tbl.Filters)-1)
	// the Equipment animates itself (Ensouled Scimitar)
	tbl.Overrides = append(append([]kop.Override(nil), tbl.Overrides...), kop.Override{AddTypes: kop.TCreature})
	ov := int16(len(tbl.Overrides) - 1)
	eq := CardID(len(tbl.Cards))
	bear := eq + 1
	tbl.Cards = append(append([]kop.CardDef(nil), tbl.Cards...),
		// an animated Equipment: ability 0 is its Equip, ability 1 a plain
		// ability targeting a creature you control (itself included)
		kop.CardDef{Name: "Test Animated Sword", Types: kop.TArtifact | kop.TCreature | kop.TEquipment, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{
				{Kind: kop.AbActivated, Cost: kop.Cost{Flags: kop.CostSorcery}, AFlags: kop.AfEquip, NTargets: 1,
					Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: fOther}},
					Prog:    []kop.Instr{{Op: kop.OpAttach, Obj: kop.RefTarget0}}},
				{Kind: kop.AbActivated, NTargets: 1,
					Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: f}},
					Prog:    []kop.Instr{{Op: kop.OpPutCounter, Obj: kop.RefTarget0, A: kop.Const(1), C: int16(kop.CtrP1P1)}}},
				{Kind: kop.AbActivated, Prog: []kop.Instr{{Op: kop.OpOverride, Obj: kop.RefSelf, C: ov}}},
			}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTableLib(t, tbl, 0, Config{NoElide: true})
	sw := sc.perm(eq, 0)

	// the Equip (ability 0) and the plain ability (ability 1) offered for sw
	offers := func() (equip, plain bool) {
		d := sc.s.Step()
		if d.Kind != DkPriority {
			t.Fatalf("decision kind %d, want priority", d.Kind)
		}
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(sw)) {
				equip = equip || a.Mode() == 0
				plain = plain || a.Mode() == 1
			}
		}
		return
	}
	if equip, plain := offers(); equip || !plain {
		t.Fatalf("only creature is the Equipment: Equip offered %v (want false), plain ability offered %v (want true)", equip, plain)
	}
	b := sc.perm(bear, 0)
	if equip, _ := offers(); !equip {
		t.Fatal("a second creature: Equip not offered")
	}
	// the Equip's target ask offers only the other creature
	sc.s.Apply(MakeAction(AkActivate, TCode(sw), 0, 0))
	d := sc.s.Step()
	if d.Kind != DkTarget || d.N != 1 || d.Options[0].Target() != TCode(uint8(b)) {
		t.Fatalf("Equip target ask: kind %d, %d options %v, want the Bear (obj %d) alone", d.Kind, d.N, d.Options[:d.N], b)
	}
}

// TestEquipAnimOffInShippedPools: no FDN or BLB Equipment can be a creature (the
// only animators are self-animating lands and Vehicles and a creature-only
// static), so the hot-path guards of the Equipment-creature rules stay false
// there (BenchmarkTransitionFDN and BLB).
func TestEquipAnimOffInShippedPools(t *testing.T) {
	fdnSet := cardsettest.Set(t, "fdn")
	fdn, err := fdnSet.Pool()
	if err != nil {
		t.Fatal(err)
	}
	blbSet := cardsettest.Set(t, "blb")
	blb, err := blbSet.Pool()
	if err != nil {
		t.Fatal(err)
	}
	for _, pt := range []struct {
		name string
		tbl  *kop.Table
	}{{"fdn", fdn}, {"blb", blb}} {
		name, tbl := pt.name, pt.tbl
		var g genPrep
		prepEquipAnim(tbl, &g)
		if g.equipAnim {
			t.Errorf("%s pool: equipAnim is set", name)
		}
	}
}

// TestETBAttachOffersItself: a TRIGGERED attach ("When this Equipment enters,
// attach it to target creature you control": Embercleave, Celestial Armor,
// Starforged Sword) keeps its source as a legal target, unlike Equip.
// gorge's cast_targets.go excludes the source only for a player-cast ability
// (excludeSelf, CR 701.3a); a trigger's ask excludes the stack object itself
// (CR 115.5), so with an animator in effect (March of the Machines) gorge
// offers the entering Equipment as its own target, and effAttach then
// refuses the self-attach. dirac/oracle TestGorgeETBAttachOffersItself pins
// gorge's side; this pins the kernel's: the Equipment is offered, and
// resolving the trigger on it attaches nothing.
func TestETBAttachOffersItself(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	fdnSet := cardsettest.Set(t, "fdn")
	tbl, _, _, err := fdnSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	f := kop.FilterID(len(tbl.Filters) - 1)
	cleave := CardID(len(tbl.Cards))
	bear := cleave + 1
	tbl.Cards = append(append([]kop.CardDef(nil), tbl.Cards...),
		// an Equipment that is a creature as it enters, with Embercleave's trigger
		kop.CardDef{Name: "Test Cleave", Types: kop.TArtifact | kop.TCreature | kop.TEquipment, Power: 1, Toughness: 1,
			Keywords: kop.KwFlash, Timing: kop.TimingInstant,
			Abilities: []kop.Ability{
				{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigETB, Flags: kop.TfSelf}, NTargets: 1,
					Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: f}},
					Prog:    []kop.Instr{{Op: kop.OpAttach, Obj: kop.RefTarget0}}},
			}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})

	// cast offers the targets of the ETB ask, then resolves the trigger on the
	// first one; it returns the options and the Equipment's slot
	run := func(withBear bool) (opts []uint8, eq Obj, attached Slot) {
		sc := newScenTableLib(t, tbl, 0, Config{NoElide: true})
		if withBear {
			sc.perm(bear, 0)
		}
		eq = sc.hand(cleave, 0)
		sc.castCard(t, eq)
		for i := 0; i < 40; i++ {
			d := sc.s.Step()
			switch {
			case d.Kind == DkTarget:
				for j := 0; j < int(d.N); j++ {
					opts = append(opts, uint8(d.Options[j].Target()))
				}
				sc.s.Apply(d.Options[0])
				sc.resolveAll(t)
				if !sc.onBF(eq) {
					t.Fatal("the Equipment left the battlefield")
				}
				return opts, eq, sc.s.perms[sc.slot(eq)].attached
			case d.Kind == DkPriority:
				sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			default:
				sc.s.Apply(d.Options[0])
			}
		}
		t.Fatal("no ETB target ask")
		return
	}

	opts, eq, att := run(false)
	if len(opts) != 1 || opts[0] != uint8(eq) {
		t.Fatalf("the Equipment its controller's only creature: target options %v, want itself (obj %d) alone", opts, eq)
	}
	if att != NoSlot {
		t.Fatalf("the Equipment resolved its attach on itself: attached to slot %d", att)
	}
	if opts, eq, _ = run(true); len(opts) != 2 || (opts[0] != uint8(eq) && opts[1] != uint8(eq)) {
		t.Fatalf("with a Bear: target options %v, want the Bear and the Equipment (obj %d)", opts, eq)
	}
}
