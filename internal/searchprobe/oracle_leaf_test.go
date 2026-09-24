package searchprobe

import (
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// oppHandNames is every non-actor hand in v, sorted, joined.
func oppHandNames(v view.View, actor state.PlayerID) string {
	var names []string
	for _, p := range v.Players {
		if p.ID == actor {
			continue
		}
		for _, cv := range p.Hand {
			names = append(names, cv.Name)
		}
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

// handLeaf is a pure leaf that reads ONLY the opponents' hands: a hash of
// their card names squashed into (0,1), 0.5 when no opponent hand is visible.
func handLeaf(v view.View, actor state.PlayerID) float64 {
	s := oppHandNames(v, actor)
	if s == "" {
		return 0.5
	}
	h := fnv.New64a()
	h.Write([]byte(s))
	return 0.05 + 0.9*float64(h.Sum64()%1009)/1009
}

// swappedOppHand is a clone of w whose opponent hand differs from w's in one
// card and nothing else a view can show: one opponent hand card and one
// differently named opponent library card trade zones (two MoveZone events,
// so hand and library sizes are unchanged and library order is hidden).
func swappedOppHand(t *testing.T, w World) (World, string, string) {
	t.Helper()
	e := w.Engine.Clone()
	actor := e.Pending().Player
	var opp state.PlayerID
	for _, p := range e.G.Players {
		if p.ID != actor {
			opp = p.ID
		}
	}
	name := func(id state.ObjID) string { return e.G.Obj(id).Face().Name }
	hand, lib := e.G.Zone(state.ZHand, opp), e.G.Zone(state.ZLibrary, opp)
	if len(hand) == 0 {
		t.Fatal("fixture opponent hand is empty")
	}
	h := hand[0]
	for _, l := range lib {
		if name(l) == name(h) {
			continue
		}
		hn, ln := name(h), name(l)
		e.Emit(events.Event{Kind: events.MoveZone, Player: opp, Obj: h, From: state.ZHand, To: state.ZLibrary})
		e.Emit(events.Event{Kind: events.MoveZone, Player: opp, Obj: l, From: state.ZLibrary, To: state.ZHand})
		if len(e.G.Zone(state.ZHand, opp)) != len(hand) || len(e.G.Zone(state.ZLibrary, opp)) != len(lib) {
			t.Fatal("swap changed a zone size")
		}
		return World{Engine: e, Observer: w.Observer}, hn, ln
	}
	t.Fatal("no differently named library card to swap in")
	return World{}, "", ""
}

// The omniscient leaf reads the SAMPLED world's opponent hand: two worlds that
// differ only in that hand score differently under a hand-reading leaf, and
// the view it gets holds exactly the world's hand. The redacted leaf never
// sees an opponent hand, so the same two worlds score identically.
func TestTeacherChoiceOmniscientLeafSeesTheSampledOpponentHand(t *testing.T) {
	// The real-deck bench fixture: the synthetic leafFixture's decks are one
	// card name throughout, so no swap there could change a hand's names.
	worlds, cands := benchTeacherInputs(t)
	if len(cands) < 2 {
		t.Fatalf("bench fixture offers %d candidates", len(cands))
	}
	cands = cands[:2]
	a := worlds[0]
	b, out, in := swappedOppHand(t, a)
	actor := a.Engine.Pending().Player
	oppHand := func(w World) string {
		return oppHandNames(view.ProjectFor(w.Engine.G, w.Engine, actor, view.Omniscient, nil), actor)
	}
	if oppHand(a) == oppHand(b) {
		t.Fatalf("swap %s -> %s left the opponent hand unchanged", out, in)
	}
	// MaxSubmits 1: every rollout is the root candidate alone, then a capped
	// (non-terminal) leaf, so no bot decision can redraw the hands.
	run := func(w World, omni bool) (TeacherResult, []string) {
		var seen []string
		leaf := func(v view.View, actor state.PlayerID) float64 {
			seen = append(seen, oppHandNames(v, actor))
			return handLeaf(v, actor)
		}
		res, err := TeacherChoice([]World{w}, cands, TeacherOptions{Seed: 3, MaxSubmits: 1, Leaf: leaf, LeafOmniscient: omni})
		if err != nil {
			t.Fatal(err)
		}
		if res.Capped != res.Rollouts || len(seen) != res.Rollouts {
			t.Fatalf("want every rollout capped and scored once: %d calls, %+v", len(seen), res)
		}
		return res, seen
	}

	ra, seenA := run(a, true)
	rb, seenB := run(b, true)
	for i := range seenA {
		if seenA[i] != oppHand(a) || seenB[i] != oppHand(b) {
			t.Fatalf("omniscient leaf %d saw %q / %q, want the worlds' hands %q / %q", i, seenA[i], seenB[i], oppHand(a), oppHand(b))
		}
	}
	if math.Float64bits(ra.Values[0]) == math.Float64bits(rb.Values[0]) {
		t.Fatalf("omniscient leaf value unchanged by the opponent hand: %v vs %v", ra.Values, rb.Values)
	}

	ra, seenA = run(a, false)
	rb, seenB = run(b, false)
	for i := range seenA {
		if seenA[i] != "" || seenB[i] != "" {
			t.Fatalf("redacted leaf saw an opponent hand: %q / %q", seenA[i], seenB[i])
		}
	}
	for i := range ra.Values {
		if math.Float64bits(ra.Values[i]) != math.Float64bits(rb.Values[i]) {
			t.Fatalf("redacted leaf changed with the hidden opponent hand: %v vs %v", ra.Values, rb.Values)
		}
	}
}

// The heuristic leaf reads hand SIZES only, so LeafOmniscient with a nil Leaf
// is still the pre-Leaf golden bit for bit (TestTeacherChoiceNilLeafIs...).
func TestTeacherChoiceOmniscientNilLeafIsTheHeuristicBitForBit(t *testing.T) {
	worlds, cands := leafFixture(t)
	base, err := TeacherChoice(worlds, cands, TeacherOptions{Seed: 3, HorizonTurns: 2, MaxSubmits: 5000})
	if err != nil {
		t.Fatal(err)
	}
	omni, err := TeacherChoice(worlds, cands, TeacherOptions{Seed: 3, HorizonTurns: 2, MaxSubmits: 5000, LeafOmniscient: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []uint64{0x3fe06660f0a5ea1e, 0x3fe06660f0a5ea1e, 0}
	for i := range want {
		if math.Float64bits(base.Values[i]) != want[i] || math.Float64bits(omni.Values[i]) != want[i] {
			t.Fatalf("value %d: base %#x omni %#x, want %#x", i, math.Float64bits(base.Values[i]), math.Float64bits(omni.Values[i]), want[i])
		}
	}
	if base.Index != omni.Index || base.Submits != omni.Submits || base.Terminal != omni.Terminal {
		t.Fatalf("counts differ: %+v vs %+v", base, omni)
	}
}

// The omniscient leaf is refused on the clairvoyant ceiling: that world is
// the real engine, so the leaf would read the real opponent's hand.
func TestTeacherChoiceRefusesOmniscientLeafOnClairvoyant(t *testing.T) {
	worlds, cands := leafFixture(t)
	_, err := TeacherChoice(worlds, cands, TeacherOptions{Seed: 3, MaxSubmits: 5000, Clairvoyant: true, LeafOmniscient: true, Leaf: handLeaf})
	if err == nil || !strings.Contains(err.Error(), "clairvoyant") {
		t.Fatalf("want the clairvoyant refusal, got %v", err)
	}
}
