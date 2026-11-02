package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The trigger-own discard-your-hand lockstep:
// Narset, Jeskai Waymaster (TDM), Vaultguard Trooper (EOE, Discard<0/Hand>)
// and Byway Barterer (BLB) pay a printed trigger's own Cost$ Discard<N/Hand>
// ("you may discard your hand. If you do, draw ...", CR 118.3, 603.4, 701.9a).
// Gorge's triggered-cost window pays the whole hand whatever the count and is
// payable with an EMPTY hand (CR 118.3, Byway Barterer's ruling), so the
// kernel's PayDiscardHand branch must offer pay on an empty hand too and pay
// it by discarding nothing.
var ktrigdischandDecks = [][2][]cardset.Entry{{
	{
		{Name: "Narset, Jeskai Waymaster", Count: 4}, {Name: "Vaultguard Trooper", Count: 4},
		{Name: "Raging Goblin", Count: 6}, {Name: "Grizzly Bears", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 9}, {Name: "Plains", Count: 5}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Byway Barterer", Count: 4}, {Name: "Storm Crow", Count: 6}, {Name: "Opt", Count: 6},
		{Name: "Shock", Count: 4}, {Name: "Mind Stone", Count: 2},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 9},
	},
}}

var ktrigdischandCarriers = map[string]bool{
	"Narset, Jeskai Waymaster": true,
	"Vaultguard Trooper":       true,
	"Byway Barterer":           true,
}

// ktrigdischandTally counts, on gorge's side, games per branch, keyed
// "DH+" (the cost paid with at least one card in hand; the settle's
// "discarded as a cost" MoveZone events confirm it), "DH0" (paid with an
// empty hand: no discard event at all) and "DH-" (declined), plus
// "<carrier>/DH+" per carrier.
type ktrigdischandTally struct {
	games   int
	total   map[string]int
	perGame map[string]int
}

// ktrigdischandAsk is one answered pay election awaiting its event
// confirmation at the next check.
type ktrigdischandAsk struct {
	name   string
	player state.PlayerID
	paid   bool
	hand   int
}

// ktrigdischandGame is one game's counting state: the event cursor, the
// discard-as-cost events since the last ask, and the pending ask.
type ktrigdischandGame struct {
	evAt     int
	discards int
	ask      *ktrigdischandAsk
	countedG map[string]bool
}

func (g *ktrigdischandGame) note(tal *ktrigdischandTally, k string) {
	tal.total[k]++
	if !g.countedG[k] {
		g.countedG[k] = true
		tal.perGame[k]++
	}
}

func ktrigdischandPolicy(seed uint64, tal *ktrigdischandTally, st *ktrigdischandGame) Policy {
	rnd := RandomPolicy(seed)
	asks := 0
	payCode, _ := chooseCode("trigger_cost_pay")
	declCode, _ := chooseCode("trigger_cost_decline")
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			d := c.G.Pending()
			if d != nil && d.Kind == decision.KChoose {
				var payCanon, declCanon Canon
				foundPay, foundDecl := false, false
				for _, a := range c.D.Options {
					switch a.Kind() {
					case ActChoose + CanonKind(payCode):
						payCanon, foundPay = a, true
					case ActChoose + CanonKind(declCode):
						declCanon, foundDecl = a, true
					}
				}
				if foundPay {
					if name := sourcecountercostsName(c, d); ktrigdischandCarriers[name] {
						asks++
						paid := (seed+uint64(asks))%2 == 0
						if paid {
							st.ask = &ktrigdischandAsk{name: name, player: d.Player,
								paid: true, hand: len(c.G.E.G.Zone(state.ZHand, d.Player))}
							return payCanon
						}
						st.note(tal, "DH-")
						if foundDecl {
							return declCanon
						}
					}
				} else if foundDecl {
					// A decline-only ask: the cost is unpayable (never the
					// case for a Hand discard, which an empty hand pays).
					if ktrigdischandCarriers[sourcecountercostsName(c, d)] {
						st.note(tal, "DH-")
					}
					return declCanon
				}
			}
		}
		var buf []Canon
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActActivate {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestTrigdischandLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("trigdischand")
	if err != nil {
		t.Fatal(err)
	}
	tal := &ktrigdischandTally{total: map[string]int{}, perGame: map[string]int{}}
	div, games, stallOK := 0, 0, 0
	for mi, md := range ktrigdischandDecks {
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
		for seed := uint64(1); seed <= 300; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"td-a", "td-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			st := &ktrigdischandGame{countedG: map[string]bool{}}
			check := func(ref *GorgeSide, _ Side) *Divergence {
				evs := ref.E.L.Events
				for ; st.evAt < len(evs); st.evAt++ {
					e := &evs[st.evAt]
					if e.Kind == events.MoveZone && e.From == state.ZHand && e.Text == "discarded as a cost" {
						st.discards++
					}
				}
				if a := st.ask; a != nil {
					st.ask = nil
					if a.paid {
						if a.hand > 0 && st.discards > 0 {
							st.note(tal, "DH+")
							st.note(tal, a.name+"/DH+")
						} else if a.hand == 0 && st.discards == 0 {
							// paid by discarding nothing: no event at all
							st.note(tal, "DH0")
						}
					}
				}
				st.discards = 0
				return nil
			}
			res := Lockstep(m, seed, k, ktrigdischandPolicy(seed, tal, st), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
			CloseSide(k)
			games++
			if res.Stall == KernelStallPrefix+"castring" {
				// a greedy caster pushes more than castRingN spells in a
				// turn (Narset's draw reads the cast ring): a known capacity
				// stall, not a divergence (permcopy_test.go)
				stallOK++
			} else if res.Stall != "" && res.Stall != "turns" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				div++
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, %d divergences, %d castring stalls; games with a branch (total): %v / %v", games, div, stallOK, tal.perGame, tal.total)
	if tal.perGame["DH+"] < 10 || tal.perGame["DH0"] < 10 || tal.perGame["DH-"] < 10 {
		t.Errorf("branches: DH+ %d, DH0 %d, DH- %d; want at least 10 games each", tal.perGame["DH+"], tal.perGame["DH0"], tal.perGame["DH-"])
	}
	for _, c := range []string{"Narset, Jeskai Waymaster", "Vaultguard Trooper", "Byway Barterer"} {
		if tal.perGame[c+"/DH+"] < 3 {
			t.Errorf("%s: paid with cards in %d games; want at least 3", c, tal.perGame[c+"/DH+"])
		}
	}
}
