//go:build spellbench

package mapping_test

// The CR 401.4 arrangement the engine's libraryPlacementOrder poses
// (engine/effects/zone_library.go): the moved cards are already in their
// owner's library and the ask is a full permutation over them whose options
// all carry Kind "library_placement". The mapping poses it as one forced
// partition (presentational dest ops) plus n-1 real ordering picks committed
// in pick order: the first pick goes nearest the top for BOTH positions.

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

const placementTopPrompt = "Put these cards on the top of your library in any order; the first card you pick goes nearest the top"

// libraryPlacementAsk is the pending CR 401.4 ask: a KArrange whose options
// are all library_placement, a full permutation (Min == Max == the offered
// count) of at least two cards. A token swept into the library ceases to
// exist after the sweep (CR 111.7), which would shift the placed block under
// the test's feet, so only asks over real cards still in the asker's library
// qualify.
func libraryPlacementAsk(d *decision.Decision, e *rules.Engine) bool {
	if !(d.Kind == decision.KArrange && len(d.Options) >= 2 && d.Options[0].Kind == "library_placement" &&
		d.Min == d.Max && d.Min == len(d.Options)) {
		return false
	}
	for _, o := range d.Options {
		obj := e.G.Obj(o.Obj)
		if obj == nil || obj.IsToken || !slices.Contains(e.G.Zone(state.ZLibrary, d.Player), o.Obj) {
			return false
		}
	}
	return true
}

