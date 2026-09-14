package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestFieryEmancipationTriplesDamage uses the unmodified compiled corpus
// replacement: ReplaceEffect changes DamageAmount before DamageDone is logged.
func TestFieryEmancipationTriplesDamage(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Fiery Emancipation"))
	source := onBoard(t, e, 0, "Name:Red Source\nManaCost:R\nTypes:Creature\nPT:1/1\nOracle:x\n")

	e.damaging = source
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.damaging = 0
	if got := e.G.Players[1].Life; got != 14 {
		t.Fatalf("life = %d, want 14 after Fiery Emancipation tripled 2 damage", got)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Damage && ev.Player == 1 && ev.Amount != 6 {
			t.Fatalf("logged damage = %d, want rewritten amount 6", ev.Amount)
		}
	}
}

// TestChandraCannotBeCountered uses Chandra's real R:Event$ Counter line and
// Counterspell's real effect: the Counter primitive must ask the replacement
// before it moves the target off the stack.
func TestChandraCannotBeCountered(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	chandra := e.G.AddObject(mustCorpusCard(t, reg, "Chandra, Awakened Inferno"), 0)
	chandra.Zone = state.ZStack
	e.G.SetZone(state.ZStack, 0, []state.ObjID{chandra.ID})
	e.G.Stack = []state.ObjID{chandra.ID}

	counterspell := mustCorpusCard(t, reg, "Counterspell").Faces[0].SpellAbility()
	effects.Resolve(e, &effects.Ctx{Controller: 1, Targets: []state.Target{{Obj: chandra.ID}}}, counterspell)
	if got := e.G.Obj(chandra.ID).Zone; got != state.ZStack {
		t.Fatalf("Chandra zone = %s, want stack after its counter replacement", got)
	}
	if len(e.G.Stack) != 1 || e.G.Stack[0] != chandra.ID {
		t.Fatalf("stack = %v, want Chandra still present", e.G.Stack)
	}
}

// TestSpiderPunkStopsProtectionPrevention uses Spider-Punk's real global
// CantPreventDamage static. Protection is the engine's current prevention
// path, so the protected creature must still take the blue source's damage.
func TestSpiderPunkStopsProtectionPrevention(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Spider-Punk"))
	target := onBoard(t, e, 1, "Name:Protected\nTypes:Creature\nPT:1/3\nK:Protection from blue\nOracle:x\n")
	source := onBoard(t, e, 0, "Name:Blue Source\nManaCost:U\nTypes:Creature\nPT:1/1\nOracle:x\n")

	e.damaging = source
	e.emit(events.Event{Kind: events.Damage, Obj: target, Amount: 2})
	e.damaging = 0
	if got := e.G.Obj(target).Damage; got != 2 {
		t.Fatalf("damage = %d, want 2; Spider-Punk forbids protection prevention", got)
	}
}

// TestOjerAxonilRaisesSmallNoncombatRedDamage uses the unmodified compiled
// corpus replacement: DamageAmount$ LTX (less than Ojer's power),
// ValidSource$ Card.RedSource+YouCtrl (the <Colour>Source predicate family),
// IsCombat$ False, and a VarValue$ X whose SVar is Count$CardPower. Noncombat
// red damage below the power is raised to the power; combat damage is not.
func TestOjerAxonilRaisesSmallNoncombatRedDamage(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Ojer Axonil, Deepest Might"))
	source := onBoard(t, e, 0, "Name:Red Source\nManaCost:R\nTypes:Creature\nPT:1/1\nOracle:x\n")

	e.damaging = source
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	if got := e.G.Players[1].Life; got != 16 {
		t.Fatalf("life = %d, want 16: 2 noncombat red damage below power 4 becomes 4", got)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Damage && ev.Player == 1 && ev.Amount != 4 {
			t.Fatalf("logged damage = %d, want rewritten amount 4", ev.Amount)
		}
	}

	e.combatDamaging = true
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.combatDamaging = false
	if got := e.G.Players[1].Life; got != 14 {
		t.Fatalf("life = %d, want 14: combat damage is not Ojer Axonil's replacement", got)
	}
}

