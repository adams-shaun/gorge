// Package vanilla is a hand-written kernel card table for phase-1 tests:
// basic lands, vanilla and keyword creatures across keyword combinations,
// burn, removal, a pump, a token maker, a draw spell, a counter spell, a
// +1/+1-counter spell, ETB- and dies-trigger creatures, and four synthetic
// 40-card decks built from them.
package vanilla

import "github.com/adams-shaun/gorge/dirac/kop"

// Card ids (indices into Table.Cards).
const (
	Plains kop.CardID = iota
	Island
	Swamp
	Mountain
	Forest
	SavannahLions
	GrizzlyBears
	HillGiant
	WindDrake
	GiantSpider
	SerraAngel
	VampireNighthawk
	TyphoidRats
	RagingGoblin
	YouthfulKnight
	ColossalDreadmaw
	FencingAce
	BoggartBrute
	TundraWall
	LlanowarElves
	JeskaiStudent
	LightningBolt
	Murder
	GiantGrowth
	RaiseTheAlarm
	Divination
	Battlegrowth
	ElvishVisionary
	PerilousMyr
	Cancel
	MindRot
	Opt
	SoldierToken
	GloriousAnthem
	TestCharm
	Revitalize
	PreyUpon
	PressurePoint
	EmberWalker         // planeswalker, loyalty 3: +1: 1 damage to any target; -3: destroy target creature
	LegendaryBear       // legendary 2/2
	DrillKnight         // 2/2; {1}: +1/+0 until end of turn, activate only once each turn
	ThrillOfPossibility // {1}{R} instant; additional cost: discard a card; draw two
	HungryGhoul         // {1}{B} 2/2; {1}, sacrifice another creature: put a +1/+1 counter on it
	Shock               // {R} instant: 2 damage to any target
	TreasureToken       // token: {T}, sacrifice this: add one mana of any colour
	EatenAlive          // {B} sorcery; additional cost: sacrifice a creature or pay {3}{B}; exile target creature
	BurstLightning      // {R} instant, kicker {4}: 2 damage to any target, 4 if kicked
	ThinkTwice          // {1}{U} instant, flashback {2}{U}: draw a card
	ProdigalPyromancer  // {2}{R} 1/1; {T}: 1 damage to any target
	NumCards
)

// Filter ids.
const (
	fAny kop.FilterID = iota
	fCreature
	fCreatureYou
	fCreatureOpp
	fSpell
	fCreatureYouOther
	fPlayerAny
	fNoncreature
)

func mc(generic int, pips ...int) kop.ManaCost {
	var m kop.ManaCost
	m.Generic = uint8(generic)
	for _, p := range pips {
		m.Pips[p]++
	}
	return m
}

func land(name string, col kop.ColorMask) kop.CardDef {
	return kop.CardDef{Name: name, Types: kop.TLand | kop.TBasic, Timing: kop.TimingLand,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: col, ManaN: 1}}}
}

func creature(name string, cost kop.ManaCost, col kop.ColorMask, p, t int8, kw kop.KwMask) kop.CardDef {
	return kop.CardDef{Name: name, Cost: cost, Types: kop.TCreature, Colors: col, Power: p, Toughness: t, Keywords: kw}
}

func spell(name string, cost kop.ManaCost, col kop.ColorMask, instant bool, ab kop.Ability) kop.CardDef {
	t := kop.TSorcery
	tm := kop.TimingSorcery
	if instant {
		t, tm = kop.TInstant, kop.TimingInstant
	}
	ab.Kind = kop.AbSpell
	return kop.CardDef{Name: name, Cost: cost, Types: t, Colors: col, Timing: tm, Abilities: []kop.Ability{ab}}
}

func tgt(kind kop.TargetKind, f kop.FilterID) [kop.MaxTargets]kop.TargetSpec {
	return [kop.MaxTargets]kop.TargetSpec{{Kind: kind, Filter: f}}
}

