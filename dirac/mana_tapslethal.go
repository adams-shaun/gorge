package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// availNoTapsLethal: the census keeps a Taps-deferred source whose own tap
// reaches the payer's life (tapsDeferLethal) as an ordinary unit; set only by
// instantTapsLethalPays.
const availNoTapsLethal uint16 = 16384

// instantTapsLethalPays is the plain cast gate's answer for an instant-speed
// cast the census leaves unaffordable only because every Taps-deferred land's
// tap is lethal to the payer (Manabarbs at 1 life). gorge's planner never
// plans such a tap, but its prefix script (potential_script.go) taps the lands
// one by one and offers the cast: the damage triggers wait on the stack under
// the spell (CR 603.3, 117.1a), and only a sorcery-speed play, which needs the
// stack empty, resolves them first (playNeedsEmptyStack) and ends the payer
// (CR 119.4). A cast that resolves nothing first is offered whenever the
// census with those lands counted pays it.
func (s *State) instantTapsLethalPays(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost) bool {
	if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) == 0 || k.gen == nil || !k.gen.anyTapsDefer || cost.X != 0 || !s.subs[kop.TrigTaps].any() {
		return false
	}
	if !s.tapsDeferLethalMask(k, p, s.ctrlM[p]).any() || s.castNeedsEmptyStack(k, MakeAction(AkCast, TCode(o), 0, 0)) {
		return false
	}
	var av, fav avail
	saved := s.g.availX
	s.g.availX |= availNoTapsLethal
	s.availFor(k, p, &av)
	s.g.availX = saved
	cav := s.castGrantAvail(k, p, o, &av, &fav)
	return cav == &av && s.castGateOrPoolPays(k, p, c, o, cost, cav)
}
