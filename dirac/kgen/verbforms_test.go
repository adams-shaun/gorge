package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// lowerCarriers lowers the named cards and returns their LIR dump and each
// card's refusal reasons.
func lowerCarriers(t *testing.T, names []string) (string, map[string][]string) {
	t.Helper()
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	var sb strings.Builder
	ref := map[string][]string{}
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			ref[c.Name] = append(ref[c.Name], r.Reason)
		}
		dumpLIR(&sb, c)
	}
	return sb.String(), ref
}

// checkCarrier asserts every accepted carrier lowers with no refusal, every
// refused one carries a refusal containing its reason fragment, and the
// LIR holds every wanted fragment.
func checkCarrier(t *testing.T, accepted []string, refused map[string]string, lirWant []string) {
	t.Helper()
	names := append([]string{}, accepted...)
	for n := range refused {
		names = append(names, n)
	}
	lir, ref := lowerCarriers(t, names)
	for _, n := range accepted {
		for _, r := range ref[n] {
			t.Errorf("%s: refused: %s", n, r)
		}
	}
	for n, want := range refused {
		hit := false
		for _, r := range ref[n] {
			hit = hit || strings.Contains(r, want)
		}
		if !hit {
			t.Errorf("%s: want a refusal containing %q, got %v", n, want, ref[n])
		}
	}
	for _, w := range lirWant {
		if !strings.Contains(lir, w) {
			t.Errorf("lowered IR lacks %q", w)
		}
	}
}

// TestCantAttackStatic: S:Mode$ CantAttack (gorge combat.AttackBlocked).
func TestCantAttackStatic(t *testing.T) {
	checkCarrier(t,
		[]string{"Moat", "Pacifism", "Sea Monster", "Bound in Silence", "Harbor Serpent", "Vow of Duty"},
		map[string]string{
			"Blazing Archon": "CantAttack Target$",
		},
		[]string{
			"static CantAttack F={types=0x2 WithoutFlying}",                                                 // Moat
			"static CantAttack F={types=0x2 EnchantedBy}",                                                   // Pacifism
			"static CantAttack F={types=0x0 Self} F2=nil cond=Present({types=0x0 sub=Island ctrl=2},op3,0)", // Sea Monster
			"static CantAttack F={types=0x2 ctrl=2 EnchantedBy}",                                            // Vow of Duty
		})
}

// TestInvestigate: api:Investigate (gorge effInvestigate): one-Clue
// CreateToken ops for the controller.
func TestInvestigate(t *testing.T) {
	checkCarrier(t,
		[]string{"Thraben Inspector"},
		map[string]string{"Will the Wise": "Investigate Defined$"},
		[]string{"CreateToken obj=You A=1 CS=c_a_clue_draw"})
}

// TestPayEnergy: PayEnergy<N> on an activated ability (CostPayEnergy) and
// as a trigger's resolution cost (IfPaid PayEnergy); PutCounter ENERGY on a
// player is OpGainEnergy; a mana ability's energy cost refuses.
func TestPayEnergy(t *testing.T) {
	checkCarrier(t,
		[]string{"Aether Chaser", "Aethergeode Miner", "Aether Theorist"},
		map[string]string{"Aether Hub": "mana ability with an energy cost"},
		[]string{
			"GainEnergy obj=You A=2",
			"IfPaid obj=You A=2 CS=PayEnergy",
			"Activated  cost=Energy2",
			"Activated  cost=T Energy1",
		})
}

// TestAnimateAll: api:AnimateAll (gorge effAnimateAll) as OpOverrideAll;
// the parameters the kernel cannot model refuse.
func TestAnimateAll(t *testing.T) {
	checkCarrier(t,
		[]string{"Natural Affinity", "Thelonite Druid", "Jolrael, Mwonvuli Recluse"},
		map[string]string{"Blooming Cactusfolk": "AnimateAll Zone$"},
		[]string{
			"OverrideAll CS=EOT F={types=0x1} // override{pt=2/2 addtypes=2}",
			"OverrideAll CS=EOT F={types=0x0 sub=Forest ctrl=1} // override{pt=2/3 addtypes=2}",
			"OverrideAll CS=EOT F={types=0x2 ctrl=1} // override{pt=HandSizeOf(n=1,You)/HandSizeOf(n=1,You)}",
		})
}

