package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The monarch's reads (CR 725; dirac gap wave 4), in lockstep against gorge:
// Azure Fleet Admiral's "can't be blocked by creatures the monarch controls"
// (CantBlockBy ValidBlocker$ Creature.ControlledBy Player.isMonarch), Fall
// from Favor's Untap replacement (ControlledBy EnchantedController.!isMonarch),
// Archivist of Gondor's "the monarch's end step" (Phase ValidPlayer$
// Player.isMonarch) and the "if there is no monarch" gate of Archivist and
// Crown of Gondor (PlayerCountPlayers$HasPropertyisMonarch), beside monarch
// makers and fliers so the crown changes hands.

// monarchReadsDecks are four 40-card decks of the monarchreads pool; seats 0
// and 1 are the two-seat match.
func monarchReadsDecks() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{ // W/U: the Admiral, the Archivist and the Crown
			{Name: "Azure Fleet Admiral", Count: 3}, {Name: "Archivist of Gondor", Count: 3}, {Name: "Crown of Gondor", Count: 2},
			{Name: "Isamaru, Hound of Konda", Count: 3}, {Name: "Suntail Hawk", Count: 3}, {Name: "Savannah Lions", Count: 3},
			{Name: "Elite Vanguard", Count: 3}, {Name: "Plains", Count: 10}, {Name: "Island", Count: 10},
		},
		{ // W/U: Fall from Favor and the crown makers
			{Name: "Fall from Favor", Count: 4}, {Name: "Palace Sentinels", Count: 3}, {Name: "Wind Drake", Count: 4},
			{Name: "Court of Grace", Count: 2}, {Name: "Savannah Lions", Count: 4}, {Name: "Suntail Hawk", Count: 3},
			{Name: "Plains", Count: 10}, {Name: "Island", Count: 10},
		},
		{ // R/B: more crown makers and burn
			{Name: "Queen Marchesa", Count: 3}, {Name: "Crimson Fleet Commodore", Count: 3}, {Name: "Court of Embereth", Count: 2},
			{Name: "Goblin Piker", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Lightning Bolt", Count: 2}, {Name: "Shock", Count: 2},
			{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 10},
		},
		{ // W/U: all four readers beside fliers
			{Name: "Azure Fleet Admiral", Count: 2}, {Name: "Fall from Favor", Count: 2}, {Name: "Archivist of Gondor", Count: 2},
			{Name: "Isamaru, Hound of Konda", Count: 3}, {Name: "Wind Drake", Count: 4}, {Name: "Suntail Hawk", Count: 4},
			{Name: "Palace Sentinels", Count: 3}, {Name: "Plains", Count: 10}, {Name: "Island", Count: 10},
		},
	}
}

func monarchReadsExpand(d []cardset.Entry) []string {
	var out []string
	for _, e := range d {
		out = append(out, deckOf(e.Count, e.Name)...)
	}
	return out
}

// monarchReadsFacts is what a finished gorge game did with the readers.
type monarchReadsFacts struct {
	crown int // MonarchChange events: the designation moved
	auras int // Fall from Favor attached to a creature
}

func monarchReadsCount(g *GorgeSide) (f monarchReadsFacts) {
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.MonarchChange:
			f.crown++
		case events.Attach:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Fall from Favor" {
				f.auras++
			}
		}
	}
	return f
}

// monarchReadsPod plays the four-seat pod.
type monarchReadsPod struct {
	reg   *cards.Registry
	set   cardset.Set
	tbl   *kop.Table
	decks [4][]cardset.Entry
}

func newMonarchReadsPod(t *testing.T) monarchReadsPod {
	t.Helper()
	decks := monarchReadsDecks()
	reg := synthReg(t)
	set, err := cardset.RawByName("monarchreads")
	if err != nil {
		t.Fatal(err)
	}
	lists := make([][]cardset.Entry, len(decks))
	for i, d := range decks {
		n := 0
		for _, e := range d {
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d: %d cards", i, n)
		}
		lists[i] = d
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	return monarchReadsPod{reg: reg, set: set, tbl: tbl, decks: decks}
}

func (p monarchReadsPod) play(t *testing.T, seed uint64, probe *kmonarchProbe) (Result, monarchReadsFacts) {
	t.Helper()
	d := p.decks
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{monarchReadsExpand(d[0]), monarchReadsExpand(d[1])},
		MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{monarchReadsExpand(d[2]), monarchReadsExpand(d[3])}, Seed: seed, PassElision: true, Surface: "full"}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, 60
	var f monarchReadsFacts
	rc.Finish = func(g *GorgeSide) { f = monarchReadsCount(g) }
	k, err := SetSide(p.set, p.tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := castFirstPolicy(seed)
	policy := PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && probe != nil {
			probe.observe(c.G)
		}
		return base.Pick(c)
	})
	return LockstepLed(m, seed, lead, k, policy, rc), f
}

