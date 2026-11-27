package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The hosted-Commander census blockers this ticket lowers, played decision by
// decision against gorge (gorge leads) so the kernel takes the shape the same:
//
//   - Flayer of Loyalties: its cast trigger's DB$ GainControl carries
//     `AddKWs$ Haste & Trample & Annihilator:2` beside LoseControl$ EOT. The
//     kernel gains control of an opponent creature, grants it haste and
//     trample until end of turn (OpGrantKeyword) and rides a latent Attacks
//     trigger (the granted Annihilator 2) on an end-of-turn override record;
//     the stolen creature then attacks and the defending player sacrifices two
//     permanents, and at end of turn the creature returns.
//
// The real Flayer costs {8}{C}{C}, which no 20-land mirror reaches, so its
// case only proves kgen lowers the printed card and the kernel agrees with
// gorge on every decision. The mechanism itself is driven by "Flayer Probe",
// an inline {2}{B} creature carrying the identical cast trigger script: its
// case FAILS unless some seed casts the probe, steals a creature, attacks
// with it, sees the defender sacrifice and sees the creature come back.
//
// Each case builds the kernel table with printedDelveSetIn (kgen), which is
// the RED signal: while kgen refuses the shape, printing the table fails. The
// policy casts the carrier, targets an opponent creature, attacks with
// everything and answers the rest at random from the seed's own stream.
type hostedCmdrCase struct {
	name    string
	carrier string
	seats   [2]string
	decks   [2][]string
	surface bool // GorgeAutopaySurface: offer payable casts gorge's planner will not price
	// fixtures are inline card scripts added to the corpus; real names the
	// corpus cards the decks also hold.
	fixtures map[string]string
	// reached, when set, must hold on some seed or the case fails.
	reached func(g *GorgeSide) bool
}

// flayerProbeScript is Flayer of Loyalties' cast trigger on a {2}{B} body.
const flayerProbeScript = "Name:Flayer Probe\nManaCost:2 B\nTypes:Creature Eldrazi\nPT:2/2\n" +
	"T:Mode$ SpellCast | ValidCard$ Card.Self | Execute$ TrigGainControl | TriggerDescription$ Steal a creature for the turn.\n" +
	"SVar:TrigGainControl:DB$ GainControl | Untap$ True | ValidTgts$ Creature | AddKWs$ Haste & Trample & Annihilator:2 | LoseControl$ EOT | SubAbility$ DBAnimate\n" +
	"SVar:DBAnimate:DB$ Animate | Defined$ Targeted | Toughness$ 10 | Power$ 10\n" +
	"Oracle:When you cast this spell, gain control of target creature until end of turn.\n"

func hostedCmdrCases() []hostedCmdrCase {
	flayerDeck := append(deckOf(4, "Flayer of Loyalties"), deckOf(36, "Swamp")...)
	creatureDeck := append(deckOf(8, "Grizzly Bears"), deckOf(32, "Forest")...)
	probeDeck := append(deckOf(12, "Flayer Probe"), deckOf(28, "Swamp")...)
	return []hostedCmdrCase{
		{
			name:     "flayer_probe_gain_control_granted_annihilator",
			carrier:  "Flayer Probe",
			seats:    [2]string{"hck-probe-a", "hck-probe-b"},
			decks:    [2][]string{probeDeck, creatureDeck},
			fixtures: map[string]string{"Flayer Probe": flayerProbeScript},
			reached:  flayerGrantedAnnihilatorSacrifices,
		},
		{
			name:    "flayer_gain_control_granted_annihilator",
			carrier: "Flayer of Loyalties",
			seats:   [2]string{"hck-flayer-a", "hck-flayer-b"},
			decks:   [2][]string{flayerDeck, creatureDeck},
			surface: true,
		},
	}
}

// flayerGrantedAnnihilatorSacrifices: a probe entered, a seat-1 permanent
// changed controller to seat 0 (the GainControl), after that a seat-1
// permanent was sacrificed (the granted Annihilator 2 attack trigger forcing
// the defender's sacrifices; seat 1 has no other way to lose its own
// permanents), and the stolen creature then went back to seat 1 (the
// LoseControl$ EOT expiry, the granted keywords expiring with it).
func flayerGrantedAnnihilatorSacrifices(g *GorgeSide) bool {
	probeIn := false
	var stolen state.ObjID
	sacrificed := false
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind == events.MoveZone && e.To == state.ZBattlefield {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Flayer Probe" {
				probeIn = true
			}
		}
		if probeIn && stolen == 0 && e.Kind == events.ControlChange {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Owner == 1 && e.Player == 0 {
				stolen = e.Obj
			}
			continue
		}
		if stolen != 0 && !sacrificed && events.IsSacrifice(*e) {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Owner == 1 && e.Obj != stolen {
				sacrificed = true
			}
		}
		if sacrificed && e.Kind == events.ControlChange && e.Obj == stolen && e.Player == 1 {
			return true
		}
	}
	return false
}

