package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// Mutate (CR 702.140) cast-mode tests on synthetic tables (kgen still
// refuses Mutate cards, so no corpus card reaches the kernel's mutate
// arms). Model: gorge engine/rules/mutate.go (the mutated cast mode, the
// synthesized Creature.nonHuman+YouOwn target, mutatePlaceAsk asked once
// during the cast, mutateVictim's CR 608.2b recheck and resolveMutate) and
// docs/design/mutate.md.

// mutateTable appends the mutate fixtures to a copy of the vanilla table:
// "Test Mutator" (a 2/2 {3}{G} creature with mutate {G}), "Test Bird" (a
// non-Human 2/2 creature), "Test Soldier" (a Human 2/2 creature: never a
// legal mutate target), and "Test Bolt" (an instant that destroys a target
// permanent, for the CR 702.140b un-mutate recheck). It returns the table
// and the four card ids.
func mutateTable() (*kop.Table, CardID, CardID, CardID, CardID) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Subtypes = append([]string(nil), v.Table.Subtypes...)
	human := -1
	for i, n := range tbl.Subtypes {
		if n == "Human" {
			human = i
			break
		}
	}
	if human < 0 {
		human = len(tbl.Subtypes)
		tbl.Subtypes = append(tbl.Subtypes, "Human")
	}
	tbl.Conds = []kop.Cond{{}} // a non-nil Conds enables prepareGen for the generated arms
	// the synthesized mutate target's filter: Creature, not Human, owned by
	// the caster (gorge mutateTargetSA's ValidTgts Creature.nonHuman+YouOwn)
	tgt := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Owner: kop.CtrlYou,
		NotSub: uint8(human) + 1})
	mutator := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Mutator", Types: kop.TCreature, Power: 2, Toughness: 2,
		Cost:      kop.ManaCost{Generic: 3, Pips: [kop.NumManaKinds]uint8{kop.G: 1}},
		HasMutate: true, MutateCost: kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.G: 1}},
		MutateTgt: kop.TargetSpec{Kind: kop.TgtObject, Filter: tgt},
	})
	bird := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bird", Types: kop.TCreature, Power: 1, Toughness: 3})
	soldier := CardID(len(tbl.Cards))
	var humanSub kop.SubMask
	humanSub = humanSub.With(human)
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Soldier", Types: kop.TCreature,
		Power: 2, Toughness: 2, Subtypes: humanSub})
	bolt := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bolt", Types: kop.TInstant,
		Cost: kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.R: 1}},
		Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: 0}}, NTargets: 1,
			Prog: []kop.Instr{{Op: kop.OpDestroy, Obj: kop.RefTarget0}}}}})
	return &tbl, mutator, bird, soldier, bolt
}

// mutateOffered reports whether d holds a mutate cast of hand card o.
func mutateOffered(d Decision, o Obj) bool {
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkMutate && Obj(a.Source()) == o {
			return true
		}
	}
	return false
}

// mutateCast applies o's mutate cast and answers the placement ask with top,
// then the synthesized target with tgt: gorge's continueCast poses
// mutatePlaceAsk right after the cast-mode ask, before the target ask.
func mutateCast(t *testing.T, sc *scen, o, tgt Obj, top bool) {
	t.Helper()
	sc.do(t, AkMutate, TCode(o), scAny)
	mutatePlace(t, sc, top)
	sc.do(t, AkTarget, scAny, TCode(tgt))
}

// mutatePlace answers the pending placement ask with top (On top) or under.
func mutatePlace(t *testing.T, sc *scen, top bool) {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkChoose {
		t.Fatalf("placement ask is kind %d, want DkChoose", d.Kind)
	}
	want := uint8(0)
	if top {
		want = 1
	}
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkChoose && a.Mode() == want {
			sc.s.Apply(a)
			return
		}
	}
	t.Fatalf("placement ask has no AkChoose mode %d: %v", want, d.Options[:d.N])
}

// resolveStack passes priority and answers residual asks with their first
// option until the stack and the trigger queue are empty.
func resolveStack(t *testing.T, sc *scen) {
	t.Helper()
	for i := 0; i < 64; i++ {
		if sc.s.sp == 0 && sc.s.npend == 0 {
			return
		}
		d := sc.s.Step()
		if d.N == 0 {
			t.Fatalf("decision kind %d with no options while resolving", d.Kind)
		}
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		sc.s.Apply(d.Options[0])
	}
	t.Fatalf("stack/queue did not drain: sp %d npend %d", sc.s.sp, sc.s.npend)
}

