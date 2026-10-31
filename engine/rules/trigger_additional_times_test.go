package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Doublers for CR 603.2d, one per scope the corpus's statics read: every
// trigger of your permanents, only those an entering permanent causes, only
// those an attacking creature causes, and one that also reaches your spells.
const (
	everyTriggerDoubler = "Name:Second Verse\nManaCost:2\nTypes:Artifact\n" +
		"S:Mode$ Panharmonicon | ValidCard$ Permanent.YouCtrl | Description$ Your permanents' triggers fire once more.\nOracle:x\n"
	entryTriggerDoubler = "Name:Second Arrival\nManaCost:2\nTypes:Artifact\n" +
		"S:Mode$ Panharmonicon | ValidMode$ ChangesZone,ChangesZoneAll | ValidCard$ Permanent.YouCtrl | ValidCause$ Permanent | Destination$ Battlefield | Description$ Entries trigger your permanents once more.\nOracle:x\n"
	attackTriggerDoubler = "Name:Second Charge\nManaCost:2\nTypes:Artifact\n" +
		"S:Mode$ Panharmonicon | ValidMode$ Attacks,AttackersDeclared | ValidCard$ Permanent.YouCtrl | ValidCause$ Creature | Description$ Attacks trigger your permanents once more.\nOracle:x\n"
	spellTriggerDoubler = "Name:Second Hymn\nManaCost:2\nTypes:Artifact\n" +
		"S:Mode$ Panharmonicon | ValidCard$ Card.YouCtrl+Other | ValidZone$ Battlefield,Stack | Description$ Your other spells and permanents trigger once more.\nOracle:x\n"

	gainOneBody = "SVar:GainOne:DB$ GainLife | LifeAmount$ 1\nOracle:x\n"
)

// grantKeywordSrc is a lord handing its controller's other creatures one keyword.
func grantKeywordSrc(keyword string) string {
	return "Name:Verse Lord\nManaCost:2\nTypes:Artifact\n" +
		"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddKeyword$ " + keyword + " | Description$ Your creatures have it.\nOracle:x\n"
}

func TestChapterTriggersAdditionalTimes(t *testing.T) {
	t.Parallel()
	saga := "Name:Verse Saga\nManaCost:1\nTypes:Enchantment Saga\nK:Chapter:3:GainOne,GainOne,GainOne\n" + gainOneBody
	for _, tc := range []struct {
		name    string
		doubler string
		lore    int32
		want    int
	}{
		{"no doubler", "", 1, 1},
		{"unscoped", everyTriggerDoubler, 1, 2},
		{"each chapter crossed", everyTriggerDoubler, 2, 4},
		{"a lore counter is no entry", entryTriggerDoubler, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := combatEngine(t)
			if tc.doubler != "" {
				onBoard(t, e, 0, tc.doubler)
			}
			id := onBoard(t, e, 0, saga)
			e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "LORE", Amount: tc.lore})
			if got := queuedPhaseTriggers(e, id); got != tc.want {
				t.Fatalf("queued %d chapter triggers, want %d", got, tc.want)
			}
		})
	}
}

// A plain "whenever a counter is put on" line triggers per counter (CR 603.2c),
// and each of those triggers additionally.
func TestPerCounterTriggerAdditionalTimes(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	onBoard(t, e, 0, everyTriggerDoubler)
	id := onBoard(t, e, 0, "Name:Verse Mage\nManaCost:1\nTypes:Creature Wizard\nPT:1/1\n"+
		"T:Mode$ CounterAdded | ValidCard$ Card.Self | CounterType$ P1P1 | TriggerZones$ Battlefield | Execute$ GainOne\n"+gainOneBody)
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 2})
	if got := queuedPhaseTriggers(e, id); got != 4 {
		t.Fatalf("queued %d triggers for two counters, want 4", got)
	}
}

func TestBlockTriggersAdditionalTimes(t *testing.T) {
	t.Parallel()
	const creature = "Name:Verse Foe\nManaCost:1\nTypes:Creature Soldier\nPT:1/1\nOracle:x\n"
	blocks := "Name:Verse Blocker\nManaCost:1\nTypes:Creature Wall\nPT:0/4\n" +
		"T:Mode$ Blocks | ValidCard$ Card.Self | TriggerZones$ Battlefield | Execute$ GainOne\n" + gainOneBody
	blocked := "Name:Verse Raider\nManaCost:1\nTypes:Creature Soldier\nPT:1/1\n" +
		"T:Mode$ AttackerBlocked | ValidCard$ Card.Self | TriggerZones$ Battlefield | Execute$ GainOne\n" + gainOneBody

	for _, tc := range []struct {
		name    string
		doubler string
		want    int
	}{
		{"no doubler", "", 1},
		{"unscoped", everyTriggerDoubler, 2},
		{"a block is no attack", attackTriggerDoubler, 1},
	} {
		t.Run("Blocks "+tc.name, func(t *testing.T) {
			t.Parallel()
			e := combatEngine(t)
			if tc.doubler != "" {
				onBoard(t, e, 0, tc.doubler)
			}
			blk := onBoard(t, e, 0, blocks)
			atk := onBoard(t, e, 1, creature)
			e.emit(events.Event{Kind: events.DeclareBlockers, Player: 0, Pairs: [][2]state.ObjID{{atk, blk}}})
			if got := queuedPhaseTriggers(e, blk); got != tc.want {
				t.Fatalf("queued %d, want %d", got, tc.want)
			}
		})
		t.Run("AttackerBlocked "+tc.name, func(t *testing.T) {
			t.Parallel()
			e := combatEngine(t)
			if tc.doubler != "" {
				onBoard(t, e, 0, tc.doubler)
			}
			atk := onBoard(t, e, 0, blocked)
			blk := onBoard(t, e, 1, creature)
			e.emit(events.Event{Kind: events.DeclareBlockers, Player: 1, Pairs: [][2]state.ObjID{{atk, blk}}})
			if got := queuedPhaseTriggers(e, atk); got != tc.want {
				t.Fatalf("queued %d, want %d", got, tc.want)
			}
		})
	}
}