// hostedCmdrKwCostsPolicy plays a land, casts/activates the carrier, targets an
// opponent creature with a pending prompt, and declares every attacker; the
// tail is the seed's own random stream.
func hostedCmdrKwCostsPolicy(m *Matchup, c hostedCmdrCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DTarget && p.G != nil {
			for _, a := range p.D.Options {
				o, ok := p.G.GorgeOption(a)
				if !ok || a.Kind() != ActTarget {
					continue
				}
				if ob := p.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Owner == 1 {
					return a
				}
			}
		}
		if p.D.Kind == DAttackers {
			if p.D.Player == 0 {
				for _, a := range p.D.Options {
					if a.Kind() == ActAttack {
						return a
					}
				}
			} else {
				// seat 1 never attacks: it just declares none (a non-attack
				// option), so seat 0 lives long enough to cast the carrier.
				for _, a := range p.D.Options {
					if a.Kind() != ActAttack {
						return a
					}
				}
			}
		}
		if p.D.Kind == DPriority {
			for _, a := range p.D.Options {
				if (a.Kind() == ActCast || a.Kind() == ActActivate) && m.InstanceCard(int(a.Src())) == c.carrier {
					return a
				}
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActPlayLand {
					return a
				}
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActCast || a.Kind() == ActActivate {
					return a
				}
			}
		}
		return rnd.Pick(p)
	})
}

// TestHostedCmdrKeywordsCostsLockstep plays each lowered shape's scripted game
// in both engines, gorge leading, and asserts:
//
//  1. kgen accepts the card: printedDelveSetIn runs kgen over the decks and
//     fails if a card is refused, so a case is RED while its shape is refused.
//  2. the kernel agrees with gorge on every decision (Lockstep diverges on a
//     projection, prompt or option list that differs).
//  3. a case with a reached predicate fails unless some seed reaches the
//     mechanism, so a game that never cast the carrier cannot pass it.
func TestHostedCmdrKeywordsCostsLockstep(t *testing.T) {
	for _, c := range hostedCmdrCases() {
		t.Run(c.name, func(t *testing.T) {
			dir := corpusDirOrSkip(t)
			if len(c.fixtures) > 0 {
				dir = filters11Corpus(t, c.fixtures, "Swamp", "Forest", "Grizzly Bears")
			}
			reg, err := cards.SharedCorpus(dir)
			if err != nil {
				t.Fatal(err)
			}
			entries := func(names []string) []cardset.Entry {
				es := make([]cardset.Entry, len(names))
				for i, n := range names {
					es[i] = cardset.Entry{Name: n, Count: 1}
				}
				return es
			}
			var decks [2][]*cards.Card
			for s, names := range c.decks {
				for _, n := range names {
					card, ok := reg.Lookup(n)
					if !ok {
						t.Fatalf("%s not in the corpus", n)
					}
					decks[s] = append(decks[s], card)
				}
				if len(decks[s]) != 40 {
					t.Fatalf("seat %d deck has %d cards, want 40", s, len(decks[s]))
				}
			}
			// printedDelveSet runs kgen over the decks: a refused card fails here.
			set := printedDelveSetIn(t, dir, c.decks)
			tbl, _, _, err := set.Matchup(entries(c.decks[0]), entries(c.decks[1]))
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: c.surface}
			seen := false
			played := 0
			for seed := uint64(1); seed <= 60 && !(seen && played >= 3); seed++ {
				m, err := NewMatchup(reg, c.seats, decks, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				rc := RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40}
				rc.Finish = func(g *GorgeSide) {
					if !seen && c.reached != nil && c.reached(g) {
						seen = true
					}
				}
				res := Lockstep(m, seed, k, hostedCmdrKwCostsPolicy(m, c, seed), rc)
				if res.Div != nil {
					d := res.Div
					CloseSide(k)
					t.Fatalf("seed %d: divergence at %d: %s %s %v\ngorge %v\ndirac %v", seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions)
				}
				if res.Decisions > 100 {
					played++
				}
				CloseSide(k)
			}
			if played < 3 {
				t.Fatalf("only %d seeds played a real game for %s", played, c.name)
			}
			if c.reached != nil && !seen {
				t.Fatalf("%s: the mechanism was not reached on any seed played, so the lockstep is vacuous", c.name)
			}
		})
	}
	t.Run("windgrace_commander_keyword", hostedCmdrWindgrace)
	t.Run("necrodominance_paylife_x_trigger", hostedCmdrNecrodominance)
}
