package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugAsIfHasteOff is a mutation switch: a StatAsIfHaste static exempts
// nothing, so a summoning-sick creature under Shang-Chi, Master of Kung Fu
// can't pay a {T}/{Q} cost again and the lockstep repro diverges.
var DebugAsIfHasteOff bool

// activationSick is CR 302.6's activation half, the one kernel predicate
// for a {T} or {Q} cost of the permanent at sl itself (gorge
// pay.TapFlagsSick): a creature that came under its controller's control
// since their most recent turn began, without haste, unless a
// StatAsIfHaste static (S:Mode$ ActivateAbilityAsIfHaste, gorge
// rules/activateasifhaste.go) selects it. Combat (canAttack) keeps its own
// check: the exemption is not a haste grant. Tapping another permanent to
// pay a cost (crew, convoke, waterbend) reads no sickness at all.
func (s *State) activationSick(k *prep, sl Slot) bool {
	if s.perms[sl].flags&pfSick == 0 || !s.isCreature(k, sl) || s.kw(k, sl)&kop.KwHaste != 0 {
		return false
	}
	return !(k.gen != nil && k.gen.anyAsIfHaste && !DebugAsIfHasteOff && s.staticFlagGen(k, kop.StatAsIfHaste, sl))
}
