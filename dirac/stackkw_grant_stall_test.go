package dirac

import (
	"fmt"
	"slices"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A granted Convoke reaches spells kgen refuses for the printed keyword:
// one that also has Improvise, and one with an {X} cost. gorge offers a
// single mixed announcement (ConvokeAsk with Convoke and Improvise, or with
// HasX); the kernel has no such announcement, so the cast stalls unmodelled
// instead of posing one keyword's menu (kw_convoke.go convokeOptions).

// stackKwStallScen is Chief Engineer on the battlefield with spell in hand:
// Chief Engineer grants artifact spells Convoke, Kappa Cannoneer has printed
// Improvise, and Walking Ballista is an {X} artifact spell.
func stackKwStallScen(t *testing.T, spell string) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Chief Engineer", Count: 1},
		{Name: spell, Count: 1},
		{Name: "Island", Count: 8},
	}
	set := cardsettest.Set(t, "stackgrantkw")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if _, ok := id[spell]; !ok {
		t.Fatalf("%s is not in the stackgrantkw pool", spell)
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	sc.lands(0, id["Island"], 8)
	sc.perm(id["Chief Engineer"], 0)
	return sc, sc.hand(id[spell], 0)
}

func stackKwStall(t *testing.T, spell string) {
	t.Helper()
	sc, m := stackKwStallScen(t, spell)
	sc.do(t, AkCast, TCode(m), scAny)
	d := sc.s.Step()
	if !sc.s.Stalled() {
		t.Fatalf("%s: no stall; decision %v/%d options %v", spell, d.Kind, d.Sub, d.Options[:d.N])
	}
	if got := sc.s.StallReason(); got != StallStackKwGrant {
		t.Fatalf("%s: stall %q, want %q", spell, StallName(got), StallName(StallStackKwGrant))
	}
	if !StallIsUnmodelled(StallStackKwGrant) {
		t.Fatalf("%q is not classed unmodelled", StallName(StallStackKwGrant))
	}
	if d.Kind != DkNone {
		t.Fatalf("%s: posed decision %v/%d past the stall", spell, d.Kind, d.Sub)
	}
}

// stackKwBoardScen is stackKwStallScen with a board for the mixed menus:
// Pilgrim's Eye (an artifact creature), Grizzly Bears and Mind Stone on seat
// 0 beside Chief Engineer, eight Islands, and spell in hand. The returned map
// names every card of the deck.
func stackKwBoardScen(t *testing.T, spell string) (*scen, Obj, map[string]Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Chief Engineer", Count: 1},
		{Name: spell, Count: 1},
		{Name: "Pilgrim's Eye", Count: 1},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Mind Stone", Count: 1},
		{Name: "Island", Count: 8},
	}
	set := cardsettest.Set(t, "stackgrantkw")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	sc.lands(0, id["Island"], 8)
	objs := map[string]Obj{"Chief Engineer": sc.perm(id["Chief Engineer"], 0)}
	for _, n := range []string{"Pilgrim's Eye", "Grizzly Bears", "Mind Stone"} {
		objs[n] = sc.perm(id[n], 0)
	}
	return sc, sc.hand(id[spell], 0), objs
}

// convokeMenu returns the convoke ask's picks as a sorted set and whether it
// offers Done.
func convokeMenu(t *testing.T, sc *scen, d Decision) ([]string, bool) {
	t.Helper()
	if d.Kind != DkChoose || d.Sub != ChGenConvoke {
		t.Fatalf("decision %v/%d, want the convoke ask", d.Kind, d.Sub)
	}
	var out []string
	done := false
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkDone {
			done = true
			continue
		}
		out = append(out, fmt.Sprintf("%s/%d", sc.k.t.Cards[sc.s.objs[Obj(a.Source())].card].Name, a.Mode()))
	}
	sort.Strings(out)
	return out, done
}

func wantMenu(t *testing.T, got []string, done bool, want ...string) {
	t.Helper()
	sort.Strings(want)
	if !done || !slices.Equal(got, want) {
		t.Fatalf("menu %v (done %v), want %v and Done", got, done, want)
	}
}

// TestStackKwGrantMixedKeywordsAnnounces: Chief Engineer grants Convoke to
// Kappa Cannoneer, which has printed Improvise. gorge offers creature colour,
// creature generic and artifact generic picks in one announcement
// (pay.ConvokeAsk); an artifact creature carries both generic options.
func TestStackKwGrantMixedKeywordsAnnounces(t *testing.T) {
	sc, m, _ := stackKwBoardScen(t, "Kappa Cannoneer")
	sc.do(t, AkCast, TCode(m), scAny)
	d := sc.s.Step()
	if sc.s.Stalled() {
		t.Fatalf("stalled: %q", StallName(sc.s.StallReason()))
	}
	got, done := convokeMenu(t, sc, d)
	wantMenu(t, got, done, "Chief Engineer/1", "Chief Engineer/5", "Pilgrim's Eye/5", "Pilgrim's Eye/6",
		"Grizzly Bears/5", "Mind Stone/6")
}

