package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Silence and Ranger-Captain of Eos: a no-target Effect whose end-of-turn
// emblem static is "your opponents can't cast spells" (Silence) or "can't
// cast noncreature spells" (the Captain's sacrifice ability), played decision
// by decision against gorge. Each recorded game holds the emblem to both
// halves of the card:
//
//   - Caster$ Opponent: the controller casts under its own emblem the turn it
//     is made (a Savannah Lions after Silence; a Sol Ring after the
//     sacrifice), and both engines offer it.
//   - the Captain's noncreature filter: the opposing seat, holding a Lightning
//     Bolt and an Ambush Viper with an untapped Forest, casts the flash Viper
//     after the sacrifice has resolved; the Bolt, which the emblem
//     withholds, is never cast (that seat's Bolts are hidden from the policy
//     only so they stay in hand; a Bolt the kernel offered and gorge did not
//     would be a divergence).
//
// TestReproSilenceRangerCaptainReach replays the recorded games and requires
// each to reach those casts; they are listed in TestStdSetReprosLockstep.
//
// The games are recorded by TestReproSilenceRangerCaptainRecord
// (CANT_BE_CAST_RECORD=1) on the kernel table built from the decks.
func silenceCases() []effectLongCase {
	bolts := append(deckOf(8, "Lightning Bolt"), deckOf(16, "Mountain")...)
	boltsVipers := append(append(deckOf(6, "Lightning Bolt"), deckOf(4, "Ambush Viper")...), append(deckOf(8, "Mountain"), deckOf(6, "Forest")...)...)
	return []effectLongCase{
		{
			name:    "silence_opponents_cannot_cast_spells",
			prefer:  []string{"Silence", "Savannah Lions"},
			decks:   [2][]string{append(append(deckOf(6, "Silence"), deckOf(6, "Savannah Lions")...), deckOf(12, "Plains")...), bolts},
			surface: "autopay",
			reached: func(g *GorgeSide) bool {
				return emblemCast(g, silenceEmblem, "Savannah Lions", 0)
			},
		},
		{
			name:    "ranger_captain_sacrifice_opponents_cannot_cast_noncreature_spells",
			prefer:  []string{"Ranger-Captain of Eos", "Sol Ring", "Ambush Viper"},
			decks:   [2][]string{append(append(deckOf(5, "Ranger-Captain of Eos"), deckOf(5, "Sol Ring")...), deckOf(14, "Plains")...), boltsVipers},
			surface: "autopay",
			reached: func(g *GorgeSide) bool {
				return emblemCast(g, captainEmblem, "Sol Ring", 0) && emblemCast(g, captainEmblem, "Ambush Viper", 1)
			},
		},
	}
}

// emblemSrc names how a card makes its emblem: the move that makes it, and
// whether a later stack resolution is what creates it (the Captain's
// sacrifice is a cost; the emblem exists once the ability has resolved).
type emblemSrc struct {
	card     string
	from, to state.Zone
	resolve  bool
}

var (
	silenceEmblem = emblemSrc{"Silence", state.ZStack, state.ZGraveyard, false}
	captainEmblem = emblemSrc{"Ranger-Captain of Eos", state.ZBattlefield, state.ZGraveyard, true}
)

// emblemScan walks the log: it reports whether the emblem is up now (made
// earlier this turn) and whether a card named cast was cast by seat while it
// was up.
func emblemScan(g *GorgeSide, src emblemSrc, cast string, seat state.PlayerID) (up, hit bool) {
	moved := false
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.TurnChange:
			up, moved = false, false
		case e.Kind == events.MoveZone && e.From == src.from && e.To == src.to && !moved:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == src.card {
				moved, up = true, !src.resolve
			}
		case e.Kind == events.Resolve && moved:
			up = true
		case up && e.Kind == events.PutOnStack && e.From == state.ZHand:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == cast && o.Owner == seat {
				hit = true
			}
		}
	}
	return up, hit
}

func emblemCast(g *GorgeSide, src emblemSrc, cast string, seat state.PlayerID) bool {
	_, hit := emblemScan(g, src, cast, seat)
	return hit
}

// viperReady: seat 1 holds an Ambush Viper and has an untapped Forest.
func viperReady(g *GorgeSide) bool {
	viper, forest := false, false
	for _, id := range g.E.G.Zone(state.ZHand, 1) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Ambush Viper" {
			viper = true
		}
	}
	for _, id := range g.E.G.Zone(state.ZBattlefield, 1) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Forest" && !o.Tapped {
			forest = true
		}
	}
	return viper && forest
}

// silencePolicy is cantBeCastPolicy with the emblem's timing made certain:
//   - Silence is cast on its controller's own turn, and a Savannah Lions only
//     once Silence has resolved that turn;
//   - the Captain is sacrificed (whenever offered) only while seat 1 holds a
//     Bolt and a castable Viper, a Sol Ring is cast only once the ability has
//     resolved, and so is seat 1's Viper;
//   - seat 1 never casts a Bolt, so the one the emblem withholds stays in hand.
func silencePolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	inner := cantBeCastPolicy(m, c, seed)
	src := silenceEmblem
	if c.prefer[0] != "Silence" {
		src = captainEmblem
	}
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind != DPriority || p.G == nil {
			return inner.Pick(p)
		}
		up, _ := emblemScan(p.G, src, "", 0)
		own := p.G.E.G.Active == 0
		d := *p.D
		d.Options = nil
		for _, o := range p.D.Options {
			name := m.InstanceCard(int(o.Src()))
			if o.Kind() == ActActivate && name == "Ranger-Captain of Eos" && boltHeld(p.G) && viperReady(p.G) {
				return o
			}
			if o.Kind() == ActCast {
				switch {
				case name == "Lightning Bolt":
					continue
				case name == "Silence" && !own:
					continue
				case (name == "Savannah Lions" || name == "Sol Ring" || name == "Ambush Viper") && !up:
					continue
				}
			}
			d.Options = append(d.Options, o)
		}
		return inner.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
	})
}

// TestReproSilenceRangerCaptainReach replays the recorded games: no
// divergence, every scripted action offered, the card reached.
func TestReproSilenceRangerCaptainReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range silenceCases() {
		t.Run(c.name, func(t *testing.T) {
			replayEffectLongCase(t, reg, c)
		})
	}
}

// TestReproSilenceRangerCaptainRecord plays the cases' policies over seeds
// until one reaches the card, and writes the repro. It runs only with
// CANT_BE_CAST_RECORD=1.
func TestReproSilenceRangerCaptainRecord(t *testing.T) {
	if os.Getenv("CANT_BE_CAST_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range silenceCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, silencePolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
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
