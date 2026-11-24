package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestBandingPrintedLockstep: the printed K:Banding carriers (CardDef.Banding,
// no KwMask bit is free) are read by the kernel's banding paths beside the
// granted kop.Override.Banding records (dirac/kw_banding.go hasBanding): the
// CR 702.22c band declaration ask each declared banding attacker with a
// candidate poses, and the CR 702.22j/k combat-damage division (an attacker
// blocked by a banding creature divides by the defending player, a blocker
// blocking a banding attacker by the attacking player). Both seats hold three
// of the group's carriers, so both directions run; the probe declines every
// band ask (band formation is not modelled, an accepted band stalls) and
// blocks to double up, a banding creature first. Errand of Duty, an instant,
// reaches the granted path beside the printed one.

// kbpCarriers are the printed K:Banding carriers, three groups of six; each
// group is one matchup, seat 0 the group's first three, seat 1 the rest.
var kbpCarriers = [][]string{
	{"Benalish Infantry", "Errand of Duty", "Icatian Phalanx", "Kjeldoran Escort", "Kjeldoran Knight", "Kjeldoran Phalanx"},
	{"Kjeldoran Skycaptain", "Kjeldoran Skyknight", "Kjeldoran Warrior", "Knights of Thorn", "Nalathni Dragon", "Noble Elephant"},
	{"Pikemen", "Shield Bearer", "Teremko Griffin", "Volunteer Reserves", "Wall of Shields", "War Elephant"},
}

// kbpDecks builds the group's decks: two copies of each carrier, six Grizzly
// Bears (the second attacker of a band ask, and the plain blocker) and lands.
func kbpDecks(g []string) [2][]cardset.Entry {
	var d [2][]cardset.Entry
	for s, half := range [][]string{g[:3], g[3:]} {
		for _, c := range half {
			d[s] = append(d[s], cardset.Entry{Name: c, Count: 2})
		}
		d[s] = append(d[s], cardset.Entry{Name: "Grizzly Bears", Count: 6})
		d[s] = append(d[s], cardset.Entry{Name: "Plains", Count: 12}, cardset.Entry{Name: "Island", Count: 4},
			cardset.Entry{Name: "Swamp", Count: 4}, cardset.Entry{Name: "Mountain", Count: 4},
			cardset.Entry{Name: "Forest", Count: 4})
	}
	return d
}

// kbpStats counts what the probe saw.
type kbpStats struct {
	games, divs, stalls int
	bandAsks            int            // band declaration asks the probe declined
	divAsks             int            // division asks of the player not on the turn: CR 702.22j's defender
	activePicks         int            // a division ask of the player on the turn: CR 702.22k's attacker
	blocks              int            // a carrier declared as a blocker
	seen                map[string]int // a carrier attacking, blocking or cast (the instant)
}

// kbpPolicy plays lands and casts and activates first, attacks with
// everything, blocks a banding creature first and then joins the previous
// pick, and declines every band ask (kpecDeclineBand). carriers names the
// group's printed-banding cards.
func kbpPolicy(m *Matchup, seed uint64, st *kbpStats, carriers map[string]bool) Policy {
	rnd := RandomPolicy(seed)
	division, _ := chooseCode("division")
	lastAtk := Ref(RefNone)
	return PolicyFunc(func(c *PickCtx) Canon {
		if a, ok := kpecDeclineBand(c.D); ok {
			st.bandAsks++
			return a
		}
		if c.D.Kind == DAttackers {
			lastAtk = RefNone
			for _, o := range c.D.Options {
				if o.Kind() == ActAttack {
					if n := m.InstanceCard(int(o.Src())); carriers[n] {
						st.seen[n]++
					}
					return o
				}
			}
		}
		if c.D.Kind == DBlockers {
			band, join, any := -1, -1, -1
			for i, o := range c.D.Options {
				if o.Kind() != ActBlock {
					continue
				}
				if any < 0 {
					any = i
				}
				if carriers[m.InstanceCard(int(o.Src()))] && band < 0 {
					band = i
				}
				if lastAtk != RefNone && o.Tgt() == lastAtk && join < 0 {
					join = i
				}
			}
			pick := join
			if pick < 0 {
				pick = band
			}
			if pick < 0 {
				pick = any
			}
			if pick >= 0 {
				o := c.D.Options[pick]
				if n := m.InstanceCard(int(o.Src())); carriers[n] {
					st.blocks++
					st.seen[n]++
				}
				if pick == join {
					lastAtk = o.Tgt()
				}
				return o
			}
		}
		if c.D.Kind == DChoose && c.G != nil && len(c.D.Options) > 0 && c.D.Options[0].Kind() == ActChoose+CanonKind(division) {
			if int(c.G.E.G.Active) != int(c.D.Player) {
				st.divAsks++
			} else {
				st.activePicks++
			}
		}
		for _, o := range c.D.Options {
			if k := o.Kind(); k == ActPlayLand || k == ActCast || k == ActActivate {
				if k == ActCast {
					// an instant carrier (Errand of Duty) reaches combat only
					// through its cast: its grant is the banding read
					if n := m.InstanceCard(int(o.Src())); carriers[n] {
						st.seen[n]++
					}
				}
				return o
			}
		}
		return rnd.Pick(c)
	})
}

func TestBandingPrintedLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("banding")
	if err != nil {
		t.Fatal(err)
	}
	for gi, g := range kbpCarriers {
		g := g
		t.Run(strings.Join(g, ","), func(t *testing.T) {
			t.Parallel()
			decks := kbpDecks(g)
			var gd [2][]*cards.Card
			for s, d := range decks {
				for _, e := range d {
					c, ok := reg.Lookup(e.Name)
					if !ok {
						t.Fatalf("%s not in the corpus", e.Name)
					}
					for i := 0; i < e.Count; i++ {
						gd[s] = append(gd[s], c)
					}
				}
				if len(gd[s]) != 40 {
					t.Fatalf("deck %d: %d cards", s, len(gd[s]))
				}
			}
			tbl, _, _, err := set.Matchup(decks[0], decks[1])
			if err != nil {
				t.Fatal(err)
			}
			carriers := map[string]bool{}
			for _, n := range g {
				carriers[n] = true
			}
			st := &kbpStats{seen: map[string]int{}}
			for seed := uint64(1); seed <= 200; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"kbp-a", "kbp-b"}, gd, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				gside, err := NewGorgeSide(m, seed, cfg, nil)
				if err != nil {
					t.Fatal(err)
				}
				res := LockstepLed(m, seed, gside, k, kbpPolicy(m, seed, st, carriers), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 24})
				CloseSide(k)
				st.games++
				if strings.HasPrefix(res.Stall, KernelStallPrefix) {
					st.stalls++
					t.Errorf("seed %d: kernel stall %q", seed, res.Stall)
				}
				if d := res.Div; d != nil {
					st.divs++
					t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
					break
				}
			}
			t.Logf("group %d: %d games, %d band asks, %d division asks (%d on the turn), %d banding blocks; seen %v",
				gi, st.games, st.bandAsks, st.divAsks, st.activePicks, st.blocks, st.seen)
			if st.divs != 0 || st.stalls != 0 {
				t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
			}
			if st.bandAsks < 5 {
				t.Errorf("only %d band asks (want at least 5): no printed-banding attacker attacked beside another creature", st.bandAsks)
			}
			if st.divAsks+st.activePicks < 5 {
				t.Errorf("only %d banding division asks (want at least 5): no banding blocker doubled up or blocked a banding attacker", st.divAsks+st.activePicks)
			}
			for _, n := range g {
				if st.seen[n] == 0 {
					t.Errorf("%s never reached a combat decision in the pool", n)
				}
			}
		})
	}
}
