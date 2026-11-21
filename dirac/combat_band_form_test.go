package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// bandForm declares the attackers, answers each band ask with the pick'th
// candidate (pick 0 declines) and returns the decision after the last ask.
func bandForm(t *testing.T, sc *scen, atk []Obj, picks ...int) Decision {
	t.Helper()
	sc.passUntil(t, kop.StepAttackers)
	for _, a := range atk {
		sc.do(t, AkAttack, TCode(a), scAny)
	}
	sc.do(t, AkDone, scAny, scAny)
	for _, p := range picks {
		d := sc.expect(t, DkChoose, ChBand)
		sc.s.Apply(bandOpt(t, d, uint8(p)))
	}
	return sc.s.Step()
}

// TestBandForms: CR 702.22c-e and h. An accepted candidate joins the asked
// leader's band and the leader is asked again; a plain creature closes the
// band to other plain creatures; a blocker declared against one member blocks
// every member and divides its damage among them; the band ends with combat.
func TestBandForms(t *testing.T) {
	t.Run("a candidate joins and a lone leader is not re-asked", func(t *testing.T) {
		sc := bandDivScen(t)
		a1, a2 := sc.perm(xbBear, 0), sc.perm(xbBear, 0)
		sc.grantBanding(a1)
		d := bandForm(t, sc, []Obj{a1, a2}, 1)
		if d.Kind != DkPriority {
			t.Fatalf("decision kind %d sub %d after the only candidate joined, want the priority round", d.Kind, d.Sub)
		}
		if id := uint8(sc.slot(a1)) + 1; sc.s.bandOf(sc.slot(a1)) != id || sc.s.bandOf(sc.slot(a2)) != id {
			t.Fatalf("bands %d and %d, want both %d", sc.s.bandOf(sc.slot(a1)), sc.s.bandOf(sc.slot(a2)), id)
		}
		if sc.s.g.stalled != 0 {
			t.Fatalf("stalled %d (%s)", sc.s.g.stalled, StallName(sc.s.g.stalled))
		}
	})
	t.Run("the leader is asked again and a plain member closes the band to plain creatures", func(t *testing.T) {
		sc := bandDivScen(t)
		a1, a2, a3, a4 := sc.perm(xbBear, 0), sc.perm(xbBear, 0), sc.perm(xbBear, 0), sc.perm(xbBear, 0)
		sc.grantBanding(a1)
		sc.grantBanding(a3)
		sc.passUntil(t, kop.StepAttackers)
		for _, a := range []Obj{a1, a2, a3, a4} {
			sc.do(t, AkAttack, TCode(a), scAny)
		}
		sc.do(t, AkDone, scAny, scAny)
		d := sc.expect(t, DkChoose, ChBand)
		if d.N != 4 {
			t.Fatalf("first ask has %d options, want the decline and three candidates", d.N)
		}
		sc.s.Apply(bandOpt(t, d, 1)) // a2: a creature without banding
		d = sc.expect(t, DkChoose, ChBand)
		if sc.s.lastPick != sc.slot(a1) || d.N != 2 || bandOpt(t, d, 1).Source() != TCode(a3) {
			t.Fatalf("re-ask: leader slot %d (want %d), %d options, candidate %d (want only banding %d)",
				sc.s.lastPick, sc.slot(a1), d.N, bandOpt(t, d, 1).Source(), a3)
		}
		sc.s.Apply(bandOpt(t, d, 0))
		d = sc.expect(t, DkChoose, ChBand) // the next leader, a3: a1's band is closed to it, a4 is plain-free
		if sc.s.lastPick != sc.slot(a3) || d.N != 2 || bandOpt(t, d, 1).Source() != TCode(a4) {
			t.Fatalf("a3's ask: leader slot %d, %d options, candidate %d (want a4 %d)", sc.s.lastPick, d.N, bandOpt(t, d, 1).Source(), a4)
		}
	})
	t.Run("a blocker of one member blocks the band and its controller divides", func(t *testing.T) {
		sc := bandDivScen(t)
		a1, a2, b := sc.perm(xbBear, 0), sc.perm(xbBear, 0), sc.perm(xbBear, 1)
		sc.grantBanding(a1)
		bandForm(t, sc, []Obj{a1, a2}, 1)
		sc.passUntil(t, kop.StepBlockers)
		sc.do(t, AkBlock, TCode(b), TCode(a2))
		sc.do(t, AkDone, scAny, scAny)
		s := sc.s
		if !s.blocks(sc.slot(b), sc.slot(a1)) || !s.blocks(sc.slot(b), sc.slot(a2)) {
			t.Fatal("the blocker does not block both members")
		}
		if p := &s.perms[sc.slot(a1)]; p.nblock != 1 || p.flags&pfBlocked == 0 {
			t.Fatalf("the unchosen member has %d blockers, blocked flag %v", p.nblock, p.flags&pfBlocked != 0)
		}
		// the blocker blocks a banding attacker: the attacking player divides (CR 702.22k)
		d := xbToDivide(t, sc)
		if d.Player != 0 || s.divA != sc.slot(b) {
			t.Fatalf("division of slot %d asked of player %d, want blocker %d divided by player 0", s.divA, d.Player, sc.slot(b))
		}
	})
	t.Run("an unbanded pair has no band", func(t *testing.T) {
		sc := bandDivScen(t)
		a1, a2, b := sc.perm(xbBear, 0), sc.perm(xbBear, 0), sc.perm(xbBear, 1)
		bandForm(t, sc, []Obj{a1, a2})
		sc.passUntil(t, kop.StepBlockers)
		sc.do(t, AkBlock, TCode(b), TCode(a2))
		sc.do(t, AkDone, scAny, scAny)
		if sc.s.blocks(sc.slot(b), sc.slot(a1)) {
			t.Fatal("a creature in no band was blocked with another")
		}
	})
	t.Run("the band ends with combat", func(t *testing.T) {
		sc := bandDivScen(t)
		a1, a2 := sc.perm(xbBear, 0), sc.perm(xbBear, 0)
		sc.grantBanding(a1)
		bandForm(t, sc, []Obj{a1, a2}, 1)
		sc.passUntil(t, kop.StepEndCombat)
		for i := 0; i < 8 && sc.s.step != kop.StepMain2; i++ {
			sc.s.Step()
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
		if sc.s.bandOf(sc.slot(a1)) != 0 || sc.s.bandOf(sc.slot(a2)) != 0 {
			t.Fatalf("bands %d and %d survive the combat", sc.s.bandOf(sc.slot(a1)), sc.s.bandOf(sc.slot(a2)))
		}
	})
	t.Run("a band of three blocked by one creature stalls", func(t *testing.T) {
		sc := bandDivScen(t)
		a1, a2, a3, b := sc.perm(xbBear, 0), sc.perm(xbBear, 0), sc.perm(xbBear, 0), sc.perm(xbBear, 1)
		sc.grantBanding(a1)
		sc.grantBanding(a3)
		bandForm(t, sc, []Obj{a1, a2, a3}, 1, 1)
		sc.passUntil(t, kop.StepBlockers)
		sc.do(t, AkBlock, TCode(b), TCode(a1))
		sc.do(t, AkDone, scAny, scAny)
		if sc.s.g.stalled != StallBand {
			t.Fatalf("stalled %d (%s), want StallBand", sc.s.g.stalled, StallName(sc.s.g.stalled))
		}
	})
}
