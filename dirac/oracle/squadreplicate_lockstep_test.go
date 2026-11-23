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

// squadReplicateCounts is one game's Squad and Replicate payment proofs:
// CopyPermanent's mint per squad payment and CopySpellAbility's copy per
// replicate payment, both read off gorge's event log (the two sides' events
// are compared decision by decision, so a green game proves nothing unless a
// payment actually happened).
type squadReplicateCounts struct {
	squad     int // events.CopyToken: a token copy of the squad creature
	replicate int // events.StackCopy: a copied spell
}

func countSquadReplicateEvents(g *GorgeSide) (c squadReplicateCounts) {
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.CopyToken:
			c.squad++
		case events.StackCopy:
			c.replicate++
		}
	}
	return c
}

// squadReplicatePodEntries is the fixture's four decks: Squad creatures and
// Replicate spells beside the non-mana payments (Exterminate!'s tapXType,
// Reiterating Bolt's PayEnergy, Ruthless Radrat's ExileFromGrave,
// Thrill-Kill Disciple's Discard), basic lands and cheap creatures so the
// counts are reachable.
func squadReplicatePodEntries() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{
			{Name: "Gary Clone", Count: 3}, {Name: "Imperial Hullbreachers", Count: 2}, {Name: "Securitron Squadron", Count: 2},
			{Name: "Space Marine Devastator", Count: 2}, {Name: "Ultramarines Honour Guard", Count: 2}, {Name: "Zephyrim", Count: 2},
			{Name: "Arco-Flagellant", Count: 2}, {Name: "Wasteland Raider", Count: 1}, {Name: "Ruthless Radrat", Count: 1},
			{Name: "Savannah Lions", Count: 4}, {Name: "Grizzly Bears", Count: 2},
			{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 5}, {Name: "Forest", Count: 4},
		},
		{
			{Name: "Exterminate!", Count: 3}, {Name: "Reiterating Bolt", Count: 3}, {Name: "Ruthless Radrat", Count: 3},
			{Name: "Thrill-Kill Disciple", Count: 3}, {Name: "Wasteland Raider", Count: 2}, {Name: "Arco-Flagellant", Count: 2},
			{Name: "Powder Ganger", Count: 2}, {Name: "Roadkill Rodney", Count: 2}, {Name: "Lightning Bolt", Count: 2},
			{Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 7}, {Name: "Forest", Count: 4},
		},
		{
			{Name: "Gigadrowse", Count: 3}, {Name: "Lose Focus", Count: 3}, {Name: "Sicarian Infiltrator", Count: 3},
			{Name: "Thunderheads", Count: 3}, {Name: "Train of Thought", Count: 3}, {Name: "Vacuumelt", Count: 3},
			{Name: "Super Combo", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Giant Growth", Count: 2},
			{Name: "Island", Count: 10}, {Name: "Forest", Count: 6},
		},
		{
			{Name: "Pyromatics", Count: 3}, {Name: "Shattering Spree", Count: 3}, {Name: "Siege of Towers", Count: 3},
			{Name: "Leap of Flame", Count: 3}, {Name: "Reiterating Bolt", Count: 2}, {Name: "Changing Loyalty", Count: 2},
			{Name: "Super Combo", Count: 2}, {Name: "Galadhrim Brigade", Count: 2}, {Name: "Grizzly Bears", Count: 2},
			{Name: "Mountain", Count: 7}, {Name: "Island", Count: 5}, {Name: "Forest", Count: 4}, {Name: "Swamp", Count: 2},
		},
	}
}

type squadReplicatePod struct {
	reg   *cards.Registry
	set   cardset.Set
	tbl   *kop.Table
	decks [4][]cardset.Entry
}

func newSquadReplicatePod(t *testing.T) squadReplicatePod {
	t.Helper()
	return newSquadReplicatePodOf(t, squadReplicatePodEntries())
}

func newSquadReplicatePodOf(t *testing.T, decks [4][]cardset.Entry) squadReplicatePod {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("squadreplicate")
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
	return squadReplicatePod{reg: reg, set: set, tbl: tbl, decks: decks}
}

func expandSquadReplicateDeck(d []cardset.Entry) []string {
	var out []string
	for _, e := range d {
		for j := 0; j < e.Count; j++ {
			out = append(out, e.Name)
		}
	}
	return out
}

