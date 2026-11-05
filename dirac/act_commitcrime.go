package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// CR 700.13. A player commits a crime when they cast a
// spell, activate an ability or put a triggered ability on the stack that
// targets an opponent, a permanent or spell an opponent controls, or a card
// in an opponent's graveyard. gorge (rules turn_ledgers.go
// recordTurnLedgers, trigmatch actions.go targetCommitsCrime and
// commitCrimeMatches) keeps one per-turn ledger, judged on each
// TargetsChosen event as it is applied, and fires "whenever you commit a
// crime" once per recorded stack object. State.g.crime holds the ledger
// (bit p: player p committed a crime this turn); every site below is
// guarded by genPrep.crimeOn, so a table without the mechanic pays nothing.
//
// Sites (dirac/cast.go checkWard's walk, same order as gorge's events):
//   - a spell's picks (S1: crimePick) set the bit at the pick, before
//     payment, as gorge records a spell's targets at the target answer; a
//     reversed cast keeps the bit and queues nothing;
//   - an ability or trigger target ask sets the bit when its picks complete
//     (a single-pick ask at the pick, a multi-pick window when it commits at
//     Done or Max: crimePick with done, crimeAskDone, the charm mask), the
//     moment gorge's TargetsChosen for that ask applies; TrigCommitCrime
//     still waits for placement (S2: crimeStep in checkWard), once per
//     recorded stack object;
//   - a copy's new targets (S3: crimeCopy) are a crime for the copy's
//     controller.
//
// Not a site: a mid-resolution target pick (OpChooseTargets,
// raGenResTarget) and the loop re-pointers: gorge's mid-resolution
// pre-ask (effects targets_ask.go) emits no TargetsChosen, so such a pick is
// not a crime there (a deviation from CR 601.2c, copied).

// CrimeStats counts the lifted mechanic; oracle tests only.
type CrimeStats struct {
	BySpell, ByAbility, ByTrigger int // the first criminal pick/target of each entry kind
	NonCrime                      int // an entry finished with targets, none criminal
	NonCrimeKind                  [3]int
	Fires                         int // TrigCommitCrime triggers queued
	MultiCrime                    int // entries with two or more criminal targets
	CountTrue, CountFalse         int // ExCommittedCrime arms
	GraveCrime, StackCrime        int // criminal targets in a graveyard / on the stack
	OppListener, GraveListener    int // queued for an opponent's listener / from a graveyard
	ModalAbility                  int // a modal ability with a criminal target
	ValiantCrime                  int // a crime trigger queued beside a trigger queued by the same targeting
	Reversed                      int // a cast reversed after a criminal pick
	ReversedQueued                int // ...with triggers pending (must stay 0: a reversed cast queues nothing)
	Parity                        int // ledger comparisons against gorge that disagreed (oracle)
}

// DebugCrimeStats is the test hook; nil in normal play.
var DebugCrimeStats *CrimeStats

// Mutation switches (oracle tests only); false in normal play.
var (
	DebugCrimeAnyTarget  bool // every target counts, your own included
	DebugCrimeEachTarget bool // fire once per criminal target, not once per object
	DebugCrimeNoReset    bool // the turn change keeps the ledger
	DebugCrimeModalTwice bool // a modal activation commits two crimes
)

// crimeKeep is the ledger carried over a turn change.
func crimeKeep(b uint8) uint8 {
	if DebugCrimeNoReset {
		return b
	}
	return 0
}

// CrimeLedger reports whether player p committed a crime this turn.
func (s *State) CrimeLedger(p uint8) bool { return s.g.crime>>p&1 != 0 }

// noteCrime is CR 700.13 for one target code t chosen by actor (gorge
// targetCommitsCrime): a player other than the actor, an object on the
// battlefield or stack whose controller is not the actor, a graveyard card
// whose owner is not. Exile, hand and library never count. A criminal
// target sets the ledger bit.
func (s *State) noteCrime(k *prep, actor, t uint8) bool {
	if !s.isCrime(actor, t) {
		return false
	}
	s.g.crime |= 1 << actor
	return true
}

// isCrime is noteCrime's verdict with no ledger write.
func (s *State) isCrime(actor, t uint8) bool {
	if t == NoTarget {
		return false
	}
	switch {
	case DebugCrimeAnyTarget:
		return true
	case t >= PlayerTarget0:
		return t-PlayerTarget0 != actor
	case isStackTarget(t):
		// an ability an opponent controls (CR 700.13a; tgt_stackability.go)
		i := int(t - StackTarget0)
		return i < int(s.sp) && s.stack[i].ctrl != actor
	}
	ob := &s.objs[t]
	switch ob.zone {
	case kop.ZoneBattlefield:
		return s.perms[ob.slot].ctrl != actor
	case kop.ZoneStack:
		return s.stackCtrl(Obj(t)) != actor
	case kop.ZoneGraveyard:
		return ob.owner() != actor
	}
	return false
}

