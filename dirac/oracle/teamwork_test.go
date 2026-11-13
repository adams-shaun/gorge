package oracle

import (
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Teamwork lockstep (CR 702.194): constructed matchups over the
// generated teamwork pool -- the MSH Teamwork carriers kgen lowers, vanilla
// creatures to tap, Lightning Bolt and basics. The frozen FDN and BLB pair
// pools carry no Teamwork card, so the expected answer at every decision is
// gorge's.
//
// The two decks are different: A carries the Part A carriers (a Count$Teamwork
// read, a Card.Self+Teamwork rider or a kicked-range target slot), B the
// charms whose CharmNum$ X is Count$Teamwork.2.1 (Widow's Bite, Murdock's
// Crusade) and Team Tactics. Atlantis Attacks stays refused (a 1..2 target
// bound); Go Nuts! and HULK SMASH! lower and run in the
// charmtwotgt lockstep.
//
// The policy takes the teamworked cast (ActCast with the cast-mode nibble
// full, the way gorge's "teamworked" mode canonicalises) half the time and
// the plain cast of the same card on the other half, and declines the tap
// pick (Done before any creature) one time in four. The tally counts the
// mechanic itself: teamworked casts that tapped, declined picks, both arms
// of a Count$Teamwork read (the paid flag on gorge's live stack) and the
// Part B casts on each pick count.
//
// Four workers run the 256 seeds under MaxTurns 20 and MaxDecisions 2000: a
// game that reaches the harness cap ends there ("turns" / "decisions"), and
// any other Stall (a kernel capacity stall included) is a real failure.

const kteamworkCastOther = castModeOther // the "teamworked" cast mode's nibble (castModes is full)

// kteamworkCount is every carrier that reads Count$Teamwork at resolution.
var kteamworkCount = map[string]bool{
	"Cruel Alliance":           true,
	"Earth's Mightiest Heroes": true,
	"Helicarrier Strike":       true,
	"Repulsor Blast":           true,
	"Too Evil to Stay Dead":    true,
	"We Say Thee Nay!":         true,
	"Widow's Bite":             true,
	"Murdock's Crusade":        true,
}

// kteamworkCharm is the carriers whose CharmNum$ reads Count$Teamwork.
var kteamworkCharm = map[string]bool{"Widow's Bite": true, "Murdock's Crusade": true}

// kteamworkCarrier is every Teamwork card in the decks.
var kteamworkCarrier = map[string]bool{
	"Cruel Alliance": true, "Earth's Mightiest Heroes": true, "Helicarrier Strike": true,
	"Repulsor Blast": true, "Too Evil to Stay Dead": true, "We Say Thee Nay!": true,
	"Team Tactics": true, "Widow's Bite": true, "Murdock's Crusade": true,
}

var kteamworkDecks = [2][]cardset.Entry{
	{ // A: the Part A carriers
		{Name: "Cruel Alliance", Count: 2}, {Name: "Earth's Mightiest Heroes", Count: 2},
		{Name: "Helicarrier Strike", Count: 2}, {Name: "Repulsor Blast", Count: 2},
		{Name: "Too Evil to Stay Dead", Count: 2}, {Name: "We Say Thee Nay!", Count: 2},
		{Name: "Team Tactics", Count: 2},
		{Name: "Iridescent Tiger", Count: 3}, {Name: "Burning-Tree Emissary", Count: 3},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Plains", Count: 4}, {Name: "Island", Count: 3}, {Name: "Swamp", Count: 4},
		{Name: "Mountain", Count: 4}, {Name: "Forest", Count: 3},
	},
	{ // B: the Teamwork charms and Team Tactics
		{Name: "Widow's Bite", Count: 3}, {Name: "Murdock's Crusade", Count: 2},
		{Name: "Team Tactics", Count: 2}, {Name: "Lightning Bolt", Count: 1},
		{Name: "Hill Giant", Count: 4},
		{Name: "Iridescent Tiger", Count: 4}, {Name: "Burning-Tree Emissary", Count: 4},
		{Name: "Plains", Count: 3}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 4},
		{Name: "Forest", Count: 4}, {Name: "Island", Count: 3},
	},
}

