package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The CastWithFlash Activated.Equip lockstep: under Leonin Shikari its
// controller may activate Equip any time they could cast an instant (CR 702.6;
// gorge equipAtInstantSpeed, kernel kop.StatEquipFlash). The decks pair
// Shikari and Bonesplitter with creatures, burn and a second Equipment-free
// side.
var cwfEquipDecks = [2][]cardset.Entry{
	{{Name: "Leonin Shikari", Count: 8}, {Name: "Bonesplitter", Count: 4}, {Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 9}, {Name: "Plains", Count: 9}},
	{{Name: "Bonesplitter", Count: 4}, {Name: "Lightning Bolt", Count: 6}, {Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 9}, {Name: "Plains", Count: 9}},
}

// cwfEquipLive: gorge poses a priority decision at a time that is not the
// holder's sorcery window and offers the Equip ability among its options.
func cwfEquipLive(ref *GorgeSide) bool {
	e := ref.E
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return false
	}
	if e.G.Active == d.Player && e.G.Step.IsMain() && len(e.G.Stack) == 0 {
		return false
	}
	for _, o := range d.Options {
		if o.Kind == "ability" && strings.Contains(o.Label, ": Equip") {
			return true
		}
	}
	return false
}

// TestEquipFlashLockstep plays the lowering: random and
// explore policies, no divergence and no unmodelled stall, and gorge offered
// an Equip outside the sorcery window.
func TestEquipFlashLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	_, a := sp3Run(t, "equipflash", cwfEquipDecks, pkPolicy, 24, false, nil, cwfEquipLive)
	_, b := sp3Run(t, "equipflash", cwfEquipDecks, ExplorePolicy, 24, false, nil, cwfEquipLive)
	t.Logf("an Equip was offered at instant speed in %d random and %d explore games", a, b)
	if a+b < 3 {
		t.Errorf("the instant-speed Equip was live in %d games, want at least 3", a+b)
	}
}

// TestEquipFlashMutation: with the static ignored (dirac.DebugEquipFlashOff)
// the kernel keeps Equip at sorcery speed and diverges, so TestEquipFlashLockstep
// does not pass vacuously.
func TestEquipFlashMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugEquipFlashOff = true
	defer func() { dirac.DebugEquipFlashOff = false }()
	if d, _ := sp3Run(t, "equipflash", cwfEquipDecks, ExplorePolicy, 24, true, nil, cwfEquipLive); d == nil {
		t.Error("no divergence with the Equip flash grant off")
	}
}
