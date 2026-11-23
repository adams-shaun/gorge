package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// The restricted-batch half of the UnspentMana keep: gorge's ManaClear fold
// keeps a restricted batch whose Persistent flag is set OR whose colour slot
// the keep letters protect, beside the protected slot's ordinary share
// (engine/events/apply_misc.go: `r.Persistent ||
// keep[state.ManaSlot(r.Color)]`). The four kernel boundary emptiers
// (endStep, finishCleanup, persistEmpty, persistTurn) must keep those
// batches too; an ordinary batch (State.rb kind not a restrPers mark — the
// Helga, Skittish Seer spelling) of a protected colour survives at every
// boundary, still restricted. The precondition each case pins is the
// emptier's rb half itself (a table without the static empties every batch;
// mana_restrbatch_test.go holds that shape).

// rbKeepUnitsOf sums seat p's restricted units of colour col still floating.
func rbKeepUnitsOf(s *State, p uint8, col int) (units int, restricted bool) {
	for i := range s.rb[SeatIx(p)] {
		if b := &s.rb[SeatIx(p)][i]; b.kind != 0 {
			units += int(b.n[col])
			restricted = true
		}
	}
	return units, restricted
}

// restrKeepTable builds a scen table carrying the two UnspentMana spellings;
// bearer picks which is in play (0: neither).
func restrKeepTable(t *testing.T, bearer CardID) *scen {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Upwelling", Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
				Kind: kop.StatUnspentMana, A: int16(kop.ColW | kop.ColU | kop.ColB | kop.ColR | kop.ColG | kop.ColC)}}}},
		kop.CardDef{Name: "Test Omnath", Types: kop.TCreature, Power: 0, Toughness: 0,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
				Kind: kop.StatUnspentMana, A: int16(kop.ColG), StFlags: kop.StUnspentYou}}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	if bearer != 0 {
		sc.perm(bearer, 0)
	}
	return sc
}

// TestUnspentManaKeepsRestrictedBatch: the step boundary (endStep) keeps an
// ordinary restricted batch of a protected colour, and only its protected
// colours: the You-scoped spelling keeps its controller's green batch alone,
// the bare spelling keeps both seats', a batch of an unprotected colour
// empties with the pool, and a table without the static keeps nothing.
func TestUnspentManaKeepsRestrictedBatch(t *testing.T) {
	upwelling, omnath := CardID(len(v.Table.Cards)), CardID(len(v.Table.Cards)+1)
	for _, c := range []struct {
		name                   string
		bearer                 CardID
		want0G, want0R, want1G int
		wantPool0R             uint8
	}{
		{"Upwelling: every seat's batch of every colour", upwelling, 1, 2, 1, 1},
		{"You scope: the controller's green batch only", omnath, 1, 0, 0, 0},
		{"no static: every batch empties", 0, 0, 0, 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			sc := restrKeepTable(t, c.bearer)
			// the precondition: an ordinary restricted batch (kind 1, no
			// restrPers mark) of each colour beside each seat's pool
			if !sc.s.rbPut(0, 1, 0, kop.G, 1) || !sc.s.rbPut(0, 1, 0, kop.R, 2) || !sc.s.rbPut(1, 1, 0, kop.G, 1) {
				t.Fatalf("rbPut refused")
			}
			sc.s.pl[0].pool[kop.R] = c.wantPool0R
			sc.s.endStep(kop.StepBeginCombat)
			g0, restr0 := rbKeepUnitsOf(sc.s, 0, kop.G)
			r0, _ := rbKeepUnitsOf(sc.s, 0, kop.R)
			g1, _ := rbKeepUnitsOf(sc.s, 1, kop.G)
			if g0 != c.want0G || r0 != c.want0R || g1 != c.want1G {
				t.Fatalf("batches after endStep: seat0 G%d R%d, seat1 G%d; want G%d R%d, G%d",
					g0, r0, g1, c.want0G, c.want0R, c.want1G)
			}
			if restr0 != (c.want0G != 0) {
				t.Fatalf("a kept batch must still be restricted (kind set), got %v", restr0)
			}
			if got := sc.s.pl[0].pool[kop.R]; got != c.wantPool0R {
				t.Fatalf("pool R %d after endStep, want %d", got, c.wantPool0R)
			}
		})
	}
}

// TestUnspentManaKeepsRestrictedEveryBoundary: the other three emptiers keep
// the batch too — persistEmpty (a pmOn seat's boundary), finishCleanup, and
// the turn change, where gorge's TurnChange demotes every batch before the
// cleanup-to-untap ManaClear, so only the keep-letter colours survive there:
// a persistent-restriction batch of an unprotected colour rides the step
// boundaries and empties at the turn change, exactly the demote-then-empty.
func TestUnspentManaKeepsRestrictedEveryBoundary(t *testing.T) {
	upwelling, omnath := CardID(len(v.Table.Cards)), CardID(len(v.Table.Cards)+1)
	sc := restrKeepTable(t, upwelling)
	if g := sc.s.k().gen; g == nil {
		t.Fatalf("no genPrep for the scen table")
	} else {
		g.restrPers = 1 // kind 1 is a persistent restriction (Karn, Legacy Reforged's spelling)
	}
	if !sc.s.rbPut(0, 1, 0, kop.R, 2) {
		t.Fatalf("rbPut refused")
	}
	sc.s.pl[0].pool[kop.R] = 1

	// persistEmpty's boundary (a pmOn seat rides it in finishCleanup)
	sc.s.pmOn = 1 << SeatIx(0)
	sc.s.persistEmpty(false)
	if u, restr := rbKeepUnitsOf(sc.s, 0, kop.R); u != 2 || !restr {
		t.Fatalf("persistEmpty dropped the protected batch: R%d restr %v", u, restr)
	}

	// finishCleanup's own boundary
	if !sc.s.rbPut(0, 2, 0, kop.R, 2) {
		t.Fatalf("rbPut refused")
	}
	sc.s.finishCleanup(sc.s.k())
	if u, restr := rbKeepUnitsOf(sc.s, 0, kop.R); u < 4 || !restr {
		t.Fatalf("finishCleanup dropped the protected batch: R%d restr %v", u, restr)
	}

	// the turn change keeps the protected colours (gorge demotes every
	// batch, then the cleanup-to-untap ManaClear keeps the keep-letter ones)
	sc.s.persistTurn()
	if u, restr := rbKeepUnitsOf(sc.s, 0, kop.R); u < 4 || !restr {
		t.Fatalf("persistTurn dropped the protected batch: R%d restr %v", u, restr)
	}

	// an unprotected colour: the persistent-restriction batch rides the step
	// boundary (the fold's r.Persistent half) and empties at the turn change
	// (demoted first, the keep letters hold only green)
	sc2 := restrKeepTable(t, omnath)
	if g := sc2.s.k().gen; g == nil {
		t.Fatalf("no genPrep for the scen table")
	} else {
		g.restrPers = 1
	}
	if !sc2.s.rbPut(0, 1, 0, kop.R, 2) {
		t.Fatalf("rbPut refused")
	}
	sc2.s.endStep(kop.StepBeginCombat)
	if u, restr := rbKeepUnitsOf(sc2.s, 0, kop.R); u != 2 || !restr {
		t.Fatalf("endStep dropped a persistent-restriction batch of an unprotected colour: R%d restr %v", u, restr)
	}
	sc2.s.persistTurn() // the turn change: gorge demotes the batch, the keep letters hold only green
	if u, _ := rbKeepUnitsOf(sc2.s, 0, kop.R); u != 0 {
		t.Fatalf("the turn change kept a batch of an unprotected colour: R%d", u)
	}
}
