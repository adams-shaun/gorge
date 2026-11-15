package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Dredge N (CR 702.52, kop.KwDredge; CardDef.Dredge = N): "If you would draw
// a card, you may mill N cards instead. If you do, return this card from your
// graveyard to your hand." Only with at least N cards in the library
// (702.52b). gorge (engine/effects cardflow.go drawFor) asks before every
// draw whose player has such a card in their graveyard: a KModes decision
// whose options are each candidate in graveyard order ("dredge", the card),
// then the ordinary draw ("draw"); a multi-card draw asks again before each
// card. The pick mills N (each card an ordinary library-to-graveyard move)
// and returns the card to its owner's hand (rules applyDredge); no card is
// drawn, so nothing counts as a draw. A decline is the ordinary draw, which
// then meets the draw replacements (repl_draw.go). The ask comes before a
// CantDraw static is read, as gorge's does.
//
// The kernel poses the same decision (DkChoose ChGenBranch, mode i = the
// i-th candidate, the last mode the draw: canonical as gorge's KModes
// index) at the turn draw (dredgeTurnAsk, ctlGenAsk) and at each card of an
// OpDraw (dredgeDraws, the resolution ask raDredge). Any other draw path
// (connive, recruit, the monarch's draw, a draw replacement's body, a draw
// cost) with a candidate stalls (StallDredge) rather than draw unasked.

// kwDredge is the manifest keyword CardDef.Dredge stands for.
const kwDredge = kop.KwDredge

// StallDredge: a draw the kernel cannot suspend on (not the turn draw nor an
// OpDraw card) meets a dredge candidate: gorge would ask.
const StallDredge = 178

func init() { stallNames[StallDredge] = "dredge" }

// raDredge is OpDraw's dredge ask: res.askP (= s.g.pickP) draws, res.left
// cards are left for them counting the asked one, s.g.pickLeft is the count
// per player and s.g.pickZ the players still to draw (seatsPack).
const raDredge = raGenManifestDread + 86

// cfgxDredgeAsked is transient, never a Config bit: the draw about to run is
// an answered ask's ordinary draw, so drawExcl's unasked-candidate stall
// (dredgeHeld) lets it through once.
const cfgxDredgeAsked uint8 = 1 << 4

// useDredge: the table has a card with printed Dredge.
var useDredge = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		if t.Cards[i].Dredge != 0 {
			return true
		}
	}
	return false
})

// dredgeMaxCands bounds one ask's candidates (the mode byte, the draw last).
const dredgeMaxCands = 255

// dredgeOn reports whether the table has a dredge card.
func dredgeOn(k *prep) bool { return k.gen != nil && k.gen.uses.has(useDredge) }

// dredgeCands lists player p's graveyard cards with Dredge N at most their
// library size, in graveyard order (gorge dredgeCandidates), into buf (nil:
// only counted).
func (s *State) dredgeCands(k *prep, p uint8, buf *[dredgeMaxCands]Obj) int {
	pl := &s.pl[p]
	lib := pl.libLen()
	n := 0
	for _, o := range pl.grave[:pl.graveN] {
		d := int(k.t.Cards[s.objs[o].card].Dredge)
		if d == 0 || d > lib || n == dredgeMaxCands {
			continue
		}
		if buf != nil {
			buf[n] = o
		}
		n++
	}
	return n
}

// dredgeOptions poses the ask to p: one option per candidate (mode = its
// index), then the ordinary draw (mode = the candidate count).
func (s *State) dredgeOptions(k *prep, p uint8, d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChGenBranch, p
	n := s.dredgeCands(k, p, nil)
	for i := 0; i <= n; i++ {
		d.add(MakeAction(AkChoose, 0, 0, uint8(i)))
	}
}

// dredgeTake applies answer mode to p's asked draw: a candidate's index
// mills its N and returns it to hand (gorge applyDredge), true; the draw's
// mode returns false, the caller then draws.
func (s *State) dredgeTake(k *prep, p uint8, mode uint8) bool {
	var buf [dredgeMaxCands]Obj
	if int(mode) >= s.dredgeCands(k, p, &buf) {
		return false
	}
	o := buf[mode]
	n := int(k.t.Cards[s.objs[o].card].Dredge)
	pl := &s.pl[p]
	switch {
	case k.gen.anyRepl&rbGraveLib != 0:
		s.millShuffling(k, p, n, 0)
	case len(s.cold.parked) != 0 && s.parkMills(k, p, n, 0):
	default:
		for i := 0; i < n && pl.libLo < pl.libHi; i++ {
			m := pl.lib[pl.libLo]
			pl.libLo++
			s.touch(m)
			s.objs[m].zone = kop.ZoneNone
			s.placeFromK(k, m, kop.ZoneGraveyard, false, kop.ZoneLibrary, false)
		}
	}
	if z := s.objs[o].zone; z != kop.ZoneHand && z != kop.ZoneNone && z != kop.ZoneBattlefield {
		from := s.detach(o)
		s.placeFromK(k, o, kop.ZoneHand, false, from, false)
	}
	return true
}

