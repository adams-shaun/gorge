package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The filtered activated discard cost lockstep:
// Solitary Cell's {1}{T}, Discard a legendary card: Draw a card and Page,
// Loose Leaf's grandeur Discard a card named Page, Loose Leaf (Discard<
// 1/Card.namedPage, Loose Leaf>) are payable only with the hand cards the
// filter admits (the kernel counts and pays them, gorge's
// pay.DiscardCandidates the same). Decks lean on both carriers; side A is
// R/W with the legendary discardable creatures, side B is U/G fill.
var kdiscfiltDecks = [1][2][]cardset.Entry{{
	{
		{Name: "Solitary Cell", Count: 6}, {Name: "Page, Loose Leaf", Count: 8},
		{Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Shock", Count: 2}, {Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Grizzly Bears", Count: 10}, {Name: "Island", Count: 11},
		{Name: "Forest", Count: 11},
		// Opt (pool index 5, before Page, Loose Leaf at 6) is the one pool
		// card no deck holds: Page's matchup CardID then differs from its
		// pool index, so the grandeur's PredNamed PredN -- a pool index --
		// must be remapped and the shipped table reads the wrong card
		// without it (findings r2)
		{Name: "Swamp", Count: 8},
	},
}}

// kdiscfiltTally counts, on gorge's side, the carriers' resolutions: a
// legendary card discarded as a cost followed by a draw (Solitary Cell), a
// Page, Loose Leaf discarded as a cost followed by an instant or sorcery
// moved to hand (the grandeur's DigUntil search), and how often an untapped
// Solitary Cell sits at priority with no legendary hand card to discard
// (the gate options the kernel must then refuse).
type kdiscfiltTally struct {
	scPlus, pgPlus, games int
	gateRuledOut          int
}

// kdiscfiltPolicy plays a land, a spell or a battlefield activation whenever
// it can (else random); every fourth seed plays fully at random.
func kdiscfiltPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	if seed%4 == 0 {
		return rnd
	}
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DPriority {
			buf = buf[:0]
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActPlayLand || k == ActActivate {
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

// kdiscfiltRun plays seeds 1..n of the constructed matchup in lockstep,
// failing on each divergence (report) or stopping at the first; it returns
// the number of divergent games.
func kdiscfiltRun(t *testing.T, n uint64, report bool, tal *kdiscfiltTally) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("discfilt")
	if err != nil {
		t.Fatal(err)
	}
	md := kdiscfiltDecks[0]
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, idA, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	// the grandeur's PredNamed filter names Page, Loose Leaf by pool index
	// (6) and the matchup must remap it to the CardID Page has HERE: no
	// deck holds Opt (pool index 5, before Page), so the remap is not the
	// identity and the shipped table would read the wrong card without it
	// (findings r2)
	pageID := idA[6] // md[0][1] is the eight Page, Loose Leaf copies
	if int(pageID) == 6 {
		t.Fatalf("Page, Loose Leaf's matchup CardID is its pool index 6: the deck no longer shifts the remap")
	}
	named := 0
	for i := range tbl.Filters {
		f := &tbl.Filters[i]
		if f.Pred == kop.PredNamed && f.Zone == kop.ZoneHand {
			named++
			if f.PredN != int16(pageID) {
				t.Fatalf("grandeur filter PredN %d, want Page, Loose Leaf's matchup CardID %d", f.PredN, pageID)
			}
		}
	}
	if named != 1 {
		t.Fatalf("%d PredNamed hand filters in the table, want exactly Page, Loose Leaf's", named)
	}
	div := 0
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"df-a", "df-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		pending := "" // "sc" after a legendary cost discard, "pg" after a named-Page one
		gateLogged := map[int]bool{}
		// kdiscfiltCheck tallies the resolutions and logs the gate's ruled
		// out activations on gorge's side.
		check := func(ref *GorgeSide, _ Side) *Divergence {
			if tal == nil {
				return nil
			}
			evs := ref.E.L.Events
			for ; seen < len(evs); seen++ {
				ev := evs[seen]
				switch {
				case events.IsDiscardCost(ev):
					f := ""
					if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil {
						f = o.Face().Name
					}
					switch {
					case f == "Page, Loose Leaf":
						pending = "pg"
					case f == "Isamaru, Hound of Konda":
						pending = "sc"
					default:
						pending = ""
					}
				case pending == "sc" && ev.Kind == events.Draw:
					tal.scPlus++
					pending = ""
				case pending == "pg" && ev.Kind == events.MoveZone &&
					ev.From == state.ZLibrary && ev.To == state.ZHand:
					fn := ""
					if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil {
						fn = o.Face().Name
					}
					if fn == "Opt" || fn == "Shock" {
						tal.pgPlus++
						pending = ""
					}
				}
			}
			return nil
		}
		// kdiscfiltGate logs, at each priority decision the policy sees on
		// gorge's side, an activation option of an untapped Solitary Cell
		// whose controller holds no legendary hand card (what the
		// CostDiscardFiltered gate rules out; the lockstep itself would fail
		// on any offer mismatch).
		pol := kdiscfiltPolicy(seed)
		if seed%4 != 0 {
			base := pol
			pol = PolicyFunc(func(c *PickCtx) Canon {
				if tal != nil && c.G != nil && c.D.Kind == DPriority {
					for _, a := range c.D.Options {
						if a.Kind() != ActActivate {
							continue
						}
						for _, p := range []state.PlayerID{0, 1} {
							for _, id := range c.G.E.G.Zone(state.ZBattlefield, p) {
								o := c.G.E.G.Obj(id)
								if o == nil || o.Face() == nil || o.Face().Name != "Solitary Cell" ||
									c.G.InstRef(id) != a.Src() {
									continue
								}
								leg := false
								for _, hid := range c.G.E.G.Zone(state.ZHand, p) {
									if h := c.G.E.G.Obj(hid); h != nil && h.Face() != nil &&
										(h.Face().Name == "Isamaru, Hound of Konda" || h.Face().Name == "Page, Loose Leaf") {
										leg = true
										break
									}
								}
								if !leg {
									tal.gateRuledOut++
									if !gateLogged[c.Seq] {
										gateLogged[c.Seq] = true
										t.Logf("decision %d: Solitary Cell activation offered with no legendary hand card", c.Seq)
									}
								}
							}
						}
					}
				}
				return base.Pick(c)
			})
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
		CloseSide(k)
		if tal != nil {
			tal.games++
		}
		if res.Stall != "" && res.Stall != "turns" {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			div++
			if !report {
				t.Logf("seed %d diverges (decision %d: %s)", seed, d.At, d.What)
				return div
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div
}

func TestDiscfiltLockstep(t *testing.T) {
	t.Parallel()
	var tal kdiscfiltTally
	n := kdiscfiltRun(t, 200, true, &tal)
	t.Logf("%d games: Solitary Cell draw-after-legendary-discard %d, Page grandeur search %d, gate-ruled-out activations %d",
		tal.games, tal.scPlus, tal.pgPlus, tal.gateRuledOut)
	if tal.scPlus < 10 {
		t.Errorf("Solitary Cell: %d legendary-discards followed by a draw; want at least 10", tal.scPlus)
	}
	if tal.pgPlus < 10 {
		t.Errorf("Page grandeur: %d named-discards followed by the search; want at least 10", tal.pgPlus)
	}
	if n != 0 {
		t.Errorf("%d divergent games", n)
	}
}
