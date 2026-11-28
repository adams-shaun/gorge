package oracle

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The hosted-Commander stack and cast lowerings, each played in lockstep
// against gorge in the hostedcmdrstackcast pool:
//
//   - Wastescape Battlemage's two cast triggers, "when you cast this spell, if
//     it was kicked with its {G} kicker" / "with its {1}{U} kicker"
//     (SpellCast ValidCard$ Card.Self+kicked 1 / kicked 2): TfSelf with a
//     PredKickedAlt filter read off the spell's kicker-alternative byte
//     (kernel selfCastFilterOK), exile an artifact or enchantment of the
//     opponent's, or return an opposing creature;
//   - Springheart Nantuko's landfall (a ConditionPresent$ guard over a
//     DB$ CopyPermanent with UnlessCost$ / UnlessSwitched$ / UnlessResolveSubs$
//     WhenNotPaid): the pay-or-insect branch with the node's own guard, so a
//     failed guard runs the subs (the insect) and a paid copy skips them.
//
// Each case asserts the lowered shape is in the table, plays clean, and (where
// the games can reach it) diverges with that shape broken. The cards
// kgen still refuses by name (Commandeer, Disrupting Shoal, Blazing Shoal,
// Vivi Ornitier, Venerated Loxodon) are pinned in
// dirac/kgen/hostedcmdr_stackcast_refusals_test.go.
const hostedCmdrStackPool = "hostedcmdrstackcast"

type hostedCmdrStackCase struct {
	name  string
	decks [2][]cardset.Entry
	// shape fails the test unless the lowered shape is in the table; mutate
	// breaks it (nil: the pool cannot reach the shape, see the case)
	shape  func(t *testing.T, tbl *kop.Table)
	mutate func(tbl *kop.Table)
}

var hostedCmdrStackCases = []hostedCmdrStackCase{
	{
		name: "wastescape-battlemage",
		decks: [2][]cardset.Entry{
			{
				{Name: "Wastescape Battlemage", Count: 14}, {Name: "Wastes", Count: 8},
				{Name: "Forest", Count: 8}, {Name: "Island", Count: 8}, {Name: "Grizzly Bears", Count: 2},
			},
			{
				{Name: "Darksteel Relic", Count: 6}, {Name: "Springheart Nantuko", Count: 6}, {Name: "Hill Giant", Count: 8},
				{Name: "Mountain", Count: 20},
			},
		},
		shape: func(t *testing.T, tbl *kop.Table) {
			if len(hostedCmdrKickTrigs(tbl)) != 2 {
				t.Fatal("Wastescape Battlemage has no two SpellCast self triggers reading PredKickedAlt: the spelling did not lower")
			}
		},
		// no mutate: gorge never poses the per-alternative kicked casts at a
		// priority (twoaltkicker_lockstep_test.go), so the games play the plain
		// casts clean and the pool cannot reach the triggers; the kicked casts'
		// trigger reads are dirac.TestWastescapeKickedCastTriggers's
	},
	{
		name: "springheart-nantuko",
		decks: [2][]cardset.Entry{
			{
				{Name: "Springheart Nantuko", Count: 12}, {Name: "Grizzly Bears", Count: 10}, {Name: "Forest", Count: 18},
			},
			{
				{Name: "Raging Goblin", Count: 12}, {Name: "Shock", Count: 6}, {Name: "Mountain", Count: 22},
			},
		},
		shape: func(t *testing.T, tbl *kop.Table) {
			if hostedCmdrSpringJump(tbl) == nil {
				t.Fatal("Springheart Nantuko has no UnlessSwitched whose subs a jump skips: the spelling did not lower")
			}
		},
		// the jump over the UnlessResolveSubs$ subs dropped: a paid copy
		// also makes the insect
		mutate: func(tbl *kop.Table) { hostedCmdrSpringJump(tbl).A = kop.Const(0) },
	},
}

func TestHostedCmdrStackCastLockstep(t *testing.T) {
	t.Parallel()
	_ = corpusDirOrSkip(t)
	for _, c := range hostedCmdrStackCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			set, err := cardset.RawByName(hostedCmdrStackPool)
			if errors.Is(err, cardset.ErrNoCorpus) {
				t.Skip("no kernel-synth corpus table: " + err.Error())
			}
			if err != nil {
				t.Fatal(err)
			}
			tbl, _, _, err := set.Matchup(c.decks[0], c.decks[1])
			if err != nil {
				t.Fatal(err)
			}
			c.shape(t, tbl)
			lockstepSet(t, hostedCmdrStackPool, c.decks, 200, "KHOSTEDCMDRSTACK_SEEDS")
			if c.mutate != nil && lockstepRun(t, hostedCmdrStackPool, c.decks, 200, false, c.mutate) == 0 {
				t.Fatalf("%s: no seed diverged with the lowered shape broken: the pool never reaches it", c.name)
			}
		})
	}
}

// hostedCmdrKickTrigs are the PredKickedAlt filters of Wastescape
// Battlemage's two SpellCast self triggers.
func hostedCmdrKickTrigs(tbl *kop.Table) []*kop.Filter {
	cd := mbcCard(tbl, "Wastescape Battlemage")
	if cd == nil {
		return nil
	}
	var out []*kop.Filter
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigSpellCast || a.Trig.Flags&kop.TfSelf == 0 || a.Trig.Filter == 0 {
			continue
		}
		if f := &tbl.Filters[a.Trig.Filter]; f.Pred == kop.PredKickedAlt {
			out = append(out, f)
		}
	}
	return out
}

// hostedCmdrSpringJump is Springheart Nantuko's jump over the insect: the
// SkipIf right behind its UnlessSwitched body.
func hostedCmdrSpringJump(tbl *kop.Table) *kop.Instr {
	cd := mbcCard(tbl, "Springheart Nantuko")
	if cd == nil {
		return nil
	}
	for j := range cd.Abilities {
		prog := cd.Abilities[j].Prog
		for k := 0; k+2 < len(prog); k++ {
			if prog[k].Op == kop.OpUnlessSwitched && prog[k+2].Op == kop.OpSkipUnless {
				return &prog[k+2]
			}
		}
	}
	return nil
}
