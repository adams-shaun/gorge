package hbot

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestHBotHoldRule is gorge's TimingRules.HoldInstant as (*board).instantHeld:
// a table over the rule's windows, the lethal exception, counters and combat
// tricks.
func TestHBotHoldRule(t *testing.T) {
	base := func() (*board, *CardFacts, *cardF) {
		b := &board{me: 0, myTur: true, isMain: true, step: kop.StepMain1, life: [dirac.MaxSeats]int32{20, 20}, n: 2}
		cf := &CardFacts{}
		c := &cardF{obj: 10, f: cf, castable: true, instant: true, cmc: 1, own: true}
		return b, cf, c
	}
	type tc struct {
		name string
		set  func(b *board, cf *CardFacts, c *cardF)
		held bool
	}
	step := func(b *board, s kop.Step) { b.step = s }
	opp := func(b *board) { b.myTur = false }
	foreign := func(b *board, ctrl uint8, isSpell bool) {
		b.nStack = 1
		b.stack[0] = stackE{obj: 50, ctrl: ctrl, isSpell: isSpell, cmc: 3}
	}
	cases := []tc{
		{name: "own main1", held: true},
		{name: "own main2", set: func(b *board, _ *CardFacts, _ *cardF) { step(b, kop.StepMain2) }, held: true},
		{name: "opp main1", set: func(b *board, _ *CardFacts, _ *cardF) { opp(b) }, held: true},
		{name: "opp main2", set: func(b *board, _ *CardFacts, _ *cardF) { opp(b); step(b, kop.StepMain2) }, held: false},
		{name: "opp end", set: func(b *board, _ *CardFacts, _ *cardF) { opp(b); step(b, kop.StepEnd) }, held: false},
		{name: "trick own attackers", set: func(b *board, cf *CardFacts, _ *cardF) { cf.Trick = true; step(b, kop.StepAttackers) }, held: true},
		{name: "trick own blockers", set: func(b *board, cf *CardFacts, _ *cardF) { cf.Trick = true; step(b, kop.StepBlockers) }, held: false},
		{name: "trick own first strike", set: func(b *board, cf *CardFacts, _ *cardF) { cf.Trick = true; step(b, kop.StepFirstStrikeDamage) }, held: false},
		{name: "trick own damage", set: func(b *board, cf *CardFacts, _ *cardF) { cf.Trick = true; step(b, kop.StepDamage) }, held: false},
		{name: "trick own end combat", set: func(b *board, cf *CardFacts, _ *cardF) { cf.Trick = true; step(b, kop.StepEndCombat) }, held: false},
		{name: "trick opp blockers", set: func(b *board, cf *CardFacts, _ *cardF) { cf.Trick = true; opp(b); step(b, kop.StepBlockers) }, held: false},
		{name: "non-trick own blockers", set: func(b *board, _ *CardFacts, _ *cardF) { step(b, kop.StepBlockers) }, held: true},
		{name: "foreign spell top", set: func(b *board, _ *CardFacts, _ *cardF) { foreign(b, 1, true) }, held: false},
		{name: "foreign ability top", set: func(b *board, _ *CardFacts, _ *cardF) { foreign(b, 1, false) }, held: false},
		{name: "own top", set: func(b *board, _ *CardFacts, _ *cardF) { foreign(b, 0, true) }, held: true},
		{name: "foreign below own", set: func(b *board, _ *CardFacts, _ *cardF) {
			b.nStack = 2
			b.stack[0] = stackE{obj: 50, ctrl: 1, isSpell: true, cmc: 3}
			b.stack[1] = stackE{obj: 51, ctrl: 0, isSpell: true, cmc: 2}
		}, held: true},
		{name: "counter foreign top", set: func(b *board, _ *CardFacts, c *cardF) { c.counter = true; foreign(b, 1, true) }, held: false},
		{name: "counter opp end own top", set: func(b *board, _ *CardFacts, c *cardF) {
			c.counter = true
			opp(b)
			step(b, kop.StepEnd)
			foreign(b, 0, true)
		}, held: true},
		{name: "burn lethal", set: func(b *board, cf *CardFacts, _ *cardF) { cf.FaceDmg = 3; b.life[1] = 3 }, held: false},
		{name: "burn not lethal", set: func(b *board, cf *CardFacts, _ *cardF) { cf.FaceDmg = 3; b.life[1] = 4 }, held: true},
		{name: "sorcery", set: func(_ *board, _ *CardFacts, c *cardF) { c.instant = false }, held: false},
	}
	for _, x := range cases {
		b, cf, c := base()
		if x.set != nil {
			x.set(b, cf, c)
		}
		if got := b.instantHeld(0, c); got != x.held {
			t.Errorf("%s: instantHeld = %v, want %v", x.name, got, x.held)
		}
	}
	nb := &board{me: 0, myTur: true, isMain: true, step: kop.StepMain1, life: [dirac.MaxSeats]int32{20, 20}, n: 2}
	if nb.instantHeld(0, &noCardF) {
		t.Error("noCardF: instantHeld = true, want false")
	}
}

