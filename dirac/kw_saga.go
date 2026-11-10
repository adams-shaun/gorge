package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Sagas (CR 714, kw:Chapter), copied from
// gorge's engine/rules/saga.go, engine/cards/saga.go,
// engine/events/entry_counters.go and engine/rules/turn.go:
//
//   - a Saga enters with one lore counter (zones.go enterNF, through the
//     counter replacements; gorge EntryCounterGrants), and its controller
//     adds another immediately after their precombat main begins
//     (advanceSagas; CR 505.4 / 714.3c).
//   - each chapter ability is a printed battlefield trigger on
//     TrigCounterAddedOnce of CtrLore, gated at its chapter number
//     (Ability.Gate{CtrLore, N}). A counter event crossing N triggers it
//     exactly once: chapter N is queued only while N is in
//     (oldCount, newCount], so a doubled entry counter or a proliferate
//     that crosses two chapters queues both (CR 714.2b).
//   - once the Saga carries its final chapter counter and none of its
//     chapter abilities is pending or on the stack, the CR 714.4 SBA
//     sacrifices it (sagaSBA).
//
// The SBA flag and the precombat-main hook read prep.anySaga; a table with
// no Saga never takes these paths. The kernel keeps no new State: the lore
// count is an ordinary sparse counter slot and "busy" reads the pending and
// stack queues.

// isChapterAbility reports whether ability ab of card c is a Saga chapter
// trigger (the shape kgen lowers K:Chapter into).
func isChapterAbility(k *prep, c CardID, ab uint8) bool {
	if int(c) >= len(k.t.Cards) {
		return false
	}
	cd := &k.t.Cards[c]
	if int(ab) >= len(cd.Abilities) {
		return false
	}
	a := &cd.Abilities[ab]
	return a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigCounterAddedOnce &&
		a.Trig.N == int16(kop.CtrLore) && a.Gate.Ctr == kop.CtrLore
}

// DebugIgnoreChapterCross is the lockstep's mutation switch
// (TestSagaMutation): set, no chapter ability fires on a lore crossing.
// Read only on the lore-counter trigger path (chapterCrossed), never in a
// walk; the table with no Saga never reaches it.
var DebugIgnoreChapterCross bool

// chapterCrossed reports whether the lore-counter event of amount amt on
// slot sl just reached chapter a.Gate.N (gorge checkChapterTriggers: the
// first chapter crossed is the count before the event, floored at 1).
//
//go:noinline
func (s *State) chapterCrossed(sl Slot, a *kop.Ability, amt int) bool {
	if DebugIgnoreChapterCross || sl == NoSlot || amt <= 0 {
		return false
	}
	count := s.perms[sl].ctrN(s, int(kop.CtrLore))
	first := count - amt + 1
	if first < 1 {
		first = 1
	}
	n := int(a.Gate.N)
	return n >= first && n <= count
}

// addLore adds n lore counters to the Saga at sl and fires the counter
// event (gorge's CounterChange on LORE). It reports whether any counter was
// placed.
//
//go:noinline
func (s *State) addLore(k *prep, sl Slot, n int) bool {
	mode := ctrPutEffect
	if !s.res.active {
		mode = ctrPutBy // the turn-based lore counter (gorge saga.go publishes the controller)
	}
	n = s.placeCounters(k, sl, int(kop.CtrLore), n, s.perms[sl].ctrl, mode)
	if n <= 0 {
		return false
	}
	p := &s.perms[sl]
	if k.anyCtrAdded {
		s.ctrNoteBy(k, sl, int(kop.CtrLore), s.perms[sl].ctrl) // the turn-based lore counter's adder is the Saga's controller (gorge saga.go)
	}
	if s.subs[kop.TrigCounterAddedOnce].any() {
		s.g.lastCK = kop.CtrLore
		s.fireObj(k, kop.TrigCounterAddedOnce, sl, p.obj, p.ctrl, int16(n))
	}
	return true
}

// advanceSagas is CR 505.4 / 714.3c: immediately after the active player's
// precombat main phase begins, that player puts a lore counter on each Saga
// they control, in battlefield order (gorge advanceSagas).
//
//go:noinline
func (s *State) advanceSagas(k *prep) {
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(s.active, s.ctrlM[s.active], ord[:], 0)
	for _, sl := range ord[:n] {
		if k.cards[s.cardAt(sl)].sagaN == 0 || s.genLost(k, sl) {
			continue // a Saga that lost all abilities has no chapters (CR 714.2d; gorge sagaChapters)
		}
		s.addLore(k, sl, 1)
	}
}

// sagaBusy reports whether the Saga at sl is the source of a chapter
// ability that has triggered but not yet left the stack (gorge checkSagas).
//
//go:noinline
func (s *State) sagaBusy(k *prep, sl Slot) bool {
	p := &s.perms[sl]
	for i := 0; i < int(s.npend); i++ {
		pt := &s.pend[i]
		if pt.obj == p.obj && isChapterAbility(k, pt.card, pt.ab) {
			return true
		}
	}
	for i := 0; i < int(s.sp); i++ {
		e := &s.stack[i]
		if e.obj == p.obj && isChapterAbility(k, e.card, e.ab) {
			return true
		}
	}
	return false
}

// sagaSBA is CR 714.4: a Saga with lore counters greater than or equal to
// its final chapter number, and with no chapter ability of it pending or on
// the stack, is sacrificed by its controller (gorge checkSagas). It reports
// whether anything was sacrificed, so the SBA loop runs another pass.
//
//go:noinline
func (s *State) sagaSBA(k *prep) bool {
	changed := false
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		var ord [MaxPerm]Slot
		n := s.battlefieldOrder(p, s.ctrlM[p], ord[:], 0)
		for _, sl := range ord[:n] {
			if !s.sagaDue(k, sl) {
				continue
			}
			s.fireSac(k, sl)
			s.leave(k, sl, kop.ZoneGraveyard, false)
			changed = true
		}
	}
	return changed
}

// sagaDue reports whether the Saga at sl is sacrificed by the state-based
// check now (sagaSBA's test).
func (s *State) sagaDue(k *prep, sl Slot) bool {
	final := k.cards[s.cardAt(sl)].sagaN
	return final != 0 && s.perms[sl].ctrN(s, int(kop.CtrLore)) >= int(final) && !s.genLost(k, sl) && !s.sagaBusy(k, sl) && !s.sacBlocked(k, sl)
}

// sagasDue is the battlefield's Sagas that sagaDue now (nothing without a
// Saga in the table).
//
//go:noinline
func (s *State) sagasDue(k *prep) (m slotMask) {
	if !k.anySaga {
		return m
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		var ord [MaxPerm]Slot
		n := s.battlefieldOrder(p, s.ctrlM[p], ord[:], 0)
		for _, sl := range ord[:n] {
			if s.sagaDue(k, sl) {
				m = m.with(sl)
			}
		}
	}
	return m
}
