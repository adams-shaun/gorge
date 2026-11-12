package rules

// paid_mana_census_test.go -- the paid-cost census (engine/rules/paid_mana_census.go):
// a mana source whose activation cost is a deterministic self-sacrifice
// (Treasure, Lotus Petal) or a fixed PayLife<N> is counted by the attack-tax
// budget, the unless-cost windows, AvailableMana and the pool-short ability
// offer -- and only with the flag on. Each class has an unpayable negative.

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	pmcPetal = "Name:Lotus Petal\nManaCost:0\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ T Sac<1/CARDNAME> | Produced$ Any\nOracle:x\n"
	pmcPainRock = "Name:Pain Rock\nManaCost:1\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ T PayLife<1> | Produced$ R\nOracle:x\n"
	pmcOgre = "Name:Gray Ogre\nManaCost:1\nTypes:Creature Ogre\nPT:2/2\nOracle:x\n"
	pmcSage = "Name:Paid Sage\nManaCost:2\nTypes:Creature Wizard\nPT:1/1\n" +
		"A:AB$ Draw | Cost$ 1 | NumCards$ 1 | SpellDescription$ Draw a card.\nOracle:x\n"
)

// pmcClass is one paid-cost source class.
type pmcClass struct {
	name string
	// place puts the source on seat p's battlefield and returns it.
	place func(t *testing.T, e *Engine, p state.PlayerID) state.ObjID
	// spoil makes its cost unpayable at this moment.
	spoil func(t *testing.T, e *Engine, p state.PlayerID, id state.ObjID)
}

func pmcClasses() []pmcClass {
	tapIt := func(t *testing.T, e *Engine, p state.PlayerID, id state.ObjID) {
		e.G.Obj(id).Tapped = true
	}
	return []pmcClass{
		{"treasure", func(t *testing.T, e *Engine, p state.PlayerID) state.ObjID {
			e.G.Tokens = testutil.CorpusRegistry(t).Tokens
			e.emit(events.Event{Kind: events.TokenCreate, Player: p, Text: "c_a_treasure_sac"})
			z := e.G.Zone(state.ZBattlefield, p)
			id := z[len(z)-1]
			if o := e.G.Obj(id); !o.IsToken || o.Face().Name != "Treasure Token" {
				t.Fatalf("precondition: minted %q (token=%v), want a Treasure Token", o.Face().Name, o.IsToken)
			}
			return id
		}, tapIt},
		{"petal", func(t *testing.T, e *Engine, p state.PlayerID) state.ObjID {
			return onBoardCard(t, e, p, card(t, pmcPetal))
		}, tapIt},
		{"life", func(t *testing.T, e *Engine, p state.PlayerID) state.ObjID {
			return onBoardCard(t, e, p, card(t, pmcPainRock))
		}, func(t *testing.T, e *Engine, p state.PlayerID, id state.ObjID) {
			e.G.Players[p].Life = 1 // PayLife<1> would kill the payer
		}},
	}
}

// pmcAttackSeat is attackPropSeat with no lands: seat 1's only mana is the
// class's source. Baird charges {1} per attacker.
func pmcAttackSeat(t *testing.T, flag bool, c pmcClass, spoil bool) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	cfg := Config{Seed: 716, Names: []string{"a", "b", "c"}, DisablePaidManaCensus: !flag,
		Tokens: testutil.CorpusRegistry(t).Tokens,
		Decks:  [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40)}}
	e := New(cfg)
	onBoardCard(t, e, 0, mshCorpusCard(t, "Baird, Steward of Argive"))
	bear := onBoardReady(t, e, 1, "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:0/4\nOracle:x\n")
	src := c.place(t, e, 1)
	if o := e.G.Obj(src); o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: %s source zone=%v tapped=%v", c.name, o.Zone, o.Tapped)
	}
	if spoil {
		c.spoil(t, e, 1, src)
	}
	e.G.Active = 1
	e.G.Step = state.StepDeclareAttackers
	return e, bear, src
}