// TestHBotHoldChooseCast: with the rule on, chooseCast skips a held instant
// (H1); with it off, and in a window the rule allows, the same cast ranks
// first.
func TestHBotHoldChooseCast(t *testing.T) {
	mk := func(hold bool) *Bot {
		bt := New(&Facts{})
		if hold {
			bt.EnableHoldInstant()
		}
		bt.b.card[0] = cardF{obj: 10, f: &CardFacts{}, castable: true, instant: true, cmc: 1, own: true}
		bt.b.cardX[10] = 1
		bt.b.nCard = 1
		bt.b.me = 0
		bt.b.life, bt.b.n = [dirac.MaxSeats]int32{20, 20}, 2
		bt.b.myTur = true
		bt.b.isMain = true
		bt.b.step = kop.StepMain1
		return bt
	}
	cs := []cand{{kind: dirac.AkCast, obj: 10}}
	if got := mk(false).chooseCast(0, cs); got != 0 {
		t.Errorf("rule off, own main1: chooseCast = %d, want 0", got)
	}
	if got := mk(true).chooseCast(0, cs); got != -1 {
		t.Errorf("rule on, own main1: chooseCast = %d, want -1", got)
	}
	for _, hold := range []bool{false, true} {
		bt := mk(hold)
		bt.b.myTur = false
		bt.b.isMain = false
		bt.b.step = kop.StepEnd
		if got := bt.chooseCast(0, cs); got != 0 {
			t.Errorf("rule %v, opp end: chooseCast = %d, want 0", hold, got)
		}
	}
}

// holdCensusBot builds a board with one held instant (obj 10) and one
// untapped mana source (obj 20). The source only feeds producible(): the
// zero State has no battlefield objects, so offeredColours sees no
// activatable source and the instant's pips stay empty.
func holdCensusBot(hold bool) *Bot {
	bt := New(&Facts{})
	if hold {
		bt.EnableHoldInstant()
	}
	b := &bt.b
	b.me = 0
	b.myTur = true
	b.isMain = true
	b.step = kop.StepMain1
	b.life, b.n = [dirac.MaxSeats]int32{20, 20}, 2
	b.card[0] = cardF{obj: 10, f: &CardFacts{}, castable: true, instant: true, cmc: 1, own: true}
	b.card[1] = cardF{obj: 20, f: &CardFacts{Produces: [6]int8{1}}, own: true, onBF: true}
	b.cardX[10] = 1
	b.cardX[20] = 2
	b.nCard = 2
	return bt
}

// TestHBotHoldTapGate: a held card is skipped by wantsManual's np==0 fold
// (H3) and by bestUnpayable, hence its np>0 fold (H2).
func TestHBotHoldTapGate(t *testing.T) {
	var s dirac.State
	d := dirac.Decision{Kind: dirac.DkPriority, Player: 0, N: 1}
	d.Options[0] = dirac.MakeAction(dirac.AkCast, 10, 0, 0)

	if off := holdCensusBot(false); !off.wantsManual(&s, &d, 0) {
		t.Fatal("np=0, rule off: wantsManual = false, want true")
	}
	if on := holdCensusBot(true); on.wantsManual(&s, &d, 0) {
		t.Fatal("np=0, held card: wantsManual = true, want false (H3)")
	}

	off := holdCensusBot(false)
	off.payable[0] = 99
	if !off.wantsManual(&s, &d, 1) {
		t.Fatal("np>0, rule off: wantsManual = false, want true")
	}
	on := holdCensusBot(true)
	on.payable[0] = 99
	if on.wantsManual(&s, &d, 1) {
		t.Fatal("np>0, held card: wantsManual = true, want false (H2)")
	}
	if got := on.bestUnpayable(&d, [5]bool{}); got != nil {
		t.Fatalf("held card: bestUnpayable = obj %d, want nil", got.obj)
	}
}