// TestLibraryPlacementOrderIsTheAnswerOrder drives a Terminus sweep's
// library-placement ask. The partition picks are presentational dest ops, the
// ordering picks come back in "choices", and the committed intent places the
// offered cards in pick order with the first pick nearest the top. The
// position is read off the prompt: the ask the engine poses here is a bottom
// placement (Terminus puts all creatures on the bottom of their owners'
// libraries), and the top spelling is exercised over the same ask with the
// prompt's position swapped, which the constructor reads as the "top"
// destination (no repo deck reaches a real top placement ask: every
// ChangeZoneAll-to-library-top card in the corpus sweeps with RandomOrder or
// Shuffle, which the engine resolves without asking).
func TestLibraryPlacementOrderIsTheAnswerOrder(t *testing.T) {
	g := untilRepoDeck(t, "uw-control", libraryPlacementAsk)
	env := envFor(t, g)
	d := g.E.Pending()
	// Precondition: this is the library-placement ask, over at least two
	// cards of the answering seat, before anything is answered.
	if !libraryPlacementAsk(d, g.E) {
		t.Fatalf("pending decision is not the library-placement ask: %+v", d)
	}
	if !strings.Contains(d.Prompt, "on the bottom") {
		t.Fatalf("the engine's ask does not name the bottom: %q", d.Prompt)
	}
	seat := d.Player
	lib0 := append([]state.ObjID(nil), g.E.G.Zone(state.ZLibrary, seat)...)
	if !slices.Contains(lib0, d.Options[0].Obj) {
		t.Fatalf("offered card %d is not in the asker's library", d.Options[0].Obj)
	}
	posOfCard := map[state.ObjID]uint32{}
	cardOfPos := map[uint32]state.ObjID{}
	for _, o := range d.Options {
		at := uint32(slices.Index(lib0, o.Obj))
		posOfCard[o.Obj] = at
		cardOfPos[at] = o.Obj
	}
	n := len(d.Options)

	// orderDeepestFirst picks, at every ordering pose, the unplaced card
	// deepest in the library: the committed order must land it nearest the
	// top of the placed block.
	orderDeepestFirst := func(p *mapping.Pose, placed map[state.ObjID]bool) (int, state.ObjID) {
		positions := map[string]uint32{} // look id -> distance from the top
		for _, k := range p.Known {
			if k.ObjectID != nil && k.PositionFromTop != nil {
				positions[*k.ObjectID] = *k.PositionFromTop
			}
		}
		best, bestAt := -1, -1
		var want state.ObjID
		for i, c := range p.Candidates {
			item := c.Sem.Fields["item"].(protocol.OrderItem)
			pos, ok := positions[item.Object.ObjectID]
			if !ok {
				t.Fatalf("ordering candidate without a known position: %+v", c.Sem)
			}
			card := cardOfPos[pos]
			if placed[card] {
				continue
			}
			if int(pos) > bestAt {
				best, bestAt, want = i, int(pos), card
			}
		}
		if best < 0 {
			t.Fatalf("no unplaced ordering candidate in %v", p.Candidates)
		}
		return best, want
	}

	// drive answers one transaction: forced partition picks, then
	// deepest-first ordering picks; it returns the commit's Choices order and
	// the cards in it.
	var drive func(d2 *decision.Decision, wantDest string) (pickOrder []state.ObjID, in decision.Intent)
	drive = func(d2 *decision.Decision, wantDest string) ([]state.ObjID, decision.Intent) {
		tx, err := mapping.Begin(env, d2)
		if err != nil {
			t.Fatalf("Begin(%q): %v", d2.Prompt, err)
		}
		placed := map[state.ObjID]bool{}
		var pickOrder []state.ObjID
		commit := answerAll(t, tx, func(p *mapping.Pose) int {
			assertArrangementPurpose(t, p)
			if p.SubstepCount != uint32(2*n-1) {
				t.Fatalf("group size %d, want %d", p.SubstepCount, 2*n-1)
			}
			if len(p.Candidates) == 0 {
				t.Fatalf("empty pose at substep %d", p.SubstepIndex)
			}
			if p.Candidates[0].Sem.Kind == "arrange_card" {
				// The partition pick is forced: the one destination the prompt
				// names, presentational (no native option backs it).
				if len(p.Candidates) != 1 {
					t.Fatalf("partition pose offers %d candidates, want 1", len(p.Candidates))
				}
				c := p.Candidates[0]
				if got := c.Sem.Fields["destination"].(string); got != wantDest {
					t.Fatalf("partition destination %q, want %q", got, wantDest)
				}
				want := mapping.NativeOp{Op: "dest", Option: -1, List: wantDest, Position: int(p.SubstepIndex)}
				if !reflect.DeepEqual(c.Op, want) {
					t.Fatalf("partition op %+v, want %+v", c.Op, want)
				}
				return 0
			}
			// The ordering pick is a real native choice on "choices".
			for _, c := range p.Candidates {
				if c.Op.Op != "list" || c.Op.List != "choices" {
					t.Fatalf("ordering op %+v, want a \"choices\" list pick", c.Op)
				}
			}
			pick, card := orderDeepestFirst(p, placed)
			idx := slices.IndexFunc(d2.Options, func(o decision.Option) bool { return o.Obj == card })
			if idx < 0 || p.Candidates[pick].Op.Option != d2.Options[idx].Index {
				t.Fatalf("ordering pick %d carries option %d, want the index of card %d",
					pick, p.Candidates[pick].Op.Option, card)
			}
			placed[card] = true
			pickOrder = append(pickOrder, card)
			return pick
		})
		if len(commit) != 1 {
			t.Fatalf("commit %v, want one intent", commit)
		}
		return pickOrder, commit[0]
	}

	// The ask the engine poses: a bottom placement, committed end to end.
	pickOrder, in := drive(d, "bottom")
	if len(pickOrder) != n-1 {
		t.Fatalf("%d explicit ordering picks, want n-1 = %d", len(pickOrder), n-1)
	}
	wantChoices := make([]int, 0, n)
	for _, card := range pickOrder {
		wantChoices = append(wantChoices, d.Options[slices.IndexFunc(d.Options, func(o decision.Option) bool { return o.Obj == card })].Index)
	}
	// The implied last pick (the one remaining card) closes the permutation.
	for _, o := range d.Options {
		if !slices.Contains(wantChoices, o.Index) {
			wantChoices = append(wantChoices, o.Index)
		}
	}
	if !slices.Equal(in.Choices, wantChoices) {
		t.Fatalf("commit choices %v, want the pick order %v", in.Choices, wantChoices)
	}
	c, err := g.Probe(in)
	if err != nil {
		t.Fatalf("probe rejected the placement commit: %v", err)
	}
	newLib := c.G.Zone(state.ZLibrary, seat)
	base := len(newLib) - n
	for k, card := range pickOrder {
		if newLib[base+k] != card {
			t.Fatalf("placed block position %d holds %d, want pick %d", k, newLib[base+k], card)
		}
	}
	var implied state.ObjID
	for _, o := range d.Options {
		if !slices.Contains(pickOrder, o.Obj) {
			implied = o.Obj
		}
	}
	if newLib[len(newLib)-1] != implied {
		t.Fatalf("library bottom holds %d, want the implied last pick %d", newLib[len(newLib)-1], implied)
	}
	if newLib[base] != pickOrder[0] {
		t.Fatalf("first pick %d is not nearest the top of the placed block (block top %d)",
			pickOrder[0], newLib[base])
	}

	// The top spelling: the same ask with the prompt's position swapped. The
	// constructor reads the destination off the prompt, so the partition
	// candidates name "top" and the commit is the same pick-order answer.
	d2 := *d
	d2.Prompt = placementTopPrompt
	pickOrder2, in2 := drive(&d2, "top")
	if !slices.Equal(pickOrder2, pickOrder) || !slices.Equal(in2.Choices, in.Choices) {
		t.Fatalf("top spelling drives a different answer: picks %v vs %v, choices %v vs %v",
			pickOrder2, pickOrder, in2.Choices, in.Choices)
	}
	// The same answer validates against the engine's ask (the Choices order
	// is what the engine places at the named position, first pick nearest the
	// top; the engine's own placement of a top placement is exercised by
	// engine/effects' zone_library tests, not by the mapping).
	if _, err := g.Probe(in2); err != nil {
		t.Fatalf("probe rejected the top spelling's commit: %v", err)
	}
}

