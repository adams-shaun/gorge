package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The self flash IsTargeting lockstep pool (dirac/kgen/lower_static.go
// selfFlashTargeting, dirac/cast_selfflashtgt.go): Flash Photography casts at
// instant speed only while it can target a permanent its caster controls, and
// announces only such a target (gorge castWithFlashTargets / recheckIllegal).
// Timely Ward's "targets a commander" grant never holds outside a Commander
// game, so the pool plays it at sorcery speed only; it is here as the control
// that the grant does not leak timing.

func selfFlashTgtDecks() [2][]cardset.Entry {
	a := []cardset.Entry{
		{Name: "Flash Photography", Count: 8}, {Name: "Timely Ward", Count: 4},
		{Name: "Wind Drake", Count: 6}, {Name: "Elite Vanguard", Count: 4},
		{Name: "Island", Count: 10}, {Name: "Plains", Count: 8},
	}
	b := []cardset.Entry{
		{Name: "Savannah Lions", Count: 10}, {Name: "Elite Vanguard", Count: 8},
		{Name: "Wind Drake", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 10},
	}
	return [2][]cardset.Entry{a, b}
}

func TestSelfFlashIsTargetingPoolPlayable(t *testing.T) {
	t.Parallel()
	_, set, gd := lockstepPool(t, "selfflashtgt", selfFlashTgtDecks())
	for s, d := range gd {
		if !SetDeckSupported(set, d) {
			t.Errorf("seat %d: deck not playable in the selfflashtgt pool", s)
		}
	}
}

func TestSelfFlashIsTargetingLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "selfflashtgt", selfFlashTgtDecks(), 200, "SELFFLASH_TGT_SEEDS")
}

// TestSelfFlashIsTargetingMutation: with the target spec cleared from the
// grant (Filter2 = 0: flash on any target) some seed diverges, so the pool
// reaches a cast the census withholds.
func TestSelfFlashIsTargetingMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "selfflashtgt", selfFlashTgtDecks(), 200, false, func(tbl *kop.Table) {
		cs := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cs {
			if !cs[i].SelfFlash {
				continue
			}
			ab := append([]kop.Ability(nil), cs[i].Abilities...)
			for j := range ab {
				if ab[j].Kind == kop.AbStatic && ab[j].Static.Kind == kop.StatCastWithFlash {
					ab[j].Static.Filter2 = 0
				}
			}
			cs[i].Abilities = ab
		}
		tbl.Cards = cs
	})
	if n == 0 {
		t.Error("no seed diverged with the target spec cleared: the pool never reaches the census")
	}
}
