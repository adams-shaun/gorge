package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHKeywordsRampagePumpsPerExtraBlocker plays Marhault Elsdragon's
// printed Rampage 1 (CR 702.23) through a real combat: one blocker is the
// baseline (no pump), each blocker beyond the first pumps +1/+1 until end of
// turn when the becomes-blocked trigger resolves.
func TestEDHKeywordsRampagePumpsPerExtraBlocker(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	marhault, ok := reg.Lookup("Marhault Elsdragon")
	if !ok {
		t.Fatal("Marhault Elsdragon not in the corpus")
	}
	if got, ok := marhault.Faces[0].KeywordParam("Rampage"); !ok || got != "1" {
		t.Fatalf("Marhault Elsdragon keyword param = %q (%v), want Rampage 1", got, ok)
	}
	const bear = "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

	// Two blockers: 4/6 becomes 5/6 when the trigger resolves.
	e := combatEngine(t)
	attacker := onBoardCard(t, e, 0, marhault)
	e.G.Obj(attacker).SummonSick = false
	b1 := onBoard(t, e, 1, bear)
	b2 := onBoard(t, e, 1, bear)
	e.askAttackers()
	submitAttackersOnly(t, e, attacker)
	drainCombatPriority(t, e)
	if d := e.Pending(); d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	submitBlockersOnly(t, e, b1, b2)
	if n := len(e.G.Stack); n != 1 {
		t.Fatalf("stack = %d, want one Rampage trigger", n)
	}
	if e.Power(attacker) != 4 || e.Toughness(attacker) != 6 {
		t.Fatalf("before resolution = %d/%d, want 4/6", e.Power(attacker), e.Toughness(attacker))
	}
	e.resolveTop()
	if e.Power(attacker) != 5 || e.Toughness(attacker) != 7 {
		t.Fatalf("after resolution = %d/%d, want 5/7 (one blocker beyond the first pumps +1/+1)", e.Power(attacker), e.Toughness(attacker))
	}

	// One blocker: no pump at all.
	e2 := combatEngine(t)
	alone := onBoardCard(t, e2, 0, marhault)
	e2.G.Obj(alone).SummonSick = false
	blk := onBoard(t, e2, 1, bear)
	e2.askAttackers()
	submitAttackersOnly(t, e2, alone)
	drainCombatPriority(t, e2)
	if d := e2.Pending(); d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	submitBlockersOnly(t, e2, blk)
	if n := len(e2.G.Stack); n != 1 {
		t.Fatalf("single-blocker stack = %d, want one Rampage trigger", n)
	}
	e2.resolveTop()
	if e2.Power(alone) != 4 || e2.Toughness(alone) != 6 {
		t.Fatalf("single blocker after resolution = %d/%d, want 4/6", e2.Power(alone), e2.Toughness(alone))
	}
}

// TestEDHKeywordsTransfigureTutorsSameManaValue plays Fleshwrither's printed
// Transfigure 1BB (CR 702.71) as a real activation: the cost sacs the
// creature and the search offers exactly the mana-value-four creatures from
// the library, the chosen one entering the battlefield.
func TestEDHKeywordsTransfigureTutorsSameManaValue(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	fleshwrither, ok := reg.Lookup("Fleshwrither")
	if !ok {
		t.Fatal("Fleshwrither not in the corpus")
	}
	if got, ok := fleshwrither.Faces[0].KeywordParam("Transfigure"); !ok || got != "1 B B" {
		t.Fatalf("Fleshwrither keyword param = %q (%v), want Transfigure 1 B B", got, ok)
	}
	e := handEngine(t, fleshwrither)
	fw := e.G.Zone(state.ZHand, 0)[0]
	placeOnBattlefield(t, e, fw)
	// handEngine's library is all Mountains (mana value zero, not creatures),
	// so the search would correctly find nothing unmodified. Seed a
	// mana-value-four creature for it to find.
	mv4 := card(t, "Name:Mannequin Giant\nManaCost:2 B B\nTypes:Creature Giant\nPT:4/4\nOracle:x\n")
	sObj := e.G.AddObject(mv4, 0)
	e.G.SetZone(state.ZLibrary, 0, append([]state.ObjID{sObj.ID}, e.G.Zone(state.ZLibrary, 0)...))
	wanted := sObj.ID
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MB] = 1, 2
	var opt decision.Option
	for _, o := range e.legalActions(0) {
		if o.Kind == "ability" && o.Obj == fw {
			opt = o
			break
		}
	}
	if opt.Kind != "ability" {
		t.Fatal("Fleshwrither Transfigure activation not offered")
	}
	e.beginActivation(0, opt)
	if e.G.Obj(fw).Zone != state.ZGraveyard {
		t.Fatalf("Transfigure sacrifice zone=%s", e.G.Obj(fw).Zone)
	}
	e.pending = nil // the kernel probe serves asks only with no decision posed
	e.resolveTop()
	d := e.Pending()
	choice := -1
	if d != nil {
		for i, o := range d.Options {
			if o.Obj == wanted {
				choice = i
				break
			}
		}
	}
	if choice < 0 {
		t.Fatalf("Transfigure did not offer the mana-value-four library card: %+v", d)
	}
	submitChoices(t, e, choice)
	if e.G.Obj(wanted).Zone != state.ZBattlefield {
		t.Fatalf("Transfigure target zone=%s, want battlefield", e.G.Obj(wanted).Zone)
	}
}