// TestLibraryPlacementGuardsStayUnmapped rejects, from mapping.Begin, the
// asks the constructor's guards fence: a mixed-kind option list, a
// Min/Max/count mismatch, an option of another player, and a prompt that
// names no library position. Each keeps its route, so the guard (not a
// routing miss) is what returns ErrUnmapped and the census keeps counting
// the ask as a native fallback.
func TestLibraryPlacementGuardsStayUnmapped(t *testing.T) {
	g := untilRepoDeck(t, "uw-control", func(d *decision.Decision, _ *rules.Engine) bool {
		return d.Kind == decision.KPriority
	})
	env := envFor(t, g)
	seat := state.PlayerID(0)
	lib := g.E.G.Zone(state.ZLibrary, seat)
	if len(lib) < 2 {
		t.Fatal("library too small")
	}
	bottomPrompt := "Put these cards on the bottom of your library in any order; the first card you pick goes nearest the top"
	ask := func(mutate func(d *decision.Decision)) *decision.Decision {
		d := &decision.Decision{Kind: decision.KArrange, Player: seat, Min: 2, Max: 2, Prompt: bottomPrompt,
			Options: []decision.Option{
				{Kind: "library_placement", Obj: lib[0], Index: 0, Player: seat},
				{Kind: "library_placement", Obj: lib[1], Index: 1, Player: seat},
			}}
		mutate(d)
		return d
	}
	cases := map[string]func(d *decision.Decision){
		"mixed kinds": func(d *decision.Decision) { d.Options[1].Kind = "bottom" },
		"wrong count": func(d *decision.Decision) { d.Min, d.Max = 1, 1 },
		"other player": func(d *decision.Decision) {
			d.Options[1].Player = state.PlayerID(1)
		},
		"unnamed position": func(d *decision.Decision) { d.Prompt = "Put these cards into your library in any order" },
	}
	for name, mutate := range cases {
		d := ask(mutate)
		if got := mapping.Route(d); got != "arrange/library_placement" {
			t.Errorf("%s: Route = %q, want arrange/library_placement (the guard, not routing, must refuse)", name, got)
		}
		if _, err := mapping.Begin(env, d); !errors.Is(err, mapping.ErrUnmapped) {
			t.Errorf("%s: Begin err = %v, want ErrUnmapped", name, err)
		}
	}
}
