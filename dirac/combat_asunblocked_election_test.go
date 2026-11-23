package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// stat:AssignCombatDamageAsUnblocked's controller election (CR 510.1c, gorge
// askNextCombatAsk's asunblocked KChoose before the division queue; kernel
// combat_asgngrant.go's nextElection): the assigning seat is offered the
// election for a BLOCKED attacker carrying the printed static, an accepted
// election routes the attacker's whole power to the defending player (the
// blocker takes nothing and still hits back), a declined one assigns
// ordinarily, and an unblocked carrier is asked nothing. The damage split is
// played through gorge and the kernel over the same shapes and asserted
// equal (both the corpus carrier Thorn Elemental over a real pool table, and
// gorge's fixture driving the same printed static through its own engine).

// asUnblkPool is the tail pool holding the census carrier Thorn Elemental
// (StatAssignCombatDamageAsUnblocked); Trained Armodon is its vanilla 3/3
// blocker. A corpus without this binary's synthetic pools skips.
func asUnblkPool(t *testing.T) (*kop.Table, map[string]kop.CardID) {
	t.Helper()
	const pool = "tail-31"
	set := cardsettest.Synth(t, pool)
	deck := []cardset.Entry{{Name: "Thorn Elemental", Count: 1}, {Name: "Trained Armodon", Count: 1}}
	for _, e := range deck {
		i, ok := set.Lookup(e.Name)
		if !ok || !set.Supported()[i] {
			t.Skipf("%s is not supported in the %s pool", e.Name, pool)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]kop.CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return tbl, id
}

func (sc *scen) permPair(t *testing.T, id map[string]kop.CardID) (Obj, Obj) {
	t.Helper()
	te := sc.perm(id["Thorn Elemental"], 0)
	chap := sc.perm(id["Trained Armodon"], 1)
	return te, chap
}

// permDamage is a permanent's marked damage, 0 once it left the battlefield
// (gorge reads a left permanent's Damage as 0 too, so the splits compare).
func permDamage(sc *scen, o Obj) int {
	if !sc.onBF(o) {
		return 0
	}
	return int(sc.s.perms[sc.slot(o)].damage)
}

// asUnblkCombat declares Thorn Elemental's attack and, when block, the
// Armodon's block, and stops at the damage step with the election (or the
// pass when block is false) pending.
func (sc *scen) asUnblkCombat(t *testing.T, te, chap Obj, block bool) {
	t.Helper()
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(te), scAny)
	sc.do(t, AkDone, scAny, scAny)
	sc.passUntil(t, kop.StepBlockers)
	// the CR 509.2 window: seat 1 may still hold priority in the
	// declare-blockers step; pass it to open the blockers declaration
	for i := 0; i < 10 && sc.s.ctl != ctlBlockers; i++ {
		d := sc.s.Step()
		if d.Kind != DkPriority {
			break
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	if block {
		sc.do(t, AkBlock, TCode(chap), TCode(te))
		sc.do(t, AkDone, scAny, scAny)
		sc.passUntil(t, kop.StepDamage)
		return
	}
	// the defender declares nothing; walk to the damage step declining every
	// priority round and refusing any election
	for i := 0; i < 50; i++ {
		if sc.s.step == kop.StepDamage || sc.s.step == kop.StepMain2 {
			return
		}
		d := sc.s.Step()
		if d.Kind == DkChoose && d.Sub == ChAsUnblocked {
			t.Fatalf("unblocked Thorn Elemental was offered the as-unblocked election")
		}
		if d.Kind != DkPriority && d.Kind != DkBlockers {
			t.Fatalf("unblocked: unexpected decision kind %d sub %d", d.Kind, d.Sub)
		}
		done := MakeAction(AkPass, 0, 0, 0)
		if d.Kind == DkBlockers {
			done = MakeAction(AkDone, 0, 0, 0)
		}
		sc.s.Apply(done)
	}
	t.Fatal("never reached the damage step")
}

// electionOpt is the election option of mode m.
func electionOpt(t *testing.T, d Decision, m uint8) Action {
	t.Helper()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Mode() == m {
			if a.Kind() != AkChoose || a.Source() != d.Options[0].Source() {
				t.Fatalf("election option %d is kind %d src %d, want an AkChoose over the attacker", i, a.Kind(), a.Source())
			}
			return a
		}
	}
	t.Fatalf("no election option of mode %d in %v", m, d.Options[:d.N])
	return 0
}

func TestAsUnblockedElection(t *testing.T) {
	tbl, id := asUnblkPool(t)
	sc := newScenTableLib(t, tbl, id["Thorn Elemental"], Config{NoElide: true})
	te, chap := sc.permPair(t, id)
	if int(v.Plains) >= len(tbl.Cards) {
		t.Fatal("the pool table is smaller than the scenario library expects")
	}

	t.Run("accepted routes the whole power to the defender", func(t *testing.T) {
		sc.asUnblkCombat(t, te, chap, true)
		d := sc.expect(t, DkChoose, ChAsUnblocked)
		if d.Player != 0 || d.N != 2 || d.Options[0].Source() != TCode(te) {
			t.Fatalf("election for player %d with %d options over src %d (want player 0, 2 options over %d)",
				d.Player, d.N, d.Options[0].Source(), te)
		}
		sc.s.Apply(electionOpt(t, d, 1))
		sc.passUntil(t, kop.StepMain2)
		sc.s.Step()
		if got := int(sc.s.pl[1].life); got != 13 {
			t.Fatalf("defender life %d, want 13 (20 - Thorn Elemental's 7)", got)
		}
		if !sc.onBF(chap) || int(sc.s.perms[sc.slot(chap)].damage) != 0 {
			t.Fatalf("blocker alive=%v damage=%d; the election rerouted the attacker's damage, want alive and untouched",
				sc.onBF(chap), sc.s.perms[sc.slot(chap)].damage)
		}
		if got := int(sc.s.perms[sc.slot(te)].damage); got != 3 {
			t.Fatalf("attacker damage %d, want 3 (the blocker still hit back)", got)
		}
	})

	t.Run("declined assigns ordinarily", func(t *testing.T) {
		sc2 := newScenTableLib(t, tbl, id["Thorn Elemental"], Config{NoElide: true})
		te2, chap2 := sc2.permPair(t, id)
		sc2.asUnblkCombat(t, te2, chap2, true)
		d := sc2.expect(t, DkChoose, ChAsUnblocked)
		sc2.s.Apply(electionOpt(t, d, 0))
		sc2.passUntil(t, kop.StepMain2)
		sc2.s.Step()
		if got := int(sc2.s.pl[1].life); got != 20 {
			t.Fatalf("defender life %d, want 20 (a declined election is not a Trample)", got)
		}
		if sc2.onBF(chap2) {
			t.Fatal("the blocker survived the attacker's 7")
		}
		if got := int(sc2.s.perms[sc.slot(te2)].damage); got != 3 {
			t.Fatalf("attacker damage %d, want 3 (the blocker hit back first)", got)
		}
	})

	t.Run("unblocked asks nothing", func(t *testing.T) {
		sc3 := newScenTableLib(t, tbl, id["Thorn Elemental"], Config{NoElide: true})
		te3, chap3 := sc3.permPair(t, id)
		sc3.asUnblkCombat(t, te3, chap3, false)
		sc3.passUntil(t, kop.StepMain2)
		sc3.s.Step()
		if got := int(sc3.s.pl[1].life); got != 13 {
			t.Fatalf("defender life %d, want 13", got)
		}
		if !sc3.onBF(chap3) || int(sc3.s.perms[sc.slot(chap3)].damage) != 0 {
			t.Fatalf("blocker alive=%v damage=%d, want untouched", sc3.onBF(chap3), sc3.s.perms[sc.slot(chap3)].damage)
		}
		if got := int(sc3.s.perms[sc.slot(te3)].damage); got != 0 {
			t.Fatalf("attacker damage %d, want 0", got)
		}
	})
}

// TestAsUnblockedElectionGorgeAgrees drives gorge over the same three shapes
// with Thorn Elemental's printed static and asserts the damage split equals
// the kernel's (gorge's own seat lives in engine/rules'
// combat_as_unblocked_test.go; this is the cross-engine split check).
func TestAsUnblockedElectionGorgeAgrees(t *testing.T) {
	tbl, id := asUnblkPool(t)
	thorn := cmdTestCard(t, "Name:Thorn Elemental\nManaCost:5 G G\nTypes:Creature Elemental\nPT:7/7\n"+
		"S:Mode$ AssignCombatDamageAsUnblocked | ValidCard$ Card.Self | Optional$ True | Description$ You may have CARDNAME assign its combat damage as though it weren't blocked.\n"+
		"Oracle:You may have Thorn Elemental assign its combat damage as though it weren't blocked.\n")
	armodon := cmdTestCard(t, "Name:Trained Armodon\nManaCost:1 G G\nTypes:Creature Elephant\nPT:3/3\nOracle:x\n")

	// split is the kernel's split for the named variant, read off the same
	// assertions the kernel test makes.
	type split struct {
		defLife        int
		blockerAlive   bool
		blockerDamage  int
		attackerAlive  bool
		attackerDamage int
	}
	kernelSplits := func(block bool, accept bool) split {
		sc := newScenTableLib(t, tbl, id["Thorn Elemental"], Config{NoElide: true})
		te, chap := sc.permPair(t, id)
		sc.asUnblkCombat(t, te, chap, block)
		if block {
			d := sc.expect(t, DkChoose, ChAsUnblocked)
			m := uint8(0)
			if accept {
				m = 1
			}
			sc.s.Apply(electionOpt(t, d, m))
		}
		sc.passUntil(t, kop.StepMain2)
		sc.s.Step()
		return split{
			defLife:        int(sc.s.pl[1].life),
			blockerAlive:   sc.onBF(chap),
			blockerDamage:  permDamage(sc, chap),
			attackerAlive:  sc.onBF(te),
			attackerDamage: permDamage(sc, te),
		}
	}
	gorgeSplit := func(t *testing.T, block, accept bool) split {
		t.Helper()
		e := gorgeElectionGame(t, thorn, armodon, block, accept)
		te := gorgeElectionGameAttacker
		chap := gorgeElectionGameBlocker
		return split{
			defLife:        int(e.G.Players[1].Life),
			blockerAlive:   e.G.Obj(chap).Zone == state.ZBattlefield,
			blockerDamage:  int(e.G.Obj(chap).Damage),
			attackerAlive:  e.G.Obj(te).Zone == state.ZBattlefield,
			attackerDamage: int(e.G.Obj(te).Damage),
		}
	}
	for _, tc := range []struct {
		name   string
		block  bool
		accept bool
	}{{"accepted", true, true}, {"declined", true, false}, {"unblocked", false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			g := gorgeSplit(t, tc.block, tc.accept)
			k := kernelSplits(tc.block, tc.accept)
			if g != k {
				t.Fatalf("gorge %+v, kernel %+v: the damage split diverged", g, k)
			}
		})
	}
}