func TestMutateCastMerge(t *testing.T) {
	tbl, mutator, bird, soldier, bolt := mutateTable()

	t.Run("offer gate", func(t *testing.T) {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.lands(0, v.Forest, 1)
		mut := sc.hand(mutator, 0)
		// no legal target: no mutate option (gorge gates on
		// castTargetsAvailable, legal_walk_alt.go)
		if d := sc.s.Step(); mutateOffered(d, mut) {
			t.Fatalf("mutate cast offered with no legal target")
		}
		// a Human is not a legal mutate target (CR 702.140a)
		soldierObj := sc.perm(soldier, 0)
		if d := sc.s.Step(); mutateOffered(d, mut) {
			t.Fatalf("mutate cast offered targeting a Human")
		}
		// a plain cast is offered when the pool pays it, never at the
		// mutate cost: the mutate cast is its own option, priced alone
		// (the {3}{G} printed cost is unaffordable on one Forest)
		sc.perm(bird, 0)
		d := sc.s.Step()
		if !mutateOffered(d, mut) {
			t.Fatalf("mutate cast not offered beside a legal target")
		}
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == mut {
				t.Fatalf("plain mutate-card cast offered on a {G} that cannot pay {3}{G}")
			}
		}
		// the synthesized target ask holds only the bird: not the Human
		sc.perm(bird, 0)
		sc.do(t, AkMutate, TCode(mut), scAny)
		mutatePlace(t, sc, true)
		d = sc.s.Step()
		if d.Kind != DkTarget {
			t.Fatalf("target ask is kind %d, want DkTarget", d.Kind)
		}
		seen := 0
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkTarget {
				if Obj(a.Target()) == soldierObj {
					t.Fatalf("the Human is a mutate target option")
				}
				seen++
			}
		}
		if seen != 2 {
			t.Fatalf("target options %d, want the two birds", seen)
		}
	})

	t.Run("top", func(t *testing.T) {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.lands(0, v.Forest, 1)
		b := sc.perm(bird, 0)
		bsl := sc.slot(b)
		sc.s.perms[bsl].addCtrN(sc.s, int(kop.CtrP1P1), 2)
		auraO := sc.perm(mutator, 0) // any permanent stands in as the attached Aura
		sc.s.perms[sc.slot(auraO)].attached = bsl
		mut := sc.hand(mutator, 0)
		mutateCast(t, sc, mut, b, true)
		resolveStack(t, sc)
		// same object: the permanent at b's slot is still b, counters and
		// the attachment survive (CR 702.140e)
		if !sc.onBF(b) || sc.slot(b) != bsl {
			t.Fatalf("the mutated permanent moved")
		}
		if n := sc.s.perms[bsl].ctrLow(int(kop.CtrP1P1)); n != 2 {
			t.Fatalf("counters after the merge = %d, want 2 (same object)", n)
		}
		if sc.s.perms[sc.slot(auraO)].attached != bsl {
			t.Fatalf("the attachment left its bearer")
		}
		if c := sc.s.cardAt(bsl); c != mutator {
			t.Fatalf("top card is %d, want the mutating card %d", c, mutator)
		}
		rec := sc.s.mergeRecOf(b)
		if rec == nil {
			t.Fatalf("no merge record for the merged permanent")
		}
		if rec.n != 1 || rec.tm != 1 {
			t.Fatalf("pile n=%d tm=%d, want 1/1", rec.n, rec.tm)
		}
		if c := sc.s.objs[Obj(rec.und[0]-1)].card; c != bird {
			t.Fatalf("under card is %d, want the demoted top %d", c, bird)
		}
		// the spell's object parks: on no zone list (gorge's fold parks it)
		if z := sc.s.objs[mut].zone; z != kop.ZoneNone {
			t.Fatalf("spell object zone %d, want ZoneNone", z)
		}
	})

	t.Run("under", func(t *testing.T) {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.lands(0, v.Forest, 1)
		b := sc.perm(bird, 0)
		mut := sc.hand(mutator, 0)
		mutateCast(t, sc, mut, b, false)
		resolveStack(t, sc)
		// under: the top card is unchanged (CR 702.140b)
		if c := sc.s.cardAt(sc.slot(b)); c != bird {
			t.Fatalf("top card is %d, want the unchanged %d", c, bird)
		}
		rec := sc.s.mergeRecOf(b)
		if rec == nil {
			t.Fatalf("no merge record for the merged permanent")
		}
		if rec.n != 1 || rec.tm != 1 {
			t.Fatalf("pile n=%d tm=%d, want 1/1", rec.n, rec.tm)
		}
		if c := sc.s.objs[Obj(rec.und[0]-1)].card; c != mutator {
			t.Fatalf("under card is %d, want the mutating card %d", c, mutator)
		}
		if z := sc.s.objs[mut].zone; z != kop.ZoneNone {
			t.Fatalf("spell object zone %d, want ZoneNone", z)
		}
	})

	t.Run("unmutate", func(t *testing.T) {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.lands(0, v.Forest, 1)
		sc.lands(0, v.Mountain, 1)
		b := sc.perm(bird, 0)
		mut := sc.hand(mutator, 0)
		mutateCast(t, sc, mut, b, true)
		// the bird leaves before the mutate spell resolves (CR 702.140b,
		// 608.2b: the spell ceases to be mutating and enters as a creature):
		// the caster keeps priority after the cast and answers the bird
		// before either spell resolves
		bl := sc.hand(bolt, 0)
		sc.do(t, AkCast, TCode(bl), scAny)
		sc.do(t, AkTarget, scAny, TCode(b))
		resolveStack(t, sc)
		if sc.onBF(b) {
			t.Fatalf("the bird survived the bolt")
		}
		if !sc.onBF(mut) {
			t.Fatalf("the un-mutated spell did not enter as a creature")
		}
		if c := sc.s.cardAt(sc.slot(mut)); c != mutator {
			t.Fatalf("un-mutated permanent card is %d, want %d", c, mutator)
		}
		if rec := sc.s.mergeRecOf(mut); rec != nil {
			t.Fatalf("an un-mutated entry left a merge record")
		}
	})
}

