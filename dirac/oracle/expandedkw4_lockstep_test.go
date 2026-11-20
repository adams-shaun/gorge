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

// The gap-4 expanded-keyword lockstep (CR 702.46a soulshift, CR 702.77a
// reinforce, CR 702.128a embalm, CR 702.129a eternalize): constructed pods
// over the expandedkw4 fixture, the carriers dying into graveyards of
// Spirits, reinforcing from hand and embalming / eternalizing out of the
// graveyard. The lockstep compares the two sides' option lists at every
// decision; the counts prove the mechanics actually fired, so a green
// lockstep is not a game where nothing happened.

// kw4Counts counts a finished gorge log's expanded-keyword events.
type kw4Counts struct {
	soulshift int            // a Spirit card returned from its owner's graveyard to their hand (the soulshift trigger's body)
	reinforce int            // +1/+1 counters placed (the reinforce body; the pool has no other source)
	tokens    int            // embalm / eternalize tokens that entered the battlefield
	tokByName map[string]int // the entering copy tokens, by face name
}

// kw4TokenNames are the pool's embalm and eternalize carriers: a copy token
// of one of these that entered is one of the two abilities' mints.
var kw4TokenNames = map[string]bool{
	"Adorned Pouncer": true, "Steadfast Sentinel": true, // eternalize
	"Fanatic of Rhonas": true, "Resilient Khenra": true, // eternalize
	"Labyrinth Guardian": true, "Aven Initiate": true, // embalm
}

func countKw4Events(g *GorgeSide) (c kw4Counts) {
	c.tokByName = map[string]int{}
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.MoveZone:
			if e.From == state.ZGraveyard && e.To == state.ZHand {
				if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().HasTypeWord("Spirit") {
					c.soulshift++
				}
			}
			if e.To == state.ZBattlefield {
				if o := g.E.G.Obj(e.Obj); o != nil && o.IsToken && o.Face() != nil && kw4TokenNames[o.Face().Name] {
					c.tokens++
					c.tokByName[o.Face().Name]++
				}
			}
		case events.CounterChange:
			if e.Counter == "P1P1" {
				c.reinforce++
			}
		}
	}
	return c
}

// kw4Decks is the expandedkw4 pod's four decks: the carriers beside vanilla
// creatures and burn (creatures die, graveyards fill), each seat holding
// soulshift Spirits, a reinforce carrier or two and an embalm / eternalize
// one. No other +1/+1-counter source, graveyard-to-hand return or token-copy
// maker is in the decks, so the counts read cleanly.
func kw4Decks() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{
			{Name: "Deathknell Kami", Count: 3}, {Name: "Kami of Empty Graves", Count: 2}, {Name: "Thief of Hope", Count: 2},
			{Name: "Scuttling Death", Count: 2}, {Name: "Moonlit Strider", Count: 2},
			{Name: "Adorned Pouncer", Count: 2}, {Name: "Steadfast Sentinel", Count: 2},
			{Name: "Mosquito Guard", Count: 2}, {Name: "Break Ties", Count: 2}, {Name: "Burrenton Bombardier", Count: 2},
			{Name: "Scathe Zombies", Count: 4}, {Name: "Lightning Bolt", Count: 3},
			{Name: "Plains", Count: 7}, {Name: "Swamp", Count: 5},
		},
		{
			{Name: "Vine Kami", Count: 2}, {Name: "Body of Jukai", Count: 1}, {Name: "Burr Grafter", Count: 2},
			{Name: "Nightsoil Kami", Count: 1}, {Name: "Gibbering Kami", Count: 2},
			{Name: "Fanatic of Rhonas", Count: 2}, {Name: "Resilient Khenra", Count: 2},
			{Name: "Earthbrawn", Count: 2}, {Name: "Fowl Strike", Count: 2}, {Name: "Hunting Triad", Count: 2},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Doom Blade", Count: 3},
			{Name: "Swamp", Count: 6}, {Name: "Forest", Count: 9},
		},
		{
			{Name: "Labyrinth Guardian", Count: 3}, {Name: "Aven Initiate", Count: 2},
			{Name: "Adorned Pouncer", Count: 2}, {Name: "Steadfast Sentinel", Count: 1},
			{Name: "Mosquito Guard", Count: 2}, {Name: "Break Ties", Count: 2}, {Name: "Swell of Courage", Count: 2},
			{Name: "Rustic Clachan", Count: 2},
			{Name: "Elite Vanguard", Count: 4}, {Name: "Lightning Bolt", Count: 2},
			{Name: "Plains", Count: 10}, {Name: "Island", Count: 8},
		},
		{
			{Name: "Kami of Lunacy", Count: 2}, {Name: "Thousand-legged Kami", Count: 1}, {Name: "Pus Kami", Count: 1},
			{Name: "Crawling Filth", Count: 2}, {Name: "He Who Hungers", Count: 1},
			{Name: "Elder Pine of Jukai", Count: 2}, {Name: "Promised Kannushi", Count: 2}, {Name: "Rootrunner", Count: 2},
			{Name: "Harbinger of Spring", Count: 1},
			{Name: "Wren's Run Hydra", Count: 2},
			{Name: "Hill Giant", Count: 4}, {Name: "Doom Blade", Count: 3},
			{Name: "Forest", Count: 9}, {Name: "Swamp", Count: 8},
		},
	}
}

type kw4Pod struct {
	reg   *cards.Registry
	set   cardset.Set
	tbl   *kop.Table
	decks [4][]cardset.Entry
}

func newKw4Pod(t *testing.T) kw4Pod {
	t.Helper()
	return newKw4PodOf(t, kw4Decks())
}