// kteamworkPay is the ETB mana the decks run: Iridescent Tiger adds one of
// each colour when it is cast, Burning-Tree Emissary adds {R}{G}. Gorge's
// planner prices no teamworked cast, so only a floating pool that pays the
// plain cast gets it offered (kw_casualty.go casualtyOffers).
var kteamworkPay = map[string]bool{
	"Iridescent Tiger":      true,
	"Burning-Tree Emissary": true,
}

// kteamworkTally counts the mechanic through the decisions (the policy) and
// gorge's live stack (RunConfig.Check).
type kteamworkTally struct {
	castsTW      int // the teamworked cast chosen
	castsPlain   int // the plain cast chosen while the teamworked one was offered
	charmTW      int // a Part B card cast teamworked (pick count 2)
	charmPlain   int // a Part B card cast plain (pick count 1)
	declined     int // the tap pick answered with Done before any creature
	taps         int // a teamworked cast whose paid flag stood on gorge's stack
	countYes     int // a Count$Teamwork carrier resolved with the paid flag
	countNo      int // the same without it
	inElect      bool
	seenSpell    map[state.ObjID]bool
	pendingSpell map[state.ObjID]bool
	pendingFlag  map[state.ObjID]bool
	seenPaid     map[state.ObjID]bool
}

func (c *kteamworkTally) reset() {
	c.seenSpell = map[state.ObjID]bool{}
	c.pendingSpell = map[state.ObjID]bool{}
	c.pendingFlag = map[state.ObjID]bool{}
	c.seenPaid = map[state.ObjID]bool{}
	c.inElect = false
}

// kteamworkPolicy takes the teamworked cast half the time and the plain
// cast of the same card on the other half, then answers the tap election:
// Done before any creature one time in four (a decline), otherwise random
// creatures with Done taken half the time it is offered.
func kteamworkPolicy(seed uint64, c *kteamworkTally) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x7465616d776f726b)) // "teamwork"
	var buf []Canon
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DChoose {
			if code, ok := chooseCode("teamwork"); ok {
				var picks []Canon
				done := -1
				for i, a := range p.D.Options {
					switch {
					case a.Kind() == ActChoose+CanonKind(code):
						picks = append(picks, a)
					case a.Kind() == ActDone:
						done = i
					}
				}
				if len(picks) > 0 {
					first := !c.inElect
					c.inElect = true
					if done >= 0 && first && prng.IntN(4) == 0 {
						c.declined++
						c.inElect = false
						return p.D.Options[done]
					}
					if done >= 0 && !first && prng.IntN(2) == 0 {
						c.inElect = false
						return p.D.Options[done]
					}
					return picks[prng.IntN(len(picks))]
				}
			}
		}
		c.inElect = false
		if p.D.Kind == DAttackers {
			var atk []Canon
			for _, a := range p.D.Options {
				if a.Kind() == ActAttack {
					atk = append(atk, a)
				}
			}
			if len(atk) > 0 {
				return atk[prng.IntN(len(atk))]
			}
		}
		if p.D.Kind == DPriority {
			for i, a := range p.D.Options {
				if a.Kind() != ActCast || a.Mode()>>4 != 0 || a.Mode()&0xf != kteamworkCastOther ||
					!kteamworkCarrier[refCardName(p.G, a.Src())] {
					continue
				}
				name := refCardName(p.G, a.Src())
				plain := -1
				for j, b := range p.D.Options {
					if b.Kind() == ActCast && b.Mode() == 0 && b.Src() == a.Src() {
						plain = j
						break
					}
				}
				if plain < 0 || prng.IntN(2) == 0 {
					c.castsTW++
					if kteamworkCharm[name] {
						c.charmTW++
					}
					return p.D.Options[i]
				}
				c.castsPlain++
				if kteamworkCharm[name] {
					c.charmPlain++
				}
				return p.D.Options[plain]
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && kteamworkPay[refCardName(p.G, a.Src())] {
					return a // the ETB mana first: the floating pool prices a teamworked cast
				}
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActPlayLand {
					return a
				}
			}
			buf = buf[:0]
			for _, a := range p.D.Options {
				switch a.Kind() {
				case ActCast, ActCastCasualty, ActActivate, ActPlayLand:
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				d := *p.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
			}
		}
		return rnd.Pick(p)
	})
}

