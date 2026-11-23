package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Typed mana from a resolving DB$ Mana of an artifact source (CR 605.1a, 106.3):
// gorge tags the mana a resolving trigger or spell adds by its source's face
// types (effects.ManaProducerTag reads the printed face), so a Myr Moonvessel's
// death trigger and a The Vision and Scarlet Witch's cast trigger add typed
// units the planner reads apart. The pods play the typedartmana fixture; the
// artifact mana floats into planned payments (casts) and the manual window
// (activations) and is spent there.

// typedartmanaDecks are the pods' four decks: the two artifact carriers named by
// the lifted refusal beside the spells and sinks their triggered mana pays for,
// a Treasure maker as a second tagged source on the mana-ability path, and the
// Spirit Guides whose hand mana floats beside the tagged units, so the tables
// hold the tagged-unit tally (mana_artifact.go coldArtN) gorge keys its prefix
// search on and the same colour holds plain and tagged units both.
func typedartmanaDecks() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{
			{Name: "Myr Moonvessel", Count: 2}, {Name: "The Vision and Scarlet Witch", Count: 2},
			{Name: "Cathodion", Count: 2}, {Name: "Su-Chi", Count: 2}, {Name: "Vulshok Morningstar", Count: 2},
			{Name: "Simian Spirit Guide", Count: 2}, {Name: "Tanglebloom", Count: 2}, {Name: "Lightning Bolt", Count: 2}, {Name: "Shock", Count: 2},
			{Name: "Grizzly Bears", Count: 2}, {Name: "Prodigal Sorcerer", Count: 2},
			{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 8},
		},
		{
			{Name: "Myr Moonvessel", Count: 2}, {Name: "The Vision and Scarlet Witch", Count: 2},
			{Name: "Prosperous Innkeeper", Count: 2}, {Name: "Walking Ballista", Count: 3}, {Name: "Divination", Count: 2},
			{Name: "Shock", Count: 2}, {Name: "Tithe Drinker", Count: 2}, {Name: "Vodalian Mage", Count: 2}, {Name: "Savannah Lions", Count: 2}, {Name: "Hill Giant", Count: 3},
			{Name: "Plains", Count: 6}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 8},
		},
		{
			{Name: "Myr Moonvessel", Count: 2}, {Name: "The Vision and Scarlet Witch", Count: 2},
			{Name: "Elvish Spirit Guide", Count: 2}, {Name: "Tanglebloom", Count: 2}, {Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 3},
			{Name: "Craw Wurm", Count: 4}, {Name: "Giant Growth", Count: 3},
			{Name: "Forest", Count: 9}, {Name: "Mountain", Count: 9},
		},
		{
			{Name: "Myr Moonvessel", Count: 2}, {Name: "The Vision and Scarlet Witch", Count: 2},
			{Name: "Simian Spirit Guide", Count: 2}, {Name: "Pitiless Plunderer", Count: 2}, {Name: "Prodigal Sorcerer", Count: 3},
			{Name: "Walking Ballista", Count: 2}, {Name: "Serra Angel", Count: 2}, {Name: "Divination", Count: 3},
			{Name: "Vodalian Mage", Count: 2}, {Name: "Giant Growth", Count: 2}, {Name: "Island", Count: 4}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 8},
		},
	}
}

type typedartmanaPod struct {
	reg   *cards.Registry
	set   cardset.Set
	decks [][]cardset.Entry
}

func newTypedartmanaPod(t *testing.T, n int) typedartmanaPod {
	t.Helper()
	// The corpus skip comes before the fixture table read: a fresh clone has no
	// .cards (and so no tables), and must SKIP rather than fail on RawByName.
	reg := synthReg(t)
	set, err := cardset.RawByName("typedartmana")
	if err != nil {
		t.Fatal(err)
	}
	all := typedartmanaDecks()
	p := typedartmanaPod{reg: reg, set: set}
	for i := 0; i < n; i++ {
		c := 0
		for _, e := range all[i] {
			c += e.Count
		}
		if c != 40 {
			t.Fatalf("deck %d: %d cards", i, c)
		}
		p.decks = append(p.decks, all[i])
	}
	return p
}