// TestTheSoundOfDrumsDoublesCombatDamage uses the DIRECT ReplaceCount$ form
// (VarValue$ ReplaceCount$DamageAmount/Twice, no SVar indirection) on the
// enchanted creature's combat damage.
func TestTheSoundOfDrumsDoublesCombatDamage(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	aura := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "The Sound of Drums"))
	bear := onBoard(t, e, 0, "Name:Bear\nManaCost:G\nTypes:Creature\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.Attach, Obj: aura, IDs: []state.ObjID{bear}})

	e.damaging = bear
	e.combatDamaging = true
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.combatDamaging = false
	e.damaging = 0
	if got := e.G.Players[1].Life; got != 16 {
		t.Fatalf("life = %d, want 16: the enchanted creature's 2 combat damage doubled", got)
	}
}

// TestHexingSquelcherProtectsYourSpells uses the battlefield-arm counter
// replacement (ValidSA$ Spell.YouCtrl): a counterspell from the opponent
// cannot counter a spell you control, while your own copy of the same
// replacement does not shield the opponent's spells.
func TestPalisadeGiantRedirectsDamageToItself(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	giant := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Palisade Giant"))
	source := onBoard(t, e, 1, "Name:Source\nManaCost:R\nTypes:Creature\nPT:1/1\nOracle:x\n")
	e.damaging = source
	applied := e.emit(events.Event{Kind: events.Damage, Player: 0, Amount: 2})
	e.damaging = 0
	if applied.Kind != events.Damage || applied.Obj != giant {
		t.Fatalf("applied damage = %+v, want damage redirected to Palisade Giant %d", applied, giant)
	}
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("player life = %d, want 20 after redirection", got)
	}
	if got := e.G.Obj(giant).Damage; got != 2 {
		t.Fatalf("Palisade Giant damage = %d, want 2", got)
	}
}

func TestDamageReplacementPropagatesAppliedAmountToLifelink(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Fiery Emancipation"))
	source := onBoard(t, e, 0, "Name:Lifelink Source\nManaCost:R\nTypes:Creature\nPT:1/1\nK:Lifelink\nOracle:x\n")
	sa := &cards.SA{API: "DealDamage", Params: map[string]string{"Defined": "Opponent", "NumDmg": "2"}}
	e.damaging = source
	effects.Resolve(e, &effects.Ctx{Source: source, Controller: 0}, sa)
	e.damaging = 0
	if got := e.G.Players[1].Life; got != 14 {
		t.Fatalf("target life = %d, want 14 after tripled damage", got)
	}
	if got := e.G.Players[0].Life; got != 26 {
		t.Fatalf("lifelink controller life = %d, want 26 from applied damage", got)
	}
}

func TestPreventReplacementSuppressesLifelink(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Blessed Sanctuary"))
	source := onBoard(t, e, 0, "Name:Lifelink Source\nManaCost:R\nTypes:Creature\nPT:1/1\nK:Lifelink\nOracle:x\n")
	sa := &cards.SA{API: "DealDamage", Params: map[string]string{"Defined": "Opponent", "NumDmg": "2"}}
	e.damaging = source
	effects.Resolve(e, &effects.Ctx{Source: source, Controller: 0}, sa)
	e.damaging = 0
	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("protected life = %d, want 20", got)
	}
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("lifelink controller life = %d, want 20 when damage was prevented", got)
	}
}

func TestDamageReplacementPropagatesAppliedCommanderDamage(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, _ := commanderGame(t, commanderDamageSeed, FormatCommander, 40,
		[][]string{{cmdCreature(7)}, {}})
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Fiery Emancipation"))
	cmd := fieldCommander(t, e, 0, 0)
	swing(t, e, cmd)
	if got := e.G.Players[1].CmdDamage[0]; got != 21 {
		t.Fatalf("commander damage = %d, want applied 21", got)
	}
	if !e.G.Players[1].Lost {
		t.Fatal("defender survived 21 applied commander damage")
	}
}

