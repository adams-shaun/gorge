package search

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Instant-timing windows (:instwin=). On the searching seat's OWN
// turn only, its instant-speed spell casts (an instant, or a card with
// flash) are allowed only inside the window; the opponent's turn, the
// opponent's nodes, lands, sorcery-speed casts and activations are never
// touched. 0 (InstWinOff) is the search exactly as without it.
const (
	InstWinOff           uint8 = iota
	InstWinPostBlock           // declare blockers through end of combat
	InstWinPostBlockResp       // ... or the stack holds an opponent object
	InstWinCombatResp          // beginning of combat through end of combat, or a response
	// InstWinTricks* are the diagnostic: postblock+resp applied only to the
	// combat tricks (InstantTrick cards: pump / protection instants).
	InstWinTricksResp
	// InstWinHold (bl17, the operator's rule) applies on BOTH turns: an
	// instant-speed cast is allowed only (a) in response to an opponent's
	// object on top of the stack, (b) on the opponent's turn after combat
	// (main 2, end step), or (c) for a combat trick, after blockers are
	// declared through end of combat on either turn.
	InstWinHold
	// InstWinReact applies on both turns: an
	// instant-speed cast into an empty stack is held back unless the
	// opponent did something this turn (reactTrack), or it is the card's
	// last window (the opponent's end step or later). The held casts are
	// listed at the root (Searcher.RootHeld) for the corpus record.
	InstWinReact
)

// InstWinNames are the :instwin= values, indexed by mode.
var InstWinNames = [...]string{"", "postblock", "postblock+resp", "combat+resp", "tricks+resp", "hold", "react"}

// InstantTrick reports a combat-trick instant (pump / protection) for
// InstWinTricksResp: set by the caller from a card-name list.
var InstantTrick func(c dirac.CardID, def *kop.CardDef) bool

// instBlocked reports whether a, the searching seat's priority answer at s,
// is an own-turn instant-speed cast outside the window.
func (sr *Searcher) instBlocked(s *dirac.State, a dirac.Action) bool {
	if sr.Opts.Defer > 0 && sr.deferBlocked(s, a) {
		return true // timing.go: :defer='s own upkeep/draw ban
	}
	mode := sr.Opts.InstWin
	if mode == InstWinOff || a.Kind() != dirac.AkCast || (mode != InstWinHold && mode != InstWinReact && s.ActivePlayer() != sr.seat) {
		return false
	}
	c := s.ObjCard(dirac.Obj(a.Source()))
	def := &s.Table().Cards[c]
	if def.Types&kop.TLand != 0 || (def.Types&kop.TInstant == 0 && def.Keywords&kop.KwFlash == 0) {
		return false
	}
	if mode == InstWinTricksResp && (InstantTrick == nil || !InstantTrick(c, def)) {
		return false
	}
	st := s.StepNow()
	if mode == InstWinReact {
		if sr.Opts.ReactRoot && !sr.rootDec {
			return false // :react without :reactin: the root only
		}
		if sr.Opts.ReactHorizon {
			return sr.reactHoldHorizon(s, a, st) // timing.go
		}
		return sr.reactHold(s, st)
	}
	if mode == InstWinHold {
		return !holdAllowed(sr, s, c, def, st)
	}
	switch mode {
	case InstWinCombatResp:
		if st >= kop.StepBeginCombat && st <= kop.StepEndCombat {
			return false
		}
	default:
		if st >= kop.StepBlockers && st <= kop.StepEndCombat {
			return false
		}
	}
	if mode != InstWinPostBlock {
		for i := 0; i < s.StackLen(); i++ {
			if _, _, _, ctrl := s.StackEntry(i); ctrl != sr.seat {
				return false
			}
		}
	}
	return true
}

// instBot replaces the seat's bot answer by pass when it is a blocked cast
// (the window applies to the seat's every own decision: the root, the
// in-tree points and the bot's playout answers).
func (sr *Searcher) instBot(s *dirac.State, d *dirac.Decision, bot int) int {
	if (sr.Opts.InstWin == InstWinOff && sr.Opts.Defer <= 0) || d.Kind != dirac.DkPriority || d.Player != sr.seat || !sr.instBlocked(s, d.Options[bot]) {
		if sr.Opts.ReactAim && d.Kind == dirac.DkTarget {
			return sr.aimBot(s, d, bot) // instkeys.go
		}
		return bot
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkPass {
			return i
		}
	}
	return bot
}

