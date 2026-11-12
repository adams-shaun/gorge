package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The BeholdExile<1/Type> lockstep: the Lorwyn
// Champions' own additional cost "behold a <Type> and exile it" (RaiseCost
// Cost$ BeholdExile<1/Type>), asked as the either-or behold is (battlefield
// candidates then hand ones, a lone candidate taken unasked, no candidate: the
// cast is not offered), the beheld object exiled at payment with the spell as
// its source, and the leaves trigger's Defined$ ExiledWith returning it.

func kbxDeck(entries ...cardset.Entry) []cardset.Entry { return entries }

var kbxDeckA = kbxDeck(
	cardset.Entry{Name: "Champion of the Clachan", Count: 2}, cardset.Entry{Name: "Champions of the Perfect", Count: 2}, cardset.Entry{Name: "Champions of the Shoal", Count: 2},
	cardset.Entry{Name: "Kithkin Billyrider", Count: 2}, cardset.Entry{Name: "Knight of Meadowgrain", Count: 2},
	cardset.Entry{Name: "Coral Merfolk", Count: 2}, cardset.Entry{Name: "Merfolk of the Pearl Trident", Count: 2}, cardset.Entry{Name: "Silvergill Mentor", Count: 1},
	cardset.Entry{Name: "Elvish Warrior", Count: 3}, cardset.Entry{Name: "Shock", Count: 2}, cardset.Entry{Name: "Unsummon", Count: 2},
	cardset.Entry{Name: "Plains", Count: 5}, cardset.Entry{Name: "Island", Count: 5}, cardset.Entry{Name: "Forest", Count: 5}, cardset.Entry{Name: "Mountain", Count: 3},
)

var kbxDeckB = kbxDeck(
	cardset.Entry{Name: "Champion of the Clachan", Count: 3}, cardset.Entry{Name: "Champions of the Perfect", Count: 3}, cardset.Entry{Name: "Champions of the Shoal", Count: 3},
	cardset.Entry{Name: "Kithkin Billyrider", Count: 3}, cardset.Entry{Name: "Knight of Meadowgrain", Count: 3},
	cardset.Entry{Name: "Coral Merfolk", Count: 2}, cardset.Entry{Name: "Merfolk of the Pearl Trident", Count: 2}, cardset.Entry{Name: "Silvergill Mentor", Count: 1},
	cardset.Entry{Name: "Elvish Warrior", Count: 2}, cardset.Entry{Name: "Shock", Count: 1}, cardset.Entry{Name: "Unsummon", Count: 2},
	cardset.Entry{Name: "Plains", Count: 5}, cardset.Entry{Name: "Island", Count: 5}, cardset.Entry{Name: "Forest", Count: 5},
)

var kbxDecks = [][2][]cardset.Entry{{kbxDeckA, kbxDeckA}, {kbxDeckA, kbxDeckB}}

// kbxTribe is the type each Champion beholds.
var kbxTribe = map[string]string{
	"Champion of the Clachan": "Kithkin", "Champions of the Perfect": "Elf", "Champions of the Shoal": "Merfolk",
}

// kbxTally counts what the gorge side posed.
type kbxTally struct {
	bfCasts   int // Champion casts whose beheld card came from the battlefield
	handCasts int // ... from the hand
	asked     int // casts with two or more candidates (the ask posed)
	withheld  int // priority decisions where a Champion in hand had no candidate and four untapped lands
	matched   int // leaves triggers that returned a card (the kernel's PredExiledWith matches)
}

// kbxPending is a Champion cast chosen and not yet paid: the candidates it
// could exile.
type kbxPending struct {
	on       bool
	bf, hand []state.ObjID
}

// kbxCands lists player p's behold candidates for the Champion src: its
// battlefield permanents of the type, then its hand cards of it, src excepted.
func kbxCands(g *state.Game, p state.PlayerID, src state.ObjID, ty string) (bf, hand []state.ObjID) {
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if objectHasType(g.Obj(id), ty) {
			bf = append(bf, id)
		}
	}
	for _, id := range g.Zone(state.ZHand, p) {
		if id != src && objectHasType(g.Obj(id), ty) {
			hand = append(hand, id)
		}
	}
	return bf, hand
}