func TestUnblockedTriggersAdditionalTimes(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"AttackerUnblocked", "AttackerUnblockedOnce"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			e := combatEngine(t)
			onBoard(t, e, 0, everyTriggerDoubler)
			id := onBoardReady(t, e, 0, "Name:Verse Raider\nManaCost:1\nTypes:Creature Soldier\nPT:1/1\n"+
				"T:Mode$ "+mode+" | ValidAttackers$ Creature.YouCtrl | TriggerZones$ Battlefield | Execute$ GainOne\n"+gainOneBody)
			e.askAttackers()
			submitAttackersOnly(t, e, id)
			e.pendingTriggers = nil
			if mode == "AttackerUnblocked" {
				e.checkAttackerUnblockedTriggers()
			} else {
				e.checkAttackerUnblockedOnceTriggers()
			}
			if got := queuedPhaseTriggers(e, id); got != 2 {
				t.Fatalf("queued %d, want 2", got)
			}
		})
	}
}

// An Effect's "whenever a creature blocks this turn" is a delayed trigger, which
// CR 603.2d puts outside "a triggered ability of a permanent".
func TestEffectCombatTriggerNotAdditional(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	onBoard(t, e, 0, everyTriggerDoubler)
	ectCast(t, e, corpusCard(t, "Battle Cry"), 0, "")
	atk := onBoard(t, e, 1, ectBear)
	blk := onBoard(t, e, 0, ectWhite2)
	e.emit(events.Event{Kind: events.DeclareBlockers, Player: 0, Pairs: [][2]state.ObjID{{atk, blk}}})
	if got := len(e.pendingTriggers); got != 1 {
		t.Fatalf("queued %d, want 1", got)
	}
}

func TestGrantedTriggerAdditionalTimes(t *testing.T) {
	t.Parallel()
	const bear = "Name:Verse Bear\nManaCost:1\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	addTrigger := "Name:Verse Lord\nManaCost:2\nTypes:Artifact\n" +
		"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddTrigger$ GrantedGain | Description$ Your creatures have it.\n" +
		"SVar:GrantedGain:Mode$ LifeGained | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ GainOne | TriggerDescription$ Whenever you gain life, gain 1.\n" + gainOneBody
	attackTrigger := "Name:Verse Lord\nManaCost:2\nTypes:Artifact\n" +
		"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddTrigger$ GrantedAttack | Description$ Your creatures have it.\n" +
		"SVar:GrantedAttack:Mode$ Attacks | ValidCard$ Card.Self | Execute$ GainOne | TriggerDescription$ Whenever this attacks, gain 1.\n" + gainOneBody

	lifeGain := func(e *Engine, id state.ObjID) {
		e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 1})
	}
	attack := func(e *Engine, id state.ObjID) {
		e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{id}})
	}
	entry := func(e *Engine, id state.ObjID) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield, Player: 0})
	}
	for _, tc := range []struct {
		name    string
		lord    string
		doubler string
		fire    func(*Engine, state.ObjID)
		want    int
	}{
		{"AddTrigger without a doubler", addTrigger, "", lifeGain, 1},
		{"AddTrigger", addTrigger, everyTriggerDoubler, lifeGain, 2},
		{"AddTrigger's attack, unscoped", attackTrigger, everyTriggerDoubler, attack, 2},
		{"AddTrigger's attack, attack doubler", attackTrigger, attackTriggerDoubler, attack, 2},
		{"AddTrigger's life gain is no attack", addTrigger, attackTriggerDoubler, lifeGain, 1},
		{"Melee", grantKeywordSrc("Melee"), attackTriggerDoubler, attack, 2},
		{"Dethrone", grantKeywordSrc("Dethrone"), attackTriggerDoubler, attack, 2},
		{"Decayed", grantKeywordSrc("Decayed"), attackTriggerDoubler, attack, 2},
		{"Firebending", grantKeywordSrc("Firebending:2"), attackTriggerDoubler, attack, 2},
		{"Mentor", grantKeywordSrc("Mentor"), attackTriggerDoubler, attack, 2},
		{"Exploit", grantKeywordSrc("Exploit"), entryTriggerDoubler, entry, 2},
		{"Exploit's entry is no attack", grantKeywordSrc("Exploit"), attackTriggerDoubler, entry, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := combatEngine(t)
			if tc.doubler != "" {
				onBoard(t, e, 0, tc.doubler)
			}
			onBoard(t, e, 0, tc.lord)
			id := onBoardReady(t, e, 0, bear)
			e.G.Obj(id).IsAttacking = true
			e.pendingTriggers = nil
			tc.fire(e, id)
			if got := queuedPhaseTriggers(e, id); got != tc.want {
				t.Fatalf("queued %d, want %d", got, tc.want)
			}
		})
	}
}