// TestAnimateXValue: the four Animate/AnimateAll carriers whose P/T is an
// {X} activation lower Power$/Toughness$ "X" through the value evaluator to
// override{pt=X/X}; they are not in the constructed lockstep pool, so this
// test pins their lowering.
func TestAnimateXValue(t *testing.T) {
	checkCarrier(t,
		[]string{"Chimeric Staff", "Minsc, Beloved Ranger", "Mirror Entity", "Testament of Faith"},
		nil,
		[]string{
			"Override obj=Self CS=EOT // override{pt=X/X addtypes=6 addsub=Construct rmcreaturesub}",
			"Override obj=Target0 CS=EOT // override{pt=X/X addsub=Giant}",
			"OverrideAll CS=EOT F={types=0x2 ctrl=1} // override{pt=X/X allcreaturetypes}",
			"Override obj=Self CS=EOT // override{pt=X/X addtypes=2 addsub=Wall kw=Defender}",
		})
}

// TestRollDice: api:RollDice for one die (gorge effRollDice): OpRollDie,
// the result read back by name, a range table as SkipIf blocks; several
// dice refuse.
func TestRollDice(t *testing.T) {
	checkCarrier(t,
		[]string{"Ancient Copper Dragon", "Swarming Goblins"},
		map[string]string{"Arcane Endeavor": "RollDice Amount$"},
		[]string{
			"RollDie A=20 B=0",
			"CreateToken obj=You A=RollResult CS=c_a_treasure_sac",
			"SkipIf if=Value(op1,RollResult,10) skip=2",
			"SkipIf if=Value(op2,RollResult,19) skip=1",
			"CreateToken obj=You A=2 CS=r_1_1_goblin",
		})
}

// TestOthersDoesntUntap: an Aura's / Equipment's "doesn't untap during
// its controller's untap step" (R:Event$ Untap with ValidCard$ other than
// Card.Self) lowers to OthersDoesntUntap; the Self form stays DoesntUntap.
func TestOthersDoesntUntap(t *testing.T) {
	checkCarrier(t,
		[]string{"Claustrophobia", "Dehydration"},
		nil,
		[]string{
			"repl OthersDoesntUntap F={types=0x2 EquippedBy}",
			"repl OthersDoesntUntap F={types=0x2 EnchantedBy}",
		})
}

// TestReturnCost: Return<1/CARDNAME> on an activated ability is
// CostReturnSelf; Return<1/Spec> as an unless cost is PayReturn; an
// activation's Return<N/Spec> is CostReturn; an unless return of two refuses.
func TestReturnCost(t *testing.T) {
	checkCarrier(t,
		[]string{"Glint Hawk", "Broken Fall", "Floodbringer"},
		map[string]string{"Bull Elephant": "resolution return cost of 2"},
		[]string{
			"UnlessPaid obj=You A=1 CS=Return F={types=0x4} skip=1",
			"Activated  cost=ReturnSelf",
			"Activated  cost={2} Return1{types=0x1}", // Floodbringer
		})
}

// TestAnyUntap: R:Event$ Untap | Layer$ CantHappen with no
// ValidStepTurnToController$ (Frozen in Ice, Blossombind) is the any-event
// "can't become untapped" replacement, AnyUntap.
func TestAnyUntap(t *testing.T) {
	checkCarrier(t,
		[]string{"Frozen in Ice", "Blossombind"},
		nil,
		[]string{"repl AnyUntap F={types=0x0 EnchantedBy}"})
}

// TestCantPutCounter: the object form of S:Mode$ CantPutCounter
// (gorge PutCounterBlocked) lowers to StatCantPutCounter with the
// CounterType$ kind in Static.A (-1 = every kind); the player form is
// refused rather than turned into a no-op.
func TestCantPutCounter(t *testing.T) {
	checkCarrier(t,
		[]string{"Blossombind", "Blightbeetle", "Melira's Keepers", "Tatterkite"},
		map[string]string{"Solemnity": `CantPutCounter ValidPlayer "Player"`},
		[]string{
			"static CantPutCounter F={types=0x0 EnchantedBy}", // Blossombind: every kind
			"ctr=-1",
			"static CantPutCounter F={types=0x2 ctrl=2}", // Blightbeetle: CounterType$ P1P1
			"ctr=0",
			"static CantPutCounter F={types=0x0 Self}", // Melira's Keepers / Tatterkite
		})
}