// holdAllowed is InstWinHold's window for an instant-speed cast of card c.
func holdAllowed(sr *Searcher, s *dirac.State, c dirac.CardID, def *kop.CardDef, st kop.Step) bool {
	if n := s.StackLen(); n > 0 {
		if _, _, _, ctrl := s.StackEntry(n - 1); ctrl != sr.seat {
			return true
		}
	}
	if s.ActivePlayer() != sr.seat && st > kop.StepEndCombat {
		return true
	}
	return st >= kop.StepBlockers && st <= kop.StepEndCombat && IsCombatTrick(def)
}

// IsCombatTrick derives a combat trick from the card IR: an instant-speed
// spell whose spell program only pumps, grants keywords or abilities, puts
// counters, prevents damage, regenerates or untaps, and does at least one
// of the first three (a pump with a constant negative toughness is a
// debuff) -- no damage, removal, tokens or card flow.
func IsCombatTrick(def *kop.CardDef) bool {
	if def.Types&kop.TCreature != 0 {
		return false
	}
	buff := false
	for i := range def.Abilities {
		ab := &def.Abilities[i]
		if ab.Kind != kop.AbSpell {
			continue
		}
		for _, in := range ab.Prog {
			switch in.Op {
			case kop.OpPumpUntilEOT:
				// a constant toughness cut is a debuff (Stab's -2/-2), not a trick
				if in.B.K == kop.VConst && in.B.N < 0 {
					return false
				}
				buff = true
			case kop.OpGrantKeyword, kop.OpPutCounter, kop.OpGrantAbility:
				buff = true
			case kop.OpNop, kop.OpPreventDamage, kop.OpRegenerate, kop.OpUntap, kop.OpChooseTargets,
				kop.OpSkipIf, kop.OpSkipUnless, kop.OpIfKicked, kop.OpBranch, kop.OpCharm:
			default:
				return false
			}
		}
	}
	return buff
}

// endFireNow reports the Options.EndFire window (Forge's use-it-or-lose-it):
// the searching seat holds priority at the opponent's end step with an
// empty stack, so its untapped mana is about to go unused, and one of its
// options casts an instant-speed removal or burn spell.
func (sr *Searcher) endFireNow(s *dirac.State, d *dirac.Decision) bool {
	if !sr.Opts.EndFire || d.Kind != dirac.DkPriority || d.Player != sr.seat ||
		s.ActivePlayer() == sr.seat || s.StepNow() != kop.StepEnd || s.StackLen() != 0 {
		return false
	}
	for i := 0; i < int(d.N); i++ {
		if removalCast(s, d.Options[i]) {
			return true
		}
	}
	return false
}

// endFireBot replaces a pass by the first removal cast inside the
// EndFire window (the bot's answers at the root and in playouts).
func (sr *Searcher) endFireBot(s *dirac.State, d *dirac.Decision, bot int) int {
	if !sr.endFireNow(s, d) || d.Options[bot].Kind() != dirac.AkPass {
		return bot
	}
	for i := 0; i < int(d.N); i++ {
		if removalCast(s, d.Options[i]) {
			return i
		}
	}
	return bot
}

// removalCast reports whether a casts an instant-speed removal or burn spell.
func removalCast(s *dirac.State, a dirac.Action) bool {
	if a.Kind() != dirac.AkCast {
		return false
	}
	def := &s.Table().Cards[s.ObjCard(dirac.Obj(a.Source()))]
	if def.Types&kop.TLand != 0 || def.Types&kop.TCreature != 0 ||
		(def.Types&kop.TInstant == 0 && def.Keywords&kop.KwFlash == 0) {
		return false
	}
	return IsRemoval(def)
}

// IsRemoval derives removal or burn from the card IR: the spell program
// deals damage, destroys, exiles, bounces, fights or bites, or pumps by a
// constant negative toughness (Stab's -2/-2).
func IsRemoval(def *kop.CardDef) bool {
	for i := range def.Abilities {
		ab := &def.Abilities[i]
		if ab.Kind != kop.AbSpell {
			continue
		}
		for _, in := range ab.Prog {
			switch in.Op {
			case kop.OpDealDamage, kop.OpDealDamageEx, kop.OpDealDamageDivided, kop.OpDestroy,
				kop.OpExile, kop.OpExileEx, kop.OpBounce, kop.OpFight, kop.OpBite, kop.OpDamageAll:
				return true
			case kop.OpPumpUntilEOT:
				if in.B.K == kop.VConst && in.B.N < 0 {
					return true
				}
			}
		}
	}
	return false
}