// TestEDHKeywordsOnlyAttackAloneSoleAttacker plays Master of Cruelties'
// printed "CARDNAME can only attack alone." (CR 506.5) through a real
// combat: declaring it beside another attacker is refused (and the option
// publishes the fact the wire fence repairs), while declaring it as the
// sole attacker is legal.
func TestEDHKeywordsOnlyAttackAloneSoleAttacker(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	master, ok := reg.Lookup("Master of Cruelties")
	if !ok {
		t.Fatal("Master of Cruelties not in the corpus")
	}
	const bearCard = "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e := combatEngine(t)
	moc := onBoardCard(t, e, 0, master)
	bear := onBoard(t, e, 0, bearCard)
	e.G.Obj(moc).SummonSick = false
	e.G.Obj(bear).SummonSick = false
	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision, got %+v", d)
	}
	mocIdx, bearIdx := -1, -1
	for _, o := range d.Options {
		if o.Obj == moc {
			mocIdx = o.Index
			if !o.OnlyAloneRestricted {
				t.Fatal("Master of Cruelties attack option does not publish OnlyAloneRestricted")
			}
		}
		if o.Obj == bear {
			bearIdx = o.Index
			if o.OnlyAloneRestricted {
				t.Fatal("unrestricted bear publishes OnlyAloneRestricted")
			}
		}
	}
	if mocIdx < 0 || bearIdx < 0 {
		t.Fatalf("attack options not offered: %+v", d.Options)
	}
	// The whole-declaration guard refuses the pair...
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{mocIdx, bearIdx}}); err == nil {
		t.Fatal("declaring Master of Cruelties beside another attacker was accepted")
	}
	// ...and the shared repair drops the restricted creature from the bot's
	// answer, the same home the wire fence validates.
	if got := d.FitAlone([]int{mocIdx, bearIdx}); len(got) != 1 || got[0] != bearIdx {
		t.Fatalf("FitAlone({moc, bear}) = %v, want {bear}", got)
	}
	if !d.AloneLegal([]int{mocIdx}) || d.AloneLegal([]int{mocIdx, bearIdx}) {
		t.Fatal("AloneLegal polarity inverted for only-attack-alone")
	}
	// A lone Master of Cruelties is legal: the attack goes through.
	e2 := combatEngine(t)
	alone := onBoardCard(t, e2, 0, master)
	e2.G.Obj(alone).SummonSick = false
	e2.askAttackers()
	submitAttackersOnly(t, e2, alone)
	drainCombatPriority(t, e2)
}

