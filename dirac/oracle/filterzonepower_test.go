package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Seven filter words: hasKeywordFlash (Mystical Teachings),
// !Mount (Throw from the Saddle), noName (Pompous Gadabout vs face-down
// blockers), inZoneGraveyard (Dread Summons), inZoneStack (Take It Back),
// greatestPower (Getaway Glamer) and attackingAlone (Crowd of True Believers).

var kprops7Decks = [][2][]cardset.Entry{
	{ // M1 (OTJ/MSH)
		{
			{Name: "Throw from the Saddle", Count: 4}, {Name: "Getaway Glamer", Count: 4}, {Name: "Crowd of True Believers", Count: 4},
			{Name: "Bulwark Ox", Count: 3}, {Name: "Drover Grizzly", Count: 3}, {Name: "Giant Beaver", Count: 3}, {Name: "Trained Arynx", Count: 3},
			{Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 7}, {Name: "Forest", Count: 7},
		},
		{
			{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 6}, {Name: "Glory Seeker", Count: 4},
			{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
		},
	},
	{ // M2 (MKM)
		{
			{Name: "Pompous Gadabout", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Hill Giant", Count: 4}, {Name: "Forest", Count: 20},
		},
		{
			{Name: "Manifest Dread", Count: 6}, {Name: "Defiant Survivor", Count: 4}, {Name: "Cryptid Inspector", Count: 4},
			{Name: "Grizzly Bears", Count: 6}, {Name: "Forest", Count: 20},
		},
	},
	{ // M3 (FDN/WOE)
		{
			{Name: "Mystical Teachings", Count: 4}, {Name: "Dread Summons", Count: 4}, {Name: "Spellscorn Coven", Count: 4},
			{Name: "Spectral Sailor", Count: 3}, {Name: "Brineborn Cutthroat", Count: 3}, {Name: "Divination", Count: 3},
			{Name: "Opt", Count: 3}, {Name: "Counterspell", Count: 2}, {Name: "Island", Count: 7}, {Name: "Swamp", Count: 7},
		},
		{
			{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Ambush Viper", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Cancel", Count: 2},
			{Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
		},
	},
}

// kprops7Stats is what the gorge-side event log says each word decided.
type kprops7Stats struct {
	throwMount    int // Throw from the Saddle resolved: it put a +1/+1 counter (the target is a Mount)
	throwNonMount int // ... resolved with no counter (the target is not a Mount)
	glamKill      int // Getaway Glamer's destroy mode resolved and a creature died
	glamSpare     int // ... resolved and nothing died (a creature has greater power)
	crowd         int // Crowd of True Believers activations
	gadFD         int // Pompous Gadabout attacked while the opponent held an untapped face-down creature
	takeItBack    int // Take It Back resolved
	teachings     int // Mystical Teachings resolved and a card left the library for the hand
	dread         int // Dread Summons cast with X >= 1
}

// kprops7Tally reads the gorge event log between decisions. cur is the
// resolving spell's card name (cleared at the next priority), the counters
// are what its resolution did.
type kprops7Tally struct {
	evAt    int
	cur     string
	ctr     bool // the resolution put a counter on something
	dmg     bool // damage was dealt
	died    bool // a battlefield card went to a graveyard
	search  bool // a card moved library -> hand
	glDest  map[state.ObjID]bool
	curDest bool
	curObj  state.ObjID
	bounced bool // a spell other than the resolving one went stack -> hand
}

func (p *kprops7Tally) flush(st *kprops7Stats) {
	switch p.cur {
	case "Throw from the Saddle":
		if p.ctr {
			st.throwMount++
		} else {
			st.throwNonMount++
		}
	case "Getaway Glamer":
		if p.curDest {
			if p.died {
				st.glamKill++
			} else {
				st.glamSpare++
			}
		}
	case "Take It Back", "Spellscorn Coven":
		if p.bounced {
			st.takeItBack++ // the adventure face returned a spell (the creature face's resolution moves nothing stack -> hand)
		}
	case "Mystical Teachings":
		if p.search {
			st.teachings++
		}
	}
	p.cur, p.ctr, p.dmg, p.died, p.search, p.curDest, p.bounced = "", false, false, false, false, false, false
}

func (p *kprops7Tally) scan(ref *GorgeSide, st *kprops7Stats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.Priority, events.StepChange, events.TurnChange:
			p.flush(st)
		case events.Resolve:
			p.flush(st)
			p.cur = chName(g, e.Obj)
			p.curDest = p.glDest[e.Obj]
			p.curObj = e.Obj
		case events.ModeChosen:
			if chName(g, e.Obj) == "Getaway Glamer" && strings.Contains(e.Text, "Destroy") {
				if p.glDest == nil {
					p.glDest = map[state.ObjID]bool{}
				}
				p.glDest[e.Obj] = true
			}
		case events.CounterChange:
			if p.cur != "" && e.Amount > 0 {
				p.ctr = true
			}
		case events.Damage:
			p.dmg = true
		case events.MoveZone:
			if p.cur == "" {
				break
			}
			if e.From == state.ZBattlefield && e.To == state.ZGraveyard {
				p.died = true
			}
			if e.From == state.ZStack && e.To == state.ZHand && e.Obj != p.curObj {
				p.bounced = true
			}
			if e.From == state.ZLibrary && e.To == state.ZHand {
				p.search = true
			}
		case events.AbilityPush:
			if chName(g, e.Obj) == "Crowd of True Believers" {
				st.crowd++
			}
		case events.CastInfo:
			if chName(g, e.Obj) == "Dread Summons" && e.Amount >= 1 {
				st.dread++
			}
		case events.DeclareAttackers:
			ids := e.IDs
			if len(ids) == 0 {
				ids = []state.ObjID{e.Obj}
			}
			for _, id := range ids {
				o := g.Obj(id)
				if o == nil || chName(g, id) != "Pompous Gadabout" {
					continue
				}
				opp := 1 - o.Controller
				for own := state.PlayerID(0); own < 2; own++ {
					for _, b := range g.Zone(state.ZBattlefield, own) {
						if ob := g.Obj(b); ob != nil && ob.Controller == opp && ob.FaceDown && !ob.Tapped {
							st.gadFD++
							goto next
						}
					}
				}
			next:
			}
		}
	}
	p.evAt = len(evs)
}

// kprops7Policy plays at random among casting, activating and playing lands
// (so the carriers are reached), attacks at random, and counts what the
// gorge side logs.
func kprops7Policy(seed uint64, st *kprops7Stats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x70726f7073370a)) // "props7"
	var buf []Canon
	var tl kprops7Tally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		switch c.D.Kind {
		case DAttackers:
			buf = buf[:0]
			for _, a := range c.D.Options {
				if a.Kind() == ActAttack {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 && prng.IntN(3) != 0 {
				return buf[prng.IntN(len(buf))]
			}
		case DPriority:
			buf = buf[:0]
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActActivate, ActPlayLand:
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				d := *c.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		return rnd.Pick(c)
	})
}

