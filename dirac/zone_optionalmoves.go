package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// a known-origin ChangeZone's Optional$ ask (gorge
// effects/zone_change_optional.go changeZoneOptionalConfirm). gorge
// offers each object its move loop would move to the chooser as a yes/no
// (ChGenConfirm: mode 1 = yes) and a "no" drops that object only. The
// lowered shape is the source leaving the battlefield (Intimidation
// Campaign, Mistbreath Elder), so the ask is an OpMayAsk over the move with
// the MayGuardMove guard; cold, reached only from a table that names it.

// DebugOptMoveAlways makes the MayGuardMove guard fail, so the move runs
// unasked: the earlier gorge. Test-only (TestOptmovesMutation).
var DebugOptMoveAlways bool

// optMoveAsks reports whether gorge poses its Optional$ confirmation for
// the move op next: its Obj is a live battlefield object. Otherwise the
// object is the move loop's changeZoneMoveSkip case (the Origin$ precondition:
// the source left the battlefield, or returned as a new object), kept
// unasked and unmoved, so the move is skipped here (A instructions, as a
// "no" would); the plain move would pull a dead source back from the
// graveyard.
func (s *State) optMoveAsks(k *prep, in, next *kop.Instr) bool {
	if DebugOptMoveAlways {
		return false
	}
	if s.objRef(k, next.Obj) != NoSlot {
		return true
	}
	s.res.pc += uint8(s.value(k, in.A))
	return false
}

// optMoveRememberedAsks is optMoveAsks for the card the resolution remembered
// out of the library (Into the Wilds' looked-at top card): gorge's Defined$
// library fetch (moveDefinedLibraryObjects) poses its one confirmation whenever
// the remembered set names an object, and consumes an empty set unasked, so the
// move is skipped as a "no" would skip it. The Return's Origin$ check then
// stops an object that is no longer in the library.
func (s *State) optMoveRememberedAsks(k *prep, in, next *kop.Instr) bool {
	if s.objRefObj(next.Obj) != NoObj {
		return true
	}
	s.res.pc += uint8(s.value(k, in.A))
	return false
}

// An optional ChooseCard among the looked-at library cards (Thunderous
// Debut's "reveal up to two creature cards from among them"): the
// instruction has C = chooseLibUpTo, A = the window (the preceding Peek's
// amount, so the candidates are the matching cards among the top A of the
// chooser's library -- the remembered set gorge's IsRemembered filter reads,
// held here as a window because the remembered set caps at maxRem) and
// B = the most picks. gorge asks ONE KChoose with Min 0 and Max =
// min(Amount$, candidates); the kernel poses it as one pick at a time, with
// AkDone ending it early, and a pick is not offered again.
const chooseLibUpTo = 3

// StallOptMoves: more library picks than the chosen set holds (unreachable
// below the kgen cap of two; loud, never a silent spill).
const StallOptMoves = 97

func init() { stallNames[StallOptMoves] = "optmoves" }

// libWindowCands lists (into d, or just counts) the unchosen cards matching
// f among the top A cards of player p's library, top-down.
func (s *State) libWindowCands(k *prep, f kop.FilterID, p uint8, d *Decision) int {
	in := &s.prog(k)[s.res.pc]
	if in.C != chooseLibUpTo {
		return 0
	}
	pl := &s.pl[p]
	n, w := 0, min(s.value(k, in.A), pl.libLen())
	for i := 0; i < w; i++ {
		o := pl.lib[int(pl.libLo)+i]
		if s.inSet(&s.g.chosen, s.g.nchosen, o) || !s.matchCardGen(k, f, s.objs[o].card, p, p, o) {
			continue
		}
		n++
		if d != nil {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
	return n
}

// chooseLibAnswer applies one answer of the library ChooseCard: a pick is
// stored in chosen[nchosen] and the ask repeats until B picks are taken or
// no candidate is left; AkDone ends it at any point (Min 0).
func (s *State) chooseLibAnswer(k *prep, a Action) {
	in := &s.prog(k)[s.res.pc]
	if a.Kind() != AkDone {
		if int(s.g.nchosen) >= len(s.g.chosen) {
			s.capFull(StallOptMoves)
			return
		}
		s.g.chosen[s.g.nchosen] = Obj(a.Source())
		s.g.nchosen++
		if int(s.g.nchosen) < s.value(k, in.B) && s.libWindowCands(k, in.F, s.res.askP, nil) > 0 {
			return // ask again
		}
	}
	s.continueRes(k, 0)
}