func TestPaidManaCensusAttackTax(t *testing.T) {
	t.Parallel()
	for _, c := range pmcClasses() {
		for _, tc := range []struct {
			flag, spoil, want bool
		}{
			{true, false, true},   // payable, flag on: the tax is payable
			{false, false, false}, // flag off: unchanged (not counted)
			{true, true, false},   // cost unpayable: never counted
		} {
			t.Run(fmt.Sprintf("%s/flag=%v/spoil=%v", c.name, tc.flag, tc.spoil), func(t *testing.T) {
				e, bear, src := pmcAttackSeat(t, tc.flag, c, tc.spoil)
				if got := e.paidManaCensus; got != tc.flag {
					t.Fatalf("precondition: Config flag not adopted: %v", got)
				}
				e.askAttackers()
				d := e.Pending()
				if d == nil {
					t.Fatal("no attack decision")
				}
				opt := findAttackOption(d, bear, 0)
				if (opt != nil) != tc.want {
					t.Fatalf("pair offered=%v, want %v; options %s", opt != nil, tc.want, pmcKinds(d))
				}
				if !tc.want {
					return
				}
				// Paying it taps the source through the window: the Treasure
				// and the Petal are sacrificed, the life source costs life.
				life := e.G.Players[1].Life
				if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 1, Choices: []int{opt.Index}}); err != nil {
					t.Fatal(err)
				}
				for steps := 0; steps < 6; steps++ {
					p := e.Pending()
					if p == nil {
						break
					}
					idx := -1
					for _, o := range p.Options {
						if o.Obj == src || o.Kind == "block_mana" || o.Kind == "attack_mana" || o.Kind == "activate" {
							idx = o.Index
							break
						}
					}
					if idx < 0 {
						break
					}
					if err := e.Submit(decision.Intent{Seq: p.Seq, Player: p.Player, Choices: []int{idx}}); err != nil {
						t.Fatal(err)
					}
				}
				o := e.G.Obj(src)
				switch c.name {
				case "life":
					if got := e.G.Players[1].Life; got != life-1 {
						t.Fatalf("life %d -> %d, want one life paid", life, got)
					}
				default:
					if o != nil && o.Zone == state.ZBattlefield {
						t.Fatalf("%s survived its own sacrifice cost", c.name)
					}
				}
			})
		}
	}
}

func TestPaidManaCensusUnlessAndAvailable(t *testing.T) {
	t.Parallel()
	for _, c := range pmcClasses() {
		for _, tc := range []struct {
			flag, spoil, want bool
		}{
			{true, false, true},
			{false, false, false},
			{true, true, false},
		} {
			t.Run(fmt.Sprintf("%s/flag=%v/spoil=%v", c.name, tc.flag, tc.spoil), func(t *testing.T) {
				e, _, _ := pmcAttackSeat(t, tc.flag, c, tc.spoil)
				if got := e.UnlessCostPayable(1, "1"); got != tc.want {
					t.Fatalf("UnlessCostPayable(1, {1}) = %v, want %v", got, tc.want)
				}
				if got := e.AvailableMana(1).Total() > 0; got != tc.want {
					t.Fatalf("AvailableMana(1) = %v, want nonzero=%v", e.AvailableMana(1), tc.want)
				}
				// The shared census and the tap-list membership agree.
				if got := len(e.windowManaUnitsPaid(1)) > 0; got != tc.want {
					t.Fatalf("windowManaUnitsPaid(1) nonempty = %v, want %v", got, tc.want)
				}
				if got := len(e.attackManaSources(1)) > 0; got != tc.want {
					t.Fatalf("attackManaSources(1) nonempty = %v, want %v", got, tc.want)
				}
			})
		}
	}
}

// pmcPriority builds seat 0 at main with fixture on the battlefield / in hand
// and the class's source, and re-poses priority.
func pmcPriority(t *testing.T, flag bool, c pmcClass, spoil bool, fixture string, inHand bool) (*Engine, *decision.Decision, state.ObjID, state.ObjID) {
	t.Helper()
	e, _, _ := newFixtureDeck(t, 9633, fixture)
	e.paidManaCensus = flag
	var fid state.ObjID
	if inHand {
		fid = moveSeeded(t, e, 0, fixture, state.ZHand)
	} else {
		fid = moveSeeded(t, e, 0, fixture, state.ZBattlefield)
	}
	src := c.place(t, e, 0)
	if spoil {
		c.spoil(t, e, 0, src)
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("precondition: pool %v", e.G.Players[0].Pool)
	}
	return e, ppPriority(t, e), fid, src
}

