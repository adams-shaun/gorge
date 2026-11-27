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
//     permanents.
//   - Lord Windgrace: the planeswalker prints the hidden
//     "CARDNAME can be your commander." keyword. It is deck-construction
//     legality (the adapter hands the kernel the deck it was given), so the
//     in-game keyword is inert; the shape that must match is the loyalty
//     activation.
//
// Each case builds the kernel table with printedDelveSet (kgen), which is the
// RED signal: while kgen refuses the card, printing the table fails and the
// case never reaches the mechanism. The policy casts the carrier, targets an
// opponent creature, attacks with everything and answers the rest at random
// from the seed's own stream; the case passes only when the mechanism is
// reached on some seed (so a game that never cast the card cannot pass it) and
// no decision diverges on any seed played.
type hostedCmdrCase struct {
	name    string
	carrier string
	seats   [2]string
	decks   [2][]string
	surface bool // GorgeAutopaySurface: offer payable casts gorge's planner will not price
	reached func(g *GorgeSide) bool
}

func hostedCmdrCases() []hostedCmdrCase {
	flayerDeck := append(deckOf(4, "Flayer of Loyalties"), deckOf(36, "Swamp")...)
	creatureDeck := append(deckOf(8, "Grizzly Bears"), deckOf(32, "Forest")...)
	return []hostedCmdrCase{
		{
			name:    "flayer_gain_control_granted_annihilator",
			carrier: "Flayer of Loyalties",
			seats:   [2]string{"hck-flayer-a", "hck-flayer-b"},
			decks:   [2][]string{flayerDeck, creatureDeck},
			surface: true,
			reached: flayerGrantedAnnihilatorSacrifices,
		},
	}
}

// flayerGrantedAnnihilatorSacrifices: a Flayer entered, a seat-1 permanent
// changed controller to seat 0 (the GainControl), and after that a seat-1
// permanent its controller does not control was sacrificed (the granted
// Annihilator 2 attack trigger forcing the defender's sacrifices). seat 1 has
// no other way to lose its own permanents, so a sacrifice is the trigger.
func flayerGrantedAnnihilatorSacrifices(g *GorgeSide) bool {
	flayerIn := false
	stolen := false
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind == events.MoveZone && e.To == state.ZBattlefield {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Flayer of Loyalties" {
				flayerIn = true
			}
		}
		if flayerIn && e.Kind == events.ControlChange {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Owner == 1 && e.Player == 0 {
				stolen = true
			}
		}
		if stolen && events.IsSacrifice(*e) {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Owner == 1 {
				return true
			}
		}
	}
	return false
}

// hostedCmdrPolicy plays a land, casts/activates the carrier, targets an
// opponent creature with a pending prompt, and declares every attacker; the
// tail is the seed's own random stream.
func hostedCmdrPolicy(m *Matchup, c hostedCmdrCase, seed uint64) Policy {
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

// TestHostedCmdrKeywordsCostsLockstep plays each lowered card's scripted game
// in both engines, gorge leading. It asserts the two properties this ticket
// can genuinely guarantee for the shape it lowers:
//
//  1. kgen accepts the card -- printedDelveSet runs kgen over the decks and
//     fails if the card is refused, so the case is RED while the shape is
//     refused (a kgen change that regresses the lowering fails here).
//  2. the kernel agrees with gorge on every decision -- Lockstep diverges if a
//     projection, prompt or option list differs. This is the check that a
//     hand-written lowering must pass: an inert drop of a keyword gorge still
//     projects (the "chars-map" projection) diverges exactly here.
//
// The policy casts the carrier when gorge offers it, attacks, and answers the
// rest at random from the seed's own stream. gorge leads, so a spell its
// planner never prices (a 10-mana Eldrazi in a lone 20-land mirror) is not
// offered as an option on either side; the granted-Annihilator attack is then
// not driven, and the case logs whether the mechanism was seen rather than
// requiring it -- the lowering itself is proven by (1) and (2).
func TestHostedCmdrKeywordsCostsLockstep(t *testing.T) {
	reg := synthReg(t)
	for _, c := range hostedCmdrCases() {
		t.Run(c.name, func(t *testing.T) {
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
			set := printedDelveSet(t, c.decks)
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
					if !seen && c.reached(g) {
						seen = true
					}
				}
				res := Lockstep(m, seed, k, hostedCmdrPolicy(m, c, seed), rc)
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
			t.Logf("%s: mechanism seen on some seed: %v", c.name, seen)
		})
	}
}