// holdManaState returns a vanilla state in which seat 0 has an untapped
// mana source and a Shock in hand. It plays one land from a Mountain/Shock
// deck.
func holdManaState(t *testing.T) (*dirac.State, dirac.Obj) {
	t.Helper()
	deck := make([]kop.CardID, 0, 40)
	for i := 0; i < 20; i++ {
		deck = append(deck, v.Mountain, v.Shock)
	}
	hand := make([]dirac.Obj, 0, dirac.MaxPerm)
	for seed := uint64(1); seed <= 500; seed++ {
		s := new(dirac.State)
		s.Init(v.Table, deck, deck, seed, dirac.Config{})
		if s.ActivePlayer() != 0 {
			continue
		}
		hand = s.FeatZoneList(0, kop.ZoneHand, hand[:0])
		land, shock := dirac.NoObj, dirac.NoObj
		for _, o := range hand {
			switch s.ObjCard(o) {
			case v.Mountain:
				if land == dirac.NoObj {
					land = o
				}
			case v.Shock:
				if shock == dirac.NoObj {
					shock = o
				}
			}
		}
		if land == dirac.NoObj || shock == dirac.NoObj {
			continue
		}
		var d dirac.Decision
		s.StepInto(&d)
		var play dirac.Action
		found := false
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			if a.Kind() == dirac.AkPlayLand && dirac.Obj(a.Source()) == land {
				play, found = a, true
				break
			}
		}
		if !found {
			continue
		}
		s.Apply(play)
		if s.FeatManaSources(0) == 0 || s.ObjZone(shock) != kop.ZoneHand {
			continue
		}
		return s, shock
	}
	t.Fatal("no state with a mana source and a Shock in hand")
	return nil, dirac.NoObj
}

// TestHBotHoldTimedTap: the timed-tap gate (H4) ignores a held instant and
// fires for the same unpayable instant once the window allows it.
func TestHBotHoldTimedTap(t *testing.T) {
	s, shock := holdManaState(t)
	bt := New(NewFacts(v.Table)).EnableHoldInstant()
	bt.b.fill(s, bt.f, 0)
	d := dirac.Decision{Kind: dirac.DkPriority, Player: 0, N: 1}
	d.Options[0] = dirac.MakeAction(dirac.AkCast, uint8(shock), 0, 0)

	if bt.timedTapWould(s, &d) {
		t.Fatal("own main1: held instant produced a timed tap")
	}
	bt.b.myTur = false
	bt.b.isMain = false
	bt.b.step = kop.StepEnd
	if !bt.timedTapWould(s, &d) {
		t.Fatal("opponent end: unpayable instant did not produce a timed tap")
	}
}

// holdScriptPick plays the hold-state search games: a land every turn,
// creatures (never the instant), attacks, no blocks.
func holdScriptPick(s *dirac.State, d *dirac.Decision, skip kop.CardID) int {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkPlayLand {
			return i
		}
	}
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == dirac.AkCast && s.ObjCard(dirac.Obj(a.Source())) != skip {
			return i
		}
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkAttack {
			return i
		}
	}
	for i := 0; i < int(d.N); i++ {
		if k := d.Options[i].Kind(); k == dirac.AkPass || k == dirac.AkDone {
			return i
		}
	}
	return 0
}

// holdOwnCombatState returns a state at seat 0's declare-blockers priority
// on seat 0's turn with an unpayable Giant Growth in hand and an untapped
// mana source.
func holdOwnCombatState(t *testing.T) (*dirac.State, dirac.Obj) {
	t.Helper()
	deck := make([]kop.CardID, 0, 40)
	for i := 0; i < 13; i++ {
		deck = append(deck, v.Forest)
	}
	for i := 0; i < 14; i++ {
		deck = append(deck, v.GiantGrowth)
	}
	for i := 0; i < 13; i++ {
		deck = append(deck, v.GrizzlyBears)
	}
	hand := make([]dirac.Obj, 0, dirac.MaxPerm)
	for seed := uint64(1); seed <= 300; seed++ {
		s := new(dirac.State)
		s.Init(v.Table, deck, deck, seed, dirac.Config{NoElide: true})
		var d dirac.Decision
		for n := 0; n < 20000; n++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if s.ActivePlayer() == 0 && s.StepNow() == kop.StepBlockers && d.Player == 0 {
				hand = s.FeatZoneList(0, kop.ZoneHand, hand[:0])
				for _, o := range hand {
					if s.ObjCard(o) != v.GiantGrowth || s.FeatManaSources(0) == 0 || s.FeatPoolPays(0, o, 1) {
						continue
					}
					return s, o
				}
			}
			s.Apply(d.Options[holdScriptPick(s, &d, v.GiantGrowth)])
		}
	}
	t.Fatal("no own-combat state with an unpayable trick")
	return nil, dirac.NoObj
}

