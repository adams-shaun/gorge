package rules

// quiet_paid_mana_test.go -- the quiet proof's mana ceiling against the
// widened payment model (the paid-cost census, engine/rules/paid_mana_census.go).
//
// The proof's ceiling (quietManaCeiling) sums every untapped source's printed
// production, whatever its activation cost. The census widening makes the
// walk offer more (a Treasure/Lotus Petal/PayLife source pays a cast; a
// pool-short activated ability is auto-paid). These rows assert the contract
// in both walk modes -- the ordinary option list and the PaymentActions
// extension a priority decision carries -- for every class the widening
// counts, and for the mana ADDERS the ceiling cannot see (a doubling
// replacement or a mana trigger adds units no printed production names).

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	qpmPetal = "Name:Quiet Petal\nManaCost:0\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ T Sac<1/CARDNAME> | Produced$ W | SpellDescription$ Add {W}.\nOracle:x\n"
	qpmPainRock = "Name:Quiet Pain Rock\nManaCost:1\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ T PayLife<1> | Produced$ W | SpellDescription$ Add {W}.\nOracle:x\n"
	qpmSage = "Name:Quiet Paid Sage\nManaCost:2\nTypes:Creature Wizard\nPT:1/1\n" +
		"A:AB$ Draw | Cost$ 1 | NumCards$ 1 | SpellDescription$ Draw a card.\nOracle:x\n"
	qpmSageWW = "Name:Quiet WW Sage\nManaCost:2\nTypes:Creature Wizard\nPT:1/1\n" +
		"A:AB$ Draw | Cost$ W W | NumCards$ 1 | SpellDescription$ Draw a card.\nOracle:x\n"
	// qpmWW is an instant needing two white: one untapped Plains (printed
	// ceiling 1) cannot pay it without an extra unit.
	qpmWW = "Name:Quiet WW Instant\nManaCost:W W\nTypes:Instant\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
	qpmW  = "Name:Quiet W Instant\nManaCost:W\nTypes:Instant\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
)

// qpmEngine builds quietBaseLandWith's table with the paid-cost census set. Seat 0
// holds an untapped Plains when plains is true, else an empty battlefield.
func qpmEngine(t *testing.T, flag, plains bool, extras []*cards.Card) *Engine {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	mountain := lookup(t, reg, "Mountain")
	basic := lookup(t, reg, "Plains")
	deck0 := append([]*cards.Card{basic}, extras...)
	for len(deck0) < 40 {
		deck0 = append(deck0, mountain)
	}
	deck1 := make([]*cards.Card, 40)
	for i := range deck1 {
		deck1[i] = mountain
	}
	e := New(Config{Seed: 42, Names: []string{"a", "b"}, Tokens: reg.Tokens, DisablePaidManaCensus: !flag,
		Decks: [][]*cards.Card{deck0, deck1}})
	if e.paidManaCensus != flag {
		t.Fatalf("precondition: paid-cost census not adopted: %v", e.paidManaCensus)
	}
	e.Advance()
	toMain1(t, e)
	for _, id := range append([]state.ObjID{}, e.G.Zone(state.ZHand, 0)...) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
	}
	if plains {
		pid := pullByName(t, e, 0, "Plains")
		e.emit(events.Event{Kind: events.MoveZone, Obj: pid, From: state.ZLibrary, To: state.ZBattlefield})
		if o := e.G.Obj(pid); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("precondition: Plains %d is not an untapped battlefield land", pid)
		}
	}
	return e
}

// qpmPayments asserts the PaymentActions extension of the pending priority
// decision (the autopay mode's plan offers) references only options of the
// decision, and that a quiet seat's decision offers no non-mana option.
func qpmPayments(t *testing.T, e *Engine, p state.PlayerID, quiet bool) {
	t.Helper()
	e.legalActions(p) // the walk the decision is posed from
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return // the fixture sits mid-window; the walk contract above covers it
	}
	for _, pa := range e.EnsurePaymentActions() {
		if pa.BaseOptionIndex == nil {
			continue
		}
		bi := *pa.BaseOptionIndex
		if bi < 0 || bi >= len(d.Options) {
			t.Fatalf("payment action names option %d of %d", bi, len(d.Options))
		}
		if quiet && d.Options[bi].Kind != "activate" {
			t.Fatalf("quiet seat but payment plan rides non-mana option %q", d.Options[bi].Label)
		}
	}
}

