package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestLandTypeSetterLockstep plays the attached CR 305.7 land-type setters
// kgen lowers (Spreading Seas, Sea's Claim, Evil Presence: RemoveLandTypes$
// True with AddType$ Island / Swamp; Lush Growth's three types, a
// multi-colour intrinsic view; kop.Override.SetLandSub) in lockstep with
// gorge over the landtypesetter fixture pool. Both decks enchant lands of other
// types (Forests, Plains, Wastes, the original duals), then tap them for mana
// and cast creatures of the old and the new colours: the enchanted land is
// only the new basic type, its own mana ability gone and the new type's
// intrinsic one in its place. The precondition, measured from gorge's side,
// is that a setter attached to a land whose printed types lack the new type,
// and that such a land was tapped while enchanted.
//
// The mutation keeps the land's own mana abilities beside the intrinsic one
// (dirac.DebugLandSetOwnMana, the plain AddSub reading): a
// Forest under Spreading Seas then taps for {G} too, and the kernel must
// diverge.
func TestLandTypeSetterLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run, st := runLandSetLockstep(t, 24, true)
		t.Logf("%d games, %d divergences, %d stalls; setters attached %d (%d off-type), enchanted taps %d (%d off-type, %d under three types)", run.games, run.divs, run.stalls, st.attached, st.offAttach, st.taps, st.offTaps, st.multiTaps)
		if st.offAttach < 4 || st.offTaps < 4 || st.multiTaps < 2 {
			t.Errorf("setters attached to a land without the new type %d times, such lands tapped %d times, lands under three types tapped %d times: want at least 4, 4 and 2", st.offAttach, st.offTaps, st.multiTaps)
		}
	})
	t.Run("mutation-own-mana", func(t *testing.T) {
		dirac.DebugLandSetOwnMana = true
		defer func() { dirac.DebugLandSetOwnMana = false }()
		run, _ := runLandSetLockstep(t, 24, false)
		run.requireDivergence(t, "a land-type setter's land keeping its own mana abilities")
	})
}

var landSetDecks = []fixtureDecks{{
	{{Name: "Spreading Seas", Count: 8}, {Name: "Evil Presence", Count: 4}, {Name: "Coral Merfolk", Count: 6}, {Name: "Typhoid Rats", Count: 4},
		{Name: "Island", Count: 7}, {Name: "Swamp", Count: 5}, {Name: "Tropical Island", Count: 3}, {Name: "Bayou", Count: 3}},
	{{Name: "Spreading Seas", Count: 4}, {Name: "Lush Growth", Count: 4}, {Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 3}, {Name: "Raging Goblin", Count: 3},
		{Name: "Forest", Count: 8}, {Name: "Island", Count: 4}, {Name: "Savannah", Count: 4}, {Name: "Wastes", Count: 4}},
}, {
	{{Name: "Evil Presence", Count: 8}, {Name: "Spreading Seas", Count: 4}, {Name: "Typhoid Rats", Count: 8}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Swamp", Count: 6}, {Name: "Island", Count: 3}, {Name: "Bayou", Count: 4}, {Name: "Forest", Count: 3}},
	{{Name: "Spreading Seas", Count: 3}, {Name: "Sea's Claim", Count: 3}, {Name: "Coral Merfolk", Count: 6}, {Name: "Savannah Lions", Count: 8},
		{Name: "Plains", Count: 6}, {Name: "Island", Count: 6}, {Name: "Savannah", Count: 4}, {Name: "Tropical Island", Count: 4}},
}}

var landSetWatch = map[string]bool{"Spreading Seas": true, "Sea's Claim": true, "Evil Presence": true, "Lush Growth": true}

// landSetAdds is the basic land types each watched setter adds.
var landSetAdds = map[string][]string{
	"Spreading Seas": {"Island"}, "Sea's Claim": {"Island"}, "Evil Presence": {"Swamp"}, "Lush Growth": {"Mountain", "Forest", "Plains"},
}

// landSetStats are the preconditions measured from gorge's side.
type landSetStats struct {
	attached, offAttach int // setters attached to a land; to one whose printed types lack an added type
	taps, offTaps       int // enchanted lands tapped; ones whose printed types lack an added type
	multiTaps           int // lands under Lush Growth (three intrinsic abilities) tapped
}

// landSetOn returns the basic land types the setters attached to land id add.
func landSetOn(g *state.Game, id state.ObjID) []string {
	var out []string
	for _, q := range g.AliveFrom(0) {
		for _, a := range g.Zone(state.ZBattlefield, q) {
			if o := g.Obj(a); o != nil && o.AttachedTo == id && o.Face() != nil {
				out = append(out, landSetAdds[o.Face().Name]...)
			}
		}
	}
	return out
}

// landSetObserve scans the events gorge appended since *seen.
func landSetObserve(ref *GorgeSide, seen *int, st *landSetStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	for ; *seen < len(evs); *seen++ {
		e := evs[*seen]
		switch e.Kind {
		case events.Attach:
			if len(e.IDs) != 1 {
				continue // attached to a player, or a re-attach without a bearer
			}
			src, land := g.Obj(e.Obj), g.Obj(e.IDs[0])
			if src == nil || src.Face() == nil || land == nil || land.Face() == nil {
				continue
			}
			if ws, ok := landSetAdds[src.Face().Name]; ok {
				st.attached++
				for _, w := range ws {
					if !land.Face().HasTypeWord(w) {
						st.offAttach++
						break
					}
				}
			}
		case events.Tap:
			land := g.Obj(e.Obj)
			if land == nil || land.Face() == nil {
				continue
			}
			adds := landSetOn(g, e.Obj)
			if len(adds) == 0 {
				continue
			}
			st.taps++
			if len(adds) > 1 {
				st.multiTaps++
			}
			for _, w := range adds {
				if !land.Face().HasTypeWord(w) {
					st.offTaps++
					break
				}
			}
		}
	}
}

// runLandSetLockstep plays seeds 1..seeds of the landtypesetter matchups,
// gorge leading, alternating the matchup and the option surface by seed and
// measuring the preconditions.
func runLandSetLockstep(t *testing.T, seeds int, report bool) (fixtureRun, landSetStats) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("landtypesetter")
	if err != nil {
		t.Fatal(err)
	}
	decks := make([][2][]*cards.Card, len(landSetDecks))
	tbls := make([]*kop.Table, len(landSetDecks))
	for mi, md := range landSetDecks {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for range e.Count {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[mi][s]))
			}
		}
		if tbls[mi], _, _, err = set.Matchup(md[0], md[1]); err != nil {
			t.Fatal(err)
		}
	}
	run := fixtureRun{casts: map[string]int{}}
	var st landSetStats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		mi := int(seed/2) % len(landSetDecks)
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"landset-a", "landset-b"}, decks[mi], 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbls[mi], cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			landSetObserve(ref, &seen, &st)
			return nil
		}
		res := LockstepLed(m, seed, g, k, fixtureWatchPolicy(seed, landSetWatch, run.casts), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
		CloseSide(k)
		run.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			run.stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			run.divs++
			msg := strings.TrimSpace(d.What + " " + d.Field + " " + d.Detail)
			if run.first == "" {
				run.first = msg
			}
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %v\nref %v\nker %v", seed, d.At, msg, d.Diff, d.RefOptions, d.KerOptions)
			}
			if !report || run.divs >= 3 {
				break
			}
		}
	}
	return run, st
}
