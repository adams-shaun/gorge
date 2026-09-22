package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// kw:Unleash (CR 702.86): the as-enters choice is posed on every entry path,
// "yes" enters with a +1/+1 counter, and a creature carrying the counter
// can't block. The mechanism is the Riot precedent (a construct choice rules
// reads straight off the printed keyword, a Choose "unleash" event, and
// canBlock's status gate), so these tests drive the real engine paths a real
// corpus card exercises -- TestRiotAndHideawayUseRealCorpusCards and
// TestGrantedDethroneTriggers are the harness models: battlefield entries are
// emitted as direct MoveZone events of real deck cards (never driven through
// steps, which would drag combat asks in), every posed unleash choice is
// answered before the next emit, and replayCheck proves the log-only replay
// reproduces the whole stream.

// unleashCards looks the named corpus card up and links it.
func unleashCard(t *testing.T, name string) *cards.Card {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("%s missing from corpus", name)
	}
	if d := c.Link(); len(d) != 0 {
		t.Fatalf("link %s: %v", name, d)
	}
	return c
}

// enterWithUnleashChoice emits a MoveZone that poses the parking
// applyUnleashReplacement ask and answers it: yes = index 0 (enter with the
// counter), no = index 1 (decline). It fails if the ask is not exactly a
// two-option KChoose.
func enterWithUnleashChoice(t *testing.T, e *Engine, id state.ObjID, from state.Zone, yes bool) {
	t.Helper()
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: from, To: state.ZBattlefield})
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 {
		t.Fatalf("unleash entry choice = %+v, want a two-option KChoose", d)
	}
	idx := 1
	if yes {
		idx = 0
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit unleash choice (yes=%v): %v", yes, err)
	}
}

// TestUnleashChoicePosedAndCounterApplied drives Rakdos Cackler's printed
// K:Unleash through the cast path: the as-enters choice is posed with both
// options, "yes" (index 0) records the Choose "unleash" event, and the
// battlefield entry enters WITH the +1/+1 counter.
func TestUnleashChoicePosedAndCounterApplied(t *testing.T) {
	cackler := unleashCard(t, "Rakdos Cackler")
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = cackler
	}
	cfg := seatZeroStart(Config{Seed: 401, Names: []string{"cackler", "other"}, Decks: [][]*cards.Card{deck, deck}})
	e := New(cfg)
	id := e.G.Objs[0].ID
	if f := e.G.Objs[0].Face(); id == 0 || f == nil || f.Name != "Rakdos Cackler" {
		t.Fatalf("seeded card is not Rakdos Cackler: %+v", e.G.Objs[0].Face())
	}
	// Cast path: the shared as-enters machinery must pose unleash's choice.
	e.cast = &pendingCast{player: 0, card: id, from: state.ZLibrary, ability: -1}
	e.collectETBChoices(0)
	if len(e.cast.etbs) != 1 || e.cast.etbs[0].kind != "unleash" {
		t.Fatalf("unleash choices = %#v, want one unleash etb choice", e.cast.etbs)
	}
	if len(e.cast.etbs[0].options) != 2 {
		t.Fatalf("unleash options = %#v, want take-the-counter and decline", e.cast.etbs[0].options)
	}
	e.etbAnswer(&decision.Decision{}, []decision.Option{e.cast.etbs[0].options[0]})
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("cackler is not on the battlefield: %+v", o)
	}
	if o.Counter("P1P1") != 1 {
		t.Fatalf("unleash 'yes' entry: P1P1 counters = %d, want 1", o.Counter("P1P1"))
	}
	if e.Power(id) != 2 {
		t.Fatalf("unleash 'yes' entry: power = %d, want 2 (1 printed + 1 counter)", e.Power(id))
	}
	// The consumed choice is cleared with the entry, like RiotChoice.
	if o.UnleashChoice != "" {
		t.Fatalf("UnleashChoice not cleared on entry: %q", o.UnleashChoice)
	}
	replayCheck(t, e, cfg)
}

