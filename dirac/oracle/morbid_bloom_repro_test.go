package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Morbid Bloom's "Exile target creature card from a graveyard, then create X
// 1/1 Saprolings, X = the exiled card's toughness" (CR 608.2g/h): the kernel
// read X = 0 for a graveyard card, so a Grizzly Bears target made no tokens
// where gorge makes two. One recorded game, seat 0 casting Morbid Bloom at a
// Bears in either graveyard.
//
// The game is recorded by TestMBBMorbidBloomTokenToughnessRecord
// (MORBID_BLOOM_RECORD=1) and replayed by
// TestMBBMorbidBloomTokenToughnessReprosLockstep.
type morbidBloomCase struct {
	effectLongCase
	card     string // the spell
	creature string // the card in a graveyard it exiles
}

func morbidBloomCases() []morbidBloomCase {
	decks := [2][]string{
		append(append(append(deckOf(8, "Morbid Bloom"), deckOf(10, "Grizzly Bears")...), deckOf(14, "Forest")...), deckOf(8, "Swamp")...),
		append(deckOf(14, "Grizzly Bears"), deckOf(26, "Forest")...),
	}
	return []morbidBloomCase{{
		effectLongCase: effectLongCase{
			name:    "morbid_bloom_exiles_grave_toughness",
			decks:   decks,
			surface: "autopay",
			reached: morbidBloomExiledAndMadeTokens,
		},
		card:     "Morbid Bloom",
		creature: "Grizzly Bears",
	}}
}

// morbidBloomGrave counts the case's creature cards in either graveyard.
func morbidBloomGrave(g *GorgeSide, name string) int {
	n := 0
	for _, p := range []state.PlayerID{0, 1} {
		for _, id := range g.E.G.Zone(state.ZGraveyard, p) {
			if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
				n++
			}
		}
	}
	return n
}

// morbidBloomExiledAndMadeTokens: the spell moved a creature card from a
// graveyard to exile, and a token was minted after it.
func morbidBloomExiledAndMadeTokens(g *GorgeSide) bool {
	exiled := false
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind == events.MoveZone && e.From == state.ZGraveyard && e.To == state.ZExile {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Grizzly Bears" {
				exiled = true
			}
		}
		if exiled && e.Kind == events.TokenCreate {
			return true
		}
	}
	return false
}

// morbidBloomPolicy plays lands and the case's creature, attacks with every
// attacker offered (so creatures reach a graveyard), casts Morbid Bloom once a
// creature card is there, and aims it at that card; the random tail answers
// the rest.
func morbidBloomPolicy(m *Matchup, c morbidBloomCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DAttackers {
			for _, o := range p.D.Options {
				if o.Kind() == ActAttack {
					return o
				}
			}
		}
		if p.D.Kind == DTarget && p.G != nil {
			for _, a := range p.D.Options {
				if a.Kind() != ActTarget {
					continue
				}
				if o, ok := p.G.GorgeOption(a); ok {
					if ob := p.G.E.G.Obj(o.Obj); ob != nil && ob.Zone == state.ZGraveyard && ob.Face() != nil && ob.Face().Name == c.creature {
						return a
					}
				}
			}
		}
		if p.D.Kind == DPriority && p.G != nil {
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == c.creature {
					return o
				}
			}
			if morbidBloomGrave(p.G, c.creature) > 0 {
				for _, o := range p.D.Options {
					if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == c.card {
						return o
					}
				}
			}
		}
		return rnd.Pick(p)
	})
}

// TestMBBMorbidBloomTokenToughnessRecord plays the case over seeds until one
// exiles a graveyard creature card and mints its Saprolings, and writes the
// repro. It runs only with MORBID_BLOOM_RECORD=1; the game it writes is the
// committed repro.
func TestMBBMorbidBloomTokenToughnessRecord(t *testing.T) {
	if os.Getenv("MORBID_BLOOM_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range morbidBloomCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
					Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", Pool: "custom", MaxTurns: 40}
				set := reproSet(t, "custom", r)
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, morbidBloomPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v %s last=%s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.LastSourced)
				}
				if !reached {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions", seed, len(res.Actions))
				return
			}
			t.Fatalf("no seed reached %s", c.name)
		})
	}
}

// TestMBBMorbidBloomTokenToughnessReprosLockstep replays the recorded game: no
// divergence, every scripted action offered, and the token count reached.
func TestMBBMorbidBloomTokenToughnessReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range morbidBloomCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			set := reproSet(t, "custom", r)
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("the kernel stopped: %q", res.Stall)
			}
			if !reached {
				t.Fatalf("the recorded game never reaches %s", c.name)
			}
		})
	}
}