func TestConditionalCounterReplacementUsesPaidX(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		x             int32
		wantCountered bool
	}{{0, true}, {5, false}} {
		t.Run(string(rune('0'+tc.x)), func(t *testing.T) {
			e := newSeats(t, 2)
			banefire := e.G.AddObject(mustCorpusCard(t, reg, "Banefire"), 0)
			banefire.Zone = state.ZStack
			banefire.X = tc.x
			e.G.SetZone(state.ZStack, 0, []state.ObjID{banefire.ID})
			e.G.Stack = []state.ObjID{banefire.ID}
			cs := mustCorpusCard(t, reg, "Counterspell").Faces[0].SpellAbility()
			effects.Resolve(e, &effects.Ctx{Source: onBoard(t, e, 1, "Name:Counter Source\nTypes:Creature\nPT:1/1\nOracle:x\n"), Controller: 1,
				Targets: []state.Target{{Obj: banefire.ID}}}, cs)
			countered := e.G.Obj(banefire.ID).Zone != state.ZStack
			if countered != tc.wantCountered {
				t.Fatalf("Banefire X=%d countered=%v, want %v", tc.x, countered, tc.wantCountered)
			}
		})
	}
}

func TestGuileReplacesCounterWithExile(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Guile"))
	target := e.G.AddObject(mustCorpusCard(t, reg, "Banefire"), 1)
	target.Zone = state.ZStack
	e.G.SetZone(state.ZStack, 1, []state.ObjID{target.ID})
	e.G.Stack = []state.ObjID{target.ID}
	counterSource := e.G.AddObject(mustCorpusCard(t, reg, "Counterspell"), 0)
	counterSource.Zone = state.ZStack
	e.G.SetZone(state.ZStack, 0, []state.ObjID{counterSource.ID})
	e.G.Stack = append(e.G.Stack, counterSource.ID)
	effects.Resolve(e, &effects.Ctx{Source: counterSource.ID, Controller: 0,
		Targets: []state.Target{{Obj: target.ID}}}, counterSource.Face().SpellAbility())
	if got := e.G.Obj(target.ID).Zone; got != state.ZExile {
		t.Fatalf("Guile replacement put countered spell in %s, want exile", got)
	}
}

func TestInactiveDemonfireDoesNotForbidPrevention(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	demonfire := e.G.AddObject(mustCorpusCard(t, reg, "Demonfire"), 0)
	demonfire.Zone = state.ZStack
	demonfire.X = 2
	e.G.SetZone(state.ZStack, 0, []state.ObjID{demonfire.ID})
	e.G.Stack = []state.ObjID{demonfire.ID}
	// Hellbent is false: its controller has a card in hand.
	hand := e.G.AddObject(card(t, "Name:Card in Hand\nTypes:Sorcery\nOracle:x\n"), 0)
	hand.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, []state.ObjID{hand.ID})
	target := onBoard(t, e, 1, "Name:Protected\nTypes:Creature\nPT:1/3\nK:Protection from red\nOracle:x\n")
	e.damaging = demonfire.ID
	e.emit(events.Event{Kind: events.Damage, Obj: target, Amount: 2})
	e.damaging = 0
	if got := e.G.Obj(target).Damage; got != 0 {
		t.Fatalf("damage = %d, want 0: inactive Demonfire hellbent must not bypass prevention", got)
	}
}

