package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// typedTable is the vanilla table plus a typed (artifact) mana producer whose
// one mana ability is feeCost: "{T}: Add {C}" when zero (Sol Ring's shape) or
// "{2},{T}: Add one mana of any color" (Celestial Prism's).
func typedTable(fee bool) *kop.Table {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	ab := kop.Ability{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1}
	if fee {
		ab.Cost.Mana.Generic = 2
		ab.ManaColors = kop.ColAny
	}
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Typed Source", Types: kop.TArtifact, Abilities: []kop.Ability{ab}})
	return &tbl
}

// TestTypedManaTallyForAFeeCarryingProducer: gorge's prefix search keys the
// pool by the artifact-typed units of each colour (Player.ManaUnits), and a
// payment spends a plain unit before a typed one (pay.takeUnit). The kernel
// reads that count from coldArtN, which a table holds only when some typed
// producer's own ability is paid from the pool. Celestial Prism's "{2},{T}"
// is: the {2} spends the pool, so {plain B, Prism's B} and {Lotus Blossom's B,
// Prism's B} are two states for gorge. Without the tally the kernel keyed both
// as typed = pool and cut the second as a duplicate: on the Lotus Blossom +
// Celestial Prism board (overload_cast_without_target) three swamp children
// under four Prism answers, twelve nodes gorge spent and the kernel did not.
func TestTypedManaTallyForAFeeCarryingProducer(t *testing.T) {
	if !tableAnyTypedMana(typedTable(true)) {
		t.Fatal("a typed producer whose ability pays {2} from the pool holds no tagged-unit tally")
	}
	if tableAnyTypedMana(typedTable(false)) {
		t.Fatal("a free typed producer alone holds a tally (it keeps one bit per colour, as before)")
	}
}

// ctrAnyScen: seat 0 holds a counter-cost source with the one ability
// "{T}, Sacrifice this: Add X mana of any one color, X its counters" (Lotus
// Blossom: one ability, so its activation poses only the colour ask) and a creature whose {G} activation
// is the play. The source is the board's only gap.
func ctrAnyScen(t *testing.T, cfg Config) (*scen, Obj) {
	t.Helper()
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}} // a generated table (prepareGen's gate)
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Ctr Any", Types: kop.TLand, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap | kop.CostSacSelf}, ManaColors: kop.ColAny, AFlags: kop.AfManaDynamic,
			ManaV: kop.ValueRef{K: kop.VCountersSelf, N: int16(kop.CtrPetal)}},
	}})
	// a filter gap beside it ("{2},{T}: Add one mana of any color", Celestial
	// Prism's): the board is a mixed-gap one, whose prefix search is the
	// filter search (mana_filter.go) rather than the counter source's own chain
	prism := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Prism", Types: kop.TArtifact, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap, Mana: kop.ManaCost{Generic: 2}}, ManaColors: kop.ColAny, ManaN: 1},
	}})
	// the play: an activation whose {G} only the counter source can make
	bear := CardID(len(tbl.Cards))
	cost := kop.ManaCost{}
	cost.Pips[kop.G] = 1
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbActivated, Cost: kop.Cost{Mana: cost},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}}}})
	sc := newScenTable(t, &tbl, cfg)
	land := sc.perm(src, 0)
	sl := sc.slot(land)
	sc.s.perms[sl].seeCtr(sc.s)
	sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrPetal), 1)
	sc.perm(prism, 0)
	emberO := sc.perm(bear, 0)
	if !sc.s.scriptCtr(sc.k, 0).has(sl) {
		t.Fatal("precondition: the source is no live counter-cost gap")
	}
	return sc, emberO
}

// TestCtrOneAbilityColourAskIsANode: gorge's prefix search spends one node on
// the clone the source's activation makes (it poses the colour ask), then one
// per colour answer (scriptSearch.visit). The route to the cast is the root
// node, that ask node and the visit after the first answer (the play's {G}
// first, scriptOrderAnswers): three nodes, so the cast is offered within a
// budget of 3 and not 2. The kernel charged no node for the ask of a source
// with one ability, only for the second-ability ask beside a plain {T}
// first, so it offered the cast at 2 and a board whose script sat at gorge's
// budget edge (Lotus Blossom beside Celestial Prism: 523 nodes in gorge, 522
// in the kernel) was offered by the kernel alone.
func TestCtrOneAbilityColourAskIsANode(t *testing.T) {
	for _, cfg := range []Config{{NoElide: true}, {GorgeLowering: true, NoElide: true}} {
		sc, bear := ctrAnyScen(t, cfg)
		a := MakeAction(AkActivate, TCode(bear), 0, 0)
		DebugFiltNodes = 0
		if !sc.s.filtScript(sc.k, a, 0, false) {
			t.Fatalf("gorgeLowering=%v: precondition: the search finds no path for the {G} activation", cfg.GorgeLowering)
		}
		if DebugFiltNodes != 3 {
			t.Errorf("gorgeLowering=%v: the search spent %d nodes, want 3 (root, the colour ask, the first answer)", cfg.GorgeLowering, DebugFiltNodes)
		}
	}
}