// TestUnleashDeclineEntersWithoutCounter answers "no" (index 1): the
// creature enters plain -- no counter, printed power -- and, being
// counterless, CAN block (the positive control for the can't-block gate).
func TestUnleashDeclineEntersWithoutCounter(t *testing.T) {
	cackler := unleashCard(t, "Rakdos Cackler")
	bear := card(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	seat0 := make([]*cards.Card, 40)
	for i := range seat0 {
		seat0[i] = cackler
	}
	seat1 := make([]*cards.Card, 40)
	for i := range seat1 {
		seat1[i] = bear
	}
	cfg := seatZeroStart(Config{Seed: 402, Names: []string{"cackler", "other"}, Decks: [][]*cards.Card{seat0, seat1}})
	e := New(cfg)
	id := e.G.Objs[0].ID
	enterWithUnleashChoice(t, e, id, state.ZLibrary, false)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("cackler is not on the battlefield: %+v", o)
	}
	if o.Counter("P1P1") != 0 || e.Power(id) != 1 {
		t.Fatalf("unleash decline entry: counters %d power %d, want 0 and printed 1", o.Counter("P1P1"), e.Power(id))
	}
	// Positive control: an enemy attack makes the plain creature blockable.
	enemy := unleashEnemyCreature(t, e, 1, "Bear")
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 0, IDs: []state.ObjID{enemy}})
	if !e.canBlock(id, enemy) {
		t.Fatal("a declined-unleash creature (no counter) cannot block")
	}
	replayCheck(t, e, cfg)
}

// unleashEnemyCreature moves seat p's real deck copy of the named card onto
// the battlefield with a direct logged MoveZone (the
// TestGrantedDethroneTriggers pattern; the engine is left at genesis's
// upkeep, where no step drive is possible without dragging combat asks in),
// after every outstanding unleash ask has been answered.
func unleashEnemyCreature(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Owner == p && o.Face() != nil && o.Face().Name == name {
			e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone, To: state.ZBattlefield})
			return o.ID
		}
	}
	t.Fatalf("no %q among seat %d's objects", name, p)
	return 0
}

// TestUnleashNonCastEntryPosesTheChoice drives the general MoveZone
// replacement (reanimation/blink never create pendingCast): the choice is
// parked before the entry, answered through a submitted intent, and the
// parked move then enters WITH the counter.
func TestUnleashNonCastEntryPosesTheChoice(t *testing.T) {
	cackler := unleashCard(t, "Rakdos Cackler")
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = cackler
	}
	cfg := seatZeroStart(Config{Seed: 403, Names: []string{"cackler", "other"}, Decks: [][]*cards.Card{deck, deck}})
	e := New(cfg)
	id := e.G.Objs[0].ID
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZGraveyard})
	enterWithUnleashChoice(t, e, id, state.ZGraveyard, true)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("cackler is not on the battlefield: %+v", o)
	}
	if o.Counter("P1P1") != 1 {
		t.Fatalf("non-cast Unleash 'yes' entry: P1P1 = %d, want 1", o.Counter("P1P1"))
	}
	replayCheck(t, e, cfg)
}

// TestUnleashCounteredCreatureCantBlock proves the can't-block half on the
// real card: the SAME creature against the SAME attacker cannot block once
// it carries the +1/+1 counter but can again the moment the counter is gone
// (the two canBlock reads differ, so neither half of the assertion is
// vacuous). The creature enters WITH the counter through the non-cast
// parking path (reanimation/blink never create pendingCast), which also
// proves the re-entry ask is posed and answered there.
func TestUnleashCounteredCreatureCantBlock(t *testing.T) {
	cackler := unleashCard(t, "Rakdos Cackler")
	bear := card(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	seat0 := make([]*cards.Card, 40)
	for i := range seat0 {
		seat0[i] = cackler
	}
	seat1 := make([]*cards.Card, 40)
	for i := range seat1 {
		seat1[i] = bear
	}
	cfg := seatZeroStart(Config{Seed: 404, Names: []string{"cackler", "other"}, Decks: [][]*cards.Card{seat0, seat1}})
	e := New(cfg)
	id := e.G.Objs[0].ID
	enterWithUnleashChoice(t, e, id, state.ZLibrary, true)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Counter("P1P1") != 1 {
		t.Fatalf("countered entry failed: %+v counters %d", o, o.Counter("P1P1"))
	}
	enemy := unleashEnemyCreature(t, e, 1, "Bear")
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 0, IDs: []state.ObjID{enemy}})
	if !e.HasKeyword(id, "Unleash") {
		t.Fatal("precondition failed: the creature carries no unleash keyword")
	}
	if e.canBlock(id, enemy) {
		t.Fatal("a countered unleash creature was declared as a blocker")
	}
	// Remove the counter (a logged event): the gate must release with it.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: -1})
	if e.G.Obj(id).Counter("P1P1") != 0 {
		t.Fatal("counter precondition failed: the +1/+1 counter is still there")
	}
	if !e.canBlock(id, enemy) {
		t.Fatal("the unleash creature still cannot block after its counter left")
	}
	replayCheck(t, e, cfg)
}