func TestEffectCreatedCantPreventDamage(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	skullcrack := e.G.AddObject(mustCorpusCard(t, reg, "Skullcrack"), 0)
	skullcrack.Zone = state.ZStack
	e.G.SetZone(state.ZStack, 0, []state.ObjID{skullcrack.ID})
	e.G.Stack = []state.ObjID{skullcrack.ID}
	sa := skullcrack.Face().SpellAbility()
	// Resolve only the Effect head; its chained damage is unrelated here.
	head := *sa
	head.Sub = nil
	effects.Resolve(e, &effects.Ctx{Source: skullcrack.ID, Controller: 0, SVars: skullcrack.Face().SVars}, &head)
	target := onBoard(t, e, 1, "Name:Protected\nTypes:Creature\nPT:1/3\nK:Protection from red\nOracle:x\n")
	source := onBoard(t, e, 0, "Name:Red Source\nManaCost:R\nTypes:Creature\nPT:1/1\nOracle:x\n")
	e.damaging = source
	e.emit(events.Event{Kind: events.Damage, Obj: target, Amount: 2})
	e.damaging = 0
	if got := e.G.Obj(target).Damage; got != 2 {
		t.Fatalf("damage = %d, want 2 while Skullcrack's Effect forbids prevention", got)
	}
}

func TestCompetingDamageReplacementsAskAndRecompute(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name        string
		chooseFiery bool
		want        int32
	}{{"triple then plus", true, 8}, {"plus then triple", false, 12}} {
		t.Run(tc.name, func(t *testing.T) {
			e := newSeats(t, 2)
			// Drive this event outside an existing priority window so the
			// replacement decision can become the pending decision immediately.
			e.pending = nil
			fiery := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Fiery Emancipation"))
			plus := onBoard(t, e, 0, "Name:Plus Two\nTypes:Enchantment\n"+
				"R:Event$ DamageDone | ActiveZones$ Battlefield | ValidSource$ Card.YouCtrl | ValidTarget$ Player | ReplaceWith$ D\n"+
				"SVar:D:DB$ ReplaceEffect | VarName$ DamageAmount | VarValue$ X\n"+
				"SVar:X:ReplaceCount$DamageAmount/Plus.2\nOracle:x\n")
			source := onBoard(t, e, 0, "Name:Red Source\nManaCost:R\nTypes:Creature\nPT:1/1\nOracle:x\n")
			e.damaging = source
			got := e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
			e.damaging = 0
			if got.Kind == events.Damage || e.Pending() == nil || e.Pending().Kind != decision.KReplacement {
				t.Fatalf("damage was not parked for a replacement choice: event=%v pending=%v", got.Kind, e.Pending())
			}
			wantObj := plus
			if tc.chooseFiery {
				wantObj = fiery
			}
			d := e.Pending()
			idx := -1
			for _, o := range d.Options {
				if o.Obj == wantObj {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("replacement source %d absent from options %+v", wantObj, d.Options)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
				t.Fatal(err)
			}
			if got := e.G.Players[1].Life; got != 20-tc.want {
				t.Fatalf("life = %d, want %d after %d damage", got, 20-tc.want, tc.want)
			}
		})
	}
}

func TestHexingSquelcherProtectsYourSpells(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Hexing Squelcher"))
	mine := e.G.AddObject(mustCorpusCard(t, reg, "Counterspell"), 0)
	mine.Zone = state.ZStack
	e.G.SetZone(state.ZStack, 0, []state.ObjID{mine.ID})
	e.G.Stack = []state.ObjID{mine.ID}

	theirs := e.G.AddObject(mustCorpusCard(t, reg, "Counterspell"), 1)
	theirs.Zone = state.ZStack
	e.G.SetZone(state.ZStack, 1, []state.ObjID{theirs.ID})
	e.G.Stack = append(e.G.Stack, theirs.ID)

	cs := mustCorpusCard(t, reg, "Counterspell").Faces[0].SpellAbility()
	effects.Resolve(e, &effects.Ctx{Controller: 1, Targets: []state.Target{{Obj: mine.ID}}}, cs)
	if got := e.G.Obj(mine.ID).Zone; got != state.ZStack {
		t.Fatalf("your spell left the stack (zone %s): Hexing Squelcher forbids countering it", got)
	}
	effects.Resolve(e, &effects.Ctx{Controller: 0, Targets: []state.Target{{Obj: theirs.ID}}}, cs)
	if got := e.G.Obj(theirs.ID).Zone; got == state.ZStack {
		t.Fatalf("the opponent's spell stayed on the stack: the replacement only shields YOUR spells")
	}
}
