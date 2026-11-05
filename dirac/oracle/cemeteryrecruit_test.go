package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Cemetery Recruitment returns target creature card from its caster's
// graveyard to hand (RememberTargets$ True), then draws a card when the
// remembered card, now in hand, is a Zombie (gorge rememberChosenTargets).
// The first pool's creatures are all Zombies (Walking Corpse: a draw follows
// every return), the second's none (Grizzly Bears: no draw); burn kills them
// so the graveyard has targets. The expected answer at every decision is
// gorge's.
var kcemeteryOpp = []cardset.Entry{
	{Name: "Shock", Count: 8}, {Name: "Lightning Bolt", Count: 8}, {Name: "Savannah Lions", Count: 6},
	{Name: "Mountain", Count: 18},
}

var kcemeteryZombieDecks = [2][]cardset.Entry{
	{{Name: "Cemetery Recruitment", Count: 10}, {Name: "Walking Corpse", Count: 12}, {Name: "Swamp", Count: 18}},
	kcemeteryOpp,
}

var kcemeteryBearDecks = [2][]cardset.Entry{
	{{Name: "Cemetery Recruitment", Count: 10}, {Name: "Grizzly Bears", Count: 12}, {Name: "Swamp", Count: 10}, {Name: "Forest", Count: 8}},
	kcemeteryOpp,
}

func TestCemeteryRecruitmentLockstep(t *testing.T) {
	t.Parallel()
	t.Run("zombie", func(t *testing.T) {
		lockstepSet(t, "cemeteryrecruit", kcemeteryZombieDecks, 200, "KCEMETERYRECRUIT_SEEDS")
	})
	t.Run("nonzombie", func(t *testing.T) {
		lockstepSet(t, "cemeteryrecruit", kcemeteryBearDecks, 200, "KCEMETERYRECRUIT_SEEDS")
	})
}

// kcemeteryMutate rewrites Cemetery Recruitment's spell program.
func kcemeteryMutate(tbl *kop.Table, f func(tbl *kop.Table, p []kop.Instr) bool) bool {
	hit := false
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		if cards[i].Name != "Cemetery Recruitment" {
			continue
		}
		abs := append([]kop.Ability(nil), cards[i].Abilities...)
		for j := range abs {
			p := append([]kop.Instr(nil), abs[j].Prog...)
			hit = f(tbl, p) || hit
			abs[j].Prog = p
		}
		cards[i].Abilities = abs
	}
	tbl.Cards = cards
	return hit
}

// TestCemeteryRecruitmentRememberMutation: with the return's MvRemember
// dropped nothing is remembered and no Zombie return draws, so the Zombie
// pool diverges: it reaches a return that draws.
func TestCemeteryRecruitmentRememberMutation(t *testing.T) {
	t.Parallel()
	hit := false
	n := lockstepRun(t, "cemeteryrecruit", kcemeteryZombieDecks, 200, false, func(tbl *kop.Table) {
		hit = kcemeteryMutate(tbl, func(_ *kop.Table, p []kop.Instr) bool {
			h := false
			for x := range p {
				if p[x].Op == kop.OpReturnEx && (p[x].C>>3)&kop.MvRemember != 0 {
					p[x].C &^= kop.MvRemember << 3
					h = true
				}
			}
			return h
		})
	})
	if !hit {
		t.Fatal("no ReturnEx with MvRemember in Cemetery Recruitment's program")
	}
	if n == 0 {
		t.Fatal("no seed diverged with the remember dropped")
	}
}

// TestCemeteryRecruitmentConditionMutation: with the draw's condition
// negated a non-Zombie return draws, so the Grizzly Bears pool diverges: it
// reaches a return that does not draw.
func TestCemeteryRecruitmentConditionMutation(t *testing.T) {
	t.Parallel()
	hit := false
	n := lockstepRun(t, "cemeteryrecruit", kcemeteryBearDecks, 200, false, func(tbl *kop.Table) {
		hit = kcemeteryMutate(tbl, func(tbl *kop.Table, p []kop.Instr) bool {
			h := false
			for x := range p {
				if p[x].Op == kop.OpSkipUnless {
					conds := append([]kop.Cond(nil), tbl.Conds...)
					conds[p[x].C].Neg = !conds[p[x].C].Neg
					tbl.Conds = conds
					h = true
				}
			}
			return h
		})
	})
	if !hit {
		t.Fatal("no SkipUnless in Cemetery Recruitment's program")
	}
	if n == 0 {
		t.Fatal("no seed diverged with the draw's condition negated")
	}
}