// TestUnleashFaceDownEntryPosesNothing is the face-down pin the Siege row's
// own boundary holds (CR 708.5): a manifest or cloak entry of an Unleash
// card is a vanilla 2/2 creature -- no as-enters ask is parked, no public
// Choose "unleash" event names the hidden card, and no +1/+1 counter is
// folded on. Both markers are driven, the way
// TestBattleFaceDownEntryEmitsNoProtectorChoose does for Siege.
func TestUnleashFaceDownEntryPosesNothing(t *testing.T) {
	cackler := unleashCard(t, "Rakdos Cackler")
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = cackler
	}
	cfg := seatZeroStart(Config{Seed: 406, Names: []string{"cackler", "other"}, Decks: [][]*cards.Card{deck, deck}})
	e := New(cfg)
	id := e.G.Objs[0].ID
	for _, tc := range []struct{ name, counter string }{
		{"manifest", events.FaceDownEntryCounter},
		{"cloak", events.CloakEntryCounter},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary,
				To: state.ZBattlefield, Counter: tc.counter})
			if d := e.Pending(); d != nil {
				t.Fatalf("face-down unleash entry posed a decision: %+v (%s)", d, d.Prompt)
			}
			for _, ev := range e.L.Events {
				if ev.Kind == events.Choose && ev.Obj == id && ev.Counter == "unleash" {
					t.Fatalf("face-down unleash entry emitted a public Choose %q", ev.Counter)
				}
			}
			o := e.G.Obj(id)
			if o == nil || o.Zone != state.ZBattlefield || !o.FaceDown {
				t.Fatalf("face-down precondition failed: %+v", o)
			}
			if o.Counter("P1P1") != 0 {
				t.Fatalf("face-down unleash entry folded a +1/+1 counter: %d", o.Counter("P1P1"))
			}
		})
	}
	replayCheck(t, e, cfg)
}

// TestRiotFaceDownEntryPosesNothing is the same boundary for the Riot
// precedent applyRiotReplacement (the same hole, closed in the same round):
// a face-down entry of a Riot card poses no counter-or-haste ask and folds
// no counter on the manifested 2/2.
func TestRiotFaceDownEntryPosesNothing(t *testing.T) {
	goblin := unleashCard(t, "Zhur-Taa Goblin")
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = goblin
	}
	cfg := seatZeroStart(Config{Seed: 407, Names: []string{"vandal", "other"}, Decks: [][]*cards.Card{deck, deck}})
	e := New(cfg)
	id := e.G.Objs[0].ID
	for _, tc := range []struct{ name, counter string }{
		{"manifest", events.FaceDownEntryCounter},
		{"cloak", events.CloakEntryCounter},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary,
				To: state.ZBattlefield, Counter: tc.counter})
			if d := e.Pending(); d != nil {
				t.Fatalf("face-down riot entry posed a decision: %+v (%s)", d, d.Prompt)
			}
			for _, ev := range e.L.Events {
				if ev.Kind == events.Choose && ev.Obj == id && ev.Counter == "riot" {
					t.Fatalf("face-down riot entry emitted a public Choose %q", ev.Counter)
				}
			}
			o := e.G.Obj(id)
			if o == nil || o.Zone != state.ZBattlefield || !o.FaceDown {
				t.Fatalf("face-down precondition failed: %+v", o)
			}
			if o.Counter("P1P1") != 0 {
				t.Fatalf("face-down riot entry folded a +1/+1 counter: %d", o.Counter("P1P1"))
			}
		})
	}
	replayCheck(t, e, cfg)
}