// reactSig counts the opponent's public footprint: permanents, +1/+1
// counters on them, and cards in their graveyard and exile (by owner).
type reactSig struct{ perm, ctr, gy, ex uint16 }

// reactTrack is the InstWinReact turn tracker of one seat: the opponent's
// baseline at the turn's first observed decision, and whether anything
// they did since has fired it. No previous-decision baseline (measured
// worse than the turn's) and the seat's own objects never count.
type reactTrack struct {
	turn  int32
	have  bool
	fired bool
	base  reactSig
}

func reactCount(s *dirac.State, seat uint8) (g reactSig) {
	for i, n := 0, s.NumObjs(); i < n; i++ {
		o := dirac.Obj(i)
		switch s.ObjZone(o) {
		case kop.ZoneBattlefield:
			if s.Controller(o) != seat {
				g.perm++
				g.ctr += uint16(s.Counters(o, kop.CtrP1P1))
			}
		case kop.ZoneGraveyard:
			if s.ObjOwner(o) != seat && !s.ObjToken(o) {
				g.gy++
			}
		case kop.ZoneExile:
			if s.ObjOwner(o) != seat && !s.ObjToken(o) {
				g.ex++
			}
		}
	}
	return g
}

// observe is called at every decision of the walk (either seat): a new
// turn resets the tracker and takes the baseline; the sticky events (a
// stack entry the opponent controls; a creature marked attacking or
// blocking, which exists only from the beginning of combat to its end)
// fire it at once. The count rises wait for fired.
func (t *reactTrack) observe(s *dirac.State, seat uint8) {
	if turn := int32(s.Turn()); !t.have || turn != t.turn {
		*t = reactTrack{turn: turn, have: true, base: reactCount(s, seat)}
	}
	if t.fired {
		return
	}
	for i, n := 0, s.StackLen(); i < n; i++ {
		if _, _, _, ctrl := s.StackEntry(i); ctrl != seat {
			t.fired = true
			return
		}
	}
	if st := s.StepNow(); st >= kop.StepBeginCombat && st <= kop.StepEndCombat {
		for i, n := 0, s.NumObjs(); i < n; i++ {
			o := dirac.Obj(i)
			if _, atk := s.HBotAttackTarget(o); atk || s.HBotBlocking(o) != dirac.NoObj {
				t.fired = true
				return
			}
		}
	}
}

// firedNow reports whether the opponent did something this turn, comparing
// the counts against the baseline lazily (only when a cast is about to be
// held).
func (t *reactTrack) firedNow(s *dirac.State, seat uint8) bool {
	if !t.have || t.turn != int32(s.Turn()) {
		t.observe(s, seat)
	}
	if t.fired {
		return true
	}
	c, b := reactCount(s, seat), t.base
	t.fired = c.perm > b.perm || c.ctr > b.ctr || c.gy > b.gy || c.ex > b.ex
	return t.fired
}

// ObserveReact feeds seat's turn tracker the decision s poses (a host calls
// it at every decision of either seat, so a seat that was auto-passed
// earlier in the turn never holds a turns-old baseline).
func (st *SeatState) ObserveReact(s *dirac.State, seat uint8) {
	st.react.observe(s, seat)
}

// reactHold reports whether the instant-speed cast at s is held by
// InstWinReact: an empty stack, not the card's last window, and the
// opponent has done nothing this turn.
func (sr *Searcher) reactHold(s *dirac.State, st kop.Step) bool {
	if s.StackLen() != 0 || (s.ActivePlayer() != sr.seat && st >= kop.StepEnd) {
		return false
	}
	if sr.curReact == nil {
		sr.curReact = &sr.rootReact
	}
	return !sr.curReact.firedNow(s, sr.seat)
}

// RootHeld is the last Search's root options (indices into the root
// decision's Options) InstWinReact pruned, at most MaxCands minus the
// root candidates. Valid until the next Search.
func (sr *Searcher) RootHeld() []uint8 { return sr.held[:sr.nHeld] }