// monarchReadsSeeds are the seeds each lockstep plays (the count is each test's).
const monarchReadsSeeds = 24

// TestMonarchReadsFourSeatLockstep plays four-seat pods of the monarchreads
// fixture: every decision matches gorge and the kernel never stalls, the crown
// changes hands, Fall from Favor is attached, and Archivist of Gondor's end-step trigger and the "no monarch"
// count are put to use.
func TestMonarchReadsFourSeatLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	p := newMonarchReadsPod(t)
	st := &dirac.MonarchStats{}
	dirac.DebugMonarchStats = st
	defer func() { dirac.DebugMonarchStats = nil }()
	probe := &kmonarchProbe{played: map[string]int{}, abs: map[string]map[uint8]int{}}
	var sum monarchReadsFacts
	for seed := uint64(1); seed <= monarchReadsSeeds; seed++ {
		res, f := p.play(t, seed, probe)
		if res.Div != nil {
			lo := max(res.Div.At-12, 0)
			t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
				res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.crown += f.crown
		sum.auras += f.auras
	}
	t.Logf("kernel: %+v; gorge: %+v", *st, sum)
	if sum.crown == 0 || st.Became == 0 {
		t.Fatalf("the crown never moved (gorge %d, kernel %d)", sum.crown, st.Became)
	}
	if st.ExYes+st.ExNo == 0 {
		t.Fatal("no monarch read was evaluated")
	}
	for _, n := range []string{"Azure Fleet Admiral", "Fall from Favor", "Archivist of Gondor", "Crown of Gondor"} {
		if probe.played[n] == 0 {
			t.Errorf("%s: never on the battlefield or the stack", n)
		}
	}
	if sum.auras == 0 {
		t.Error("Fall from Favor was never attached")
	}
	if probe.abs["Archivist of Gondor"][0x81] == 0 {
		t.Error("Archivist of Gondor's end-step trigger never went on the stack")
	}
}

// monarchReadsTwoSeat is the two-seat match: the first two decks.
func monarchReadsTwoSeat() [2][]cardset.Entry {
	d := monarchReadsDecks()
	return [2][]cardset.Entry{d[0], d[1]}
}

// TestMonarchReadsTwoSeatLockstep is the two-seat form.
func TestMonarchReadsTwoSeatLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := &dirac.MonarchStats{}
	dirac.DebugMonarchStats = st
	defer func() { dirac.DebugMonarchStats = nil }()
	probe := &kmonarchProbe{played: map[string]int{}, abs: map[string]map[uint8]int{}}
	policy := func(seed uint64) Policy {
		base := castFirstPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				probe.observe(c.G)
			}
			return base.Pick(c)
		})
	}
	if div := lockstepRunPolicy(t, "monarchreads", monarchReadsTwoSeat(), monarchReadsSeeds, true, nil, nil, policy); div != 0 {
		t.Errorf("%d of %d seeds diverged", div, monarchReadsSeeds)
	}
	t.Logf("kernel: %+v", *st)
	if st.Became == 0 || st.ExYes+st.ExNo == 0 {
		t.Fatalf("the crown or its reads never happened: %+v", *st)
	}
	for _, n := range []string{"Azure Fleet Admiral", "Fall from Favor", "Archivist of Gondor", "Crown of Gondor"} {
		if probe.played[n] == 0 {
			t.Errorf("%s: never on the battlefield or the stack", n)
		}
	}
}

// TestMonarchFilterMutation inverts every isMonarch read of the monarchreads
// cards (dirac.DebugMonarchFilterInverted): Azure Fleet Admiral's blockers,
// Fall from Favor's untap and Archivist's end-step trigger then follow the
// wrong player, so some seed diverges.
func TestMonarchFilterMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugMonarchFilterInverted = true
	defer func() { dirac.DebugMonarchFilterInverted = false }()
	if !lockstepDiverges(t, "monarchreads", monarchReadsTwoSeat(), monarchReadsSeeds) {
		t.Fatal("no seed diverged with the isMonarch reads inverted")
	}
}
