package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// Sephiroth, Fabled SOLDIER // Sephiroth, One-Winged Angel (the
// rakdos-muscle-scam-exe hosted deck): its back face carries two shapes kgen
// used to refuse, each played here decision by decision against gorge (gorge
// leads):
//
//   - the Attacks trigger's own Cost$ Sac<X/Creature.Other> with NumCards$ X
//     (Count$xPaid): gorge's triggered-cost window has no settle for an
//     announced sacrifice (cumulative.go costCarriesUnfoldedX,
//     triggeredCostComponentsPayable), so it poses the decline-only ask and
//     the draw never runs. The kernel's PaySac with PayBSubSacXTrig poses the
//     same one-option ask (interp_ask.go payAnswerGate).
//   - the Transform replacement (R:Event$ Transform | ValidCard$ Card.Self |
//     ReplaceWith$ DB$ Effect): gorge runs the body as the card flips into
//     the face (replacement_phase.go applyTransformReplacement), creating the
//     emblem whose dies trigger drains. The kernel runs the destination face's
//     kop.ReplTransform program before the flip (act_transform.go).
//
// Neither shape is reachable with the printed card in a 40-card mirror (it
// transforms on the fourth death in one turn), so each test pairs the real
// card (kgen must lower it, and the game must agree) with an inline probe
// whose case FAILS unless some seed reaches the shape.
type sephirothCase struct {
	name     string
	seats    [2]string
	decks    [2][]string
	fixtures map[string]string
	carrier  string
	// reached reports, at a game's end, whether the shape was exercised;
	// asked is set by the policy when gorge posed the probe's trigger cost.
	reached func(g *GorgeSide, asked bool) bool
}

// sacDrawProbeScript is the One-Winged Angel's attack trigger shape (a
// trigger's own Sac<X> cost, NumCards$ X over Count$xPaid) on a {B} body.
const sacDrawProbeScript = "Name:Sac Draw Probe\nManaCost:B\nTypes:Creature Human\nPT:2/2\n" +
	"T:Mode$ Attacks | ValidCard$ Card.Self | TriggerZones$ Battlefield | Execute$ ProbeSacDraw | TriggerDescription$ Probe: as it attacks, sacrifice X others to draw X.\n" +
	"SVar:ProbeSacDraw:AB$ Draw | NumCards$ X | Cost$ Sac<X/Creature.Other/another creature>\n" +
	"SVar:X:Count$xPaid\n" +
	"Oracle:Probe: as it attacks, sacrifice X others to draw X.\n"

// novaProbeScript transforms as it enters into a back face carrying the
// One-Winged Angel's Super Nova shape: a Transform replacement whose body is a
// permanent emblem with a dies-drain trigger.
const novaProbeScript = "Name:Nova Probe\nManaCost:B\nTypes:Creature Human\nPT:1/1\n" +
	"T:Mode$ ChangesZone | Destination$ Battlefield | Origin$ Any | ValidCard$ Card.Self | Execute$ ProbeFlip | TriggerDescription$ Probe: flips as it enters.\n" +
	"SVar:ProbeFlip:DB$ SetState | Mode$ Transform | Defined$ Self\n" +
	"AlternateMode:DoubleFaced\n" +
	"Oracle:Probe: flips as it enters.\n" +
	"\nALTERNATE\n\n" +
	"Name:Nova Probe Angel\nManaCost:no cost\nColors:black\nTypes:Creature Angel\nPT:2/2\n" +
	"R:Event$ Transform | ReplaceWith$ ProbeNova | ValidCard$ Card.Self | Description$ Probe nova: an emblem as it flips.\n" +
	"SVar:ProbeNova:DB$ Effect | Duration$ Permanent | Triggers$ ProbeNovaDies | Name$ Emblem — Nova Probe Angel\n" +
	"SVar:ProbeNovaDies:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature | TriggerZones$ Command | Execute$ ProbeNovaDrain | TriggerDescription$ Probe: a creature dies, drain 1.\n" +
	"SVar:ProbeNovaDrain:DB$ LoseLife | LifeAmount$ 1 | ValidTgts$ Opponent | SubAbility$ ProbeNovaGain\n" +
	"SVar:ProbeNovaGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You\n" +
	"Oracle:Probe nova: an emblem as it flips.\n"

const sephirothName = "Sephiroth, Fabled SOLDIER"

func sephirothRealCase(tag string) sephirothCase {
	return sephirothCase{
		name:    "sephiroth_printed",
		seats:   [2]string{"seph-" + tag + "-a", "seph-" + tag + "-b"},
		decks:   [2][]string{append(deckOf(8, sephirothName), deckOf(32, "Swamp")...), append(deckOf(8, "Grizzly Bears"), deckOf(32, "Forest")...)},
		carrier: sephirothName,
	}
}