// crimeModalPick is gorge's recordActivatedCharmTargets: a target
// a modal ACTIVATED ability's resolving mode asks for (modal_activated.go)
// is recorded as the ability's TargetsChosen, so a criminal one is a crime,
// once per ability (the first criminal target; later ones only append).
// e is the resolving entry before the pick c is stored in slot i.
//
//go:noinline
func (s *State) crimeModalPick(k *prep, e *stackEntry, i int, peekI uint8, c uint8) {
	if !s.noteCrime(k, e.ctrl, c) {
		return
	}
	for j := 0; j < int(e.nt); j++ {
		if (j != i || peekI > 0) && s.isCrime(e.ctrl, e.tgt[j]) {
			return
		}
	}
	if e.xi != 0 {
		x := s.xtOf(e)
		for j := 0; j < int(x.n); j++ {
			if s.isCrime(e.ctrl, x.code[j]) {
				return
			}
		}
	}
	cs := crimeSt{n0: int(s.npend)}
	s.crimeFire(k, e, &cs)
}

// crimePick is S1: one target ask's pick. A spell's pick is judged at the
// pick (gorge records a spell's targets at the target answer). An ability's
// ask is recorded with its TargetsChosen when the ask's picks complete --
// gorge commits a multi-pick window at Done or Max picks, so a pick that
// leaves the window open (done false) sets nothing; crimeAskDone judges the
// slot's whole recorded set at completion. Either way the bit only: gorge
// fires TrigCommitCrime once per recorded stack object, at placement
// (crimeStep/crimeEnd), and the TargetsChosen events say nothing yet. A
// build reversed after the ask keeps the bit: the TargetsChosen is an
// applied event there.
//
//go:noinline
func (s *State) crimePick(k *prep, t uint8, done bool) {
	e := s.bld().e
	if e.kind != seSpell {
		if done {
			s.noteCrime(k, e.ctrl, t)
		}
		return
	}
	if !s.noteCrime(k, e.ctrl, t) {
		return
	}
	if st := DebugCrimeStats; st != nil && s.bld().bfl&bflCrime == 0 {
		st.BySpell++
	}
	s.bld().bfl |= bflCrime
}

// crimeAskDone is an ability multi-pick ask's completion (finishSlot): the
// slot's recorded picks are judged there, where gorge's TargetsChosen events
// for the committed window apply. Bit only; TrigCommitCrime waits for
// placement (crimeStep). A slot whose picks left nothing recorded sets
// nothing, as an empty TargetsChosen emits nothing.
//
//go:noinline
func (s *State) crimeAskDone(k *prep) {
	e := s.bld().e
	mt := &s.g.mt
	if mt.picked == 0 {
		return
	}
	s.noteCrime(k, e.ctrl, e.tgt[mt.slot])
	if e.xi == 0 {
		return
	}
	x := s.xtOf(&e)
	for j := 0; j < int(x.n); j++ {
		if x.slot[j] == mt.slot {
			s.noteCrime(k, e.ctrl, x.code[j])
		}
	}
}

// crimeSt is checkWard's per-entry crime walk state.
type crimeSt struct {
	n0           int // npend when the walk began
	nT, nC       int // targets seen, criminal targets
	fired        bool
	first        bool
	copy         bool // a copy's new targets (S3): judged now whatever the build flags say
	graveN, stkN int
}

// crimeStep is S2: one target of the entry in checkWard's walk order, after
// that target's BecomesTarget / ward triggers (gorge emits one
// TargetsChosen per target; the trigger fires on the first criminal one).
//
//go:noinline
func (s *State) crimeStep(k *prep, e *stackEntry, cs *crimeSt, t uint8) {
	if t == NoTarget {
		return
	}
	cs.nT++
	if e.kind == seSpell && !cs.copy && s.bld().bfl&bflCrime == 0 {
		return // judged at the pick
	}
	if !s.noteCrime(k, e.ctrl, t) {
		return
	}
	cs.nC++
	if st := DebugCrimeStats; st != nil && isObjTarget(t) {
		switch s.objs[t].zone {
		case kop.ZoneGraveyard:
			st.GraveCrime++
		case kop.ZoneStack:
			st.StackCrime++
		}
	}
	if cs.fired && !DebugCrimeEachTarget {
		return
	}
	s.crimeFire(k, e, cs)
}

