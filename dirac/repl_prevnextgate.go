package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// repl_prevnextgate.go: a plus-N damage replacement whose granted gate is off
// is no candidate for the order a prevent-the-next-N shield poses (CR 616.1:
// only a replacement that would apply to the event is one). Far Fortune, End
// Boss grants "if a source you control would deal damage ... plus 1" only at
// max speed; below it gorge sees the Loxodon Anchorite shield alone, so the
// kernel must not stall StallPrevNext beside the idle host.

// plusHostIdle reports whether the permanent at b carries ReplDamagePlus
// abilities and every one of them is gated off now (kw_maxspeed.go
// replHostLive). A host with no such ability (a static prevention host also
// carries rbPlusDmg) or with a live one is not idle.
func (s *State) plusHostIdle(k *prep, b Slot) bool {
	cd := &k.t.Cards[s.cardAt(b)]
	found := false
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplDamagePlus {
			continue
		}
		if a.Repl.Cond == 0 || s.replHostLive(k, a, b) {
			return false
		}
		found = true
	}
	return found
}
