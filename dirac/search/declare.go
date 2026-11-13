package search

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Attack and block declarations (searchprobe AttackCandidates /
// BlockCandidates): a candidate is a whole declaration, played as the
// dirac's single picks closed by AkDone.

// botDeclaration runs the bot's picks for the declaration d opens on a
// scratch copy of s, writing them (without the closing AkDone) to out.
// A bot that answers a whole declaration in one call (Declarer) is asked
// once; the walk then serves its list one pick at a time exactly as its
// Pick would (Declarer's contract), without re-asking per pick.
func (sr *Searcher) botDeclaration(s *dirac.State, d *dirac.Decision, bot int, out *[MaxPerm]uint32) int {
	a := d.Options[bot]
	if a.Kind() == dirac.AkDone {
		return 0
	}
	if dc, ok := sr.Bot.(Declarer); ok {
		return sr.declWalk(dc, s, d, a, out)
	}
	n := 0
	sr.tmp.CopyFrom(s)
	kind := d.Kind
	for n < len(out) {
		out[n] = uint32(a)
		n++
		sr.tmp.Apply(a)
		sr.tmp.StepInto(&sr.td)
		// A turn change closes the declaration: the next turn's attackers
		// (or blockers) decision is a new combat, not this walk's.
		if sr.td.Kind != kind || sr.td.Player != d.Player || sr.tmp.Turn() != s.Turn() {
			break
		}
		a = sr.td.Options[sr.Bot.Pick(&sr.tmp, &sr.td)]
		if a.Kind() == dirac.AkDone {
			break
		}
	}
	return n
}

// MaxPerm mirrors dirac's battlefield size (declaration bound).
const MaxPerm = dirac.MaxPerm

func (sr *Searcher) addDecl(pt *point, picks []uint32, limit int) {
	if pt.n >= limit {
		return
	}
	// dedup: an identical pick list (order included) is not repeated
	for i := 0; i < pt.n; i++ {
		c := pt.acts[pt.off[i]:pt.off[i+1]]
		if len(c) == len(picks)+1 {
			same := true
			for j := range picks {
				if c[j] != picks[j] {
					same = false
					break
				}
			}
			if same {
				return
			}
		}
	}
	var buf [MaxPerm + 1]uint32
	n := copy(buf[:], picks)
	buf[n] = uint32(dirac.MakeAction(dirac.AkDone, 0, 0, 0))
	pt.addSeq(buf[:n+1])
}

// attackCands: the bot's declaration, no attack, all-in (each creature at
// its first defender), then the bot's declaration with each option
// toggled (toggling a creature on at a defender moves it there).
func (sr *Searcher) attackCands(s *dirac.State, d *dirac.Decision, bot, limit int, pt *point) {
	var botSet [MaxPerm]uint32
	nb := sr.botDeclaration(s, d, bot, &botSet)
	sr.addDecl(pt, botSet[:nb], limit)
	sr.addDecl(pt, nil, limit)
	var all [MaxPerm]uint32
	na := 0
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != dirac.AkAttack || hasSource(all[:na], uint8(a.Source())) {
			continue
		}
		all[na] = uint32(a)
		na++
	}
	sr.addDecl(pt, all[:na], limit)
	for i := 0; i < int(d.N); i++ {
		o := d.Options[i]
		if o.Kind() != dirac.AkAttack {
			continue
		}
		var t [MaxPerm]uint32
		nt := 0
		on := !has(botSet[:nb], uint32(o))
		for _, x := range botSet[:nb] {
			if x == uint32(o) {
				continue // toggled off
			}
			if on && dirac.Action(x).Source() == o.Source() {
				continue // the creature moves to o's defender
			}
			t[nt] = x
			nt++
		}
		if on {
			nt = insertInOrder(d, t[:nt], uint32(o), &t)
		}
		sr.addDecl(pt, t[:nt], limit)
	}
}

func hasSource(l []uint32, src uint8) bool {
	for _, x := range l {
		if dirac.Action(x).Source() == dirac.TCode(src) {
			return true
		}
	}
	return false
}

// insertInOrder inserts a into l (a prefix of buf) keeping d's option order.
func insertInOrder(d *dirac.Decision, l []uint32, a uint32, buf *[MaxPerm]uint32) int {
	pos := func(x uint32) int {
		for i := 0; i < int(d.N); i++ {
			if uint32(d.Options[i]) == x {
				return i
			}
		}
		return 1 << 20
	}
	pa := pos(a)
	n := len(l)
	j := n
	for j > 0 && pos(buf[j-1]) > pa {
		buf[j] = buf[j-1]
		j--
	}
	buf[j] = a
	return n + 1
}

// blockCands: the bot's declaration, no blocks, the bot's with each other
// pair added last (replacing that blocker's pair), the bot's minus each of
// its pairs; a declaration leaving a menace attacker with one blocker is
// not offered (gorge Decision.Validate's MinBlockers).
func (sr *Searcher) blockCands(s *dirac.State, d *dirac.Decision, bot, limit int, pt *point) {
	var botSet [MaxPerm]uint32
	nb := sr.botDeclaration(s, d, bot, &botSet)
	sr.addBlock(s, pt, botSet[:nb], limit)
	sr.addBlock(s, pt, nil, limit)
	for i := 0; i < int(d.N); i++ {
		o := d.Options[i]
		if o.Kind() != dirac.AkBlock || has(botSet[:nb], uint32(o)) {
			continue
		}
		var t [MaxPerm]uint32
		nt := 0
		for _, x := range botSet[:nb] {
			if dirac.Action(x).Source() != o.Source() {
				t[nt] = x
				nt++
			}
		}
		t[nt] = uint32(o)
		nt++
		sr.addBlock(s, pt, t[:nt], limit)
	}
	for i := 0; i < nb; i++ {
		var t [MaxPerm]uint32
		nt := copy(t[:], botSet[:i])
		nt += copy(t[nt:], botSet[i+1:nb])
		sr.addBlock(s, pt, t[:nt], limit)
	}
}

func has(l []uint32, a uint32) bool {
	for _, x := range l {
		if x == a {
			return true
		}
	}
	return false
}

func (sr *Searcher) addBlock(s *dirac.State, pt *point, picks []uint32, limit int) {
	// menace: an attacker with menace blocked by exactly one creature
	for _, x := range picks {
		at := dirac.Action(x).Target()
		n := 0
		for _, y := range picks {
			if dirac.Action(y).Target() == at {
				n++
			}
		}
		if n == 1 && s.HasKeyword(dirac.Obj(at), kop.KwMenace) {
			return
		}
	}
	// dedup on the sorted list
	for i := 0; i < pt.n; i++ {
		c := pt.acts[pt.off[i] : pt.off[i+1]-1]
		if len(c) != len(picks) {
			continue
		}
		same := true
		for _, x := range picks {
			if !has(c, x) {
				same = false
				break
			}
		}
		if same {
			return
		}
	}
	sr.addDecl(pt, picks, limit)
}