// crimeEnd closes the walk: a spell whose pick was a crime fires even if a
// payment changed the verdict; the stats classify the entry.
//
//go:noinline
func (s *State) crimeEnd(k *prep, e *stackEntry, cs *crimeSt) {
	if e.kind == seSpell && !cs.copy && s.bld().bfl&bflCrime != 0 && !cs.fired {
		s.crimeFire(k, e, cs)
	}
	st := DebugCrimeStats
	if st == nil {
		return
	}
	if cs.nC >= 2 {
		st.MultiCrime++
	}
	if cs.nT > 0 && !cs.fired {
		st.NonCrime++
		if e.kind <= seTrigger {
			st.NonCrimeKind[e.kind]++
		}
	}
}

func (s *State) crimeFire(k *prep, e *stackEntry, cs *crimeSt) {
	cs.fired = true
	if st := DebugCrimeStats; st != nil && !cs.first {
		cs.first = true
		switch e.kind {
		case seAbility:
			st.ByAbility++
			if e.mode&0x0f != 0 {
				st.ModalAbility++
			}
		case seTrigger:
			st.ByTrigger++
		}
	}
	before := s.npend
	s.fireCrime(k, e.ctrl)
	if DebugCrimeModalTwice && e.kind == seAbility && e.mode&0x0f != 0 {
		s.fireCrime(k, e.ctrl)
	}
	if st := DebugCrimeStats; st != nil && int(before) > cs.n0 && s.npend > before {
		st.ValiantCrime++
	}
}

// fireCrime queues the "whenever a player commits a crime" listeners for
// actor: battlefield listeners in gorge's walk order, then graveyard cards
// (TriggerZones$ Graveyard). No slot subscription.
//
//go:noinline
func (s *State) fireCrime(k *prep, actor uint8) {
	st := DebugCrimeStats
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigCommitCrime }, func(l trigListener, a *kop.Ability) {
			b := s.npend
			s.playerTrigGen(k, kop.TrigCommitCrime, l, a, actor, 0)
			if st != nil && s.npend > b {
				st.Fires++
				if l.ctrl != actor {
					st.OppListener++
				}
			}
		})
	}
	for _, c := range k.gen.graveTrig {
		cd := &k.t.Cards[c]
		for pp := uint8(0); pp < 2; pp++ {
			pl := &s.pl[pp]
			for i := 0; i < int(pl.graveN); i++ {
				o := pl.grave[i]
				if s.objs[o].card != c {
					continue
				}
				for j := range cd.Abilities {
					a := &cd.Abilities[j]
					if a.Kind != kop.AbTriggered || a.Trig.Zone != kop.ZoneGraveyard || a.Trig.Event != kop.TrigCommitCrime {
						continue
					}
					l := trigListener{obj: o, sl: NoSlot, ctrl: pp, card: c, ab: uint8(j)}
					b := s.npend
					s.playerTrigGen(k, kop.TrigCommitCrime, l, a, actor, 0)
					if st != nil && s.npend > b {
						st.Fires++
						st.GraveListener++
						if pp != actor {
							st.OppListener++
						}
					}
				}
			}
		}
	}
}

// crimeCopy is S3: a copy's new targets (gorge resolution.go records the
// copy's whole answer once, for the copy's controller).
//
//go:noinline
func (s *State) crimeCopy(k *prep, e *stackEntry) {
	cs := crimeSt{n0: int(s.npend), copy: true}
	for i := 0; i < int(e.nt); i++ {
		s.crimeStep(k, e, &cs, e.tgt[i])
		if e.xi != 0 {
			x := s.xtOf(e)
			for j := 0; j < int(x.n); j++ {
				if x.slot[j] == uint8(i) {
					s.crimeStep(k, e, &cs, x.code[j])
				}
			}
		}
	}
	s.crimeEnd(k, e, &cs)
}

// committedCrime evaluates ExCommittedCrime (gorge cov3BranchHolds
// CommittedCrimeThisTurn).
func (s *State) committedCrime(k *prep, x *kop.Expr, ctx evalCtx) int {
	if s.g.crime>>ctx.ctrl&1 != 0 {
		if st := DebugCrimeStats; st != nil {
			st.CountTrue++
		}
		return s.evalV(k, x.Sub[0], ctx)
	}
	if st := DebugCrimeStats; st != nil {
		st.CountFalse++
	}
	return s.evalV(k, x.Sub[1], ctx)
}
