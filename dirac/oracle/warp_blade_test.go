package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Blade of the Swarm's mode 2 `ValidTgts$ Card.withWarp`
// over `Origin$ Exile`, the filter word withWarp (PredWord7 11, the printed
// keyword in any zone) and the kernel's exile card-target walk (CR 406.1,
// State.exileCardCands). Deck A casts Blade after warp creatures were exiled
// at the end step; deck B's Tormod's Crypt exiles whole graveyards, so the
// exile zone also holds non-warp cards that mode 2 must never offer.
var bladeWarpDecks = [2][]cardset.Entry{
	{
		{Name: "Blade of the Swarm", Count: 4}, {Name: "Haliya, Guided by Light", Count: 4},
		{Name: "Rayblade Trooper", Count: 4}, {Name: "All-Fates Stalker", Count: 4},
		{Name: "Knight Luminary", Count: 4}, {Name: "Swamp", Count: 8}, {Name: "Plains", Count: 12},
	},
	{
		{Name: "Memorial Team Leader", Count: 4}, {Name: "Red Tiger Mechan", Count: 4},
		{Name: "Weftstalker Ardent", Count: 4}, {Name: "Nova Hellkite", Count: 4},
		{Name: "Bygone Colossus", Count: 4}, {Name: "Tormod's Crypt", Count: 4},
		{Name: "Mountain", Count: 16},
	},
}

// bladeTally counts the arms from gorge's own decisions and event log.
type bladeTally struct {
	evAt     int
	mode2    int // Blade's Charm mode-2 option offered
	tgt      int // target asks of mode 2 that offer an exiled warp card
	badTgt   int // exiled non-warp cards offered by such an ask (must stay 0)
	plainExi int // such asks made while a non-warp card sat in exile
	toLib    int // exiled warp cards put into a library
	casts    int // Blade casts picked
}

func (b *bladeTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < b.evAt {
		b.evAt = 0
	}
	for j := b.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind == events.MoveZone && e.From == state.ZExile && e.To == state.ZLibrary {
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil && warpNames[o.Face().Name] {
				b.toLib++
			}
		}
	}
	b.evAt = len(evs)
	fr := ref.fr
	if fr == nil || fr.d == nil {
		return nil
	}
	d := fr.d
	switch d.Kind {
	case decision.KModes:
		for _, o := range d.Options {
			if strings.Contains(o.Label, "exiled card with warp") {
				b.mode2++
				break
			}
		}
	case decision.KTarget:
		// gorge's prompt names the source ("Choose a target for Blade of the
		// Swarm"), not the card's TgtPrompt; mode 1 has no target, so the
		// only target ask of a Blade is mode 2's
		if d.Prompt != "Choose a target for Blade of the Swarm" {
			return nil
		}
		warp := false
		for _, o := range d.Options {
			ob := g.Obj(o.Obj)
			if o.Obj == 0 || ob == nil || ob.Face() == nil {
				continue
			}
			if ob.Zone == state.ZExile && ob.Face().HasKeyword("Warp") {
				warp = true
			} else {
				b.badTgt++
			}
		}
		if warp {
			b.tgt++
		}
		for pid := range g.Players {
			for _, id := range g.Zone(state.ZExile, state.PlayerID(pid)) {
				if ob := g.Obj(id); ob != nil && ob.Face() != nil && !ob.Face().HasKeyword("Warp") {
					b.plainExi++
					return nil
				}
			}
		}
	}
	return nil
}

// exiledWarp is the number of exiled cards that print Warp.
func exiledWarp(g *state.Game) (n int) {
	for pid := range g.Players {
		for _, id := range g.Zone(state.ZExile, state.PlayerID(pid)) {
			if ob := g.Obj(id); ob != nil && ob.Face() != nil && ob.Face().HasKeyword("Warp") {
				n++
			}
		}
	}
	return n
}

