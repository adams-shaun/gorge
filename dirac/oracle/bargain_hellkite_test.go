package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The bargained Realm-Scorcher Hellkite lockstep (CR 702.166, 106.3, 605.1a).
// The autopay surface offers a mode cast only when the floating pool alone
// pays it (gorge's planner never plans one), so the bargained {4}{R}{R}
// needs six mana with RR already floating. Iridescent Tiger floats WUBRG;
// Lotus Cobra's landfall adds the sixth, red, after it: cast Cobra early,
// cast Tiger from lands, play a land. The bargained Hellkite's triggered
// "add four mana in any combination of colors" is a resolving Mana effect (a
// triggered ability is no mana ability), so each of its four colour picks is
// a real decision the gorge side poses and the kernel's ExtManaChoice asks.
//
// The ask frame is asserted on every pick: unitDedupe (a Combo allocation
// offers each colour once), ResumeKind "mana_color", ResumeSA.Kind "DB" (the
// triggered ability, not an "AB" mana ability) and every option Src RefNone
// (a colour, exactly as Chandra's Combo ask).

var kbargainHellkiteDecks = [2][]cardset.Entry{
	{
		{Name: "Realm-Scorcher Hellkite", Count: 4}, {Name: "Iridescent Tiger", Count: 4},
		{Name: "Lotus Cobra", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 12},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 18}, {Name: "Mountain", Count: 18},
	},
}

type kbargainHellkiteTally struct {
	casts int // bargained Hellkite casts taken
	picks int // Combo-Any colour picks asked
}

// kbargainHellkiteZone is the name -> count of one of seat p's zones.
func kbargainHellkiteZone(g *GorgeSide, z state.Zone, p uint8) map[string]int {
	out := map[string]int{}
	for _, id := range g.E.G.Zone(z, state.PlayerID(p)) {
		if o := g.E.G.Obj(id); o != nil && o.Card != nil && len(o.Card.Faces) > 0 {
			out[o.Card.Faces[0].Name]++
		}
	}
	return out
}

// kbargainHellkitePolicy drives the Cobra -> Tiger -> land line while a
// Hellkite is in hand, takes the bargained cast the moment it is offered,
// and answers the Combo ask with the first colour. Everything else is random.
func kbargainHellkitePolicy(seed uint64, c *kbargainHellkiteTally, t *testing.T) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		g := p.G
		if p.D.Kind == DChoose {
			if code, ok := chooseCode("bargain"); ok {
				for _, a := range p.D.Options {
					if a.Kind() == ActChoose+CanonKind(code) {
						return a
					}
				}
			}
			if g != nil && g.fr != nil && g.fr.unitDedupe {
				if g.fr.d.Kind != decision.KChoose || g.fr.d.ResumeKind != "mana_color" ||
					g.fr.d.ResumeSA == nil || g.fr.d.ResumeSA.Kind != "DB" || len(p.D.Options) != 5 {
					t.Errorf("seq %d: Combo ask frame: kind %v resume %q sa %v, %d options",
						p.Seq, g.fr.d.Kind, g.fr.d.ResumeKind, g.fr.d.ResumeSA, len(p.D.Options))
				}
				for _, a := range p.D.Options {
					if a.Src() != RefNone {
						t.Errorf("seq %d: Combo ask option %v has src %v, want RefNone", p.Seq, a, a.Src())
					}
				}
				c.picks++
				return p.D.Options[0]
			}
		}
		if p.D.Kind != DPriority || g == nil {
			return rnd.Pick(p)
		}
		for _, a := range p.D.Options {
			if a.Kind() == ActCast && a.Mode()>>4 == 0 && a.Mode()&0xf == kbargainCastOther &&
				refCardName(g, a.Src()) == "Realm-Scorcher Hellkite" {
				c.casts++
				return a
			}
		}
		hand := kbargainHellkiteZone(g, state.ZHand, p.D.Player)
		if hand["Realm-Scorcher Hellkite"] == 0 {
			return rnd.Pick(p)
		}
		bf := kbargainHellkiteZone(g, state.ZBattlefield, p.D.Player)
		pl := &g.E.G.Players[p.D.Player]
		pool, landed := pl.Pool.Total(), pl.LandsPlayed
		castOf := func(name string) (Canon, bool) {
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && a.Mode() == 0 && refCardName(g, a.Src()) == name {
					return a, true
				}
			}
			return 0, false
		}
		land := func() (Canon, bool) {
			for _, a := range p.D.Options {
				if a.Kind() == ActPlayLand {
					return a, true
				}
			}
			return 0, false
		}
		if bf["Ornithopter"] == 0 {
			if a, ok := castOf("Ornithopter"); ok {
				return a // the free bargain sacrifice
			}
		}
		if bf["Lotus Cobra"] > 0 && hand["Iridescent Tiger"] > 0 && pool == 0 && landed == 0 {
			if a, ok := castOf("Iridescent Tiger"); ok {
				return a
			}
		}
		if pool >= 5 && landed == 0 {
			if a, ok := land(); ok {
				return a // the Cobra's landfall trigger adds the sixth mana
			}
		}
		if bf["Lotus Cobra"] == 0 && pool == 0 {
			if a, ok := castOf("Lotus Cobra"); ok {
				return a
			}
		}
		if pool == 0 {
			if a, ok := land(); ok {
				return a
			}
		}
		for _, a := range p.D.Options {
			if a.Kind() == ActPass {
				return a
			}
		}
		return rnd.Pick(p)
	})
}

func TestBargainHellkiteLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("bargain")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kbargainHellkiteDecks {
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
	tbl, _, _, err := set.Matchup(kbargainHellkiteDecks[0], kbargainHellkiteDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{GorgeAutopaySurface: true}
	var c kbargainHellkiteTally
	games := 0
	for seed := uint64(1); seed <= 64; seed++ {
		m, err := NewMatchup(reg, [2]string{"kbargain-hk-a", "kbargain-hk-b"}, decks, 0)
		if err != nil {
			t.Fatalf("seed %d: matchup: %v", seed, err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: side: %v", seed, err)
		}
		before := c
		res := Lockstep(m, seed, k, kbargainHellkitePolicy(seed, &c, t),
			RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 100})
		CloseSide(k)
		games++
		if got, want := c.picks-before.picks, 4*(c.casts-before.casts); got != want {
			t.Errorf("seed %d: %d Combo-Any picks for %d bargained casts, want %d",
				seed, c.picks-before.picks, c.casts-before.casts, want)
		}
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
		if res.Stall != "" {
			t.Errorf("seed %d: stalled: %s after %d turns", seed, res.Stall, res.Turns)
		}
	}
	t.Logf("%d games, bargained Hellkite casts %d, Combo-Any picks %d", games, c.casts, c.picks)
	if c.casts < 1 {
		t.Errorf("bargained Hellkite casts: %d, want >= 1", c.casts)
	}
	if c.picks != 4*c.casts {
		t.Errorf("Combo-Any picks %d, want 4 per cast (%d)", c.picks, 4*c.casts)
	}
}