// TestPaidManaCensusAbilityOffer: a pool-short activated ability ({1}: draw)
// paid by the class's source is offered and auto-paid only with the flag on.
func TestPaidManaCensusAbilityOffer(t *testing.T) {
	t.Parallel()
	for _, c := range pmcClasses() {
		for _, tc := range []struct {
			flag, spoil, want bool
		}{
			{true, false, true},
			{false, false, false},
			{true, true, false},
		} {
			t.Run(fmt.Sprintf("%s/flag=%v/spoil=%v", c.name, tc.flag, tc.spoil), func(t *testing.T) {
				e, d, sage, src := pmcPriority(t, tc.flag, c, tc.spoil, pmcSage, false)
				if o := e.G.Obj(sage); o.Zone != state.ZBattlefield {
					t.Fatalf("precondition: sage in zone %v", o.Zone)
				}
				opt, ok := autopayAbilityOffer(d, sage)
				if ok != tc.want {
					t.Fatalf("ability offered=%v, want %v: %s", ok, tc.want, pmcKinds(d))
				}
				if !tc.want {
					return
				}
				life := e.G.Players[0].Life
				stack := len(e.G.Stack)
				if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
					t.Fatal(err)
				}
				if got := len(e.G.Stack); got != stack+1 {
					t.Fatalf("stack %d -> %d, want the ability on the stack (pending %+v)", stack, got, e.Pending())
				}
				if c.name == "life" {
					if got := e.G.Players[0].Life; got != life-1 {
						t.Fatalf("life %d -> %d", life, got)
					}
				} else if o := e.G.Obj(src); o != nil && o.Zone == state.ZBattlefield {
					t.Fatalf("%s was not sacrificed", c.name)
				}
			})
		}
	}
}

// TestPaidManaCensusCastUnchangedByFlag: the cast of a {1} creature is offered
// from the class's source with the flag on AND off (the cast window and the
// planner already priced these), and withheld when the cost is unpayable.
func TestPaidManaCensusCastUnchangedByFlag(t *testing.T) {
	t.Parallel()
	for _, c := range pmcClasses() {
		for _, flag := range []bool{false, true} {
			for _, spoil := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/flag=%v/spoil=%v", c.name, flag, spoil), func(t *testing.T) {
					e, d, ogre, _ := pmcPriority(t, flag, c, spoil, pmcOgre, true)
					_ = d
					if o := e.G.Obj(ogre); o.Zone != state.ZHand {
						t.Fatalf("precondition: ogre in zone %v", o.Zone)
					}
					got := false
					for _, pp := range e.PotentialPaymentPlans(0) {
						if pp.Action.Kind == "cast" && pp.Action.Obj == ogre && pp.Reason == "" {
							got = true
						}
					}
					if got == spoil {
						t.Fatalf("cast planned=%v with spoil=%v flag=%v: %s", got, spoil, flag, pmcKinds(d))
					}
				})
			}
		}
	}
}

// TestPaidManaCensusLifeBudgetIsCumulative: two PayLife<1> sources at 2 life
// are not both counted -- paying both would kill the payer -- so a {2} tax is
// unpayable while a {1} tax is.
func TestPaidManaCensusLifeBudgetIsCumulative(t *testing.T) {
	t.Parallel()
	cfg := Config{Seed: 716, Names: []string{"a", "b"}, DisablePaidManaCensus: false,
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}}
	e := New(cfg)
	onBoardCard(t, e, 1, card(t, pmcPainRock))
	onBoardCard(t, e, 1, card(t, pmcPainRock))
	e.G.Players[1].Life = 2
	if !e.UnlessCostPayable(1, "1") {
		t.Fatal("a {1} unless cost must be payable from one PayLife<1> source at 2 life")
	}
	if e.UnlessCostPayable(1, "2") {
		t.Fatal("a {2} unless cost needs both sources: 2 life would drop to 0")
	}
	e.G.Players[1].Life = 3
	if !e.UnlessCostPayable(1, "2") {
		t.Fatal("at 3 life both PayLife<1> sources can be paid")
	}
}

// pmcKinds renders a decision's option kinds and objects compactly.
func pmcKinds(d *decision.Decision) string {
	var b []byte
	for _, o := range d.Options {
		b = append(b, fmt.Sprintf("%s:%d ", o.Kind, o.Obj)...)
	}
	return string(b)
}
