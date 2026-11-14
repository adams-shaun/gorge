package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The as-enters opponent choice (CR 614.12): an enters program opening with
// OpExt ExtChoosePlayer (kgen lower_chooseplayerenter.go; The Rack, Black
// Vise). gorge runs the body through effChoosePlayer as the permanent enters:
// a KChoose of one "player" option per opponent of the controller, in
// AliveFrom(controller) order, with the permanent already on the battlefield
// (so the kernel queues the ask, never a pre-entry one on the stack). With a
// single opponent the oracle answers that ask in place (forcedPlayerChoice),
// so the kernel records it unasked. The answer is kept in the permanent's chosen byte (the seat; 255 = none),
// which RefChosenPlayer and WhoseChosen read.

// playerOptions fills the as-enters opponent ask: one option per opponent of
// the chooser (d.Player) in turn order after them, mode = the seat.
//
//go:noinline
func (s *State) playerOptions(d *Decision) {
	d.Sub = ChGenPlayer
	q := s.oppsFrom(uint8(d.Player))
	for i := uint8(0); i < q.n; i++ {
		d.add(MakeAction(AkChoose, 0, 0, q.s[i]))
	}
}

// soleOpponent is p's one living opponent when p has exactly one (gorge's
// single-option ChoosePlayer, answered without an ask), else 255.
func (s *State) soleOpponent(p uint8) uint8 {
	if q := s.oppsFrom(p); q.n == 1 {
		return q.s[0]
	}
	return 255
}

// The mid-resolution opponent choice (Skullwinder: a battlefield trigger
// whose DB chain is `ChoosePlayer | Defined$ You | Choices$ Player.Opponent`
// and whose later links read the choice, kgen lower_chooseplayer.go): gorge
// runs effChoosePlayer mid-resolution, an ask of one "player" option per
// opponent of the resolving controller in AliveFrom order, answered in place
// with a single opponent (forcedPlayerChoice) and recorded on the resolution
// (Ctx.Chosen, and on the source's Object.Chosen until the chain's
// ClearChosenPlayer$ Cleanup clears it). OpExt ExtChoosePlayerMid asks it
// (or records a sole opponent unasked) into res.chPl; RefChosenPlayer reads
// it while the resolution holds it, and a reader past the Cleanup finds
// none, as gorge's cleared choice is.

// raChoosePlayerMid: ExtChoosePlayerMid's resolution ask (ChGenPlayer, mode
// = the seat; interp_ask.go poses and applies it). +8: the offsets above
// raGenManifestDread are claimed by addition (+1..+7 modal/mill/dig/cmdzone/
// extort/libplace, +40.. act_mayask.go, ...), so the next free one is taken.
const raChoosePlayerMid = raGenManifestDread + 8

// choosePlayerMid is OpExt ExtChoosePlayerMid: false = the ask suspended the
// program (a sole opponent is recorded unasked; with none gorge records no
// choice, and a reader finds none).
func (s *State) choosePlayerMid(k *prep) bool {
	q := s.oppsFrom(s.res.e.ctrl)
	switch {
	case q.n == 1:
		s.res.chPl = q.s[0] + 1
		return true
	case q.n == 0:
		return true
	}
	s.res.ask, s.res.askP = raChoosePlayerMid, s.res.e.ctrl
	return false
}

// chosenPlayerMidOptions fills the mid-resolution ChoosePlayer ask: one
// option per opponent of the asker in AliveFrom order (gorge effChoosePlayer's
// "player" options), mode = the seat.
func (s *State) chosenPlayerMidOptions(d *Decision) {
	if s.res.ask != raChoosePlayerMid {
		panic("kernel: bad resolution ask")
	}
	d.Sub = ChGenPlayer
	q := s.oppsFrom(s.res.askP)
	for i := uint8(0); i < q.n; i++ {
		d.add(MakeAction(AkChoose, 0, 0, q.s[i]))
	}
}

// applyAskChosenPlayerMid keeps the seat for the resolution.
func (s *State) applyAskChosenPlayerMid(k *prep, a Action) {
	if s.res.ask != raChoosePlayerMid {
		panic("kernel: bad resolution answer")
	}
	s.res.chPl = a.Mode() + 1
	s.continueRes(k, 0)
}