func TestMutateTrigFires(t *testing.T) {
	tbl, mutator, bird, _, _ := mutateTable()
	// TimesMutated (Count$TimesMutated): the merge record's tm, read as an
	// expression amount (gorge value_mutated.go; CR 702.140f)
	tbl.Exprs = append([]kop.Expr(nil), tbl.Exprs...)
	tbl.Exprs = append(tbl.Exprs, kop.Expr{Kind: kop.ExTimesMutated})
	tmIx := int16(len(tbl.Exprs) - 1)
	// the mutating card carries "whenever this creature mutates, put N
	// time counters on it" with N = its TimesMutated (Kologran's shape)
	tbl.Cards[mutator].Abilities = []kop.Ability{{Kind: kop.AbTriggered,
		Trig: kop.Trigger{Event: kop.TrigMutates, Flags: kop.TfSelf},
		Prog: []kop.Instr{{Op: kop.OpPutCounter, Obj: kop.RefTriggerObj,
			A: kop.ValueRef{K: kop.VExpr, N: tmIx}, C: int16(kop.CtrQuest)}}}}
	// the lord shape: "whenever a creature you control mutates, put a +1/+1
	// counter on this permanent" on a separate permanent (gorge's Essence
	// Symbiote's shape; a draw would lose on the emptied library)
	creatureF := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	witness := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Witness", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigMutates, Filter: creatureF},
			Prog: []kop.Instr{{Op: kop.OpPutCounter, Obj: kop.RefSelf,
				A: kop.Const(1), C: int16(kop.CtrP1P1)}}}}})

	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.lands(0, v.Forest, 3) // one untapped {G} per mutate cast in the one main phase
	w := sc.perm(witness, 0)
	b1 := sc.perm(bird, 0)
	b2 := sc.perm(bird, 0)
	m1 := sc.hand(mutator, 0)
	m2 := sc.hand(mutator, 0)
	m3 := sc.hand(mutator, 0)
	// three merges: two onto b1, one onto b2 (top each time); the queue is
	// drained between casts so each merge's triggers run before the next
	mutateCast(t, sc, m1, b1, true)
	resolveStack(t, sc)
	mutateCast(t, sc, m2, b1, true)
	resolveStack(t, sc)
	mutateCast(t, sc, m3, b2, true)
	resolveStack(t, sc)
	// once per merge: three counters on the witness (two piles, three merges)
	if n := sc.s.perms[sc.slot(w)].ctrLow(int(kop.CtrP1P1)); n != 3 {
		t.Fatalf("witness counters %d, want one per merge (3)", n)
	}
	// TimesMutated reads the record's tm at fire time (the fold lands before
	// triggers run); the pile has every component's triggers (CR 702.140d),
	// so b1's second merge fires m2's trigger and the demoted m1's: it got
	// 1 + (2+2) counters, b2's 1
	if n := sc.s.perms[sc.slot(b1)].ctrLow(int(kop.CtrQuest)); n != 5 {
		t.Fatalf("b1 pile time counters %d, want 1+2+2=5 (TimesMutated)", n)
	}
	if n := sc.s.perms[sc.slot(b2)].ctrLow(int(kop.CtrQuest)); n != 1 {
		t.Fatalf("b2 pile time counters %d, want 1", n)
	}
}