// qpmPotential is the autopay mode's walk: the legal-offer walk priced
// against PotentialMana (what Decision.PaymentActions plans from). The proof
// must dominate it too: quiet implies it offers nothing but mana, pass and
// concede, and the bound it prices against never exceeds the proof's ceiling
// (unless the ceiling is unbounded). It returns the bound and its options.
func qpmPotential(t *testing.T, e *Engine, p state.PlayerID, quiet bool) (state.Mana, []decision.Option) {
	t.Helper()
	bound, opts := e.potentialWalkOf(p, true)
	ceiling, unbounded, _ := e.quietManaCeiling(p)
	if quiet {
		for i := range opts {
			switch opts[i].Kind {
			case "activate", "pass", "concede":
			default:
				t.Fatalf("quiet proof contract violated by the potential walk: quiet but it offered %s %q (bound %v, ceiling %d)",
					opts[i].Kind, opts[i].Label, bound, ceiling)
			}
		}
		if !unbounded && int32(bound.Total()) > ceiling {
			t.Fatalf("potential bound %v (%d units) exceeds the proof's ceiling %d while quiet", bound, bound.Total(), ceiling)
		}
	}
	return bound, opts
}

func qpmOffers(opts []decision.Option, kind string) bool {
	for i := range opts {
		if opts[i].Kind == kind {
			return true
		}
	}
	return false
}

// qpmRow is one fixture: the battlefield cards (a "treasure" entry mints a
// Treasure token), an optional hand card, and the option kind the walk must
// offer with the census on ("" when the row only checks the contract).
type qpmRow struct {
	name   string
	plains bool
	bf     []string
	hand   string
	want   string // kind the census-on walk must offer, "" for none required
}

// qpmCard resolves a row's card: "corpus:<Name>" is the registry's card,
// anything else an inline script.
func qpmCard(t *testing.T, reg *cards.Registry, src string) *cards.Card {
	t.Helper()
	if name, ok := strings.CutPrefix(src, "corpus:"); ok {
		return lookup(t, reg, name)
	}
	return card(t, src)
}

// qpmRun builds one row's board under flag and returns the engine, the walk's
// options (quietContract has already enforced quiet => no non-mana option)
// and the proof's verdict.
func qpmRun(t *testing.T, r qpmRow, flag bool) (*Engine, []decision.Option, bool) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var extras, onBoard []*cards.Card
	treasure := false
	for _, src := range r.bf {
		if src == "treasure" {
			treasure = true
			continue
		}
		c := qpmCard(t, reg, src)
		extras = append(extras, c)
		onBoard = append(onBoard, c)
	}
	var hand *cards.Card
	if r.hand != "" {
		hand = qpmCard(t, reg, r.hand)
		extras = append(extras, hand)
	}
	e := qpmEngine(t, flag, r.plains, extras)
	for _, c := range onBoard {
		id := addZone(t, e, 0, c, state.ZBattlefield)
		if o := e.G.Obj(id); o.Tapped || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: %s %d tapped=%v zone=%v", c.Faces[0].Name, id, o.Tapped, o.Zone)
		}
	}
	if treasure {
		e.G.Tokens = reg.Tokens
		e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "c_a_treasure_sac"})
		z := e.G.Zone(state.ZBattlefield, 0)
		if o := e.G.Obj(z[len(z)-1]); !o.IsToken || o.Face().Name != "Treasure Token" {
			t.Fatalf("precondition: minted %q, want a Treasure Token", o.Face().Name)
		}
	}
	if hand != nil {
		addHand(t, e, 0, hand)
	}
	opts, quiet, _ := quietContract(t, e, 0)
	qpmPayments(t, e, 0, quiet)
	qpmPotential(t, e, 0, quiet)
	return e, opts, quiet
}