// kbxPolicy casts whenever it can (preferring a Champion), else random;
// seed%4 == 0 is plain random so the removal and the bounce are cast too.
func kbxPolicy(seed uint64, tl *kbxTally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	var pend kbxPending
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		if c.G == nil || c.G.fr == nil || c.G.fr.d == nil {
			return rnd.Pick(c)
		}
		g, d := c.G.E.G, c.G.fr.d
		if pend.on {
			// the cast's payment exiled one candidate (tally its zone), or the
			// next priority decision came with nothing exiled: the cast was undone
			done := false
			for _, id := range pend.bf {
				if o := g.Obj(id); o != nil && o.Zone == state.ZExile {
					tl.bfCasts++
					done = true
				}
			}
			for _, id := range pend.hand {
				if o := g.Obj(id); o != nil && o.Zone == state.ZExile {
					tl.handCasts++
					done = true
				}
			}
			if done || c.G.fr.dk == DPriority {
				pend.on = false
			}
		}
		if c.G.fr.dk == DPriority {
			for _, id := range g.Zone(state.ZHand, d.Player) {
				ty := kbxTribe[g.Obj(id).Face().Name]
				if ty == "" {
					continue
				}
				if bf, hand := kbxCands(g, d.Player, id, ty); len(bf)+len(hand) == 0 && !kbeholdSpent(g, d.Player) {
					lands := 0
					for _, lid := range g.Zone(state.ZBattlefield, d.Player) {
						if o := g.Obj(lid); !o.Tapped && objectHasType(o, "Land") {
							lands++
						}
					}
					if lands >= 4 {
						tl.withheld++
						break
					}
				}
			}
		}
		champ := false
		for _, a := range c.D.Options {
			if a.Kind() != ActCast {
				continue
			}
			if o, ok := c.G.GorgeOption(a); ok && kbxTribe[g.Obj(o.Obj).Face().Name] != "" {
				if !champ {
					buf = buf[:0]
				}
				champ = true
				buf = append(buf, a)
			} else if !champ && seed%4 != 0 {
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) == 0 || seed%4 == 0 && !champ {
			a = rnd.Pick(c)
		} else {
			dd := *c.D
			dd.Options = buf
			a = rnd.Pick(&PickCtx{D: &dd, G: c.G, Seq: c.Seq})
		}
		if a.Kind() == ActCast && c.G.fr.dk == DPriority {
			if o, ok := c.G.GorgeOption(a); ok {
				if ty := kbxTribe[g.Obj(o.Obj).Face().Name]; ty != "" {
					bf, hand := kbxCands(g, d.Player, o.Obj, ty)
					pend = kbxPending{on: true, bf: bf, hand: hand}
					if len(bf)+len(hand) >= 2 {
						tl.asked++
					}
				}
			}
		}
		return a
	})
}

// runBeholdexile plays the matchups for seeds 1..seeds and returns the
// games, the divergences, the first divergence's text and the tally. mutate
// drops the cost exile's link.
func runBeholdexile(t *testing.T, seeds int, mutate bool) (games, divs int, tl kbxTally, first string) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("beholdexile")
	if err != nil {
		t.Fatal(err)
	}
	old, oldSt := dirac.DebugBeholdExileNoLink, dirac.DebugExwStats
	dirac.DebugBeholdExileNoLink = mutate
	var st dirac.ExwStats
	dirac.DebugExwStats = &st
	defer func() { dirac.DebugBeholdExileNoLink, dirac.DebugExwStats = old, oldSt }()
	for mi, md := range kbxDecks {
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
			m, err := NewMatchup(reg, [2]string{"kbeholdexile-a", "kbeholdexile-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kbxPolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			games++
			if d := res.Div; d != nil {
				divs++
				if first == "" {
					first = strings.TrimSpace(d.What + " " + d.Field)
				}
				if !mutate {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
			CloseSide(k)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) && !mutate {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
		}
	}
	tl.matched = st.Matched
	return games, divs, tl, first
}

func TestBeholdexileLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, tl, _ := runBeholdexile(t, 256, false)
	t.Logf("%d games, %d divergences, Champion casts paid from the battlefield %d / the hand %d, asks posed %d, leaves returns matched %d, priority decisions withholding a candidate-less Champion %d",
		games, divs, tl.bfCasts, tl.handCasts, tl.asked, tl.matched, tl.withheld)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if tl.bfCasts < 20 || tl.handCasts < 20 || tl.asked < 10 || tl.matched < 10 || tl.withheld < 1 {
		t.Errorf("thin coverage: battlefield %d (want 20), hand %d (want 20), asked %d (want 10), matched %d (want 10), withheld %d (want 1)",
			tl.bfCasts, tl.handCasts, tl.asked, tl.matched, tl.withheld)
	}
}

// TestBeholdexileMutation drops the cost exile's link: the Champion's leaves
// trigger finds nothing to return in the kernel, which gorge returns.
func TestBeholdexileMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, first := runBeholdexile(t, 128, true)
	if divs == 0 {
		t.Fatalf("mutation wrote no link in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games (first: %s)", divs, games, first)
}
