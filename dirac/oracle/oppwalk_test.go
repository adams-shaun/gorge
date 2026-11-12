package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The "target opponent or planeswalker" lockstep (CR 115.4, 115.2): Burning
// Sun's Avatar's enters trigger (that target plus an up-to-one creature slot),
// Jeskai Charm's DealDmg mode, Rakdos's Return (X damage, then that player or
// that planeswalker's controller discards X), Aether Charge, Zealot of the
// God-Pharaoh, Scuttling Doom Engine and Theater of Horrors all carry
// ValidTgts$ Opponent,Planeswalker: an opponent of the chooser or a
// planeswalker, never the chooser and never a creature. The expected answer at
// every decision is gorge's.

var koppwalkDecks = [2][]cardset.Entry{
	{
		{Name: "Burning Sun's Avatar", Count: 3}, {Name: "Jeskai Charm", Count: 3},
		{Name: "Rakdos's Return", Count: 3}, {Name: "Aether Charge", Count: 3},
		{Name: "Zealot of the God-Pharaoh", Count: 2}, {Name: "Scuttling Doom Engine", Count: 1},
		{Name: "Theater of Horrors", Count: 2}, {Name: "Baloth Pup", Count: 3},
		{Name: "Ajani, Caller of the Pride", Count: 2}, {Name: "Kaito, Cunning Infiltrator", Count: 2},
		{Name: "Mountain", Count: 7}, {Name: "Swamp", Count: 3}, {Name: "Plains", Count: 2},
		{Name: "Island", Count: 2}, {Name: "Forest", Count: 2},
	},
	{
		{Name: "Burning Sun's Avatar", Count: 2}, {Name: "Jeskai Charm", Count: 2},
		{Name: "Rakdos's Return", Count: 3}, {Name: "Aether Charge", Count: 3},
		{Name: "Zealot of the God-Pharaoh", Count: 2}, {Name: "Scuttling Doom Engine", Count: 2},
		{Name: "Theater of Horrors", Count: 2}, {Name: "Baloth Pup", Count: 2},
		{Name: "Aura Gnarlid", Count: 2}, {Name: "Ajani, Caller of the Pride", Count: 2},
		{Name: "Kaito, Cunning Infiltrator", Count: 2},
		{Name: "Mountain", Count: 7}, {Name: "Swamp", Count: 3}, {Name: "Plains", Count: 2},
		{Name: "Island", Count: 2}, {Name: "Forest", Count: 2},
	},
}

var koppwalkCards = map[string]bool{
	"Burning Sun's Avatar": true, "Jeskai Charm": true, "Rakdos's Return": true, "Aether Charge": true,
	"Zealot of the God-Pharaoh": true, "Scuttling Doom Engine": true, "Theater of Horrors": true,
	// Aether Charge's damage is dealt by the Beast that entered
	// (DamageSource$ TriggeredCardLKICopy), so its provenance names the Beast
	"Baloth Pup": true, "Aura Gnarlid": true,
}

// koppwalkStats counts the resolutions the pool must reach: dmg is a landed
// noncombat damage event from one of the carriers, dmgPw one whose recipient
// was a planeswalker, dmgPl one whose recipient was a player, and cards the
// distinct carriers that dealt any.
type koppwalkStats struct {
	dmg, dmgPw, dmgPl int
	cards             map[string]int
}

type koppwalkTally struct{ evAt int }

func (p *koppwalkTally) scan(ref *GorgeSide, st *koppwalkStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	if st.cards == nil {
		st.cards = map[string]int{}
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind != events.DamageProvenance || e.Amount <= 0 || strings.HasPrefix(e.Text, events.DamageProvenanceCombat) {
			continue
		}
		nm := chName(g, e.Obj)
		if !koppwalkCards[nm] || len(e.IDs) == 0 {
			continue
		}
		st.dmg++
		st.cards[nm]++
		if _, isPlayer := e.IDs[0].PlayerRef(); isPlayer {
			st.dmgPl++
		} else if o := g.Obj(e.IDs[0]); o != nil && o.Face() != nil && o.Face().IsPlaneswalker() {
			st.dmgPw++
		} else {
			st.cards[nm+" (creature)"]++ // Burning Sun's Avatar's second slot
		}
	}
	p.evAt = len(evs)
}

// koppwalkPolicy develops mana and casts, aims at a planeswalker whenever one
// is offered (the seed's parity: half the games prefer the player), picks
// Jeskai Charm's mode by seed and feeds the event log to the tally.
func koppwalkPolicy(seed uint64, st *koppwalkStats, tl *koppwalkTally) Policy {
	cast := castFirstPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		switch c.D.Kind {
		case DTarget:
			if c.G != nil && seed%2 == 1 {
				for _, a := range c.D.Options {
					if a.Kind() != ActTarget || int(a.Tgt()) >= int(RefPlayer) {
						continue
					}
					if o := c.G.E.G.Obj(c.G.id.obj[a.Tgt()]); o != nil && o.Face() != nil && o.Face().IsPlaneswalker() {
						return a
					}
				}
			}
		case DModes:
			// Jeskai Charm is the pool's only modal card: mode 1 is DealDmg,
			// the other two (top of library, PumpAll) are played in the
			// remaining seeds.
			for _, a := range c.D.Options {
				if a.Kind() == ActMode && uint64(a.Mode()) == seed/2%3 {
					return a
				}
			}
		}
		return cast.Pick(c)
	})
}

func runOppwalk(t *testing.T, seeds int) (games, divs, stalls int, st koppwalkStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("oppwalk")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range koppwalkDecks {
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
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		tbl, _, _, err := set.Matchup(koppwalkDecks[0], koppwalkDecks[1])
		if err != nil {
			t.Fatal(err)
		}
		m, err := NewMatchup(reg, [2]string{"koppwalk-a", "koppwalk-b"}, decks, 1)
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
		var tl koppwalkTally
		res := LockstepLed(m, seed, g, k, koppwalkPolicy(seed, &st, &tl), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		tl.scan(g, &st)
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: kernel stall %q", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return games, divs, stalls, st
}

func TestOppwalkLockstep(t *testing.T) {
	t.Parallel()
	games, divs, stalls, st := runOppwalk(t, 300)
	t.Logf("%d games, %d divergences, %d kernel stalls; damage %d (planeswalker %d, player %d) by card %v",
		games, divs, stalls, st.dmg, st.dmgPw, st.dmgPl, st.cards)
	if divs != 0 || stalls != 0 {
		t.Fatalf("lockstep: %d divergences, %d kernel stalls", divs, stalls)
	}
	if st.dmgPw < 3 || st.dmgPl < 20 {
		t.Fatalf("damage to planeswalkers %d (want >= 3), to players %d (want >= 20)", st.dmgPw, st.dmgPl)
	}
	for _, nm := range []string{"Burning Sun's Avatar", "Burning Sun's Avatar (creature)", "Jeskai Charm", "Rakdos's Return", "Baloth Pup"} {
		if st.cards[nm] == 0 {
			t.Errorf("%s never dealt damage: the pool does not reach it", nm)
		}
	}
}
