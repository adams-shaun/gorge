package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// hybColours returns hybrid pip i's two colours in the printed order the
// caster announces them (gorge's AnnouncePip: Hybrid[i].A then .B), as
// WUBRG indexes. i must name a set pip. The mask alone cannot answer this:
// the lowering records a reversed printed order in ManaCost.HybOrd (see
// kop.ManaCost), and this is the one reader of it.
func hybColours(c kop.ManaCost, i int) (first, second int) {
	m := uint8(c.Hyb[i])
	lo := bits.TrailingZeros8(m)
	hi := 7 - bits.LeadingZeros8(m)
	if c.HybOrd&(uint8(1)<<i) != 0 {
		return hi, lo
	}
	return lo, hi
}
