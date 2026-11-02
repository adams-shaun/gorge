package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// opus: TriggeredCard$CastTotalManaSpent in a SpellCast trigger (kop.
// Tf2CastSpent). The trigger amount is the triggering spell's pool mana
// spent, read from the spell entry's recorded spend (recordSpent) as the
// trigger is queued, so it survives the spell leaving the stack (gorge
// TriggerManaSpent, rules/trigger_referents.go). A table holding such a
// trigger also turns on the spend record (anySpent, anyHist).
//
// gorge captures a cast's spend only while a reader is on a battlefield
// (triggeredCastSpendReaderOut) and, while one is, its payment planner
// prices no cast at all (payment_plan_search.go paymentPlanBoardSpendReaderOut,
// "shape:mana_spent_reader"): spendReaderOut is that board test, read by
// plannedCastCost beside the convoke decline.

// gfSpendRd marks a card with a Tf2CastSpent trigger.
const gfSpendRd uint32 = 1 << 17

// prepOpus sets genPrep.anyCastSpent and the per-card flag.
func prepOpus(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigSpellCast && a.Trig.Flags2&kop.Tf2CastSpent != 0 {
				g.setFlag(CardID(i), gfSpendRd)
				g.anyCastSpent, g.anySpent, g.anyHist = true, true, true
			}
		}
	}
}

// The mutation switches of TestOpusMutation (dirac/oracle/opus_test.go):
// each must make the lockstep diverge.
var (
	DebugOpusMV        bool // the amount is the spell's printed mana value, not its spend
	DebugOpusLive      bool // the amount is read from the live spell entry at resolution, not the snapshot
	DebugOpusNoDecline bool // gorge's planner decline is ignored
)

// castSpentAmt is the amount a Tf2CastSpent trigger queues for spell e.
//
//go:noinline
func (s *State) castSpentAmt(k *prep, e *stackEntry) int16 {
	if DebugOpusMV {
		return int16(k.t.Cards[e.card].Cost.MV())
	}
	return s.castSpent(e.obj)
}

// liveSpentAmt is ExTriggerAmount's mutation arm (DebugOpusLive).
//
//go:noinline
func (s *State) liveSpentAmt(k *prep, e *stackEntry) int16 {
	if e.kind != seTrigger || !k.gen.has(e.card, gfSpendRd) || int(e.ab) >= len(k.t.Cards[e.card].Abilities) || k.t.Cards[e.card].Abilities[e.ab].Trig.Flags2&kop.Tf2CastSpent == 0 {
		return e.trigAmt
	}
	return s.castSpent(e.trigObj)
}

// OpusReaderOut reports gorge's planner decline (oracle checks).
func (s *State) OpusReaderOut() bool {
	k := s.k()
	return k.gen != nil && k.gen.anyCastSpent && s.spendReaderOut(k)
}

// OpusReaderCtrl reports whether player p controls a mana-spent reader.
func (s *State) OpusReaderCtrl(p uint8) bool {
	k := s.k()
	if k.gen == nil || !k.gen.anyCastSpent {
		return false
	}
	for m := s.occ; m.any(); m = m.rest() {
		if sl := m.first(); s.perms[sl].ctrl == p && k.gen.has(s.cardAt(sl), gfSpendRd) {
			return true
		}
	}
	return false
}

// OpusSpentTrigger: stack entry i is a Tf2CastSpent trigger; amt is its
// snapshot, spell the card of the spell that triggered it (oracle counts).
func (s *State) OpusSpentTrigger(i int) (amt int16, spell CardID, ok bool) {
	k := s.k()
	e := &s.stack[i]
	if e.kind != seTrigger || k.gen == nil || !k.gen.anyCastSpent || int(e.ab) >= len(k.t.Cards[e.card].Abilities) {
		return 0, 0, false
	}
	if k.t.Cards[e.card].Abilities[e.ab].Trig.Flags2&kop.Tf2CastSpent == 0 {
		return 0, 0, false
	}
	return e.trigAmt, s.objs[e.trigObj].card, true
}

// castSpent is the pool mana spent on the spell object o still on the
// stack (0 when absent).
//
//go:noinline
func (s *State) castSpent(o Obj) int16 {
	for i := int(s.sp) - 1; i >= 0; i-- {
		if e := &s.stack[i]; e.kind == seSpell && e.obj == o {
			return e.trigAmt
		}
	}
	return 0
}

// spendReaderOut: some permanent of either player holds a Tf2CastSpent
// trigger (the printed or copied card; face-down status is ignored, as
// gorge's Object.Face()).
//
//go:noinline
func (s *State) spendReaderOut(k *prep) bool {
	if DebugOpusNoDecline {
		return false
	}
	for m := s.occ; m.any(); m = m.rest() {
		if k.gen.has(s.cardAt(m.first()), gfSpendRd) {
			return true
		}
	}
	return false
}
