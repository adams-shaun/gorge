package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Three and four hybrid pips in one printed cost (CR 107.4e,
// 601.2b). gorge announces each pip one by one (pay.ManaAsk, options in
// AnnouncePip order, each gated on the whole cost's feasibility with the
// later pips free) and its potential planner enumerates the resolutions
// with pip 0 the fastest varying (rules planHybridCost, planHybridLimit 4).
// Every kernel walk that tried the two pips of a hybrid cost by nested
// loops enumerates a bit mask instead: bit i set = pip i takes its second
// face. The walks stay iterative and their costs stack-local: a call cycle
// would move the resolved costs to the heap (see affordsPlain).

// maxHyb is the most hybrid pips a cost carries (len(kop.ManaCost{}.Hyb)).
const maxHyb = len(kop.ManaCost{}.Hyb)

// DebugHybridNPipOrder is the mutation: the witness enumerates
// pip n-1 fastest instead of pip 0. Test-only; never set in a real run.
var DebugHybridNPipOrder bool

// DebugHybridNPipGate is the second mutation: the feasibility
// gate of pip asks 2 and 3 offers every colour, as if the later pips were
// not folded into the whole-cost check. Test-only.
var DebugHybridNPipGate bool

// hybGateMutation reports whether the gate mutation waives pip n's check.
func hybGateMutation(pipN uint8) bool { return DebugHybridNPipGate && pipN >= 2 }

// hybFaces are a cost's hybrid pips' two colours each, in announced order.
type hybFaces [maxHyb][2]uint8

// hybFacesOf reads c's n unresolved hybrid pips (c is no twobrid cost).
func hybFacesOf(c *kop.ManaCost) (n int, f hybFaces) {
	for i, h := range c.Hyb {
		if h == 0 {
			break
		}
		a, b := hybColours(*c, i)
		f[i] = [2]uint8{uint8(a), uint8(b)}
		n = i + 1
	}
	return n, f
}

// resolve returns c with its n hybrid pips paid by mask m (bit i set: pip i
// takes its second colour), c's other fields unchanged.
func (f *hybFaces) resolve(c kop.ManaCost, n, m int) kop.ManaCost {
	for i := 0; i < n; i++ {
		c.Pips[f[i][m>>uint(i)&1]]++ // seat-ok: a hybrid half, not a seat
	}
	c.Hyb = [maxHyb]kop.ColorMask{}
	c.HybOrd = 0
	return c
}

// pipCol is the colour announced for hybrid pip i (W U B R G = 0..4): three
// bits per pip across build.pipC[0] | pipC[1]<<8. A twobrid or Phyrexian
// cost keeps its answers as a bit mask in pipC[0] and never calls this.
func (b *build) pipCol(i uint8) int {
	return int((uint16(b.pipC[0])|uint16(b.pipC[1])<<8)>>(3*uint(i))) & 7
}

// setPipCol records colour c as hybrid pip i's answer.
func (b *build) setPipCol(i uint8, c uint8) {
	v := uint16(b.pipC[0]) | uint16(b.pipC[1])<<8
	v = v&^(7<<(3*uint(i))) | uint16(c&7)<<(3*uint(i))
	b.pipC[0], b.pipC[1] = uint8(v), uint8(v>>8)
}

// hybWitnessMask is the m-th resolution gorge's planner tries (planHybridCost:
// pip 0 the fastest varying), as a resolve mask. The mutation reverses it.
func hybWitnessMask(m, n int) int {
	if !DebugHybridNPipOrder {
		return m
	}
	r := 0
	for i := 0; i < n; i++ {
		r |= (m >> uint(i) & 1) << uint(n-1-i)
	}
	return r
}

// hybOverflow is addCost's loud refusal of a sum with more hybrid pips than
// gorge's planner resolves (planHybridLimit): a kicker or additional cost
// stacked on a hybrid cost. addCost has no State to stall, and silently
// dropping the pip (the old two-pip clamp overwrote the last one) would pay
// a cheaper cost than gorge's.
func hybOverflow() {
	panic("kernel: a summed cost carries more than four hybrid pips (gorge planHybridLimit)")
}