// play plays one seed of the four-seat pod and returns the decision log and
// the Squad/Replicate proof counts.
func (p squadReplicatePod) play(t *testing.T, seed uint64, maxTurns int32) (Result, squadReplicateCounts) {
	t.Helper()
	decks := p.decks
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expandSquadReplicateDeck(decks[0]), expandSquadReplicateDeck(decks[1])},
		MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{expandSquadReplicateDeck(decks[2]), expandSquadReplicateDeck(decks[3])}, Seed: seed, PassElision: true, Surface: "full"}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	var c squadReplicateCounts
	rc.Finish = func(g *GorgeSide) { c = countSquadReplicateEvents(g) }
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
	return res, c
}

// playTwo plays one seed of the two-seat pod built from decks 0 and 2 (a
// Squad deck against a Replicate deck).
func (p squadReplicatePod) playTwo(t *testing.T, seed uint64, maxTurns int32) (Result, squadReplicateCounts) {
	t.Helper()
	decks := p.decks
	lists := [][]cardset.Entry{decks[0], decks[2]}
	var tbl *kop.Table
	if p.set != nil {
		var err error
		tbl, _, err = p.set.MatchupN(lists)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expandSquadReplicateDeck(decks[0]), expandSquadReplicateDeck(decks[2])}, Seed: seed, PassElision: true, Surface: "full"}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	var c squadReplicateCounts
	rc.Finish = func(g *GorgeSide) { c = countSquadReplicateEvents(g) }
	k, err := SetSide(p.set, tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc)
	return res, c
}

// squadReplicateFail reports a seed's divergence with the action window that
// reached it.
func squadReplicateFail(t *testing.T, seed uint64, res Result) {
	t.Helper()
	lo := max(res.Div.At-12, 0)
	t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
		res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
}

// squadReplicateSeeds are the seeds the lockstep plays.
var squadReplicateSeeds = []uint64{1, 2, 3, 5}

// TestSquadReplicateFourSeatLockstep plays four-seat pods of the fixture: a
// Squad creature's ETB mints one token copy per payment, a Replicate spell's
// SpellCast trigger copies it once per payment, the non-mana payments are
// taken once per payment, and a declined count is an exactly plain cast. No
// divergence and no kernel stall, with at least one Squad and one Replicate
// payment over the seeds.
func TestSquadReplicateFourSeatLockstep(t *testing.T) {
	p := newSquadReplicatePod(t)
	var sum squadReplicateCounts
	for _, seed := range squadReplicateSeeds {
		res, c := p.play(t, seed, 40)
		if res.Div != nil {
			squadReplicateFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.squad += c.squad
		sum.replicate += c.replicate
		t.Logf("seed %d: %d decisions, over %v %s; %d squad copy(ies), %d replicate copy(ies)", seed, res.Decisions, res.Over, res.Stall, c.squad, c.replicate)
	}
	if sum.squad == 0 {
		t.Fatal("no squad payment fired over the seeds")
	}
	if sum.replicate == 0 {
		t.Fatal("no replicate payment fired over the seeds")
	}
}

// TestSquadReplicateTwoSeatLockstep is the two-seat form: a Squad deck
// against a Replicate deck, the same fixture and assertions.
func TestSquadReplicateTwoSeatLockstep(t *testing.T) {
	p := newSquadReplicatePod(t)
	var sum squadReplicateCounts
	for _, seed := range squadReplicateSeeds {
		res, c := p.playTwo(t, seed, 40)
		if res.Div != nil {
			squadReplicateFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.squad += c.squad
		sum.replicate += c.replicate
		t.Logf("seed %d: %d decisions, over %v %s; %d squad copy(ies), %d replicate copy(ies)", seed, res.Decisions, res.Over, res.Stall, c.squad, c.replicate)
	}
	if sum.squad == 0 || sum.replicate == 0 {
		t.Fatalf("squad/replicate copies = %d/%d: want both above zero", sum.squad, sum.replicate)
	}
}

// TestSquadReplicateCountMutation is the mutation test: storing one payment
// fewer than the cast pays changes the token count and the mana spent, so it
// must diverge on the first seed that squads.
func TestSquadReplicateCountMutation(t *testing.T) {
	p := newSquadReplicatePod(t)
	dirac.DebugSquadCountOffByOne = true
	defer func() { dirac.DebugSquadCountOffByOne = false }()
	res, _ := p.play(t, squadReplicateSeeds[0], 40)
	if res.Div == nil {
		t.Fatal("storing one squad payment fewer diverged on no decision")
	}
	t.Logf("mutation diverged at %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)
}
