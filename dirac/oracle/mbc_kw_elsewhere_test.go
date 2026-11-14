package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestMBCKeywordsElsewhereLockstep plays, in the MBC draft pool that holds each
// carrier, a mirror of the carrier and its basic, gorge leading. Each keyword is
// one gorge's engine/cards expands into ordinary lines kgen lowers (Transmute and
// Outlast an activated ability, Fabricate, Afterlife, Afflict and For Mirrodin a
// trigger; kgen expandedKeywords) or, for Madness, CardDef.HasMadness and its
// native trigger (TestPauperkwMadnessLockstep plays the discard and the exile
// cast). None is a battlefield characteristic, so the chars projection lists it
// in gorgeKwElsewhere; before it did, the first carrier on the battlefield was a
// projection error ("has no kernel bit").
//
// The policy casts and activates whatever it is offered. Every carrier must
// reach gorge's stack through its keyword's ability (counted in gorge's event
// log by the expansion's Keyword$ tag), except the Madness carrier, which must be cast.
func TestMBCKeywordsElsewhereLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cases := []struct {
		kw, carrier, basic string
		ability            bool // the keyword's ability is pushed by the carrier
	}{
		{"Transmute", "Tolaria West", "Island", true},
		{"Outlast", "Longshot Squad", "Forest", true},
		{"Fabricate", "Weaponcraft Enthusiast", "Swamp", true},
		{"Afterlife", "Ministrant of Obligation", "Plains", true},
		{"Afflict", "Neheb, the Eternal", "Mountain", true},
		{"For Mirrodin", "Barbed Batterfist", "Mountain", true},
		{"Madness", "Arrogant Wurm", "Forest", false},
	}
	reg := synthReg(t)
	for _, c := range cases {
		t.Run(c.kw, func(t *testing.T) {
			t.Parallel()
			pool := ""
			var set cardset.Set
			for _, name := range cardsettest.SynthNames(t) {
				if !strings.HasPrefix(name, cardset.SynthDraftPrefix+"mbc") {
					continue
				}
				s := cardsettest.Synth(t, name)
				if i, ok := s.Lookup(c.carrier); ok {
					if st := s.Status(i); st == cardset.StatusRefused {
						t.Fatalf("%s refuses %s: %s", name, c.carrier, s.Reason(i))
					}
					pool, set = name, s
					break
				}
			}
			if pool == "" {
				t.Fatalf("no MBC draft pool holds %s", c.carrier)
			}
			deck := []cardset.Entry{{Name: c.carrier, Count: 16}, {Name: c.basic, Count: 24}}
			card, ok := reg.Lookup(c.carrier)
			if !ok {
				t.Fatalf("%s not in the corpus", c.carrier)
			}
			basic, ok := reg.Lookup(c.basic)
			if !ok {
				t.Fatalf("%s not in the corpus", c.basic)
			}
			var cs [2][]*cards.Card
			for s := range cs {
				for i := 0; i < 16; i++ {
					cs[s] = append(cs[s], card)
				}
				for i := 0; i < 24; i++ {
					cs[s] = append(cs[s], basic)
				}
			}
			tbl, _, _, err := set.Matchup(deck, deck)
			if err != nil {
				t.Fatalf("%s: %v", pool, err)
			}
			played := map[string]int{}
			pushed := 0
			for seed := uint64(1); seed <= 40; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"kw-a", "kw-b"}, cs, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				var gs *GorgeSide
				inner := eagerPolicy(seed, played)
				pol := PolicyFunc(func(pc *PickCtx) Canon {
					if pc.G != nil {
						gs = pc.G
					}
					return inner.Pick(pc)
				})
				res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
				CloseSide(k)
				if d := res.Div; d != nil {
					t.Fatalf("%s seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", pool, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
				}
				if res.StallUnmodelled {
					t.Fatalf("%s seed %d: unmodelled stall %s", pool, seed, res.Stall)
				}
				if gs == nil {
					continue
				}
				for _, ev := range gs.E.L.Events {
					o := gs.E.G.Obj(ev.Obj)
					if o == nil || o.Face() == nil || o.Face().Name != c.carrier {
						continue
					}
					// the keyword's own ability: its expansion's Keyword$ tag
					f, i := o.Face(), int(ev.Amount)
					switch {
					case ev.Kind == events.TriggerPush && i >= 0 && i < len(f.Triggers):
						if f.Triggers[i].ParamStr(cards.PKKeyword) == c.kw {
							pushed++
						}
					case ev.Kind == events.AbilityPush && i >= 0 && i < len(f.Abilities):
						if f.Abilities[i].ParamStr(cards.PKKeyword) == c.kw {
							pushed++
						}
					}
				}
			}
			t.Logf("%s (%s): %s cast or activated %d times, %d keyword abilities pushed", c.kw, pool, c.carrier, played[c.carrier], pushed)
			if played[c.carrier] == 0 {
				t.Errorf("%s was never cast or activated in 40 games", c.carrier)
			}
			if c.ability && pushed == 0 {
				t.Errorf("%s never put its %s ability on gorge's stack in 40 games", c.carrier, c.kw)
			}
		})
	}
}