// check counts what only the live gorge stack shows: the paid flag of each
// Count$Teamwork carrier, remembered while it is on the stack and counted
// into its arm when it leaves.
func (c *kteamworkTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	onStack := map[state.ObjID]bool{}
	for _, id := range g.Stack {
		onStack[id] = true
		o := g.Obj(id)
		if o == nil || o.Ability != nil || o.Card == nil || len(o.Card.Faces) == 0 {
			continue
		}
		name := o.Card.Faces[0].Name
		if !kteamworkCarrier[name] {
			continue
		}
		if o.TeamworkPaid && !c.seenPaid[id] {
			c.seenPaid[id] = true
			c.taps++
		}
		if !kteamworkCount[name] {
			continue
		}
		if !c.seenSpell[id] {
			c.seenSpell[id] = true
			c.pendingSpell[id] = true
		}
		if o.TeamworkPaid {
			c.pendingFlag[id] = true
		}
	}
	for id := range c.pendingSpell {
		if onStack[id] {
			continue
		}
		delete(c.pendingSpell, id)
		if c.pendingFlag[id] {
			c.countYes++
		} else {
			c.countNo++
		}
		delete(c.pendingFlag, id)
	}
	return nil
}

func (c *kteamworkTally) add(o *kteamworkTally) {
	c.castsTW += o.castsTW
	c.castsPlain += o.castsPlain
	c.charmTW += o.charmTW
	c.charmPlain += o.charmPlain
	c.declined += o.declined
	c.taps += o.taps
	c.countYes += o.countYes
	c.countNo += o.countNo
}

// TestTeamworkLockstep runs the 256-seed sweep in four workers (the test
// budget's four CPUs): every game is independent, and the tally merges.
func TestTeamworkLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("teamwork")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kteamworkDecks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("deck %d: %s not in the corpus", s, e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], card)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}

	const workers = 4
	tallies := make([]*kteamworkTally, workers)
	gameCounts := make([]int, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			tbl, _, _, err := set.Matchup(kteamworkDecks[0], kteamworkDecks[1])
			if err != nil {
				t.Errorf("worker %d: matchup: %v", w, err)
				return
			}
			c := &kteamworkTally{}
			for seed := uint64(w + 1); seed <= 256; seed += workers {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"kteamwork-a", "kteamwork-b"}, decks, 1)
				if err != nil {
					t.Errorf("seed %d: matchup: %v", seed, err)
					return
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Errorf("seed %d: side: %v", seed, err)
					return
				}
				c.reset()
				res := Lockstep(m, seed, k, kteamworkPolicy(seed, c),
					RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: c.check})
				CloseSide(k)
				gameCounts[w]++
				if res.Div != nil {
					lo := res.Div.At - 12
					if lo < 0 {
						lo = 0
					}
					t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:res.Div.At]))
					t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
						seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail,
						res.Div.RefOptions, res.Div.KerOptions)
				}
				if res.Stall != "" && res.Stall != "turns" && res.Stall != "decisions" { // the harness caps are not stalls
					t.Errorf("seed %d: stalled: %s after %d turns", seed, res.Stall, res.Turns)
				}
			}
			tallies[w] = c
		}(w)
	}
	wg.Wait()
	c := &kteamworkTally{}
	games := 0
	for w, wc := range tallies {
		if wc != nil {
			c.add(wc)
		}
		games += gameCounts[w]
	}
	t.Logf("%d games: teamworked casts %d, plain casts beside the offer %d, tapped %d, declined %d, Count$Teamwork yes %d no %d, Part B teamworked %d plain %d",
		games, c.castsTW, c.castsPlain, c.taps, c.declined, c.countYes, c.countNo, c.charmTW, c.charmPlain)
	for _, a := range []struct {
		name string
		got  int
		want int
	}{
		{"teamworked casts with taps", c.taps, 30},
		{"declined tap picks", c.declined, 10},
		{"Count$Teamwork yes arm", c.countYes, 10},
		{"Count$Teamwork no arm", c.countNo, 10},
		{"Part B casts, pick count 2 (teamworked)", c.charmTW, 10},
		{"Part B casts, pick count 1 (plain)", c.charmPlain, 10},
	} {
		if a.got < a.want {
			t.Errorf("%s: %d, want >= %d", a.name, a.got, a.want)
		}
	}
}