// NewTable builds the vanilla table (a fresh value each call).
func NewTable() *kop.Table {
	W, U, B, R, G := kop.W, kop.U, kop.B, kop.R, kop.G
	t := &kop.Table{Name: "vanilla"}
	t.Filters = []kop.Filter{
		fAny:              {},
		fCreature:         {Types: kop.TCreature},
		fCreatureYou:      {Types: kop.TCreature, Ctrl: kop.CtrlYou},
		fCreatureOpp:      {Types: kop.TCreature, Ctrl: kop.CtrlOpp},
		fSpell:            {Flags: kop.FSpell},
		fCreatureYouOther: {Types: kop.TCreature, Ctrl: kop.CtrlYou, Flags: kop.FOther},
		fPlayerAny:        {},
		fNoncreature:      {NotTypes: kop.TCreature},
	}
	c := make([]kop.CardDef, NumCards)
	c[Plains] = land("Plains", kop.ColW)
	c[Island] = land("Island", kop.ColU)
	c[Swamp] = land("Swamp", kop.ColB)
	c[Mountain] = land("Mountain", kop.ColR)
	c[Forest] = land("Forest", kop.ColG)
	c[SavannahLions] = creature("Savannah Lions", mc(0, W), kop.ColW, 2, 1, 0)
	c[GrizzlyBears] = creature("Grizzly Bears", mc(1, G), kop.ColG, 2, 2, 0)
	c[HillGiant] = creature("Hill Giant", mc(3, R), kop.ColR, 3, 3, 0)
	c[WindDrake] = creature("Wind Drake", mc(2, U), kop.ColU, 2, 2, kop.KwFlying)
	c[GiantSpider] = creature("Giant Spider", mc(3, G), kop.ColG, 2, 4, kop.KwReach)
	c[SerraAngel] = creature("Serra Angel", mc(3, W, W), kop.ColW, 4, 4, kop.KwFlying|kop.KwVigilance)
	c[VampireNighthawk] = creature("Vampire Nighthawk", mc(1, B, B), kop.ColB, 2, 3, kop.KwFlying|kop.KwDeathtouch|kop.KwLifelink)
	c[TyphoidRats] = creature("Typhoid Rats", mc(0, B), kop.ColB, 1, 1, kop.KwDeathtouch)
	c[RagingGoblin] = creature("Raging Goblin", mc(0, R), kop.ColR, 1, 1, kop.KwHaste)
	c[YouthfulKnight] = creature("Youthful Knight", mc(1, W), kop.ColW, 2, 1, kop.KwFirstStrike)
	c[ColossalDreadmaw] = creature("Colossal Dreadmaw", mc(4, G, G), kop.ColG, 6, 6, kop.KwTrample)
	c[FencingAce] = creature("Fencing Ace", mc(1, W), kop.ColW, 1, 1, kop.KwDoubleStrike)
	c[BoggartBrute] = creature("Boggart Brute", mc(2, R), kop.ColR, 3, 2, kop.KwMenace)
	c[TundraWall] = creature("Tundra Wall", mc(1, W), kop.ColW, 0, 4, kop.KwDefender)
	c[LlanowarElves] = creature("Llanowar Elves", mc(0, G), kop.ColG, 1, 1, 0)
	c[LlanowarElves].Abilities = []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1}}
	c[JeskaiStudent] = creature("Jeskai Student", mc(1, W), kop.ColW, 1, 3, kop.KwProwess)
	// prowess as the triggered ability gorge expands it to
	c[JeskaiStudent].Abilities = []kop.Ability{{Kind: kop.AbTriggered,
		Trig: kop.Trigger{Event: kop.TrigSpellCast, Filter: fNoncreature, Whose: kop.WhoseYou},
		Prog: []kop.Instr{{Op: kop.OpPumpUntilEOT, Obj: kop.RefSelf, A: kop.Const(1), B: kop.Const(1)}}}}
	c[LightningBolt] = spell("Lightning Bolt", mc(0, R), kop.ColR, true, kop.Ability{
		Targets: tgt(kop.TgtAny, fAny), NTargets: 1,
		Prog: []kop.Instr{{Op: kop.OpDealDamage, Obj: kop.RefTarget0, A: kop.Const(3)}}})
	c[Murder] = spell("Murder", mc(1, B, B), kop.ColB, true, kop.Ability{
		Targets: tgt(kop.TgtObject, fCreature), NTargets: 1,
		Prog: []kop.Instr{{Op: kop.OpDestroy, Obj: kop.RefTarget0}}})
	c[GiantGrowth] = spell("Giant Growth", mc(0, G), kop.ColG, true, kop.Ability{
		Targets: tgt(kop.TgtObject, fCreature), NTargets: 1,
		Prog: []kop.Instr{{Op: kop.OpPumpUntilEOT, Obj: kop.RefTarget0, A: kop.Const(3), B: kop.Const(3)}}})
	c[RaiseTheAlarm] = spell("Raise the Alarm", mc(1, W), kop.ColW, true, kop.Ability{
		Prog: []kop.Instr{{Op: kop.OpCreateToken, Obj: kop.RefController, A: kop.Const(2), C: int16(SoldierToken)}}})
	c[Divination] = spell("Divination", mc(2, U), kop.ColU, false, kop.Ability{
		Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(2)}}})
	c[Battlegrowth] = spell("Battlegrowth", mc(0, G), kop.ColG, true, kop.Ability{
		Targets: tgt(kop.TgtObject, fCreature), NTargets: 1,
		Prog: []kop.Instr{{Op: kop.OpPutCounter, Obj: kop.RefTarget0, A: kop.Const(1), C: int16(kop.CtrP1P1)}}})
	c[ElvishVisionary] = creature("Elvish Visionary", mc(1, G), kop.ColG, 1, 1, 0)
	c[ElvishVisionary].Abilities = []kop.Ability{{Kind: kop.AbTriggered,
		Trig: kop.Trigger{Event: kop.TrigETB, Flags: kop.TfSelf},
		Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)}}}}
	c[PerilousMyr] = kop.CardDef{Name: "Perilous Myr", Cost: mc(2), Types: kop.TArtifact | kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig:    kop.Trigger{Event: kop.TrigDies, Flags: kop.TfSelf},
			Targets: tgt(kop.TgtAny, fAny), NTargets: 1,
			Prog: []kop.Instr{{Op: kop.OpDealDamage, Obj: kop.RefTarget0, A: kop.Const(2)}}}}}
	c[Cancel] = spell("Cancel", mc(1, U, U), kop.ColU, true, kop.Ability{
		Targets: tgt(kop.TgtSpell, fAny), NTargets: 1,
		Prog: []kop.Instr{{Op: kop.OpCounter, Obj: kop.RefTarget0}}})
	c[MindRot] = spell("Mind Rot", mc(2, B), kop.ColB, false, kop.Ability{
		Targets: tgt(kop.TgtPlayer, fPlayerAny), NTargets: 1,
		Prog: []kop.Instr{{Op: kop.OpDiscard, Obj: kop.RefTarget0, A: kop.Const(2), C: kop.ChooseSelf}}})
	c[Opt] = spell("Opt", mc(0, U), kop.ColU, true, kop.Ability{
		Prog: []kop.Instr{
			{Op: kop.OpScry, Obj: kop.RefController, A: kop.Const(1)},
			{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)}}})
	c[SoldierToken] = kop.CardDef{Name: "Soldier Token", Types: kop.TCreature | kop.TToken, Colors: kop.ColW, Power: 1, Toughness: 1}
	c[GloriousAnthem] = kop.CardDef{Name: "Glorious Anthem", Cost: mc(1, W, W), Types: kop.TEnchantment, Colors: kop.ColW,
		Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: fCreatureYou, A: 1, B: 1}}}}
	c[TestCharm] = spell("Test Charm", mc(0, R), kop.ColR, true, kop.Ability{
		NModes: 2, ModesPick: 1,
		Targets:  [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: fCreature}, {Kind: kop.TgtPlayer, Filter: fPlayerAny}},
		NTargets: 2, TgtModes: [kop.MaxTargets]uint8{1, 2},
		Prog: []kop.Instr{
			{Op: kop.OpCharm, A: kop.Const(1), C: 0},
			{Op: kop.OpDealDamage, Obj: kop.RefTarget0, A: kop.Const(2)},
			{Op: kop.OpCharm, A: kop.Const(1), C: 1},
			{Op: kop.OpLoseLife, Obj: kop.RefTarget1, A: kop.Const(2)}}})
	c[Revitalize] = spell("Revitalize", mc(1, W), kop.ColW, true, kop.Ability{
		Prog: []kop.Instr{
			{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(3)},
			{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)}}})
	c[PreyUpon] = spell("Prey Upon", mc(0, G), kop.ColG, false, kop.Ability{
		Targets:  [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: fCreatureYou}, {Kind: kop.TgtObject, Filter: fCreatureOpp}},
		NTargets: 2,
		Prog:     []kop.Instr{{Op: kop.OpFight, Obj: kop.RefTarget0, C: int16(kop.RefTarget1)}}})
	c[PressurePoint] = spell("Pressure Point", mc(1, W), kop.ColW, true, kop.Ability{
		Targets: tgt(kop.TgtObject, fCreature), NTargets: 1,
		Prog: []kop.Instr{
			{Op: kop.OpTap, Obj: kop.RefTarget0},
			{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)}}})
	c[EmberWalker] = kop.CardDef{Name: "Ember Walker", Cost: mc(2, R, R), Types: kop.TPlaneswalker | kop.TLegendary, Colors: kop.ColR, Loyalty: 3,
		Abilities: []kop.Ability{
			{Kind: kop.AbActivated, Cost: kop.Cost{Flags: kop.CostLoyalty | kop.CostAddCtr, N: 1, Ctr: kop.CtrLoyalty},
				Targets: tgt(kop.TgtAny, fAny), NTargets: 1,
				Prog: []kop.Instr{{Op: kop.OpDealDamage, Obj: kop.RefTarget0, A: kop.Const(1)}}},
			{Kind: kop.AbActivated, Cost: kop.Cost{Flags: kop.CostLoyalty | kop.CostRemoveCtr, N: 3, Ctr: kop.CtrLoyalty},
				Targets: tgt(kop.TgtObject, fCreature), NTargets: 1,
				Prog: []kop.Instr{{Op: kop.OpDestroy, Obj: kop.RefTarget0}}},
		}}
	c[LegendaryBear] = creature("Legendary Bear", mc(1, G), kop.ColG, 2, 2, 0)
	c[LegendaryBear].Types |= kop.TLegendary
	c[DrillKnight] = creature("Drill Knight", mc(1, W), kop.ColW, 2, 2, 0)
	c[DrillKnight].Abilities = []kop.Ability{{Kind: kop.AbActivated,
		Cost: kop.Cost{Mana: mc(1), Flags: kop.CostOncePerTurn},
		Prog: []kop.Instr{{Op: kop.OpPumpUntilEOT, Obj: kop.RefSelf, A: kop.Const(1)}}}}
	c[ThrillOfPossibility] = spell("Thrill of Possibility", mc(1, R), kop.ColR, true, kop.Ability{
		Cost: kop.Cost{Flags: kop.CostDiscard, N: 1},
		Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(2)}}})
	c[HungryGhoul] = creature("Hungry Ghoul", mc(1, B), kop.ColB, 2, 2, 0)
	c[HungryGhoul].Abilities = []kop.Ability{{Kind: kop.AbActivated,
		Cost: kop.Cost{Mana: mc(1), Flags: kop.CostSac, N: 1, Filter: fCreatureYouOther},
		Prog: []kop.Instr{{Op: kop.OpPutCounter, Obj: kop.RefSelf, A: kop.Const(1), C: int16(kop.CtrP1P1)}}}}
	c[Shock] = spell("Shock", mc(0, R), kop.ColR, true, kop.Ability{
		Targets: tgt(kop.TgtAny, fAny), NTargets: 1,
		Prog: []kop.Instr{{Op: kop.OpDealDamage, Obj: kop.RefTarget0, A: kop.Const(2)}}})
	c[TreasureToken] = kop.CardDef{Name: "Treasure Token", Types: kop.TArtifact | kop.TToken,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap | kop.CostSacSelf}, ManaColors: kop.ColAny, ManaN: 1}}}
	c[EatenAlive] = spell("Eaten Alive", mc(0, B), kop.ColB, false, kop.Ability{
		Cost:    kop.Cost{Flags: kop.CostSac, N: 1, Filter: fCreatureYou},
		Targets: tgt(kop.TgtObject, fCreature), NTargets: 1,
		Prog: []kop.Instr{{Op: kop.OpExile, Obj: kop.RefTarget0}}})
	c[EatenAlive].AltAddCost = kop.Cost{Mana: mc(3, B)}
	c[EatenAlive].HasAltAdd = true
	c[BurstLightning] = spell("Burst Lightning", mc(0, R), kop.ColR, true, kop.Ability{
		Targets: tgt(kop.TgtAny, fAny), NTargets: 1,
		Prog: []kop.Instr{
			{Op: kop.OpIfKicked, A: kop.Const(2)},
			{Op: kop.OpDealDamage, Obj: kop.RefTarget0, A: kop.Const(4)},
			{Op: kop.OpSkipIf, C: 2, B: kop.Const(1)}, // skip the unkicked damage
			{Op: kop.OpDealDamage, Obj: kop.RefTarget0, A: kop.Const(2)}}})
	c[BurstLightning].Keywords |= kop.KwKicker
	c[BurstLightning].Kicker = mc(4)
	c[ThinkTwice] = spell("Think Twice", mc(1, U), kop.ColU, true, kop.Ability{
		Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)}}})
	c[ThinkTwice].Keywords |= kop.KwFlashback
	c[ThinkTwice].AltCost = mc(2, U)
	c[ProdigalPyromancer] = creature("Prodigal Pyromancer", mc(2, R), kop.ColR, 1, 1, 0)
	c[ProdigalPyromancer].Abilities = []kop.Ability{{Kind: kop.AbActivated,
		Cost:    kop.Cost{Flags: kop.CostTap},
		Targets: tgt(kop.TgtAny, fAny), NTargets: 1,
		Prog: []kop.Instr{{Op: kop.OpDealDamage, Obj: kop.RefTarget0, A: kop.Const(1)}}}}
	t.Cards = c
	return t
}

