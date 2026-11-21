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

// spectacleRiotDecks are the fixture's four decks: a red-green riot deck, a
// black-red spectacle deck, and two mixed ones, each with burn or evasive
// creatures so an opponent loses life early and the spectacle cast opens.
func spectacleRiotDecks() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{
			{Name: "Burning-Tree Vandal", Count: 2}, {Name: "Clamor Shaman", Count: 2}, {Name: "Ghor-Clan Wrecker", Count: 2}, {Name: "Zhur-Taa Goblin", Count: 2},
			{Name: "Frenzied Arynx", Count: 2}, {Name: "Gruul Spellbreaker", Count: 2}, {Name: "Gruul Beastmaster", Count: 2}, {Name: "Rampaging Rendhorn", Count: 2},
			{Name: "Wrecking Beast", Count: 1}, {Name: "Skarrgan Hellkite", Count: 2}, {Name: "Arcbound Slasher", Count: 2}, {Name: "Lightning Bolt", Count: 2},
			{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 8},
		},
		{
			{Name: "Skewer the Critics", Count: 3}, {Name: "Light Up the Stage", Count: 2}, {Name: "Rafter Demon", Count: 3}, {Name: "Rix Maadi Reveler", Count: 3},
			{Name: "Hackrobat", Count: 2}, {Name: "Spikewheel Acrobat", Count: 1}, {Name: "Blade Juggler", Count: 1}, {Name: "Spawn of Mayhem", Count: 2},
			{Name: "Dead Revels", Count: 1}, {Name: "Drill Bit", Count: 1}, {Name: "Body Count", Count: 1}, {Name: "Lightning Bolt", Count: 2},
			{Name: "Shock", Count: 2}, {Name: "Vampire Interloper", Count: 2}, {Name: "Raging Goblin", Count: 2}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		},
		{
			{Name: "Skewer the Critics", Count: 2}, {Name: "Light Up the Stage", Count: 1}, {Name: "Spikewheel Acrobat", Count: 2}, {Name: "Rix Maadi Reveler", Count: 2},
			{Name: "Burning-Tree Vandal", Count: 2}, {Name: "Clamor Shaman", Count: 2}, {Name: "Zhur-Taa Goblin", Count: 2}, {Name: "Ghor-Clan Wrecker", Count: 2},
			{Name: "Arcbound Slasher", Count: 1}, {Name: "Skarrgan Hellkite", Count: 1}, {Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 2},
			{Name: "Searing Spear", Count: 2}, {Name: "Raging Goblin", Count: 2}, {Name: "Mountain", Count: 14},
		},
		{
			{Name: "Blade Juggler", Count: 3}, {Name: "Dead Revels", Count: 2}, {Name: "Body Count", Count: 2}, {Name: "Spawn of Mayhem", Count: 2},
			{Name: "Drill Bit", Count: 2}, {Name: "Gruul Beastmaster", Count: 3}, {Name: "Rampaging Rendhorn", Count: 4}, {Name: "Wrecking Beast", Count: 2},
			{Name: "Vampire Interloper", Count: 4}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 8},
		},
	}
}

// spectacleRiotTally is what a game exercised, counted off gorge's event log.
type spectacleRiotTally struct {
	spectacle      int // spells cast for their spectacle cost
	etbReaders     int // of those, Rafter Demon and Rix Maadi Reveler, whose enters trigger reads "its spectacle cost was paid"
	counter, haste int // riot answers
}

func (a *spectacleRiotTally) add(b spectacleRiotTally) {
	a.spectacle += b.spectacle
	a.etbReaders += b.etbReaders
	a.counter += b.counter
	a.haste += b.haste
}

func countSpectacleRiot(g *GorgeSide) (c spectacleRiotTally) {
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.CastInfo:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && strings.Contains(e.Counter, "altcostpaid") {
				if _, ok := o.Face().KeywordParam("Spectacle"); ok {
					c.spectacle++
					if n := o.Face().Name; n == "Rafter Demon" || n == "Rix Maadi Reveler" {
						c.etbReaders++
					}
				}
			}
		case events.Choose:
			if e.Counter == "riot" {
				if e.Text == "counter" {
					c.counter++
				} else if e.Text == "haste" {
					c.haste++
				}
			}
		}
	}
	return c
}

type spectacleRiotPod struct {
	reg   *cards.Registry
	set   cardset.Set
	tbl   *kop.Table
	decks [][]cardset.Entry
}

