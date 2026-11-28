package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A plus-N counter replacement whose ValidObject$ names a player as well as
// permanents (Lae'zel, Vlaakith's Champion: "on a creature or planeswalker
// you control or on yourself, put that many plus one of each of those kinds
// of counters instead") rewrites the player counters its controller gets too
// (CR 122.6): gorge routes a PutCounter on a player through the same
// AddCounter replacements as a permanent's. kgen appends the player
// alternative to the replacement's filter chain (an FPlayer entry that no
// permanent matches); plusPlayerCtr reads it for the player counters
// OpGainEnergy gives (energy, experience).

// plusPlayerCtr is n player counters player p would get, after every plus-N
// AddCounter replacement that names p. Only a kind-less line (every kind) can
// apply: the player counters have no kop.CounterKind. A plus beside an
// Innkeeper's Talent doubler asks gorge for the CR 616.1 order and the kernel
// has no such ask, so that game stalls instead of guessing.
func (s *State) plusPlayerCtr(k *prep, p uint8, n int) int {
	g := k.gen
	if g == nil || n <= 0 || g.anyRepl&rbCtrOn == 0 {
		return n
	}
	plus, plusN := 0, 0
	for m := s.statM; m.any(); m = m.rest() {
		b := m.first()
		if s.genLost(k, b) {
			continue
		}
		lc := s.perms[b].ctrl
		s.eachPileCard(k, b, func(c CardID) bool {
			if g.replBits[c]&rbCtrOn == 0 {
				return true
			}
			cd := &k.t.Cards[c]
			for i := range cd.Abilities {
				a := &cd.Abilities[i]
				if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplAddCountersOn || a.Repl.C >= 0 {
					continue
				}
				if s.ctrKindMatch(a, -1, lc) && s.castTargetsPlayerRead(k, a.Repl.Filter, p, lc) { // repl_addcounter.go, trig_qualifiers.go
					plus += int(a.Repl.A)
					plusN++
				}
			}
			return true
		})
	}
	if plusN == 0 {
		return n
	}
	if g.anyRepl&rbDblCtrYou != 0 && s.dblCtrYou(k, s.counterAdder()) > 0 {
		s.capFull(StallReplCtr)
		return n
	}
	return max(n+plus, 0)
}
