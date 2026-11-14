package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// CR 702.103b with CR 205.3h: Nyxborn Hydra, bestowed onto a creature, is an
// Aura and not a creature, and "Aura" is a subtype as well as a type. Gorge
// projects the Aura subtype of the attached card; the kernel carried the Aura
// type only (ev-dr-mbc-bbaa pair 100: Cards[72].Chars.Subtypes[0] ref=1
// kernel=0).
func TestMBCBestowedAuraSubtypeReprosLockstep(t *testing.T) {
	t.Parallel()
	replayRepro(t, synthReg(t), cardsettest.Set(t, "mbc"), "bestow_attached_card_is_an_aura_subtype", true, false)
}