// runFilterzonepower plays the constructed matchups, each in both seatings, and
// returns the games, divergences, stalls and the word counts. mutate negates
// every kernel PredWord7 answer (the mutation must diverge).
func runFilterzonepower(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st kprops7Stats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("filterzonepower")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugWord7Flip
	dirac.DebugWord7Flip = mutate
	defer func() { dirac.DebugWord7Flip = old }()
	for mi, md0 := range kprops7Decks {
		for swap := 0; swap < 2; swap++ {
			md := md0
			if swap == 1 {
				md = [2][]cardset.Entry{md0[1], md0[0]}
			}
			var decks [2][]*cards.Card
			for s, d := range md {
				for _, e := range d {
					c, ok := reg.Lookup(e.Name)
					if !ok {
						t.Fatalf("%s not in the corpus", e.Name)
					}
					for i := 0; i < e.Count; i++ {
						decks[s] = append(decks[s], c)
					}
				}
				if len(decks[s]) != 40 {
					t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
				}
			}
			tbl, _, _, err := set.Matchup(md[0], md[1])
			if err != nil {
				t.Fatal(err)
			}
			for seed := uint64(1); seed <= uint64(seeds); seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"kprops7-a", "kprops7-b"}, decks, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				g, err := NewGorgeSide(m, seed, cfg, nil)
				if err != nil {
					t.Fatal(err)
				}
				res := LockstepLed(m, seed, g, k, kprops7Policy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
				CloseSide(k)
				games++
				if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
					stalls++
					if !mutate {
						t.Errorf("matchup %d swap %d seed %d: kernel stall %q", mi, swap, seed, res.Stall)
					}
				}
				if d := res.Div; d != nil {
					divs++
					if !mutate {
						t.Errorf("matchup %d swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
					}
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestFilterWordsZoneAndPowerLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// 96 seeds per seating: 192 per matchup
	games, divs, stalls, st := runFilterzonepower(t, 96, false)
	t.Logf("%d games, %d divergences, %d stalls", games, divs, stalls)
	t.Logf("%+v", st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"Throw from the Saddle on a Mount", st.throwMount}, {"Throw from the Saddle on a non-Mount", st.throwNonMount},
		{"Getaway Glamer destroyed", st.glamKill}, {"Getaway Glamer spared", st.glamSpare},
		{"Crowd of True Believers activations", st.crowd}, {"Gadabout attacks into a face-down creature", st.gadFD},
		{"Take It Back resolutions", st.takeItBack}, {"Mystical Teachings searches", st.teachings},
		{"Dread Summons with X >= 1", st.dread},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}

// TestFilterWordsZoneAndPowerMutation negates every kernel PredWord7 answer: the games must
// diverge. A green mutation means the lockstep never reaches a decision the
// words shape.
func TestFilterWordsZoneAndPowerMutation(t *testing.T) {
	games, divs, _, _ := runFilterzonepower(t, 32, true)
	if divs == 0 {
		t.Fatalf("mutation negated every PredWord7 answer in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
