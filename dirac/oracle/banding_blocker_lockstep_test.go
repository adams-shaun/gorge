package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestBandingBlockerLockstep: a creature that has banding while it blocks (CR
// 702.22j) reaches the kernel through the grants kgen lowers to a
// kop.Override record with Banding: Dire Wolves' and Banding Sliver's static
// AddKeyword$ Banding (read from the static's own condition and filter), and
// Beast Walkers' and Icatian Infantry's until-end-of-turn Pump KW$ Banding.
// An attacker the second seat double-blocks with such a creature is divided by
// the defending player, not its controller (dirac/combat_band.go bandDivider);
// gorge asks the same player at the same decision with the same options.
// Neither engine forms a band (the probe declines every band ask: band
// formation is not modelled), so only the blocker half of CR 702.22j runs.

var kbbDecks = [2][]cardset.Entry{
	{
		{Name: "Grizzly Bears", Count: 10},
		{Name: "Hill Giant", Count: 6},
		{Name: "Mountain", Count: 12},
		{Name: "Forest", Count: 12},
	},
	{
		{Name: "Dire Wolves", Count: 6},
		{Name: "Beast Walkers", Count: 4},
		{Name: "Icatian Infantry", Count: 4},
		{Name: "Banding Sliver", Count: 2},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 12},
		{Name: "Forest", Count: 8},
	},
}

// kbbBanding are the second seat's creatures that gain banding.
var kbbBanding = map[string]bool{"Dire Wolves": true, "Beast Walkers": true, "Icatian Infantry": true, "Banding Sliver": true}

type kbbStats struct {
	games, divs, stalls int
	bandAsks            int            // band declaration asks the probe declined
	divAsks             int            // division asks of the player not on the turn: CR 702.22j's defender
	activePicks         int            // a division ask of the player on the turn (the attacker's own division)
	blocks              map[string]int // a banding creature declared as a blocker, by name
	doubled             int            // a second blocker declared on an attacker a banding creature already blocks
}

// kbbPolicy plays lands and casts and activates first, attacks with
// everything, and blocks to double up: a banding creature blocks first, then
// every other blocker joins the attacker the previous pick blocked.
func kbbPolicy(m *Matchup, seed uint64, st *kbbStats) Policy {
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
				if kbbBanding[m.InstanceCard(int(o.Src()))] && band < 0 {
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
				if n := m.InstanceCard(int(o.Src())); kbbBanding[n] {
					st.blocks[n]++
				}
				if pick == join {
					st.doubled++
				}
				lastAtk = o.Tgt()
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
				return o
			}
		}
		return rnd.Pick(c)
	})
}

func TestBandingBlockerLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("bandingblock")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kbbDecks {
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
	tbl, _, _, err := set.Matchup(kbbDecks[0], kbbDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st := &kbbStats{blocks: map[string]int{}}
	for seed := uint64(1); seed <= 96; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kbb-a", "kbb-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kbbPolicy(m, seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 24})
		CloseSide(k)
		st.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			st.stalls++
			t.Errorf("seed %d: kernel stall %q", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			st.divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%+v", *st)
	if st.divAsks < 5 {
		t.Errorf("only %d banding-blocker division asks (want at least 5): a banding blocker never double-blocked", st.divAsks)
	}
}