// TestQuietPaidManaClasses: for every cost class the census widening counts
// (self-sacrifice Treasure/Lotus Petal, fixed PayLife, a pool-short paid
// ability) the contract holds in both modes, and where the census-on walk
// offers the cast/ability the proof is not quiet.
func TestQuietPaidManaClasses(t *testing.T) {
	rows := []qpmRow{
		{name: "treasure + cast", bf: []string{"treasure"}, hand: qpmW},
		{name: "petal + cast", bf: []string{qpmPetal}, hand: qpmW},
		{name: "pain rock + cast", bf: []string{qpmPainRock}, hand: qpmW},
		{name: "plains + cast (control)", plains: true, hand: qpmW},
		{name: "plains + paid ability", plains: true, bf: []string{qpmSage}, want: "ability"},
		{name: "treasure + paid ability", bf: []string{qpmSage, "treasure"}, want: "ability"},
		{name: "petal + paid ability", bf: []string{qpmSage, qpmPetal}, want: "ability"},
		{name: "pain rock + paid ability", bf: []string{qpmSage, qpmPainRock}, want: "ability"},
		{name: "black lotus + bolt", bf: []string{"corpus:Black Lotus"}, hand: "corpus:Lightning Bolt"},
		{name: "lion's eye diamond + bolt", bf: []string{"corpus:Lion's Eye Diamond"}, hand: "corpus:Lightning Bolt"},
		{name: "plains + petal + cast ww", plains: true, bf: []string{qpmPetal}, hand: qpmWW},
		{name: "plains + pain rock + ww ability", plains: true, bf: []string{qpmPainRock, qpmSageWW}, want: "ability"},
	}
	for _, r := range rows {
		for _, flag := range []bool{false, true} {
			t.Run(r.name+map[bool]string{false: "/off", true: "/census"}[flag], func(t *testing.T) {
				e, opts, quiet := qpmRun(t, r, flag)
				offered := qpmOffers(opts, "cast") || qpmOffers(opts, "ability")
				if flag && r.want != "" && !qpmOffers(opts, r.want) {
					t.Fatalf("precondition: census-on walk did not offer %q: %v", r.want, optKinds(opts))
				}
				if offered && quiet {
					t.Fatalf("quiet while the walk offered %v", optKinds(opts))
				}
				bound, popts := qpmPotential(t, e, 0, quiet)
				pOffered := qpmOffers(popts, "cast") || qpmOffers(popts, "ability")
				if pOffered && quiet {
					t.Fatalf("quiet while the potential walk offered %v", optKinds(popts))
				}
				t.Logf("walk offered=%v | potential bound=%v offered=%v | quiet=%v blocker=%s opts=%v", offered, bound, pOffered, quiet, quietBlockerNames[e.quietBlocker(0)], optKinds(popts))
			})
		}
	}
}

// TestQuietPaidManaAdders: a mana ADDER (a doubling replacement or a
// mana-adding trigger) lets one Plains pay {W}{W}, a unit the proof's
// printed-production ceiling (1) does not count. A WW cast and a WW ability
// are probed in both census modes; the contract must hold whatever the walk
// does with the adder.
func TestQuietPaidManaAdders(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Mana Reflection", "Mana Flare", "Heartbeat of Spring", "Zhur-Taa Ancient", "Caged Sun", "Gauntlet of Power"} {
		adder, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("adder %q is not in the corpus", name)
		}
		for _, shape := range []string{"cast", "ability"} {
			for _, flag := range []bool{false, true} {
				t.Run(name+"/"+shape+map[bool]string{false: "/off", true: "/census"}[flag], func(t *testing.T) {
					var spell, sage *cards.Card
					extras := []*cards.Card{adder}
					if shape == "cast" {
						spell = card(t, qpmWW)
						extras = append(extras, spell)
					} else {
						sage = card(t, qpmSageWW)
						extras = append(extras, sage)
					}
					e := qpmEngine(t, flag, true, extras)
					id := addZone(t, e, 0, adder, state.ZBattlefield)
					e.G.Obj(id).ChosenColor = "White"
					if spell != nil {
						addHand(t, e, 0, spell)
					} else {
						addZone(t, e, 0, sage, state.ZBattlefield)
					}
					opts, quiet, _ := quietContract(t, e, 0)
					ceiling, unbounded, _ := e.quietManaCeiling(0)
					if ceiling != 1 || unbounded {
						t.Fatalf("precondition: ceiling=%d unbounded=%v, want the Plains alone", ceiling, unbounded)
					}
					qpmPayments(t, e, 0, quiet)
					bound, popts := qpmPotential(t, e, 0, quiet)
					t.Logf("%s %s flag=%v: walk cast=%v ability=%v | potential bound=%v cast=%v ability=%v | ceiling=%d quiet=%v blocker=%s",
						name, shape, flag, qpmOffers(opts, "cast"), qpmOffers(opts, "ability"), bound,
						qpmOffers(popts, "cast"), qpmOffers(popts, "ability"), ceiling, quiet,
						quietBlockerNames[e.quietBlocker(0)])
				})
			}
		}
	}
}