// newSpectacleRiotPod builds the table of the first n decks of the fixture.
func newSpectacleRiotPod(t *testing.T, n int) spectacleRiotPod {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("spectacleriot")
	if err != nil {
		t.Fatal(err)
	}
	all := spectacleRiotDecks()
	lists := make([][]cardset.Entry, n)
	for i := range lists {
		c := 0
		for _, e := range all[i] {
			c += e.Count
		}
		if c != 40 {
			t.Fatalf("deck %d: %d cards", i, c)
		}
		lists[i] = all[i]
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	return spectacleRiotPod{reg: reg, set: set, tbl: tbl, decks: lists}
}

func (p spectacleRiotPod) play(t *testing.T, seed uint64, maxTurns int32) (Result, spectacleRiotTally, dirac.SpectacleStats, dirac.RiotStats) {
	t.Helper()
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expand(p.decks[0]), expand(p.decks[1])}, Seed: seed, PassElision: true, Surface: "full"}
	for i := 2; i < len(p.decks); i++ {
		r.MoreDecks = append(r.MoreDecks, "x"+string(rune('0'+i)))
		r.MoreCards = append(r.MoreCards, expand(p.decks[i]))
	}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	sp, rt := &dirac.SpectacleStats{}, &dirac.RiotStats{}
	dirac.DebugSpectacleStats, dirac.DebugRiotStats = sp, rt
	defer func() { dirac.DebugSpectacleStats, dirac.DebugRiotStats = nil, nil }()
	var c spectacleRiotTally
	rc.Finish = func(g *GorgeSide) { c = countSpectacleRiot(g) }
	k, err := SetSide(p.set, p.tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc)
	return res, c, *sp, *rt
}

func spectacleRiotFail(t *testing.T, seed uint64, res Result) {
	t.Helper()
	lo := max(res.Div.At-12, 0)
	t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
		res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
}

// spectacleRiotSeeds are the seeds the lockstep plays.
var spectacleRiotSeeds = func() (s []uint64) {
	for i := uint64(1); i <= 60; i++ {
		s = append(s, i)
	}
	return s
}()

func runSpectacleRiot(t *testing.T, seats int) {
	p := newSpectacleRiotPod(t, seats)
	var sum spectacleRiotTally
	var sp dirac.SpectacleStats
	// (the four-seat pods rarely draw the two cards whose enters trigger reads
	// the word, so only the two-seat run asserts it)
	for _, seed := range spectacleRiotSeeds {
		res, c, st, _ := p.play(t, seed, 40)
		if res.Div != nil {
			spectacleRiotFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.add(c)
		sp.Offers += st.Offers
		sp.Paid += st.Paid
		t.Logf("seed %d: %d decisions, over %v %s; %d spectacle cast(s), riot %d counter / %d haste, %d offer(s)", seed, res.Decisions, res.Over, res.Stall, c.spectacle, c.counter, c.haste, st.Offers)
	}
	if sum.spectacle == 0 || sp.Offers == 0 {
		t.Fatalf("spectacle casts %d, offers %d: want both above zero", sum.spectacle, sp.Offers)
	}
	if seats == 2 && sum.etbReaders == 0 {
		t.Fatalf("no Rafter Demon or Rix Maadi Reveler was cast for its spectacle cost: the `spectacle` word was never read")
	}
	if sum.counter == 0 || sum.haste == 0 {
		t.Fatalf("riot answers %d counter / %d haste: want both above zero", sum.counter, sum.haste)
	}
}

// TestSpectacleRiotFourSeatLockstep plays four-seat pods of the spectacleriot
// fixture: the hand offers the spectacle cast once any of the three opponents
// lost life this turn, and a riot creature asks counter or haste as it
// enters. No divergence and no stall; both riot answers and the spectacle cast
// are exercised.
func TestSpectacleRiotFourSeatLockstep(t *testing.T) { runSpectacleRiot(t, 4) }

// TestSpectacleRiotTwoSeatLockstep is the same pod at two seats.
func TestSpectacleRiotTwoSeatLockstep(t *testing.T) { runSpectacleRiot(t, 2) }

// TestSpectacleGateMutation is the mutation test: offering the spectacle cast
// without the "an opponent lost life this turn" gate adds an option gorge does
// not list, so it must diverge.
func TestSpectacleGateMutation(t *testing.T) {
	p := newSpectacleRiotPod(t, 2)
	dirac.DebugSpectacleAlwaysOpen = true
	defer func() { dirac.DebugSpectacleAlwaysOpen = false }()
	for _, seed := range spectacleRiotSeeds {
		if res, _, _, _ := p.play(t, seed, 40); res.Div != nil {
			t.Logf("mutation diverged at %d (seed %d): %s %s", res.Div.At, seed, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("opening the spectacle gate diverged on no seed")
}

// TestRiotHasteMutation: dropping the haste half of the election leaves a
// creature sick that gorge lets attack, so it must diverge.
func TestRiotHasteMutation(t *testing.T) {
	p := newSpectacleRiotPod(t, 2)
	dirac.DebugRiotNoHaste = true
	defer func() { dirac.DebugRiotNoHaste = false }()
	for _, seed := range spectacleRiotSeeds {
		if res, _, _, _ := p.play(t, seed, 40); res.Div != nil {
			t.Logf("mutation diverged at %d (seed %d): %s %s", res.Div.At, seed, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("dropping riot's haste diverged on no seed")
}