// resChosenPlayer is the mid-resolution ChoosePlayer's seat (255 none): the
// one the live resolution chose.
func (s *State) resChosenPlayer() uint8 {
	if s.res.active && s.res.chPl != 0 {
		return s.res.chPl - 1
	}
	return 255
}

// chosenPlayerOf is the seat the permanent at sl chose as it entered (255 =
// none).
func (s *State) chosenPlayerOf(sl Slot) uint8 {
	if sl == NoSlot || !s.occ.has(sl) {
		return 255
	}
	if ch := s.perms[sl].chosen; ch < objWord(s.nSeats()) {
		return uint8(ch)
	}
	return 255
}

// chosenPlayerSeat is kop.RefChosenPlayer for entry e: the seat the live
// resolution chose mid-resolution (ExtChoosePlayerMid), else the seat e's
// source permanent chose as it entered, while the source is still that
// permanent on the battlefield (gorge reads the in-flight Ctx.Chosen, else
// the source's live Object.Chosen, which it clears as the permanent leaves,
// CR 400.7).
func (s *State) chosenPlayerSeat(e *stackEntry) uint8 {
	if p := s.resChosenPlayer(); p != 255 {
		return p
	}
	if e == nil || e.obj == NoObj {
		return 255
	}
	ob := &s.objs[e.obj]
	if ob.zone != kop.ZoneBattlefield || ob.ver != e.srcVer {
		return 255
	}
	return s.chosenPlayerOf(ob.slot)
}

// chosenIs is kop.WhoseChosen: the listener at sl chose player p as it
// entered.
func (s *State) chosenIs(sl Slot, p uint8) bool {
	ch := s.chosenPlayerOf(sl)
	return ch != 255 && ch == p
}

// StallChoosePlayerOrder: a card that chooses an opponent as it enters
// beside another object's enters replacement (kop.ReplOthersEnterProgram: Orb
// of Dreams' enters tapped, Grumgully's counter). gorge composes two Updated
// replacements over one entry by asking the CR 616.1 order unless every body
// commutes (rules/replacement_updated.go composeUpdatedReplacements), and its
// commute classification does not name a ChoosePlayer body
// (updatedNeutralBodySet), so it asks the order before the move. The kernel's
// entry-order ask cannot be posed on a resolving spell's route (StallEntryOrder),
// so such a pair is refused before its game (initStall).
const StallChoosePlayerOrder = 187

func init() { stallNames[StallChoosePlayerOrder] = "chooseplayerorder" }

// prepChoosePlayer marks a table with both halves of StallChoosePlayerOrder.
func prepChoosePlayer(t *kop.Table, g *genPrep) {
	g.chPlOrder = cardsWith(t, choosesPlayerEntering) && cardsWith(t, othersEnterRepl)
}

// cardsWith reports whether some card of t has pred.
func cardsWith(t *kop.Table, pred func(*kop.CardDef) bool) bool {
	for i := range t.Cards {
		if pred(&t.Cards[i]) {
			return true
		}
	}
	return false
}

// choosesPlayerEntering: c's enters program chooses an opponent.
func choosesPlayerEntering(c *kop.CardDef) bool {
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplEntersProgram && len(a.Prog) > 0 && a.Prog[0].Ext() == kop.ExtChoosePlayer {
			return true
		}
	}
	return false
}

// othersEnterRepl: c has an enters replacement over other objects.
func othersEnterRepl(c *kop.CardDef) bool {
	for j := range c.Abilities {
		if a := &c.Abilities[j]; a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplOthersEnterProgram {
			return true
		}
	}
	return false
}

// choosePlayerStallPair is StallChoosePlayerOrder's refusal text and halves.
func choosePlayerStallPair() (string, func(*kop.CardDef, int) bool, func(*kop.CardDef, int) bool) {
	return "an as-enters opponent choice beside another object's enters replacement: gorge asks the CR 616.1 order of the two, which the kernel does not pose on a resolving spell's entry",
		func(c *kop.CardDef, _ int) bool { return choosesPlayerEntering(c) },
		func(c *kop.CardDef, _ int) bool { return othersEnterRepl(c) }
}
