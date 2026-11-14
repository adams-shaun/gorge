package search

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Bot answers the decisions the tree does not search (azmcts: gorge's
// auto-pay heuristic bot, bots/heuristic + bots/botpolicy, which has no
// dirac port yet) and supplies candidate 0 (the bot's answer) at every
// searched point. Pick returns an index into d.Options[:d.N].
type Bot interface {
	Pick(s *dirac.State, d *dirac.Decision) int
}

// PureBot is a Bot whose Pick is a pure function of its arguments: no RNG,
// no state carried between calls, no side effects. advance's lazyPoint
// shortcut may skip such a bot's Pick when the candidate set does not
// depend on the answer; that is only exact for a pure bot. A stateful bot
// (oracle.SBT: it mutates its decision count, sequence and RNG) must not
// implement this marker, so the shortcut never skips its calls.
type PureBot interface {
	Bot
	PurePick()
}

// Heuristic is dirac's simple stand-in for gorge's heuristic bot:
// deterministic, allocation-free, a few lines per decision kind. It is
// pure (PureBot): Pick reads only the state and decision it is given.
type Heuristic struct{}

func (Heuristic) PurePick() {}

func mv(t *kop.Table, c dirac.CardID) int { return t.Cards[c].Cost.MV() }

// beneficial reports whether an ability's program helps its target (pump,
// counters, keywords, untap, gain life): the bot then targets its own side.
func beneficial(t *kop.Table, c dirac.CardID, ab uint8) bool {
	cd := &t.Cards[c]
	if int(ab) >= len(cd.Abilities) {
		return cd.Types&kop.TAura != 0 && false
	}
	for _, in := range cd.Abilities[ab].Prog {
		switch in.Op {
		case kop.OpPumpUntilEOT, kop.OpPutCounter, kop.OpGrantKeyword, kop.OpUntap, kop.OpGainLife, kop.OpRegenerate:
			return true
		case kop.OpDealDamage, kop.OpDestroy, kop.OpExile, kop.OpBounce, kop.OpTap, kop.OpFight, kop.OpCounter, kop.OpDiscard, kop.OpLoseLife:
			return false
		}
	}
	return false
}

func (Heuristic) Pick(s *dirac.State, d *dirac.Decision) int {
	t := s.Table()
	p := d.Player
	switch d.Kind {
	case dirac.DkPriority:
		best, bestMV := -1, -1
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			switch a.Kind() {
			case dirac.AkPlayLand:
				return i
			case dirac.AkCast, dirac.AkCastCasualty, dirac.AkCastConspire, dirac.AkActivate:
				m := 0
				if a.Kind() == dirac.AkCast || a.Kind() == dirac.AkCastCasualty || a.Kind() == dirac.AkCastConspire {
					m = mv(t, s.ObjCard(dirac.Obj(a.Source())))
					if a.Mode()&1 != 0 {
						m += 10 // prefer the kicked cast
					}
				}
				if m > bestMV {
					best, bestMV = i, m
				}
			}
		}
		if best >= 0 && (p == s.ActivePlayer() || bestMV >= 0) {
			return best
		}
		return passIndex(d)
	case dirac.DkAttackers:
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			if a.Kind() != dirac.AkAttack || dirac.IsObjTarget(a.Target()) {
				continue
			}
			def := 1 - p
			if s.NumSeats() > 2 {
				def = s.FeatAttackDefender(a.Target()) // the option names the attacked player (CR 802.4a)
			}
			if safeAttack(s, dirac.Obj(a.Source()), def) {
				return i
			}
		}
		// A declaration with a creature that must attack has no Done option
		// until the requirement is met: take the first option then, as the
		// blockers case does.
		if i := doneIndex(d); i >= 0 {
			return i
		}
		return 0
	case dirac.DkBlockers:
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			if a.Kind() != dirac.AkBlock {
				continue
			}
			bp, bt := s.PowerToughness(dirac.Obj(a.Source()))
			ap, at := s.PowerToughness(dirac.Obj(a.Target()))
			if bt > ap || (bp >= at && mv(t, s.ObjCard(dirac.Obj(a.Source()))) <= mv(t, s.ObjCard(dirac.Obj(a.Target())))) {
				return i
			}
		}
		if i := doneIndex(d); i >= 0 {
			return i
		}
		return 0
	case dirac.DkTarget:
		card, ab, ctrl, ok := s.Building()
		own := ok && beneficial(t, card, ab)
		if !ok {
			ctrl = p
		}
		best, bestV := 0, -1<<30
		for i := 0; i < int(d.N); i++ {
			tg := d.Options[i].Target()
			v := 0
			switch {
			case tg == dirac.NoTarget:
				v = -1 << 20
			case tg >= dirac.PlayerTarget0:
				mine := tg-dirac.PlayerTarget0 == dirac.TCode(ctrl)
				if mine == own {
					v = 1
				} else {
					v = -1000
				}
			default:
				o := dirac.Obj(tg)
				mine := s.Controller(o) == ctrl
				pw, _ := s.PowerToughness(o)
				v = 10 + pw*10 + mv(t, s.ObjCard(o))
				if mine != own {
					v -= 10000
				}
			}
			if v > bestV {
				best, bestV = i, v
			}
		}
		return best
	case dirac.DkChoose:
		switch d.Sub {
		case dirac.ChX, dirac.ChTrigX, dirac.ChMultikick:
			return int(d.N) - 1
		case dirac.ChCostDiscard, dirac.ChCostSacrifice, dirac.ChDiscard, dirac.ChSacrifice, dirac.ChCleanupDiscard, dirac.ChBottom:
			best, bestV := 0, 1<<30
			for i := 0; i < int(d.N); i++ {
				o := dirac.Obj(d.Options[i].Source())
				v := mv(t, s.ObjCard(o))
				if v < bestV {
					best, bestV = i, v
				}
			}
			return best
		case dirac.ChOptional, dirac.ChWardPay, dirac.ChOptDiscMay:
			for i := 0; i < int(d.N); i++ {
				if d.Options[i].Mode() == 1 {
					return i
				}
			}
		}
		return 0
	}
	return 0
}

func passIndex(d *dirac.Decision) int {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkPass {
			return i
		}
	}
	return 0
}

func doneIndex(d *dirac.Decision) int {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkDone {
			return i
		}
	}
	return -1
}

// safeAttack: no untapped creature of opp could block o and kill it.
func safeAttack(s *dirac.State, o dirac.Obj, opp uint8) bool {
	_, t := s.PowerToughness(o)
	for i := 0; i < s.NumObjs(); i++ {
		b := dirac.Obj(i)
		if s.ObjZone(b) != kop.ZoneBattlefield || s.Controller(b) != opp || !s.IsCreature(b) || s.Tapped(b) {
			continue
		}
		if bp, _ := s.PowerToughness(b); bp >= t {
			return false
		}
	}
	return true
}
