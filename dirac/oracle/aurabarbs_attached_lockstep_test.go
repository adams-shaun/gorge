package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Aura Barbs' two RelativeTarget$ lines played over a battlefield of attached
// Auras: "each enchantment deals 2 damage to its controller, then each Aura
// attached to a creature deals 2 damage to the creature it's attached to".
// Deck A is the only source of enchantments (Pacifisms, all played by its
// seat), so that seat's life total pins the first line's per-source pass and
// the second line's attached-bearer pass at once.
var (
	auraBarbsDeckA = []cardset.Entry{
		{Name: "Aura Barbs", Count: 4}, {Name: "Pacifism", Count: 4}, {Name: "Mana Confluence", Count: 32},
	}
	auraBarbsDeckB = []cardset.Entry{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Wind Drake", Count: 4}, {Name: "Mana Confluence", Count: 24},
	}
	auraBarbsMatchups = []fixtureDecks{{auraBarbsDeckA, auraBarbsDeckB}, {auraBarbsDeckB, auraBarbsDeckA}}
	auraBarbsWatch    = map[string]bool{
		"Aura Barbs": true, "Pacifism": true, "Grizzly Bears": true,
		"Savannah Lions": true, "Hill Giant": true, "Wind Drake": true,
	}
)

// TestAuraBarbsAttachedLockstep: 40 seeds (twenty per option surface) on the
// aurapacifism fixture. Every divergence fails. The floor asserts the Barbs
// and the Pacifisms were actually cast, so a clean run cannot be a quiet run.
func TestAuraBarbsAttachedLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "aurapacifism", auraBarbsMatchups, auraBarbsWatch, 40, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if run.divs != 0 {
		t.Errorf("%d divergences: %s", run.divs, run.first)
	}
	if run.stalls != 0 {
		t.Errorf("%d kernel capacity stalls", run.stalls)
	}
	if run.casts["Aura Barbs"] < 10 {
		t.Errorf("only %d Aura Barbs casts: the per-source passes were barely played", run.casts["Aura Barbs"])
	}
	if run.casts["Pacifism"] < 10 {
		t.Errorf("only %d Pacifism casts: no attached Aura was on the battlefield enough to pin the lines", run.casts["Pacifism"])
	}
}

// TestAuraBarbsAttachedLockstepMutation: quiet each line's amount in turn and
// the run must diverge. A zeroed per-controller pass loses the Barbs seat's
// 2 life per enchantment; a zeroed attached pass leaves the 1/1s a Pacifism
// sits on alive. A no-op mutation (neither line in the table) fails outright.
func TestAuraBarbsAttachedLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, tc := range []struct {
		mode int16
		what string
	}{{kop.DmgFromEachCtrl, "the each-enchantment-to-its-controller pass"},
		{kop.DmgFromEachAttached, "the each-attached-Aura-to-its-bearer pass"}} {
		t.Run(tc.what, func(t *testing.T) {
			run := runFixtureLockstep(t, "aurapacifism", auraBarbsMatchups, auraBarbsWatch, 24, false,
				func(tbl *kop.Table) {
					n := 0
					mutateCard(t, tbl, "Aura Barbs", func(_ *kop.Ability, in *kop.Instr) {
						if in.Op == kop.OpDealDamageFrom && in.B.N == tc.mode {
							in.A, n = kop.Const(0), n+1
						}
					})
					if n == 0 {
						t.Fatalf("Aura Barbs: no DealDamageFrom mode %d in the table (the pass is not lowered)", tc.mode)
					}
				})
			run.requireDivergence(t, tc.what)
		})
	}
}
