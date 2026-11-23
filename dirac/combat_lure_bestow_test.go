package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLureBestowOnToken: Noble Quarry bestowed on an attacking token — the
// tail-13 lockstep pair that pitted a lure attacker (the token) beside two
// plain attackers against two blockers. Every able blocker must block the
// lure attacker (CR 509.1c), so each blocker is offered only its lure pairs
// and Done only once every duty blocker is on a lure attacker (combat_lure.go).
func TestLureBestowOnToken(t *testing.T) {
	set := cardsettest.Set(t, "luregrant")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Skipf("%s not in the luregrant pool", name)
		}
		return CardID(set.PoolID(i))
	}
	sc := newScenTableLib(t, tbl, lookup("Forest"), Config{NoElide: true})

	// p0 attacks with a token enchanted by a bestowed Noble Quarry and a
	// plain creature; p1 defends with two creatures.
	tok := sc.s.newToken(lookup("Grizzly Bears"), 0)
	sc.s.enter(sc.k, tok, 0, false, kop.ZoneNone)
	sc.s.npend = 0
	sl := sc.slot(tok)
	sc.s.perms[sl].flags &^= pfSick
	plain := sc.perm(lookup("Hill Giant"), 0)
	aura := sc.perm(lookup("Noble Quarry"), 0)
	sc.s.perms[sc.slot(aura)].attached = sl
	b1 := sc.perm(lookup("Raging Goblin"), 1)
	b2 := sc.perm(lookup("Llanowar Elves"), 1)

	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(tok), scAny)
	sc.do(t, AkAttack, TCode(plain), scAny)
	sc.do(t, AkDone, scAny, scAny)
	sc.passUntil(t, kop.StepBlockers)
	d := sc.s.Step()
	if d.Kind != DkBlockers || d.Player != 1 {
		t.Fatalf("decision kind %d player %d, want blockers for p1", d.Kind, d.Player)
	}
	nb1, nb2, done := 0, 0, 0
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		switch a.Source() {
		case TCode(b1):
			nb1++
		case TCode(b2):
			nb2++
		case 0:
			done++
		}
	}
	if nb1 != 1 || nb2 != 1 {
		t.Fatalf("duty blockers got %d and %d block options, want 1 lure pair each (options %v)", nb1, nb2, d.Options[:d.N])
	}
	if done != 0 {
		t.Fatalf("Done offered while the lure duty is unmet (options %v)", d.Options[:d.N])
	}
}