// TestStackKwGrantXCostAnnounces: Chief Engineer grants Convoke to Walking
// Ballista ({X}{X}). gorge announces before X and offers every creature a
// possible generic payment although the composed generic is {0}.
func TestStackKwGrantXCostAnnounces(t *testing.T) {
	sc, m, _ := stackKwBoardScen(t, "Walking Ballista")
	sc.do(t, AkCast, TCode(m), scAny)
	d := sc.s.Step()
	if sc.s.Stalled() {
		t.Fatalf("stalled: %q", StallName(sc.s.StallReason()))
	}
	got, done := convokeMenu(t, sc, d)
	wantMenu(t, got, done, "Chief Engineer/5", "Pilgrim's Eye/5", "Grizzly Bears/5")
	// Walking Ballista has no Improvise: only the grant is live, so the
	// artifacts offer no improvise pick. (The kernel's pick cap, the option
	// count under {X}, is not observable on the Decision.)
}

// TestStackKwGrantXCostPickPaysX: a generic pick announced before X pays one
// generic of the X total: X = 0 is not offered, X = 2 taps three Islands.
func TestStackKwGrantXCostPickPaysX(t *testing.T) {
	sc, m, objs := stackKwBoardScen(t, "Walking Ballista")
	sc.do(t, AkCast, TCode(m), scAny)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenConvoke {
		t.Fatalf("decision %v/%d, want the convoke ask", d.Kind, d.Sub)
	}
	pe := objs["Pilgrim's Eye"]
	sc.do(t, AkChoose, TCode(pe), scAny)
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenConvoke {
		t.Fatalf("decision %v/%d after the pick, want the convoke re-ask", d.Kind, d.Sub)
	}
	sc.do(t, AkDone, scAny, scAny)
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChX {
		t.Fatalf("decision %v/%d, want the X ask", d.Kind, d.Sub)
	}
	has := map[uint8]bool{}
	for i := 0; i < int(d.N); i++ {
		has[d.Options[i].Mode()] = true
	}
	if has[0] || !has[1] {
		t.Fatalf("X menu %v: want x(1) and no x(0)", has)
	}
	sc.chooseX(t, 2)
	sc.passBoth(t)
	if !sc.onBF(m) {
		t.Fatalf("Walking Ballista did not resolve")
	}
	if n := sc.s.perms[sc.slot(m)].ctrLow(int(kop.CtrP1P1)); n != 2 {
		t.Fatalf("%d +1/+1 counters, want 2", n)
	}
	if !sc.tapped(pe) {
		t.Fatalf("the convoke pick is untapped")
	}
	islands := 0
	for sl := Slot(0); sl < MaxPerm; sl++ {
		if sc.s.occ.has(sl) && sc.k.t.Cards[sc.s.objs[sc.s.perms[sl].obj].card].Name == "Island" && sc.s.perms[sl].flags&pfTapped != 0 {
			islands++
		}
	}
	if islands != 3 {
		t.Fatalf("%d Islands tapped, want 3", islands)
	}
}

// TestStackKwGrantPlainCastDoesNotStall: the same grant on a spell without
// Improvise or {X} still announces and casts (the guard is narrow).
func TestStackKwGrantPlainCastDoesNotStall(t *testing.T) {
	sc, m := stackKwStallScen(t, "Pilgrim's Eye")
	sc.do(t, AkCast, TCode(m), scAny)
	d := sc.s.Step()
	if sc.s.Stalled() {
		t.Fatalf("Pilgrim's Eye stalled: %q", StallName(sc.s.StallReason()))
	}
	if d.Kind != DkChoose || d.Sub != ChGenConvoke {
		t.Fatalf("Pilgrim's Eye: decision %v/%d, want the convoke ask", d.Kind, d.Sub)
	}
}

// TestStackKwGrantHarmonizeStalls: Chief Engineer grants Convoke to a spell
// cast with Harmonize. gorge's convokeAsk offers harmonize and convoke picks
// together; the kernel would pose harmonize only, so it stalls.
func TestStackKwGrantHarmonizeStalls(t *testing.T) {
	sc, m := stackKwStallScen(t, "Pilgrim's Eye")
	k := sc.k
	k.t.Cards[sc.s.objs[m].card].Harmonize = true
	sc.do(t, AkCast, TCode(m), scAny)
	sc.s.bld().e.flags |= sfFlashback
	if !harmonizeEntry(&k.t.Cards[sc.s.bld().e.card], &sc.s.bld().e) {
		t.Fatal("entry is not a harmonize cast")
	}
	d := sc.s.Step()
	if !sc.s.Stalled() || sc.s.StallReason() != StallStackKwGrant {
		t.Fatalf("no StallStackKwGrant; stalled=%v decision %v/%d", sc.s.Stalled(), d.Kind, d.Sub)
	}
	if d.Kind != DkNone {
		t.Fatalf("posed decision %v/%d past the stall", d.Kind, d.Sub)
	}
}

// TestStackKwGrantAltRouteStalls: a granted Convoke on a card with an
// alternative-cost route (here Plot) stalls rather than pose a menu
// unverified against gorge. Impending is no such route: buildCost prices its
// impending cost, the cost gorge's convoke ask reads.
func TestStackKwGrantAltRouteStalls(t *testing.T) {
	sc, m := stackKwStallScen(t, "Pilgrim's Eye")
	sc.k.t.Cards[sc.s.objs[m].card].HasPlot = true
	sc.do(t, AkCast, TCode(m), scAny)
	d := sc.s.Step()
	if !sc.s.Stalled() || sc.s.StallReason() != StallStackKwGrant || d.Kind != DkNone {
		t.Fatalf("no StallStackKwGrant; stalled=%v decision %v/%d", sc.s.Stalled(), d.Kind, d.Sub)
	}
}