// bladePolicy prefers Blade's mode 2 and an exiled warp target, casts Blade
// and the warp creatures, and feeds Tormod's Crypt; else it plays at random.
func bladePolicy(seed uint64, b *bladeTally) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x626c616465)) // "blade"
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			switch c.D.Kind {
			case DModes:
				for _, a := range c.D.Options {
					if oa, ok := c.G.GorgeOption(a); ok && strings.Contains(oa.Label, "exiled card with warp") {
						return a
					}
				}
			case DTarget:
				for _, a := range c.D.Options {
					if oa, ok := c.G.GorgeOption(a); ok && oa.Obj != 0 {
						if ob := g.Obj(oa.Obj); ob != nil && ob.Zone == state.ZExile && ob.Face() != nil && ob.Face().HasKeyword("Warp") {
							return a
						}
					}
				}
			case DPriority:
				for _, a := range c.D.Options {
					oa, ok := c.G.GorgeOption(a)
					if !ok || oa.Obj == 0 {
						continue
					}
					switch {
					case a.Kind() == ActCast && chName(g, oa.Obj) == "Blade of the Swarm" && (exiledWarp(g) >= 2 || prng.IntN(6) == 0):
						// two candidates make gorge ask the target (one is taken unasked)
						b.casts++
						return a
					case a.Kind() == ActCast && oa.Mode == "warped" && prng.IntN(3) == 0:
						return a
					case a.Kind() == ActCastFace && oa.Mode == "warp_recast" && prng.IntN(2) == 0:
						return a // keeps the play-permission records under their cap (maxMay)
					case a.Kind() == ActActivate && chName(g, oa.Obj) == "Tormod's Crypt" && prng.IntN(2) == 0:
						return a
					}
				}
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
		}
		return rnd.Pick(c)
	})
}

// runBlade plays the matchup in both seatings. mutate negates the kernel's
// props9 answers (withWarp included), which must diverge.
func runBlade(t *testing.T, seeds int, mutate bool) (games, divs, mayStalls int, b bladeTally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("warp")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugWord9Flip
	dirac.DebugWord9Flip = mutate
	defer func() { dirac.DebugWord9Flip = old }()
	for swap := 0; swap < 2; swap++ {
		md := bladeWarpDecks
		if swap == 1 {
			md = [2][]cardset.Entry{md[1], md[0]}
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
				t.Fatalf("swap %d deck %d: %d cards", swap, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"blade-a", "blade-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, bladePolicy(seed, &b), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30, Check: b.check})
			CloseSide(k)
			games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				// the warp pool's play-from-zone permission cap (maxMay: one
				// DurWarp record per warp exile, kcaps2x) ends a long game;
				// the decisions before it were in lockstep. Any other stall
				// is an error.
				if res.Stall == KernelStallPrefix+"mayplay" {
					mayStalls++
				} else if !mutate {
					t.Errorf("swap %d seed %d: kernel stall %q", swap, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					t.Errorf("swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, mayStalls, b
}

func TestBladeWithWarpLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, b := runBlade(t, 40, false)
	t.Logf("%d games, %d divergences, %d mayplay-cap stalls: %+v", games, divs, stalls, b)
	if stalls*4 > games {
		t.Errorf("%d of %d games hit the mayplay cap", stalls, games)
	}
	if b.mode2 < 1 {
		t.Error("Blade's exiled-card-with-warp mode was never offered")
	}
	if b.tgt < 1 {
		t.Error("no mode-2 target ask offered an exiled warp card")
	}
	if b.badTgt != 0 {
		t.Errorf("%d exiled non-warp cards offered by mode 2, want 0", b.badTgt)
	}
	if b.toLib < 1 {
		t.Error("no exiled warp card was put into a library")
	}
	if b.plainExi < 1 {
		t.Error("no mode-2 ask was made with a non-warp card in exile (the bad-target check is vacuous)")
	}
}

func TestBladeWithWarpMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, b := runBlade(t, 40, true)
	t.Logf("%d games, %d divergences: %+v", games, divs, b)
	if divs == 0 {
		t.Error("withWarp negated never diverged")
	}
}
