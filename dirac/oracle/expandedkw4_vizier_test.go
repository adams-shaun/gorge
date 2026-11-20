package oracle

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Vizier of Many Faces lockstep (CR 707.9b, CR 702.128a): the as-enters
// clone's Embalm$ gate. A two-seat pod of Vizier of Many Faces and Grizzly
// Bears (blue-green) against burn and Hill Giants: Vizier dies, embalms out of
// the graveyard, and the token enters as a copy of another creature that is
// white, a Zombie and has no mana cost; a hard-cast Vizier that copies a
// creature is a plain copy of it (no Zombie, its own colour and mana cost).
// The lockstep compares the two sides' option lists and projected
// characteristics at every decision, and the observed counts prove both shapes
// of the gate fired.

// vizierSeen counts what the Check hook saw on gorge's battlefield.
type vizierSeen struct {
	embalmed  int // an embalm token of Vizier that entered as a copy of another creature
	embalmBad int // ... whose copy is not white, a Zombie with no mana cost
	hardCast  int // a hard-cast Vizier that entered as a copy of another creature (not of an embalm token, whose riders are copiable)
	hardBad   int // ... whose copy took any of the embalm riders
	detail    string
}

func vizierDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Vizier of Many Faces", Count: 6}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Island", Count: 13}, {Name: "Forest", Count: 13},
		},
		{
			{Name: "Hill Giant", Count: 6}, {Name: "Lightning Bolt", Count: 5}, {Name: "Doom Blade", Count: 5},
			{Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 12},
		},
	}
}

func TestExpandedKeywordsVizierCloneLockstep(t *testing.T) {
	t.Parallel()
	d := vizierDecks()
	pod := newKw4PodOf2(t, d)
	var sum vizierSeen
	for _, seed := range []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12} {
		var seen vizierSeen
		seenObj := map[state.ObjID]bool{}
		r := pod.repro(d, seed)
		m, err := r.Matchup(pod.reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 8000, 40
		rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
			g := ref.E.G
			for p := range g.Players {
				for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
					o := g.Obj(id)
					if o == nil || seenObj[id] || o.Card == nil || o.Card.Faces[0].Name != "Vizier of Many Faces" {
						continue
					}
					f := o.Face()
					if f == nil || f.Name == "Vizier of Many Faces" {
						continue // entered as itself: no copy
					}
					seenObj[id] = true
					cols := ref.E.Colors(id)
					white := cols == "W"
					zombie := slices.Contains(ref.E.Chars(id).Types, "Zombie")
					mv := state.ManaValue(g, o)
					if o.IsToken {
						seen.embalmed++
						if !white || !zombie || mv != 0 || !f.CopyNoManaCost() {
							seen.embalmBad++
							seen.detail += " embalm:" + f.Name + "/" + cols + "/" + strconv.Itoa(int(mv))
						}
					} else if white && zombie && mv == 0 && f.CopyNoManaCost() {
						continue // a copy of an embalm token: CR 707.3 makes its riders copiable, the copy keeps them
					} else {
						seen.hardCast++
						if white || zombie || mv == 0 || f.CopyNoManaCost() {
							seen.hardBad++
							seen.detail += " hard:" + f.Name + "/" + cols + "/" + strconv.Itoa(int(mv))
						}
					}
				}
			}
			return nil
		}
		k, err := SetSide(pod.set, pod.tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		lead, err := NewGorgeSide(m, seed, rc.Config, nil)
		if err != nil {
			CloseSide(k)
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc)
		CloseSide(k)
		if res.Div != nil {
			kw4Fail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		t.Logf("seed %d: %d decisions, over %v; embalmed %d (bad %d), hard-cast %d (bad %d)%s",
			seed, res.Decisions, res.Over, seen.embalmed, seen.embalmBad, seen.hardCast, seen.hardBad, seen.detail)
		sum.embalmed += seen.embalmed
		sum.embalmBad += seen.embalmBad
		sum.hardCast += seen.hardCast
		sum.hardBad += seen.hardBad
	}
	if sum.embalmed == 0 {
		t.Fatal("no embalmed Vizier token entered as a copy of another creature")
	}
	if sum.hardCast == 0 {
		t.Fatal("no hard-cast Vizier entered as a copy of another creature")
	}
	if sum.embalmBad != 0 || sum.hardBad != 0 {
		t.Fatalf("gorge's copies broke the Embalm$ gate: %d embalmed token(s) and %d hard-cast copy(ies) off", sum.embalmBad, sum.hardBad)
	}
}

// newKw4PodOf2 is newKw4Pod for a two-deck pod.
func newKw4PodOf2(t *testing.T, d [2][]cardset.Entry) kw4Pod {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("expandedkw4")
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, err := set.MatchupN([][]cardset.Entry{d[0], d[1]})
	if err != nil {
		t.Fatal(err)
	}
	return kw4Pod{reg: reg, set: set, tbl: tbl}
}

// repro is the two-seat Repro of the pod's decks.
func (p kw4Pod) repro(d [2][]cardset.Entry, seed uint64) *Repro {
	expand := func(es []cardset.Entry) []string {
		var out []string
		for _, e := range es {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	return &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expand(d[0]), expand(d[1])}, Seed: seed, PassElision: true, Surface: "full"}
}