// Table is the shared vanilla table.
var Table = NewTable()

func deck(parts ...any) []kop.CardID {
	var d []kop.CardID
	for i := 0; i < len(parts); i += 2 {
		n := parts[i].(int)
		id := parts[i+1].(kop.CardID)
		for j := 0; j < n; j++ {
			d = append(d, id)
		}
	}
	if len(d) != 40 {
		panic("vanilla: deck is not 40 cards")
	}
	return d
}

// Decks are four synthetic 40-card decks: RG aggro, WB midrange, UW fliers,
// BR removal.
var Decks = [5][]kop.CardID{
	deck(8, Mountain, 9, Forest, 3, GrizzlyBears, 2, HillGiant, 1, ColossalDreadmaw, 2, LlanowarElves,
		2, RagingGoblin, 2, BoggartBrute, 1, GiantSpider, 2, ElvishVisionary, 3, LightningBolt, 2, GiantGrowth,
		1, PreyUpon, 2, Battlegrowth),
	deck(8, Plains, 9, Swamp, 3, SavannahLions, 1, SerraAngel, 2, VampireNighthawk, 2, TyphoidRats,
		2, YouthfulKnight, 1, FencingAce, 1, TundraWall, 2, PerilousMyr, 2, Murder, 2, RaiseTheAlarm,
		1, MindRot, 1, Revitalize, 1, GloriousAnthem, 2, PressurePoint),
	deck(8, Island, 9, Plains, 3, WindDrake, 2, SerraAngel, 2, JeskaiStudent, 2, SavannahLions,
		2, YouthfulKnight, 1, TundraWall, 2, Cancel, 2, Divination, 3, Opt, 2, RaiseTheAlarm,
		1, PressurePoint, 1, Revitalize),
	deck(8, Swamp, 9, Mountain, 2, TyphoidRats, 2, VampireNighthawk, 2, RagingGoblin, 3, BoggartBrute,
		3, HillGiant, 2, PerilousMyr, 3, LightningBolt, 2, Murder, 2, MindRot, 2, Shock),
	deck(8, Island, 9, Mountain, 3, BurstLightning, 3, ThinkTwice, 3, ProdigalPyromancer, 3, WindDrake,
		3, HillGiant, 2, Shock, 2, Opt, 2, Cancel, 2, RagingGoblin),
}