func (p typedartmanaPod) play(t *testing.T, seed uint64, maxTurns int32) (Result, dirac.TypedManaStats) {
	t.Helper()
	st := &dirac.TypedManaStats{}
	dirac.DebugTypedManaStats = st
	defer func() { dirac.DebugTypedManaStats = nil }()
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expand(p.decks[0]), expand(p.decks[1])},
		Seed: seed, PassElision: true, Surface: "full"}
	if len(p.decks) == 4 {
		r.MoreDecks = []string{"x2", "x3"}
		r.MoreCards = [][]string{expand(p.decks[2]), expand(p.decks[3])}
	}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	tbl, _, err := p.set.MatchupN(p.decks)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(p.set, tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	return LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc), *st
}

func typedartmanaFail(t *testing.T, seed uint64, res Result) {
	t.Helper()
	lo := max(res.Div.At-12, 0)
	t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
		res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
}

var typedartmanaSeeds = []uint64{1, 2, 3, 4}

func typedartmanaLockstep(t *testing.T, seats int) {
	p := newTypedartmanaPod(t, seats)
	var sum dirac.TypedManaStats
	for _, seed := range typedartmanaSeeds {
		res, st := p.play(t, seed, 40)
		if res.Div != nil {
			typedartmanaFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.Added += st.Added
		sum.Planned += st.Planned
		sum.Manual += st.Manual
		t.Logf("seed %d: %d decisions, over %v %s; typed added %d, spent planned %d manual %d", seed, res.Decisions, res.Over, res.Stall, st.Added, st.Planned, st.Manual)
	}
	if sum.Added == 0 {
		t.Fatal("no game resolved a DB$ Mana of an artifact source")
	}
	if seats == 4 && (sum.Planned == 0 || sum.Manual == 0) {
		t.Fatalf("typed mana added %d, spent planned %d manual %d: want each above zero", sum.Added, sum.Planned, sum.Manual)
	}
	if seats == 2 && sum.Planned+sum.Manual == 0 {
		t.Fatalf("typed mana added %d was never spent: want a planned or manual spend", sum.Added)
	}
}

// TestTypedArtifactManaFourSeatLockstep plays four-seat pods of the typedartmana
// fixture: no divergence and no stall, and the artifact mana a resolving
// DB$ Mana added was spent.
func TestTypedArtifactManaFourSeatLockstep(t *testing.T) { typedartmanaLockstep(t, 4) }

// TestTypedArtifactManaTwoSeatLockstep is the same over the first two decks.
func TestTypedArtifactManaTwoSeatLockstep(t *testing.T) { typedartmanaLockstep(t, 2) }

// TestTypedArtifactManaMutation is the mutation test. The design's step 1
// (the refusal lifted, no kernel tag) found no divergence at either seat
// count, and no step-3 rule exists (gorge's planner reads a typed unit as
// plain mana of its colour), so the mutation's observable is the tag-count
// projection: with DebugTypedManaNoTag the resolving path tags nothing and
// the kernel's tagged-unit tally (and with it the stats) never moves, where
// the unmutated run's does.
func TestTypedArtifactManaMutation(t *testing.T) {
	p := newTypedartmanaPod(t, 4)
	res, st := p.play(t, typedartmanaSeeds[0], 40)
	if res.Div != nil {
		typedartmanaFail(t, typedartmanaSeeds[0], res)
	}
	if st.Added == 0 {
		t.Fatal("unmutated run tagged no resolving typed mana")
	}
	dirac.DebugTypedManaNoTag = true
	defer func() { dirac.DebugTypedManaNoTag = false }()
	res, st = p.play(t, typedartmanaSeeds[0], 40)
	if res.Div != nil {
		typedartmanaFail(t, typedartmanaSeeds[0], res)
	}
	if st.Added != 0 {
		t.Fatalf("DebugTypedManaNoTag still tagged %d units", st.Added)
	}
}