// dredgeHeld is drawExcl's guard: a draw that no ask was posed for, by a
// player with a dredge candidate, stalls (true: the draw is not made). The
// ordinary draw of an answered ask passes once.
func (s *State) dredgeHeld(p uint8) bool {
	if s.cfgx&cfgxDredgeAsked != 0 {
		s.cfgx &^= cfgxDredgeAsked
		return false
	}
	k := s.k()
	if !dredgeOn(k) || s.dredgeCands(k, p, nil) == 0 {
		return false
	}
	s.capFull(StallDredge)
	return true
}

// --- the turn draw (CR 504.1) ---

// dredgeTurnAsk suspends the draw step on the active player's dredge ask,
// before the turn draw; false: nothing to ask, the step draws as usual.
// The ask is known by genState.drawFirst, true only from the step's start to
// its draw (dredgeTurnAsked).
func (s *State) dredgeTurnAsk(k *prep) bool {
	if !dredgeOn(k) || s.dredgeCands(k, s.active, nil) == 0 {
		return false
	}
	s.g.ctlRet, s.ctl = ctlStepBegin, ctlGenAsk
	return true
}

// dredgeTurnAsked reports whether the pending generated ask is the turn
// draw's dredge ask.
func (s *State) dredgeTurnAsked() bool {
	return s.g.drawFirst && s.step == kop.StepDraw && s.ctl == ctlGenAsk
}

// dredgeTurnAnswer applies the turn draw's answer and finishes the draw step
// as beginStep does after its draw.
func (s *State) dredgeTurnAnswer(k *prep, a Action) {
	s.ctl = ctlStepBegin
	if !s.dredgeTake(k, s.active, a.Mode()) {
		s.cfgx |= cfgxDredgeAsked
		s.draw(s.active)
		s.cfgx &^= cfgxDredgeAsked
	}
	s.g.drawFirst = false
	if s.over {
		return
	}
	s.firePhase(k, kop.StepDraw)
	s.openPriority()
}

// --- OpDraw ---

// drawOp is OpDraw: in.A cards for each player in.Obj names; in a dredge
// table each card asks first (dredgeDraws). false: an ask is posed.
func (s *State) drawOp(k *prep, in *kop.Instr) bool {
	if dredgeOn(k) {
		return s.dredgeDraws(k, in)
	}
	q := s.players(k, in.Obj)
	n := s.value(k, in.A)
	for i := uint8(0); i < q.n; i++ {
		for j := 0; j < n && !s.over; j++ {
			s.dredgeDrawOne(in, q.s[i])
		}
	}
	return true
}

// dredgeDraws is OpDraw in a dredge table: in.A cards for each player in.Obj
// names, each card asked first when its player has a candidate; false: an
// ask is posed.
func (s *State) dredgeDraws(k *prep, in *kop.Instr) bool {
	q := s.players(k, in.Obj)
	if q.n == 0 {
		return true
	}
	n := min(max(s.value(k, in.A), 0), 255)
	return s.dredgeDrawLoop(k, in, q.s[0], n, seatsPack(q, 1), n)
}

// dredgeDrawLoop draws left cards for p, then n for each player of the
// chain rest, asking before each card that has a candidate.
func (s *State) dredgeDrawLoop(k *prep, in *kop.Instr, p uint8, left int, rest uint8, n int) bool {
	for {
		for ; left > 0 && !s.over; left-- {
			if s.dredgeCands(k, p, nil) != 0 {
				s.g.pickP, s.g.pickZ, s.g.pickLeft = p, rest, uint8(n)
				s.res.left = uint8(left)
				s.res.ask, s.res.askP = raDredge, p
				return false
			}
			s.dredgeDrawOne(in, p)
		}
		if s.over {
			return true
		}
		if p = seatsPop(&rest); p == 255 {
			return true
		}
		left = n
	}
}

// dredgeDrawOne is one card of OpDraw: RememberDrawn$ draws remember
// (zone_rememberwrites.go).
func (s *State) dredgeDrawOne(in *kop.Instr, p uint8) {
	if in.C&kop.MvRemember != 0 {
		s.drawRemember(p, in.C&kop.DrawRememberAll != 0)
		return
	}
	s.draw(p)
}

// dredgeAskOptions poses OpDraw's dredge ask.
//
//kmanifest:askopts raDredge
func (s *State) dredgeAskOptions(k *prep, d *Decision) {
	s.dredgeOptions(k, s.res.askP, d)
}

// dredgeAskAnswer applies OpDraw's dredge answer and draws on.
//
//kmanifest:askapply raDredge
func (s *State) dredgeAskAnswer(k *prep, a Action) {
	s.res.ask = raNone
	p := s.g.pickP
	in := &s.prog(k)[s.res.pc]
	if !s.dredgeTake(k, p, a.Mode()) {
		s.cfgx |= cfgxDredgeAsked
		s.dredgeDrawOne(in, p)
		s.cfgx &^= cfgxDredgeAsked
	}
	if s.dredgeDrawLoop(k, in, p, int(s.res.left)-1, s.g.pickZ, int(s.g.pickLeft)) {
		s.continueRes(k, 0)
	}
}