// TestHostedCmdrSephirothSacrificeDrawLockstep: the attack trigger's own
// Sac<X> cost. The probe case fails unless gorge posed the probe's trigger
// cost ask on some seed (the kernel must pose the same options).
func TestHostedCmdrSephirothSacrificeDrawLockstep(t *testing.T) {
	runSephirothCases(t, []sephirothCase{
		{
			name:     "sac_draw_probe",
			seats:    [2]string{"seph-sac-a", "seph-sac-b"},
			decks:    [2][]string{append(deckOf(16, "Sac Draw Probe"), deckOf(24, "Swamp")...), append(deckOf(8, "Grizzly Bears"), deckOf(32, "Forest")...)},
			fixtures: map[string]string{"Sac Draw Probe": sacDrawProbeScript},
			carrier:  "Sac Draw Probe",
			reached:  func(_ *GorgeSide, asked bool) bool { return asked },
		},
		sephirothRealCase("sac"),
	})
}

// TestHostedCmdrSephirothTransformEmblemLockstep: the Transform replacement's
// emblem. The probe case fails unless some seed flipped a probe and then saw
// the emblem's drain (seat 1 loses 1 life, seat 0 gains 1 at once: neither
// face drains on its own).
func TestHostedCmdrSephirothTransformEmblemLockstep(t *testing.T) {
	runSephirothCases(t, []sephirothCase{
		{
			name:     "nova_probe_transform_emblem",
			seats:    [2]string{"seph-nova-a", "seph-nova-b"},
			decks:    [2][]string{append(deckOf(16, "Nova Probe"), deckOf(24, "Swamp")...), append(deckOf(12, "Grizzly Bears"), deckOf(28, "Forest")...)},
			fixtures: map[string]string{"Nova Probe": novaProbeScript},
			carrier:  "Nova Probe",
			reached:  func(g *GorgeSide, _ bool) bool { return novaEmblemDrained(g) },
		},
		sephirothRealCase("nova"),
	})
}

// novaEmblemDrained: a probe flipped, and after it seat 1 lost exactly 1 life
// with seat 0 gaining 1 as the next life change.
func novaEmblemDrained(g *GorgeSide) bool {
	flipped := false
	ev := g.E.L.Events
	for i := range ev {
		e := &ev[i]
		if e.Kind == events.FlipFace {
			flipped = true
			continue
		}
		if !flipped || e.Kind != events.LifeChange || e.Player != 1 || e.Amount != -1 {
			continue
		}
		for _, n := range ev[i+1:] {
			if n.Kind == events.LifeChange {
				if n.Player == 0 && n.Amount == 1 {
					return true
				}
				break
			}
		}
	}
	return false
}

// sephirothPolicy is hostedCmdrKwCostsPolicy (cast the carrier, target
// opponents' things, seat 0 attacks with everything, seat 1 never attacks),
// recording when gorge poses the carrier's own trigger-cost ask.
func sephirothPolicy(m *Matchup, c sephirothCase, seed uint64, asked *bool) Policy {
	inner := hostedCmdrKwCostsPolicy(m, hostedCmdrCase{carrier: c.carrier}, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.G != nil && p.D.Player == 0 {
			for _, a := range p.D.Options {
				o, ok := p.G.GorgeOption(a)
				if !ok || o.Kind != "trigger_cost_decline" {
					continue
				}
				if ob := p.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == c.carrier {
					*asked = true
				}
			}
		}
		return inner.Pick(p)
	})
}

func runSephirothCases(t *testing.T, cases []sephirothCase) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := corpusDirOrSkip(t)
			if len(c.fixtures) > 0 {
				dir = filters11Corpus(t, c.fixtures, "Swamp", "Forest", "Grizzly Bears")
			}
			reg, err := cards.SharedCorpus(dir)
			if err != nil {
				t.Fatal(err)
			}
			var decks [2][]*cards.Card
			var ents [2][]cardset.Entry
			for s, names := range c.decks {
				for _, n := range names {
					card, ok := reg.Lookup(n)
					if !ok {
						t.Fatalf("%s not in the corpus", n)
					}
					decks[s] = append(decks[s], card)
					ents[s] = append(ents[s], cardset.Entry{Name: n, Count: 1})
				}
				if len(decks[s]) != 40 {
					t.Fatalf("seat %d deck has %d cards, want 40", s, len(decks[s]))
				}
			}
			// printedDelveSetIn runs kgen over the decks: a refused card fails here.
			set := printedDelveSetIn(t, dir, c.decks)
			tbl, _, _, err := set.Matchup(ents[0], ents[1])
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{}
			seen, played := false, 0
			for seed := uint64(1); seed <= 60 && !(seen && played >= 3); seed++ {
				m, err := NewMatchup(reg, c.seats, decks, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				asked := false
				rc := RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40}
				rc.Finish = func(g *GorgeSide) {
					if c.reached != nil && c.reached(g, asked) {
						seen = true
					}
				}
				res := Lockstep(m, seed, k, sephirothPolicy(m, c, seed, &asked), rc)
				CloseSide(k)
				if d := res.Div; d != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v\ngorge %v\ndirac %v", seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions)
				}
				if res.Decisions > 100 {
					played++
				}
			}
			if played < 3 {
				t.Fatalf("only %d seeds played a real game for %s", played, c.name)
			}
			if c.reached != nil && !seen {
				t.Fatalf("%s: the shape was not reached on any seed played, so the lockstep is vacuous", c.name)
			}
		})
	}
}