const qpmSage2 = "Name:Quiet Two Sage\nManaCost:2\nTypes:Creature Wizard\nPT:1/1\n" +
	"A:AB$ Draw | Cost$ 2 | NumCards$ 1 | SpellDescription$ Draw a card.\nOracle:x\n"

// TestQuietPaidManaGrantedAndExtraAdders probes the wider adder family the
// six cards stand for: a granted tap-for-mana ability (Cryptolith
// Rite, Citanul Hierophants, Song of Freyalise), a doubling or extra-mana
// replacement/trigger (Mirari's Wake, Nyxbloom Ancient, Vorinclex, Dictate of
// Karametra, Crypt Ghast, Overgrowth, Wild Growth) and a land-type grant
// (Urborg, Prismatic Omen). One Plains plus two abilities ({W}{W}, {2}); the
// proof must stay sound against both the walk and the potential walk, in both
// census modes. A card absent from the corpus is logged, not failed.
func TestQuietPaidManaGrantedAndExtraAdders(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names := []string{
		"Cryptolith Rite", "Citanul Hierophants", "Song of Freyalise", "Mirari's Wake",
		"Nyxbloom Ancient", "Vorinclex, Voice of Hunger", "Dictate of Karametra", "Crypt Ghast",
		"Overgrowth", "Wild Growth", "Urborg, Tomb of Yawgmoth", "Prismatic Omen",
		"Dryad of the Ilysian Grove", "Mana Echoes", "Extraplanar Lens", "Wolfwillow Haven",
	}
	for _, name := range names {
		adder, ok := reg.Lookup(name)
		if !ok {
			t.Logf("%s: not in corpus", name)
			continue
		}
		for _, flag := range []bool{false, true} {
			t.Run(name+map[bool]string{false: "/off", true: "/census"}[flag], func(t *testing.T) {
				ww, two := card(t, qpmSageWW), card(t, qpmSage2)
				e := qpmEngine(t, flag, true, []*cards.Card{adder, ww, two})
				id := addZone(t, e, 0, adder, state.ZBattlefield)
				o := e.G.Obj(id)
				o.ChosenColor = "White"
				o.SummonSick = false
				addZone(t, e, 0, ww, state.ZBattlefield)
				addZone(t, e, 0, two, state.ZBattlefield)
				opts, quiet, _ := quietContract(t, e, 0)
				ceiling, unbounded, _ := e.quietManaCeiling(0)
				bound, popts := qpmPotential(t, e, 0, quiet)
				qpmPayments(t, e, 0, quiet)
				t.Logf("%s flag=%v: walk ability=%v | potential bound=%v ability=%v | ceiling=%d unbounded=%v quiet=%v blocker=%s",
					name, flag, qpmOffers(opts, "ability"), bound, qpmOffers(popts, "ability"),
					ceiling, unbounded, quiet, quietBlockerNames[e.quietBlocker(0)])
			})
		}
	}
}