// TestRiotFaceDownEntryDoesNotDisableTheNextRiotAsk is the regression pin
// for the placement defect findings-sol1 named on the r2 guard: the
// face-down check MUST run BEFORE applyRiotReplacement parks its move. A
// face-down entry that parked e.riotMove and then returned false would
// leave a stale parked move that is never emitted and never cleared
// (chooseRiot's answer arm cannot fire for it), so the parked-move guard at
// the top of applyRiotReplacement would suppress every later non-cast Riot
// entry's ask for the rest of the match. The pin drives both face-down
// markers and, after each, a SECOND (face-up) goblin entry whose
// counter-or-haste ask must still be posed and answerable.
func TestRiotFaceDownEntryDoesNotDisableTheNextRiotAsk(t *testing.T) {
	goblin := unleashCard(t, "Zhur-Taa Goblin")
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = goblin
	}
	cfg := seatZeroStart(Config{Seed: 408, Names: []string{"vandal", "other"}, Decks: [][]*cards.Card{deck, deck}})
	for _, tc := range []struct{ name, counter string }{
		{"manifest", events.FaceDownEntryCounter},
		{"cloak", events.CloakEntryCounter},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A fresh engine per marker: answering an ask lands the engine at
			// priority, so each marker's sequence is driven on its own stream.
			e := New(cfg)
			var libs []state.ObjID
			for i := range e.G.Objs {
				o := &e.G.Objs[i]
				if o.Owner == 0 && o.Zone == state.ZLibrary && o.Face() != nil && o.Face().Name == "Zhur-Taa Goblin" {
					libs = append(libs, o.ID)
				}
			}
			if len(libs) < 2 {
				t.Fatalf("precondition failed: only %d library goblins", len(libs))
			}
			// First entry: face-down. No ask, no public Choose, no counter.
			fd := libs[0]
			e.emit(events.Event{Kind: events.MoveZone, Obj: fd, From: state.ZLibrary,
				To: state.ZBattlefield, Counter: tc.counter})
			if d := e.Pending(); d != nil {
				t.Fatalf("face-down riot entry posed a decision: %+v (%s)", d, d.Prompt)
			}
			o := e.G.Obj(fd)
			if o == nil || o.Zone != state.ZBattlefield || !o.FaceDown {
				t.Fatalf("face-down precondition failed: %+v", o)
			}
			if e.riotMove != nil {
				t.Fatalf("face-down riot entry left a stale parked riotMove: %+v", *e.riotMove)
			}
			// Second entry: face-up. The ask MUST still be posed.
			next := libs[1]
			e.emit(events.Event{Kind: events.MoveZone, Obj: next, From: state.ZLibrary,
				To: state.ZBattlefield})
			d := e.Pending()
			if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 ||
				d.Options[0].Kind != "riot" {
				t.Fatalf("the follow-up riot entry's ask was suppressed: %+v", d)
			}
			// Answer "counter" and prove the parked move applied end to end.
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
				t.Fatalf("submit riot choice: %v", err)
			}
			o2 := e.G.Obj(next)
			if o2 == nil || o2.Zone != state.ZBattlefield || o2.Counter("P1P1") != 1 {
				t.Fatalf("follow-up countered entry failed: %+v counters %d", o2, o2.Counter("P1P1"))
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestGrantedUnleashCounteredDogCantBlock uses Tesak, Judith's Hellhound's
// real static ("Other Dogs you control have unleash"): a Dog that carries a
// +1/+1 counter cannot block, because canBlock reads the DERIVED keyword
// list, not the printed face -- and the grant is the only unleash on it.
func TestGrantedUnleashCounteredDogCantBlock(t *testing.T) {
	tesak := unleashCard(t, "Tesak, Judith's Hellhound")
	bear := card(t, "Name:Bear Dog\nManaCost:1 G\nTypes:Creature Dog Bear\nPT:2/2\nOracle:x\n")
	seat0 := make([]*cards.Card, 40)
	seat0[0] = tesak
	for i := 1; i < len(seat0); i++ {
		seat0[i] = bear
	}
	seat1 := make([]*cards.Card, 40)
	for i := range seat1 {
		seat1[i] = bear
	}
	cfg := seatZeroStart(Config{Seed: 405, Names: []string{"tesak", "other"}, Decks: [][]*cards.Card{seat0, seat1}})
	e := New(cfg)
	var tm, bd state.ObjID
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Owner != 0 || o.Face() == nil {
			continue
		}
		switch o.Face().Name {
		case "Tesak, Judith's Hellhound":
			tm = o.ID
		case "Bear Dog":
			bd = o.ID
		}
	}
	if tm == 0 || bd == 0 {
		t.Fatalf("fixture ids Tesak=%d Bear Dog=%d", tm, bd)
	}
	// Tesak's own printed K:Unleash poses the as-enters choice on its entry;
	// answer "no" (the plain entry). The Dog's printed face carries no
	// unleash, so its entry poses nothing (a battlefield-scoped grant does
	// not reach a card before it enters).
	enterWithUnleashChoice(t, e, tm, state.ZLibrary, false)
	e.emit(events.Event{Kind: events.MoveZone, Obj: bd, To: state.ZBattlefield})
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if !e.HasKeyword(bd, "Unleash") {
		t.Fatal("Tesak did not grant unleash to the Dog")
	}
	enemy := unleashEnemyCreature(t, e, 1, "Bear Dog")
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 0, IDs: []state.ObjID{enemy}})
	if !e.canBlock(bd, enemy) {
		t.Fatal("precondition failed: the counterless granted-unleash Dog should be able to block")
	}
	// A +1/+1 counter from an unrelated source (not the unleash choice, which
	// a battlefield-scoped grant does not pose for a card already in play):
	// while the Dog carries ANY +1/+1 counter it cannot block.
	e.emit(events.Event{Kind: events.CounterChange, Obj: bd, Counter: "P1P1", Amount: 1})
	if e.G.Obj(bd).Counter("P1P1") != 1 {
		t.Fatal("counter precondition failed: the Dog carries no +1/+1 counter")
	}
	if e.canBlock(bd, enemy) {
		t.Fatal("a granted-unleash Dog with a +1/+1 counter cannot block")
	}
	replayCheck(t, e, cfg)
}
