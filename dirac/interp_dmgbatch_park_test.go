package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRefEachOpponentBatchParksOpen: OpDealDamage Obj=RefEachOpponent is one
// damage batch (gorge BeginDamageBatch/EndDamageBatch around a multi-recipient
// DealDamage). A CR 616.1 replacement-order ask on the first opponent's event
// parks the whole op with the batch still open and leaves the later
// opponents to the resume; it must not deal them and close the batch around
// the parked event (the DamageAll latch would then queue a second time).
func TestRefEachOpponentBatchParksOpen(t *testing.T) {
	deck := []cardset.Entry{{Name: "Twinflame Tyrant", Count: 2}, {Name: "Burst Lightning", Count: 1},
		{Name: "Erudite Wizard", Count: 2}, {Name: "Mountain", Count: 12}}
	sc, id := replScen(t, deck)
	s, k := sc.s, sc.k
	if k.gen == nil || k.gen.anyRepl&rbDblDmg == 0 {
		t.Fatal("precondition: the table has no damage doubler")
	}
	// four seats: seat 1 resolves the sweep; its opponents in order are
	// seats 2, 3, 0.
	s.g.nseat = 4
	for p := uint8(0); p < 4; p++ {
		s.pl[p].life = 20
	}
	sc.perm(id["Twinflame Tyrant"], 1)
	sc.perm(id["Twinflame Tyrant"], 1)
	s.active = 0
	s.res.e.ctrl = 1
	in := &kop.Instr{Op: kop.OpDealDamage, Obj: kop.RefEachOpponent, A: kop.Const(3)}
	if !s.exec(k, in) {
		t.Fatal("the op suspended on a decision; the replacement ask is posed after the op")
	}
	if s.Stalled() {
		t.Fatalf("stalled %d after the first parked event, want the op to park", s.StallReason())
	}
	if !s.g.dmgSeqOn || !s.g.dmgBOn {
		t.Fatalf("first event parked but dmgSeqOn=%v dmgBOn=%v, want both open", s.g.dmgSeqOn, s.g.dmgBOn)
	}
	if s.g.nc.n == 0 {
		t.Fatal("precondition: no replacement-order ask was parked")
	}
	if s.pl[2].life != 20 {
		t.Fatalf("seat 2 at %d life, want 20 (its event is parked on the ask)", s.pl[2].life)
	}
	if s.pl[3].life != 20 || s.pl[0].life != 20 {
		t.Fatalf("later opponents dealt inside the parked batch (seats 3/0 at %d/%d)", s.pl[3].life, s.pl[0].life)
	}
}
