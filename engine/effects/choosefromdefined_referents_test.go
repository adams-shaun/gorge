package effects

// The ChangeZone ChooseFromDefined$ referent spellings beyond the original
// dotted AttachedTo form: each measured non-AttachedTo selector either
// resolves the intended object pool (offered as the pick's options and
// enforced on the answered picks) or stays loud-fail-closed (one Note, an
// empty pool, nothing offered). Every card here is the REAL compiled corpus
// card, so a corpus pin that moves the scripts fails the lookup.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// cfdFixture is one placed object: its inline card text, owner and zone.
type cfdFixture struct {
	text  string
	owner state.PlayerID
	zone  state.Zone
}

// cfdBoard builds a 2-seat ask host and places every fixture in its zone.
// Zones are rebuilt through SetZone so the game's zone slices agree with the
// object fields, and the returned ids follow the fixtures' order.
func cfdBoard(t *testing.T, specs ...cfdFixture) (*askHost, []state.ObjID) {
	t.Helper()
	h := &askHost{}
	h.g = state.NewGame(names(2))
	ids := make([]state.ObjID, 0, len(specs))
	byOwner := map[state.Zone]map[state.PlayerID][]state.ObjID{}
	for _, s := range specs {
		o := h.g.AddObject(mkCard(t, s.text), s.owner)
		o.Zone = s.zone
		ids = append(ids, o.ID)
		if byOwner[s.zone] == nil {
			byOwner[s.zone] = map[state.PlayerID][]state.ObjID{}
		}
		byOwner[s.zone][s.owner] = append(byOwner[s.zone][s.owner], o.ID)
	}
	for z, owners := range byOwner {
		for p, zs := range owners {
			h.g.SetZone(z, p, zs)
		}
	}
	return h, ids
}

const (
	whiteCreature = "Name:White Runt\nManaCost:W\nTypes:Creature\nPT:1/1\nOracle:x\n"
	blueCreature  = "Name:Blue Drake\nManaCost:1 U\nTypes:Creature\nPT:2/2\nOracle:x\n"
	plainLand     = "Name:Test Plains\nTypes:Land\nOracle:x\n"
)

// TestDefinedTopThirdOfLibraryRounding pins the rounding rule directly: top
// third rounded up over 1, 3, 4 and 7-card libraries.
func TestDefinedTopThirdOfLibraryRounding(t *testing.T) {
	for n, want := range map[int]int{1: 1, 3: 1, 4: 2, 7: 3} {
		h, ids := cfdBoard(t, nCards(n)...)
		c := &Ctx{Source: ids[0], Controller: 0}
		ts, ok := knownDefinedTargets(h, c, "TopThirdOfLibrary")
		if !ok {
			t.Fatalf("TopThirdOfLibrary over %d cards: not resolvable", n)
		}
		if len(ts) != want {
			t.Fatalf("TopThirdOfLibrary over %d cards = %d targets, want %d", n, len(ts), want)
		}
		for i, tgt := range ts {
			if tgt.Obj != ids[i] {
				t.Fatalf("TopThirdOfLibrary slot %d = %d, want the zone-order card %d", i, tgt.Obj, ids[i])
			}
		}
	}
}

// nCards builds n distinct library cards.
func nCards(n int) []cfdFixture {
	out := make([]cfdFixture, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, cfdFixture{cfdCreatureText(i+1, i+1), 0, state.ZLibrary})
	}
	return out
}

func cfdCreatureText(cost, power int) string {
	return "Name:Round Card " + string(rune('A'+cost)) + "\nManaCost:" + itoa(cost) +
		"\nTypes:Creature\nPT:" + itoa(power) + "/" + itoa(power) + "\nOracle:x\n"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