// newKw4PodOf is newKw4Pod over the given four decks.
func newKw4PodOf(t *testing.T, decks [4][]cardset.Entry) kw4Pod {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("expandedkw4")
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
	return kw4Pod{reg: reg, set: set, tbl: tbl, decks: decks}
}

// play plays one seed of the four-seat pod.
func (p kw4Pod) play(t *testing.T, seed uint64, maxTurns int32) (Result, kw4Counts) {
	t.Helper()
	decks := p.decks
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expand(decks[0]), expand(decks[1])},
		MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{expand(decks[2]), expand(decks[3])}, Seed: seed, PassElision: true, Surface: "full"}
	return p.run(t, seed, r, maxTurns)
}

// playTwo plays one seed of the two-seat form: the same Repro with only
// Decks/Cards.
func (p kw4Pod) playTwo(t *testing.T, seed uint64, maxTurns int32) (Result, kw4Counts) {
	t.Helper()
	decks := p.decks
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expand(decks[0]), expand(decks[1])}, Seed: seed, PassElision: true, Surface: "full"}
	return p.run(t, seed, r, maxTurns)
}

func (p kw4Pod) run(t *testing.T, seed uint64, r *Repro, maxTurns int32) (Result, kw4Counts) {
	t.Helper()
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	var c kw4Counts
	rc.Finish = func(g *GorgeSide) { c = countKw4Events(g) }
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

// kw4Fail reports a seed's divergence with the action window that reached it.
func kw4Fail(t *testing.T, seed uint64, res Result) {
	t.Helper()
	lo := max(res.Div.At-12, 0)
	t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
		res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
}

// kw4Seeds are the seeds the locksteps play; kw4MutationSeeds is the wider
// window the mutation test sweeps (a divergence needs a soulshift ask with a
// dearer Spirit in the graveyard, which not every seed reaches).
var kw4Seeds = []uint64{1, 2, 3, 5, 7, 8}
var kw4MutationSeeds = []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

// TestExpandedKeywordsFourSeatLockstep plays four-seat pods of the
// expandedkw4 fixture: no divergence and no stall, and over the seeds at
// least one soulshift return, one reinforce activation and one embalm or
// eternalize token.
func TestExpandedKeywordsFourSeatLockstep(t *testing.T) {
	t.Parallel()
	p := newKw4Pod(t)
	var sum kw4Counts
	sum.tokByName = map[string]int{}
	for _, seed := range kw4Seeds {
		res, c := p.play(t, seed, 40)
		if res.Div != nil {
			kw4Fail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.soulshift += c.soulshift
		sum.reinforce += c.reinforce
		sum.tokens += c.tokens
		for k, n := range c.tokByName {
			sum.tokByName[k] += n
		}
		t.Logf("seed %d: %d decisions, over %v %s; %d soulshift return(s), %d +1/+1 put(s), %d token(s) %v",
			seed, res.Decisions, res.Over, res.Stall, c.soulshift, c.reinforce, c.tokens, c.tokByName)
	}
	if sum.soulshift == 0 {
		t.Fatal("no soulshift return fired")
	}
	if sum.reinforce == 0 {
		t.Fatal("no reinforce activation fired")
	}
	if sum.tokens == 0 {
		t.Fatal("no embalm or eternalize token fired")
	}
}

// TestExpandedKeywordsTwoSeatLockstep is the two-seat form of the same pod:
// no divergence and no stall, with the same three fired counts.
func TestExpandedKeywordsTwoSeatLockstep(t *testing.T) {
	t.Parallel()
	decks := kw4Decks()
	p := newKw4PodOf(t, decks)
	var sum kw4Counts
	sum.tokByName = map[string]int{}
	for _, seed := range kw4Seeds {
		res, c := p.playTwo(t, seed, 40)
		if res.Div != nil {
			kw4Fail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.soulshift += c.soulshift
		sum.reinforce += c.reinforce
		sum.tokens += c.tokens
		for k, n := range c.tokByName {
			sum.tokByName[k] += n
		}
		t.Logf("seed %d: %d decisions, over %v %s; %d soulshift return(s), %d +1/+1 put(s), %d token(s) %v",
			seed, res.Decisions, res.Over, res.Stall, c.soulshift, c.reinforce, c.tokens, c.tokByName)
	}
	if sum.soulshift == 0 {
		t.Fatal("no soulshift return fired")
	}
	if sum.reinforce == 0 {
		t.Fatal("no reinforce activation fired")
	}
	if sum.tokens == 0 {
		t.Fatal("no embalm or eternalize token fired")
	}
}

// TestExpandedKeywordsSoulshiftMutation is the mutation test: with the
// soulshift target's mana-value bound ignored (dirac.DebugSoulshiftNoBound,
// filters_gen.go's card PredCMCLE read) a seed where a soulshift trigger
// fires while a dearer Spirit is in the graveyard diverges: the kernel
// offers the dearer Spirit, gorge does not. Sequential, like the venture
// mutation tests: the flag is kernel-global and the sequential tests run
// before the parallel ones.
func TestExpandedKeywordsSoulshiftMutation(t *testing.T) {
	dirac.DebugSoulshiftNoBound = true
	defer func() { dirac.DebugSoulshiftNoBound = false }()
	p := newKw4Pod(t)
	for _, seed := range kw4MutationSeeds {
		res, _ := p.play(t, seed, 40)
		if res.Div != nil {
			t.Logf("mutation diverged at seed %d, decision %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("dropping the soulshift mana-value bound diverged on no seed")
}
