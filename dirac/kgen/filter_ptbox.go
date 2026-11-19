package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// ptBoxWith folds a second constant power or toughness compare into a clause
// whose one compare is already a constant power, toughness or box compare:
// "Creature.powerEQ1+toughnessEQ1" (Pendelhaven, Sword of the Meek, Lovestruck
// Beast) and "Creature.powerGE4+powerLE6" (Sarkhan's Unsealing). gorge reads
// each conjunct on the same current power and toughness (filter_word.go's
// power/toughness words: the layered values on the battlefield, the printed
// ones elsewhere), so their conjunction is the one box kop.PredStatBox tests.
// It returns the box bounds (kop.PTBoxPack) and false for any other pair: an
// X or value bound, NE, a bound outside 0..14, or a stat other than power
// and toughness, which keep the two-comparisons refusal.
func (c *lClause) ptBoxWith(stat, op uint8, n int, x string) (int, bool) {
	if x != "" || c.CmpX != "" || c.CmpV != nil || c.CmpCounter != "" {
		return 0, false
	}
	var b [4]int // power lo, hi; toughness lo, hi; kop.PTBoxOpen is no bound
	switch c.CmpStat {
	case cmpPTBox:
		b = kop.PTBoxUnpack(int16(c.CmpN))
	case cmpPower, cmpToughness:
		b = [4]int{kop.PTBoxOpen, kop.PTBoxOpen, kop.PTBoxOpen, kop.PTBoxOpen}
		if !ptBoxNarrow(&b, c.CmpStat, c.CmpOp, c.CmpN) {
			return 0, false
		}
	default:
		return 0, false
	}
	if !ptBoxNarrow(&b, stat, op, n) {
		return 0, false
	}
	return int(uint16(kop.PTBoxPack(b))), true
}

// ptBoxNarrow intersects box b with one constant compare of power or
// toughness; false for anything the box cannot hold.
func ptBoxNarrow(b *[4]int, stat, op uint8, n int) bool {
	i := 0
	switch stat {
	case cmpPower:
	case cmpToughness:
		i = 2
	default:
		return false
	}
	lo, hi := kop.PTBoxOpen, kop.PTBoxOpen
	bound := n // the one bound the compare sets, which must fit 0..14
	switch op {
	case opEQ:
		lo, hi = n, n
	case opGE:
		lo = n
	case opGT:
		lo, bound = n+1, n+1
	case opLE:
		hi = n
	case opLT:
		hi, bound = n-1, n-1
	default:
		return false
	}
	if bound < 0 || bound >= kop.PTBoxOpen {
		return false
	}
	if lo != kop.PTBoxOpen && (b[i] == kop.PTBoxOpen || lo > b[i]) {
		b[i] = lo
	}
	if hi != kop.PTBoxOpen && (b[i+1] == kop.PTBoxOpen || hi < b[i+1]) {
		b[i+1] = hi
	}
	return true
}
