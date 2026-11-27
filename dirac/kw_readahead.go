package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// kw:Read ahead (CR 702.155 / 714.3b; gorge engine/rules/readahead.go): "As
// this Saga enters, choose a number between one and this Saga's final chapter
// number. This Saga enters with the chosen number of lore counters on it", and
// (702.155a) its chapter abilities can't trigger the turn it entered unless it
// has exactly the number of lore counters its chapter symbol names.
//
// kgen lowers the keyword to the as-enters number ask (OpExt ExtChooseNumber,
// 1..the final chapter) on a Saga face, which no other card prints: the table
// is marked by that shape (readAheadCard, read only where a Saga enters and
// where a lore count crosses a chapter: cold paths). The ask is the number
// ask's whole path (repl_choosenumber.go: pre-entry on the stack for a cast Saga, queued for any
// other entry, defaulted when no run serves the entry); only its options carry
// their own kind (ChGenReadAhead, gorge's "readahead" KChoose, the chapter on
// Amount) and the answer is a lore count rather than a stored number:
//
//   - the entry places no lore counter of its own (zones.go skips the one a
//     Saga enters with; gorge's EntryCounterGrantsForFace grants the chosen
//     number instead of 1);
//   - the answer puts the chosen number of lore counters through the entry
//     settlement (placeEntryCounters, CR 614.12). A pre-entry or defaulted
//     answer lands inside the entry, where the entry's counter notice fires
//     the chapters as for any entry counter; a queued answer is after the
//     entry, so it fires that notice itself;
//   - chapterCrossed gates the entry turn to the chapter equal to the count.

// readAheadCard reports whether card c is a read-ahead Saga: a Saga whose
// enters program opens with the number ask (kgen's only source of one).
func readAheadCard(c *kop.CardDef) bool {
	if c.SagaN == 0 {
		return false
	}
	for i := range c.Abilities {
		a := &c.Abilities[i]
		if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplEntersProgram && len(a.Prog) > 0 &&
			a.Prog[0].Op == kop.OpExt && a.Prog[0].Ext() == kop.ExtChooseNumber {
			return true
		}
	}
	return false
}

// readAheadPlace puts n lore counters on the read-ahead Saga at sl as its
// entry counters (CR 702.155b). fire is set for an answer given after the
// entry, which must itself report the counters to the chapter triggers the
// entry would have (the held-counter notice of enter); an answer inside the
// entry leaves that notice to enter.
func (s *State) readAheadPlace(k *prep, sl Slot, n int, fire bool) {
	if n < 1 {
		return
	}
	p := &s.perms[sl]
	n = s.placeEntryCounters(k, sl, int(kop.CtrLore), n, p.ctrl)
	if n <= 0 || !fire || !s.occ.has(sl) {
		return
	}
	if s.evLive(kop.TrigCounterAddedOnce) {
		s.g.lastCK = kop.CtrLore
		s.g.ctrEntry = true
		s.fireObj(k, kop.TrigCounterAddedOnce, sl, p.obj, p.ctrl, int16(n))
		s.g.ctrEntry = false
	}
}

// readAheadAnswer applies the answer of a queued number ask of a read-ahead
// Saga (a no-op for any other card): the chosen number of lore counters.
func (s *State) readAheadAnswer(k *prep, ask askRec, a Action) {
	if ask.card == NoCard || ask.sl == NoSlot || !readAheadCard(&k.t.Cards[ask.card]) {
		return
	}
	s.readAheadPlace(k, ask.sl, int(a.Mode()), true)
}

// readAheadOptions fills the ask: one option per chapter 1..final, ascending
// as gorge lists them, each over the entering Saga.
func (s *State) readAheadOptions(d *Decision, o Obj, lo, hi int) {
	d.Sub = ChGenReadAhead
	for n := lo; n <= hi; n++ {
		d.add(MakeAction(AkChoose, TCode(o), 0, uint8(n)))
	}
}

// DebugReadAheadNoGate (TestReadAheadMutation): set, a read-ahead Saga's entry
// turn triggers every chapter its lore count crossed, as any Saga's does.
var DebugReadAheadNoGate bool

// readAheadEntryTurn reports whether the Saga at sl is a read-ahead Saga on
// the turn it entered (CR 702.155a; gorge saga.go's EnteredThisTurn test).
func (s *State) readAheadEntryTurn(k *prep, sl Slot) bool {
	return !DebugReadAheadNoGate && readAheadCard(&k.t.Cards[s.cardAt(sl)]) && s.g.zoneTurn.has(s.perms[sl].obj)
}
