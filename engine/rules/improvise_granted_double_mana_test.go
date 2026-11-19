package rules

import "testing"

// CR 702.66a with CR 601.2h: a {3}{U}{U} Improvise spell is payable when one
// artifact credits a generic and an enchanted land's granted "{T}: Add two
// mana of any one color" pays the blue pips. The improvise plan withholds
// the credit artifact from the sources, so the granted ability must still be
// priced by the potential planner.
const reGrantSpellSrc = "Name:Gear Up\nManaCost:3 U U\nTypes:Sorcery\nK:Improvise\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"

const reGrantPlainSpellSrc = "Name:Gear Down\nManaCost:2 U U\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"

const reGrantAuraSrc = "Name:Test Path\nManaCost:2 G\nTypes:Enchantment Aura\nK:Enchant:Land\n" +
	"S:Mode$ Continuous | AffectedDefined$ Enchanted | Affected$ Land | AddAbility$ ABMana | Description$ x\n" +
	"SVar:ABMana:AB$ Mana | Cost$ T | Produced$ Any | Amount$ 2 | SpellDescription$ x\nOracle:x\n"

const reGrantPlainLandSrc = "Name:Test Destination\nTypes:Land\nOracle:x\n"

func reGrantBoard(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 4; i++ {
		onBoard(t, e, 0, "Name:Test Forest Land\nTypes:Land Forest\nOracle:x\n")
	}
	onBoard(t, e, 0, "Name:Test Island Land\nTypes:Land Island\nOracle:x\n")
	onBoard(t, e, 0, plainArtifactSrc)
	land := onBoard(t, e, 0, reGrantPlainLandSrc)
	aura := onBoard(t, e, 0, reGrantAuraSrc)
	attachFixture(t, e, aura, land)
}

// The control: the same board and two blue pips with no Improvise. The
// potential planner prices the granted two-mana source.
func TestGrantedDoubleManaPaysPlainCast(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9613, reGrantPlainSpellSrc)
	toMain1(t, e)
	reGrantBoard(t, e)
	ids := setHandSrcs(t, e, 0, reGrantPlainSpellSrc)
	ppPriority(t, e)
	if pp := ppFind(t, e.PotentialPaymentPlans(0), "cast", ids[0]); pp.Reason != "" {
		t.Fatalf("plain cast plan reason %q detail %q, want payable", pp.Reason, pp.Detail)
	}
}

// The bug: adding Improvise must not lose the granted source, which the plan
// still needs after the artifact credits a generic.
func TestImproviseCastPaysGrantedDoubleMana(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9612, reGrantSpellSrc)
	toMain1(t, e)
	reGrantBoard(t, e)
	ids := setHandSrcs(t, e, 0, reGrantSpellSrc)
	ppPriority(t, e)
	if pp := ppFind(t, e.PotentialPaymentPlans(0), "cast", ids[0]); pp.Reason != "" {
		t.Fatalf("improvise cast plan reason %q detail %q, want payable", pp.Reason, pp.Detail)
	}
}