// TestEDHKeywordsMustBeBlockedByExactlyOne plays Nacatl War-Pride's printed
// "CARDNAME must be blocked by exactly one creature if able." (CR 509.1c)
// through a real declare-blockers step: one blocker is the only blocking
// declaration, two are refused, and with no able blocker the attack is
// simply unblocked.
func TestEDHKeywordsMustBeBlockedByExactlyOne(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	nacatl, ok := reg.Lookup("Nacatl War-Pride")
	if !ok {
		t.Fatal("Nacatl War-Pride not in the corpus")
	}
	const bear = "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e := combatEngine(t)
	atk := onBoardCard(t, e, 0, nacatl)
	e.G.Obj(atk).SummonSick = false
	b1 := onBoard(t, e, 1, bear)
	b2 := onBoard(t, e, 1, bear)
	e.askAttackers()
	submitAttackersOnly(t, e, atk)
	drainCombatPriority(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Attacker == atk && o.MaxBlockers != 1 {
			t.Fatalf("pair option publishes MaxBlockers %d, want 1: %+v", o.MaxBlockers, o)
		}
		if o.Attacker == atk && !o.AttackMust {
			t.Fatalf("pair option does not publish the block duty: %+v", o)
		}
	}
	// Two blockers are refused by the wire fence the published bound drives.
	b1Idx, b2Idx := -1, -1
	for _, o := range d.Options {
		if o.Obj == b1 {
			b1Idx = o.Index
		}
		if o.Obj == b2 {
			b2Idx = o.Index
		}
	}
	if b1Idx < 0 || b2Idx < 0 {
		t.Fatalf("block options not offered: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{b1Idx, b2Idx}}); err == nil {
		t.Fatal("blocking Nacatl War-Pride with two creatures was accepted")
	}
	// Exactly one is legal.
	e2 := combatEngine(t)
	atk2 := onBoardCard(t, e2, 0, nacatl)
	e2.G.Obj(atk2).SummonSick = false
	blk := onBoard(t, e2, 1, bear)
	e2.askAttackers()
	submitAttackersOnly(t, e2, atk2)
	drainCombatPriority(t, e2)
	if d := e2.Pending(); d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	submitBlockersOnly(t, e2, blk)
	// No able blocker: the attack is simply unblocked ("if able").
	e3 := combatEngine(t)
	atk3 := onBoardCard(t, e3, 0, nacatl)
	e3.G.Obj(atk3).SummonSick = false
	e3.askAttackers()
	submitAttackersOnly(t, e3, atk3)
	drainCombatPriority(t, e3)
	if d := e3.Pending(); d != nil && d.Kind == decision.KBlockers && len(d.Options) != 0 {
		t.Fatalf("no-blocker engine still offered pairs: %+v", d.Options)
	}
}

// TestEDHKeywordsRecoverPaysToHandOrExiles plays Icefall's printed Recover
// {R}{R} (CR 702.59) through real deaths: paying the cost returns the card
// from the graveyard to the hand, declining exiles it.
func TestEDHKeywordsRecoverPaysToHandOrExiles(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	icefall, ok := reg.Lookup("Icefall")
	if !ok {
		t.Fatal("Icefall not in the corpus")
	}
	if got, ok := icefall.Faces[0].KeywordParam("Recover"); !ok || got != "R R" {
		t.Fatalf("Icefall keyword param = %q (%v), want Recover R R", got, ok)
	}
	const bear = "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	// (a) pay: the pool can pay {R}{R} outright, so the election offers pay
	// first and the pay arm returns Icefall to hand.
	e := handEngine(t, icefall, card(t, bear))
	id := e.G.Zone(state.ZHand, 0)[0]
	bearID := e.G.Zone(state.ZHand, 0)[1]
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard})
	e.emit(events.Event{Kind: events.MoveZone, Obj: bearID, From: state.ZHand, To: state.ZBattlefield})
	e.G.Players[0].Pool[state.MR] = 2
	e.emit(events.Event{Kind: events.Damage, Obj: bearID, Amount: 3})
	e.checkStateBased()
	e.priorityRound()
	driveUntilRecoverAsk(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the recover election, got %+v", d)
	}
	if len(d.Options) != 2 || d.Options[0].Kind != "recover_pay" || d.Options[1].Kind != "recover_exile" {
		t.Fatalf("recover election = %+v", d.Options)
	}
	submitChoices(t, e, 0)
	if o := e.G.Obj(id); o.Zone != state.ZHand {
		t.Fatalf("paid recover but Icefall is in %s", o.Zone)
	}
	if o := e.G.Obj(bearID); o.Zone != state.ZGraveyard {
		t.Fatalf("the bear is in %s, want graveyard", o.Zone)
	}

	// (b) decline: no pool, so the funding route rides first and the exile
	// option sits between it and the manual one; choosing exile moves the
	// card to exile.
	e2 := handEngine(t, icefall, card(t, bear))
	id2 := e2.G.Zone(state.ZHand, 0)[0]
	bear2 := e2.G.Zone(state.ZHand, 0)[1]
	e2.emit(events.Event{Kind: events.MoveZone, Obj: id2, From: state.ZHand, To: state.ZGraveyard})
	e2.emit(events.Event{Kind: events.MoveZone, Obj: bear2, From: state.ZHand, To: state.ZBattlefield})
	e2.emit(events.Event{Kind: events.Damage, Obj: bear2, Amount: 3})
	e2.checkStateBased()
	e2.priorityRound()
	driveUntilRecoverAsk(t, e2)
	d2 := e2.Pending()
	if d2 == nil || d2.Kind != decision.KChoose {
		t.Fatalf("expected the recover election, got %+v", d2)
	}
	exile := -1
	for _, o := range d2.Options {
		if o.Kind == "recover_exile" {
			exile = int(o.Index)
		}
	}
	if exile < 0 {
		t.Fatalf("no exile option in the election: %+v", d2.Options)
	}
	submitChoices(t, e2, exile)
	if o := e2.G.Obj(id2); o.Zone != state.ZExile {
		t.Fatalf("declined recover but Icefall is in %s, want exile", o.Zone)
	}

	// (c) Garza's Assassin's life spelling: half your life, rounded up.
	garza, ok := reg.Lookup("Garza's Assassin")
	if !ok {
		t.Fatal("Garza's Assassin not in the corpus")
	}
	e3 := handEngine(t, garza, card(t, bear))
	id3 := e3.G.Zone(state.ZHand, 0)[0]
	bear3 := e3.G.Zone(state.ZHand, 0)[1]
	e3.emit(events.Event{Kind: events.MoveZone, Obj: id3, From: state.ZHand, To: state.ZGraveyard})
	e3.emit(events.Event{Kind: events.MoveZone, Obj: bear3, From: state.ZHand, To: state.ZBattlefield})
	e3.G.Players[0].Life = 20
	e3.emit(events.Event{Kind: events.Damage, Obj: bear3, Amount: 3})
	e3.checkStateBased()
	e3.priorityRound()
	driveUntilRecoverAsk(t, e3)
	d3 := e3.Pending()
	if d3 == nil || d3.Kind != decision.KChoose {
		t.Fatalf("expected the recover election, got %+v", d3)
	}
	pay := -1
	for _, o := range d3.Options {
		if o.Kind == "recover_pay" {
			pay = int(o.Index)
		}
	}
	if pay < 0 {
		t.Fatalf("no pay option in the election: %+v", d3.Options)
	}
	submitChoices(t, e3, pay)
	if o := e3.G.Obj(id3); o.Zone != state.ZHand {
		t.Fatalf("paid recover but Garza's Assassin is in %s", o.Zone)
	}
	if e3.G.Players[0].Life != 10 {
		t.Fatalf("life after recover = %d, want 10 (half of 20, rounded up)", e3.G.Players[0].Life)
	}
}

// driveUntilRecoverAsk drains priority passes and stack resolutions until the
// recover election is the pending decision (the passUntilStackEmpty shape,
// stopping at the non-priority ask instead of refusing it).
func driveUntilRecoverAsk(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d != nil && d.Kind == decision.KChoose {
			for _, o := range d.Options {
				if o.Kind == "recover_pay" || o.Kind == "recover_exile" {
					return
				}
			}
		}
		if d == nil {
			if len(e.G.Stack) == 0 {
				t.Fatal("stack drained without posing the recover election")
			}
			e.resolveTop()
			continue
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatalf("pass priority: %v", err)
		}
	}
	t.Fatal("recover election never posed")
}
