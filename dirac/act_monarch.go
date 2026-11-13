package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The monarch (CR 725), copied from gorge (effects/game.go effBecomeMonarch,
// events foldMonarchChange and the TurnChange TurnStartMonarch snapshot,
// rules/turn.go's end-step draw, rules/combat.go's combat-damage steal,
// rules/trigmatch/monarch.go). Cold and out of line: a table with no
// ExtBecomeMonarch never takes these paths (genPrep.anyMonarch); without one
// nobody can become the monarch, so every read sees no monarch.
//
// State: genState.mon, one byte of genState's tail padding, so State does not
// grow: bits 0-3 the monarch's seat + 1 (0 nobody), bits 4-7 the seat that was
// the monarch as the current turn began + 1 (gorge TurnStartMonarch).
//
//   - 725.1/725.3: ExtBecomeMonarch makes its player the one holder. A player
//     who already is the monarch becomes nothing: the designation does not
//     move and "whenever a player becomes the monarch" does not fire (gorge
//     emits no MonarchChange). Otherwise TrigBecomeMonarch fires for the
//     battlefield listeners, the new monarch as the trigger's player.
//   - 725.2: at the beginning of the monarch's end step, after the step's
//     printed and delayed triggers, a source-less trigger the monarch
//     controls draws a card (gorge "__monarch_draw"); each creature whose
//     combat damage lands on the monarch queues a source-less trigger the
//     monarch controls, which makes that creature's controller (read as it
//     triggered) the monarch when it resolves (gorge "__monarch_steal",
//     MonarchTo). Both are abDelayed entries with obj NoObj and trigAmt
//     monDrawAmt / monStealAmt.
//   - 725.4 (more than two seats): when the monarch leaves the game, the
//     active player becomes the monarch, or the next player in turn order
//     still in the game when the active player is the one leaving (gorge
//     monarchHandoffOnDeparture, designationHeir); the move is the ordinary
//     one, so TrigBecomeMonarch fires. It runs at each loss site as the seat
//     is marked lost (monarchLeft), in gorge's loss order.

// DebugNoMonarchSteal (TestMonarchLockstepMutation): combat damage to the
// monarch queues no steal trigger.
var DebugNoMonarchSteal bool

// DebugNoMonarchHandoff (TestFourSeatMonarchHandoffLockstep): a departed
// monarch keeps the designation (CR 725.4 dropped).
var DebugNoMonarchHandoff bool

// MonarchStats counts the mechanic's events; oracle tests only.
type MonarchStats struct {
	Became    int // the designation moved (ExtBecomeMonarch or a steal)
	Repeat    int // a player who already was the monarch became it again: nothing happened
	Draws     int // end-step draw triggers queued
	Steals    int // combat-damage steal triggers queued
	StealMove int // a steal trigger's resolution moved the designation
	Fires     int // TrigBecomeMonarch triggers queued
	ExYes     int // ExMonarch read its Sub[0] arm
	ExNo      int // ExMonarch read its Sub[1] arm
	Handoffs  int // the monarch left the game and the designation passed on (CR 725.4)
}

// DebugMonarchStats is the test hook; nil in normal play.
var DebugMonarchStats *MonarchStats

const (
	monDrawAmt  = -3 // the abDelayed entry's trigAmt marking the end-step draw trigger (speed's is -1, the venture's -2)
	monStealAmt = -4 // ... the combat-damage steal trigger; trigPl is the player who becomes the monarch
)

// prepMonarch sets anyMonarch: some program makes a player the monarch.
func prepMonarch(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && in.Ext() == kop.ExtBecomeMonarch {
					g.anyMonarch = true
				}
			}
		}
	}
}

// Monarch is the monarch's seat, -1 when there is no monarch (oracle).
func (s *State) Monarch() int8 { return int8(s.g.mon&15) - 1 }

// becomeMonarch is gorge's MonarchChange: p becomes the monarch, and the
// designation's "whenever a player becomes the monarch" triggers fire.
//
//go:noinline
func (s *State) becomeMonarch(k *prep, p uint8) {
	if p >= s.nSeats() {
		return
	}
	st := DebugMonarchStats
	if s.g.mon&15 == p+1 {
		if st != nil {
			st.Repeat++
		}
		return
	}
	s.g.mon = s.g.mon&0xf0 | (p + 1)
	if st != nil {
		st.Became++
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigBecomeMonarch }, func(l trigListener, a *kop.Ability) {
			if a.Trig.N == kop.TrigNMonarchBeganTurn && s.g.mon>>4 != l.ctrl+1 {
				return // BeginTurn$ You: "if you were the monarch as the turn began"
			}
			np := s.npend
			s.playerTrigGen(k, kop.TrigBecomeMonarch, l, a, p, 0)
			if st != nil && s.npend > np {
				st.Fires++
			}
		})
	}
}

// becomeMonarchOp is ExtBecomeMonarch: the player Obj becomes the monarch.
func (s *State) becomeMonarchOp(k *prep, in *kop.Instr) {
	if p := s.playerRef(k, in.Obj); p != 255 {
		s.becomeMonarch(k, p)
	}
}