// holdOppState returns a state at seat 0's priority on seat 1's turn at
// wantStep with an unpayable Burst Lightning in hand and an untapped source.
func holdOppState(t *testing.T, wantStep kop.Step) (*dirac.State, dirac.Obj) {
	t.Helper()
	deck := make([]kop.CardID, 0, 40)
	for i := 0; i < 13; i++ {
		deck = append(deck, v.Mountain)
	}
	for i := 0; i < 14; i++ {
		deck = append(deck, v.BurstLightning)
	}
	for i := 0; i < 13; i++ {
		deck = append(deck, v.RagingGoblin)
	}
	hand := make([]dirac.Obj, 0, dirac.MaxPerm)
	for seed := uint64(1); seed <= 300; seed++ {
		s := new(dirac.State)
		s.Init(v.Table, deck, deck, seed, dirac.Config{NoElide: true})
		var d dirac.Decision
		for n := 0; n < 20000; n++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if s.ActivePlayer() == 1 && s.StepNow() == wantStep && d.Player == 0 && s.StackLen() == 0 {
				hand = s.FeatZoneList(0, kop.ZoneHand, hand[:0])
				for _, o := range hand {
					if s.ObjCard(o) != v.BurstLightning || s.FeatManaSources(0) == 0 || s.FeatPoolPays(0, o, 1) {
						continue
					}
					return s, o
				}
			}
			s.Apply(d.Options[holdScriptPick(s, &d, v.BurstLightning)])
		}
	}
	t.Fatalf("no opponent-%v state with an unpayable instant", wantStep)
	return nil, dirac.NoObj
}

func TestHBotHoldPriorityOpponentTiming(t *testing.T) {
	s, o := holdOppState(t, kop.StepMain1)
	d := dirac.Decision{Player: 0, Kind: dirac.DkPriority, N: 2}
	d.Options[0] = dirac.MakeAction(dirac.AkCast, uint8(o), 0, 1)
	d.Options[1] = dirac.MakeAction(dirac.AkPass, uint8(dirac.NoObj), 0, 0)
	bt := New(NewFacts(v.Table))
	bt.hold = true
	if got := bt.Pick(s, &d); got != 1 {
		t.Fatalf("opponent main1: Pick = %d, want pass (hold the instant)", got)
	}
	s, o = holdOppState(t, kop.StepEnd)
	d = dirac.Decision{Player: 0, Kind: dirac.DkPriority, N: 2}
	d.Options[0] = dirac.MakeAction(dirac.AkCast, uint8(o), 0, 1)
	d.Options[1] = dirac.MakeAction(dirac.AkPass, uint8(dirac.NoObj), 0, 0)
	if got := bt.Pick(s, &d); got != 0 {
		t.Fatalf("opponent end: Pick = %d, want 0 (the timed cast)", got)
	}
}

// TestHBotHoldPriorityTimedTap covers H4 call site (i): at seat 0's
// non-main priority the manual list is used (a timed tap for an allowed
// unpayable instant); with the rule off the same decision passes.
// TestHBotHoldPriorityOpponentTiming covers call site (ii).
func TestHBotHoldPriorityTimedTap(t *testing.T) {
	s, trick := holdOwnCombatState(t)
	f := NewFacts(v.Table)
	f.Cards[v.GiantGrowth].Trick = true // NewFacts has no IR; the oracle facts do
	d := dirac.Decision{Kind: dirac.DkPriority, Player: 0, N: 2}
	d.Options[0] = dirac.MakeAction(dirac.AkCast, uint8(trick), 0, 1)
	d.Options[1] = dirac.MakeAction(dirac.AkPass, 0, 0, 0)

	if got := New(f).EnableHoldInstant().Pick(s, &d); got != 0 {
		t.Fatalf("rule on: Pick = %d, want 0 (the timed-tap cast)", got)
	}
	if got := New(f).Pick(s, &d); got != 1 {
		t.Fatalf("rule off: Pick = %d, want 1 (pass)", got)
	}
}

// TestDiesAtZeroX: chooseCast holds a creature with no toughness of its own
// outside its hand while the floating pool can't raise its {X} above 0; from
// hand, or with the pool to raise X, it casts.
func TestDiesAtZeroX(t *testing.T) {
	mk := func(pool int32) *Bot {
		bt := New(&Facts{})
		f := &CardFacts{CMC: 2, HasX: true, PerX: 1, Creature: true, NoToughness: true}
		bt.b.card[0] = cardF{obj: 10, f: f, castable: true, creature: true, cmc: 2, own: true}
		bt.b.cardX[10] = 1
		bt.b.nCard = 1
		bt.b.pool[0] = pool
		return bt
	}
	if got := mk(0).chooseCast(0, []cand{{kind: dirac.AkCast, obj: 10, offHand: true}}); got != -1 {
		t.Errorf("off hand, empty pool: chooseCast = %d, want -1", got)
	}
	if got := mk(0).chooseCast(0, []cand{{kind: dirac.AkCast, obj: 10}}); got != 0 {
		t.Errorf("from hand: chooseCast = %d, want 0", got)
	}
	if got := mk(3).chooseCast(0, []cand{{kind: dirac.AkCast, obj: 10, offHand: true}}); got != 0 {
		t.Errorf("off hand, X raised: chooseCast = %d, want 0", got)
	}
}