// A keyword's own printed trigger outside the face walk: an evoked permanent's
// sacrifice is caused by its entry.
func TestEvokeTriggerAdditionalTimes(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	onBoard(t, e, 0, entryTriggerDoubler)
	id := onBoard(t, e, 0, "Name:Verse Sprite\nManaCost:2\nTypes:Creature Faerie\nPT:1/1\nK:Evoke:1\nOracle:x\n")
	e.G.Obj(id).CastFlags |= state.FlagEvoked
	e.pendingTriggers = nil
	e.altCostEnter(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: state.ZBattlefield})
	if got := queuedPhaseTriggers(e, id); got != 2 {
		t.Fatalf("queued %d evoke triggers, want 2", got)
	}
}

// "A triggered ability of a permanent" does not reach a spell (ValidZone$ reads
// the battlefield unless it says otherwise); a static naming the stack does.
func TestSpellTriggerAdditionalTimesReadsValidZone(t *testing.T) {
	t.Parallel()
	const spell = "Name:Verse Engine\nManaCost:1\nTypes:Artifact\nK:Cascade\nOracle:x\n"
	const onCast = "Name:Verse Golem\nManaCost:1\nTypes:Artifact Creature Golem\nPT:1/1\n" +
		"T:Mode$ SpellCast | ValidCard$ Card.Self | TriggerZones$ Stack | Execute$ GainOne | TriggerDescription$ When you cast this, gain 1.\n" + gainOneBody
	for _, tc := range []struct {
		name    string
		src     string
		card    string
		doubler string
		want    int
	}{
		{"cascade, permanents only", spell, "Verse Engine", everyTriggerDoubler, 1},
		{"cascade, spells too", spell, "Verse Engine", spellTriggerDoubler, 2},
		{"printed cast trigger, permanents only", onCast, "Verse Golem", everyTriggerDoubler, 1},
		{"printed cast trigger, spells too", onCast, "Verse Golem", spellTriggerDoubler, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _, _ := newFixtureDeck(t, 73, tc.src)
			onBoard(t, e, 0, tc.doubler)
			id := moveByName(t, e, 0, tc.card, state.ZHand)
			addMana(t, e, 0, "C")
			mark := len(e.L.Events)
			castNamed(t, e, tc.card)
			got := queuedPhaseTriggers(e, id)
			for _, ev := range e.L.Events[mark:] {
				if ev.Obj == id && (ev.Kind == events.TriggerPush || ev.Kind == events.KeywordTriggerPush) {
					got++
				}
			}
			if got != tc.want {
				t.Fatalf("triggered %d times, want %d", got, tc.want)
			}
		})
	}
}

// A dungeon's room ability is a triggered ability of the dungeon: a static scoped
// to RoomEntered and the command zone reaches it, the unscoped one does not.
func TestRoomAbilityAdditionalTimes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		doubler func(*testing.T, *Engine)
		want    int
	}{
		{"scoped to rooms", func(t *testing.T, e *Engine) {
			onBoardCard(t, e, 0, corpusCard(t, "Hama Pashar, Ruin Seeker"))
		}, 2},
		{"permanents only", func(t *testing.T, e *Engine) { onBoard(t, e, 0, everyTriggerDoubler) }, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := testutil.CorpusRegistry(t)
			e, _, id := newVentureGame(t, reg, ventureFixtureSrc, 8)
			tc.doubler(t, e)
			mark := len(e.L.Events)
			d := castVenture(t, e, id)
			if d == nil || d.Kind != decision.KChoose {
				t.Fatalf("no dungeon choice: %+v", d)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
				t.Fatalf("submit: %v", err)
			}
			dungeon := e.G.Players[0].DungeonObj
			got := queuedPhaseTriggers(e, dungeon)
			for _, ev := range e.L.Events[mark:] {
				if ev.Kind == events.DelayedPush && ev.Obj == dungeon {
					got++
				}
			}
			if got != tc.want {
				t.Fatalf("room ability pushed %d times, want %d", got, tc.want)
			}
		})
	}
}