// The gorge fixture's attacker and blocker ids (set by gorgeElectionGame).
var (
	gorgeElectionGameAttacker state.ObjID
	gorgeElectionGameBlocker  state.ObjID
)

// gorgeElectionGame plays a gorge game with the Thorn Elemental under seat 0
// and the Armodon under seat 1, declares the attack (and the block when
// block), answers the as-unblocked election with accept, and returns the
// engine once the damage has been dealt (the game reached its second main
// phase). Every other decision is passed; an unexpected KChoose fails.
func gorgeElectionGame(t *testing.T, thorn, armodon *cards.Card, block, accept bool) *rules.Engine {
	t.Helper()
	var decks [2][]*cards.Card
	for p := range decks {
		for i := 0; i < 40; i++ {
			decks[p] = append(decks[p], thorn) // the library card is irrelevant: nothing is cast
		}
	}
	var e *rules.Engine
	for seed := uint64(1); ; seed++ {
		e = rules.New(rules.Config{Seed: seed, Names: []string{"a", "b"},
			Decks: [][]*cards.Card{decks[0], decks[1]}})
		if e.G.Active == 0 {
			break
		}
		if seed > 20 {
			t.Fatal("no seed up to 20 starts seat 0")
		}
	}
	place := func(p state.PlayerID, c *cards.Card) state.ObjID {
		o := e.G.AddObject(c, p)
		o.Zone = state.ZBattlefield
		o.SummonSick = false
		e.G.Clock++
		o.Timestamp = e.G.Clock
		e.G.SetZone(state.ZBattlefield, p, append(e.G.Zone(state.ZBattlefield, p), o.ID))
		return o.ID
	}
	te := place(0, thorn)
	chap := place(1, armodon)
	gorgeElectionGameAttacker, gorgeElectionGameBlocker = te, chap

	for i := 0; i < 400; i++ {
		if e.G.Step == state.StepEndCombat {
			return e // the combat damage has been dealt
		}
		e.Advance()
		d := e.Pending()
		if d == nil {
			if e.G.Over {
				return e
			}
			continue
		}
		var ch []int
		switch d.Kind {
		case decision.KPriority:
			for _, o := range d.Options {
				if o.Kind == "pass" {
					ch = []int{o.Index}
					break
				}
			}
		case decision.KMulligan:
			for _, o := range d.Options {
				if o.Kind == "keep" {
					ch = []int{o.Index}
					break
				}
			}
		case decision.KAttackers:
			for _, o := range d.Options {
				if o.Kind == "attacker" && o.Obj == te && int(o.Player) == 1 {
					ch = []int{o.Index}
					break
				}
			}
		case decision.KBlockers:
			if block {
				for _, o := range d.Options {
					if o.Kind == "block" && o.Obj == chap && o.Attacker == te {
						ch = []int{o.Index}
						break
					}
				}
			} else {
				ch = []int{} // the defender declares no blocks (CR 509.1; Min 0)
			}
		case decision.KChoose:
			if d.Source != te || len(d.Options) != 2 || d.Options[0].Kind != "asunblocked" {
				t.Fatalf("unexpected gorge choose decision over %d: %+v", d.Source, d.Options)
			}
			idx := 0
			if accept {
				idx = 1
			}
			ch = []int{idx}
		default:
			t.Fatalf("unexpected gorge decision %v", d.Kind)
		}
		if ch == nil {
			t.Fatalf("no option to answer gorge's %v decision: %+v", d.Kind, d.Options)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ch}); err != nil {
			t.Fatalf("gorge submit %v: %v", d.Kind, err)
		}
	}
	t.Fatal("gorge never reached its second main phase")
	return e
}
