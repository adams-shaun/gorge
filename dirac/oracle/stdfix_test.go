package oracle

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/effects"
)

// skipWithoutGorgeStdFixes skips the stdfix lockstep at a gorge pin
// without the four MageZero fixes (gorge carries them all:
// cr-trigcond-defenderctrl, cr-sleep-defined-targeted, cr-blocked-filter,
// cr-totalpt-filter). The probe
// reads the two filter words the fixes added (effects.UnknownPredicates);
// the Siege Dragon and Sleep fixes have no exported surface and landed in
// the same pin range.
func skipWithoutGorgeStdFixes(t *testing.T) {
	t.Helper()
	if len(effects.UnknownPredicates("Creature.blocked+YouCtrl")) != 0 || len(effects.UnknownPredicates("Creature.totalPT_LE5")) != 0 || !gorgePinAtLeast(t, gorgeStdFixesTime) {
		t.Skip("gorge pin lacks the blocked / totalPT filter words and the Siege Dragon / Sleep fixes")
	}
}

// gorgeStdFixesTime is the commit time (UTC, as in a Go pseudo-version) of
// gorge, the first commit carrying all four fixes. The filter-word
// probe alone is not enough: gorge (07:44:17Z) has blocked and
// totalPT but not cr-sleep-defined-targeted, so the Sleep leg cannot
// diverge there.
const gorgeStdFixesTime = "20261026075942"

// gorgePinAtLeast reports whether the linked gorge module's pseudo-version
// timestamp is at or after ts (yyyymmddhhmmss). A test binary's build info
// does not list its dependencies, so the version is read from the source
// path of a gorge function, which the module cache spells
// .../github.com/adams-shaun/gorge@v0.0.0-<time>-<sha>/...; a replace to a
// local checkout (the -modfile route used to test against an unreleased
// gorge) rewrites the path to the require version, the bare v0.0.0 for a
// versionless replace, and counts as new enough.
func gorgePinAtLeast(t *testing.T, ts string) bool {
	t.Helper()
	f := runtime.FuncForPC(reflect.ValueOf(effects.UnknownPredicates).Pointer())
	if f == nil {
		return false
	}
	file, _ := f.FileLine(f.Entry())
	at := strings.Index(file, "adams-shaun/gorge@")
	if at < 0 {
		return true
	}
	v := file[at+len("adams-shaun/gorge@"):]
	if i := strings.IndexByte(v, '/'); i >= 0 {
		v = v[:i]
	}
	// v0.0.0-yyyymmddhhmmss-abcdef123456
	parts := strings.Split(v, "-")
	if len(parts) < 3 {
		// No pseudo-version timestamp: the bare v0.0.0 rewrite means the
		// module is a directory replace, i.e. a local (unreleased) checkout.
		return v == "v0.0.0"
	}
	if len(parts[len(parts)-2]) != 14 {
		return false
	}
	return parts[len(parts)-2] >= ts
}

// The MageZero gorge-fix lockstep, for the four cards gorge
// mis-played at the f9191e666 pin:
//   - Siege Dragon: the attack trigger's intervening-if counts the Walls the
//     defending player controls (IsPresent$ Wall.DefenderCtrl), at queue
//     time and on resolution, and its damage sweeps that player's creatures
//     without flying;
//   - Tactical Advantage: target blocking or blocked creature you control
//     (gorge "blocked", kernel PredBlocked);
//   - Cut Down: target creature with total power and toughness 5 or less
//     (gorge totalPT_LE5, kernel PredTotalPT);
//   - Sleep: tap all creatures target player controls; those creatures (only
//     those) don't untap during that player's next untap step (the
//     OpSkipNextUntap sweep).
//
// The expected answer at every decision is gorge's.
var kstdfixDecks = [2][]cardset.Entry{
	{
		{Name: "Siege Dragon", Count: 5}, {Name: "Tactical Advantage", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Serra Angel", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Sleep", Count: 4}, {Name: "Cut Down", Count: 4}, {Name: "Wall of Air", Count: 4},
		{Name: "Wall of Spears", Count: 4}, {Name: "Walking Corpse", Count: 6},
		{Name: "Island", Count: 9}, {Name: "Swamp", Count: 9},
	},
}

func TestStdfixLockstep(t *testing.T) {
	t.Parallel()
	skipWithoutGorgeStdFixes(t)
	lockstepSet(t, "stdfix", kstdfixDecks, 1000, "KSTDFIX_SEEDS")
}

// TestStdfixLockstepMutation proves the pool exercises each fix: widening
// the blocked or totalPT target to any creature, or Sleep's sweep to both
// players' creatures, makes some seed diverge.
func TestStdfixLockstepMutation(t *testing.T) {
	t.Parallel()
	skipWithoutGorgeStdFixes(t)
	for _, m := range []struct {
		name string
		fn   func(*kop.Table)
	}{
		{"blocked", func(tbl *kop.Table) { widenPred(tbl, kop.PredBlocked) }},
		{"totalPT", func(tbl *kop.Table) { widenPred(tbl, kop.PredTotalPT) }},
		{"sleep", func(tbl *kop.Table) {
			for i := range tbl.Cards {
				for j := range tbl.Cards[i].Abilities {
					p := tbl.Cards[i].Abilities[j].Prog
					for k := range p {
						if p[k].Op == kop.OpSkipNextUntap && p[k].F != 0 {
							p[k].Obj = kop.RefNone
						}
					}
				}
			}
		}},
	} {
		if lockstepRun(t, "stdfix", kstdfixDecks, 300, false, m.fn) == 0 {
			t.Errorf("%s: no seed diverged with the mutation: the pool never exercises it", m.name)
		}
	}
}

// widenPred drops predicate p from every filter of the table.
func widenPred(tbl *kop.Table, p kop.Pred) {
	for i := range tbl.Filters {
		if tbl.Filters[i].Pred == p {
			tbl.Filters[i].Pred, tbl.Filters[i].PredN = kop.PredNone, 0
		}
	}
}
