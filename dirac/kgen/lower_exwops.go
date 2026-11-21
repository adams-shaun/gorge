package kgen

import "strings"

// exwOpKind classifies one LIR op name for exwOwnerMover (lower_exw.go): can
// the op move a card into exile in a way gorge's recording rule stamps as
// exiled with the source? The gate is default-refuse: an op that is not
// classified exwNotExile refuses on a card carrying the RefExiledWithOwner
// player ref, so a new or overlooked op can only be over-refused, never
// silently let through. TestExwOpTableCoversEveryOp walks every `Op: "X"`
// literal the lowering emits and fails until the op has a row here.
type exwOpKind uint8

const (
	// exwUnvetted: not proven free of an exile path (an exile folded into the
	// op's own lowering, a delayed or granted exile, a zone move with a zone
	// operand); refused under the ref.
	exwUnvetted exwOpKind = iota
	// exwNotExile: never moves a card into exile.
	exwNotExile
	// exwMover: an exile mover; exwOwnerMover bounds its count.
	exwMover
)

// exwOpTable is the classification. Anything absent is exwUnvetted.
var exwOpTable = exwBuildOpTable(
	// exwNotExile
	`
		AddMana AddOrRemoveCounter AddPhase AddTurn Amass AtkPWGuard Attach AttachObj AttacksIfAble
		BecomeCopy BecomeMonarch Bite BlocksIfAble Bolster Branch ChangeCombatants ChangeSpeed
		ChangeTargets ChangeText Charm ChooseCard ChooseCardEnter ChooseColor ChooseColorRes ChooseEachType
		ChooseName ChooseNumber ChooseNumberMid ChoosePlayer ChoosePlayerEnter ChoosePlayerMid
		ChoosePlayerRes ChooseSource ChooseTargets ChooseType Clash ClassLevelUp ClearChosen ClearCounters
		ClearImprinted ClearNamedCard ClearRemembered Cloak ColorGrant CopyAbility CopySpell CounterLose
		CounterPlusEOT CreateEmblem DamageAll DealDamage DealDamageDivided DealDamageFrom Destroy
		DestroyAll Discard DmgMark DmgRedirect DoubleCounters DoubleCountersAll DrainMana Draw DrawUpto
		EachDamage EffEtbCounters EffFinalityCtr EnterAttacking EnterCtrPick EnterTapped EnterUntapped
		EnterWithCounters ExHaste ExchangeControl Extort Fight Fog ForgetImprinted GainControl
		GainControlEOT GainEnergy GenericChoice Goad GrantKeyword GrantKwAll IfPaid LockActs LookAck
		LoseGame ManaChoice Manifest ManifestDread MayAsk Monstrosity MoveCounter MustBeBlocked
		MustBlockAttacker NameCard NumTimes Override OverrideAll Peek Phases Poison PreventColor
		PreventDamageScope PreventNext Proliferate PumpAll PumpAllEach PumpUntilEOT PutCounter
		PutCounterAll PutCounterChoice PutCounterExisting PutCountersFrom Radiation RememberAll
		RememberImprinted RememberPlayer RememberSearcher RemoveCounter RemoveCounterAll RemoveCounterAny
		RemoveFromCombat Renown Repeat RepeatEach RepeatEachCard RepeatOptional RepeatWhile RingTemptsYou
		RollDie Sacrifice SacrificeAll Scry SetLife Shuffle SkipIf SkipNextUntap SkipPhase SkipUntap
		Surveil Suspect TakeInitiative TapAll TapOrUntap TapOrUntapAll TokenPT Transform TurnFaceUp
		UnlessPaid UnlessSwitched UnlockDoor UntapAll UntapChoose Venture Ward	`,
	// exwMover
	`
		Airbend ChangeZoneAll ChooseMove Dig Exile ExileEmbRem ExileExMark ExileUntilLeaves Hideaway Search
		StackExileMark	`,
	// exwUnvetted (listed so the census test can tell "reviewed, refused"
	// from "never looked at"; Vote exiles every most-voted permanent, Discover
	// exiles the revealed cards, DigImprint/DigUntil/ExileLandChoose and the
	// delayed or granted exiles can reach exile beside the listed movers)
	`
		AegisClone AsgnGrant AtEOT Blight Bounce BreachChoose BreachReturn CastFree CloneCopy Connive
		ConniveCounters CopyAllEach CopyAsToken CopyAsTokenCtr CopyExiledWith CopyNamed CopyPicked Counter
		CounterAll CreateToken Cumulative DelayLeaveReturn DelayedTrigger Demonstrate DigBottom DigCounters
		DigImprint DigUntil Discover DistCounters DividedCounters Earthbend Empower Encore EndTurn Endure
		ExileLandChoose Exploit Explore ExtJoinForces GiveGift GrantEmbalmOwnCost GrantEncore GrantExile
		GrantPanharm ImprintTriggered Incubate KwMark LibPlace LibSweep MayPlay MayPlayGrave MayShuffleMove
		Mill Myriad Ninjutsu PlotGrant Prepare Rearrange Recruit ReflexiveTrigger Regenerate RemBottom
		ResolveDamageMap Return SacrificeDelayed Saddle SearchCounters Seek SpellToLib Support SweepFrom
		ToGraveyard ToLibrary ToLibraryPair TurnRepl TwoPiles TwoPilesSac Unearth Vote	`,
)

func exwBuildOpTable(notExile, mover, unvetted string) map[string]exwOpKind {
	t := map[string]exwOpKind{}
	for _, f := range []struct {
		names string
		k     exwOpKind
	}{{notExile, exwNotExile}, {mover, exwMover}, {unvetted, exwUnvetted}} {
		for _, n := range strings.Fields(f.names) {
			t[n] = f.k
		}
	}
	return t
}
