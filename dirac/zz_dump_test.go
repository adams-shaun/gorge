package dirac

import (
	"testing"
	"unsafe"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

func TestZZDumpCold(t *testing.T) {
	worst, worstName := 0, ""
	for _, name := range cardset.Names() {
		set := cardsettest.Set(t, name)
		tbl, err := set.Pool()
		if err != nil {
			continue
		}
		nGy := 0
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				if gyContOf(&tbl.Cards[i].Abilities[j]) {
					nGy++
					break
				}
			}
		}
		var s State
		s.Init(tbl, nil, nil, 0, Config{})
		base := int(unsafe.Sizeof(s)) + 8*len(s.cold.buf) - 976
		cap := nGy * MaxSeats * 4
		out := cap <= len(tbl.Subtypes)/2
		extra := 0
		if nGy > 0 && !out {
			extra = (cap*4 + 7) / 8 * 8
		}
		total := base + extra
		if total > worst {
			worst, worstName = total, name
		}
		if nGy > 0 {
			t.Logf("%s nGy=%d subs=%d base=%d cap=%d out=%v extra=%d total=%d", name, nGy, len(tbl.Subtypes), base, cap, out, extra, total)
		}
		ReleaseTable(tbl)
	}
	t.Logf("worst=%d (%s) bar=45056", worst, worstName)
	t.Fatalf("dump")
}