// monarchLeft runs as seat p is marked lost, at every loss site (sba's life
// and empty-library loops, poisonSBA, cmdDamageSBA, lose). Those are gorge's
// loss order (checkLoseConditions: life, then an empty library, then poison,
// then commander damage, each in seat order), and gorge's losePlayer hands the
// designation off before the batch's next loss is made, so a seat that loses
// later in the same check is still in the game when the heir is chosen. At two
// seats a loss ends the game and the call folds away.
func (s *State) monarchLeft(k *prep, p uint8) {
	if MaxSeats > 2 && k.gen != nil && k.gen.anyMonarch && s.g.mon&15 == p+1 {
		s.monarchHandoff(k, p)
	}
}

// designationHeir is who inherits a designation from departing seat p: the
// active player, else the next player in turn order still in the game (gorge
// designationHeir). ok is false when no other seat is left; the designation
// stays rather than naming a departed seat.
func (s *State) designationHeir(p uint8) (uint8, bool) {
	h := s.active
	if h == p || s.out(h) {
		h = s.nextAlive(p)
	}
	return h, h != p && !s.out(h)
}

// monarchHandoff is CR 725.4 for the departed monarch p: the active player
// becomes the monarch, else the next player in turn order still in the game
// (gorge designationHeir); with nobody else left the designation stays.
//
//go:noinline
func (s *State) monarchHandoff(k *prep, p uint8) {
	if DebugNoMonarchHandoff {
		return
	}
	h, ok := s.designationHeir(p)
	if !ok {
		return
	}
	if st := DebugMonarchStats; st != nil {
		st.Handoffs++
	}
	s.becomeMonarch(k, h)
}

// monarchTurn is the TurnChange fold's snapshot: the monarch as this turn
// begins.
func (s *State) monarchTurn() {
	m := s.g.mon & 15
	s.g.mon = m | m<<4
}

// monarchEndStep queues the monarch's end-step draw trigger (CR 725.2; gorge
// turn.go, after the step's printed and delayed triggers).
//
//go:noinline
func (s *State) monarchEndStep() {
	if s.g.mon&15 != s.active+1 || s.pl[SeatIx(s.active)].lost {
		return
	}
	s.enqueue(NoObj, 0, 0, abDelayed, s.active, NoObj, s.active, monDrawAmt)
	if st := DebugMonarchStats; st != nil {
		st.Draws++
	}
}

// monarchCombat is CR 725.2's steal after a combat damage pass (gorge
// runCombatAssignments' player branch): each creature whose damage landed on
// the monarch queues one trigger the monarch controls, the creature's
// controller the player it makes the monarch.
//
//go:noinline
func (s *State) monarchCombat(r *dmgRec) {
	m := s.g.mon & 15
	if m == 0 || DebugNoMonarchSteal {
		return
	}
	mon := m - 1
	if s.pl[SeatIx(mon)].lost {
		return
	}
	for i := 0; i < r.nev; i++ {
		ev := &r.ev[i]
		if ev.dst == NoSlot && ev.n > 0 && s.perms[ev.src].atk == PlayerCode(mon) {
			s.enqueue(NoObj, 0, 0, abDelayed, mon, NoObj, s.perms[ev.src].ctrl, monStealAmt)
			if st := DebugMonarchStats; st != nil {
				st.Steals++
			}
		}
	}
}

// monarchTrig reports whether e is one of the monarch's source-less triggers.
func monarchTrig(e *stackEntry) bool {
	return e.obj == NoObj && (e.trigAmt == monDrawAmt || e.trigAmt == monStealAmt)
}

// resolveMonarchTrig resolves the end-step draw (its controller draws a card:
// gorge's synthetic Draw Defined$ You NumCards$ 1) or the steal (the player it
// recorded becomes the monarch: gorge's BecomeMonarch Defined$ Remembered).
//
//go:noinline
func (s *State) resolveMonarchTrig(k *prep, e *stackEntry) {
	if e.trigAmt == monDrawAmt {
		if !s.pl[SeatIx(e.ctrl)].lost {
			s.draw(e.ctrl)
		}
		return
	}
	moved := s.g.mon&15 != e.trigPl+1
	s.becomeMonarch(k, e.trigPl)
	if st := DebugMonarchStats; st != nil && moved && s.g.mon&15 == e.trigPl+1 {
		st.StealMove++
	}
}

// exMonarch evaluates ExMonarch: Sub[0] when the controller (N 0) or an
// opponent of the controller still in the game (N MonarchOpp) is the monarch.
func (s *State) exMonarch(k *prep, x *kop.Expr, ctx evalCtx) int {
	m := s.g.mon & 15
	var holds bool
	if x.N == kop.MonarchOpp {
		holds = m != 0 && m-1 != ctx.ctrl && !s.out(m-1)
	} else {
		holds = m == ctx.ctrl+1
	}
	st := DebugMonarchStats
	if holds {
		if st != nil {
			st.ExYes++
		}
		return s.evalV(k, x.Sub[0], ctx)
	}
	if st != nil {
		st.ExNo++
	}
	return s.evalV(k, x.Sub[1], ctx)
}
