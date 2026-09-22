package state

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
)

// Counter is one counter kind on an object. A slice, not a map: it clones by
// copy and iterates in a fixed order.
type Counter struct {
	Kind string
	N    int32
}

// Target is a chosen target. Exactly one of Obj and Player is meaningful.
type Target struct {
	Obj      ObjID
	Player   PlayerID
	IsPlayer bool
}

// GoadEffect is one independently-lived CR 701.38 goad relationship.
// Source and Duration preserve the condition Forge attached to the effect;
// Controller is the target creature's controller when an AsLongAsControl
// relationship began. All fields are reconstructed by events.Apply.
type GoadEffect struct {
	Player     PlayerID
	Source     ObjID
	Controller PlayerID
	Duration   string
}

// SacrificedInfo is the last-known-information snapshot of an object at the
// instant it was sacrificed, captured before the sacrifice's zone change
// (CR 608.2g "last known information"): the object is in the graveyard by
// the time a resolving ability reads "the sacrificed creature's power"
// (Ghoulcaller Gisa), and Move resets its counters while a graveyard object
// has no characteristics from the layer system at all, so the values must be
// captured while the permanent is still on the battlefield. It carries only
// the characteristics the Sacrificed$<Property> SVar heads ask for -- power,
// toughness and mana value -- rather than a full Object clone, since that is
// all the corpus uses this mechanism for.
type SacrificedInfo struct {
	Obj       ObjID
	Power     int32
	Toughness int32
	ManaValue int32
}

// CastFlags bits record how an object was cast. Several can be set at once
// (a spell can be both kicked and cast via flashback), so they are
// OR-combined into one byte rather than modeled as separate bools.
const (
	FlagKicked uint64 = 1 << iota // CR 601.2b: paid an optional additional cost
	FlagSurged
	FlagFlashback
	FlagMiracle
	// Appended below the four original bits, following the enum's own
	// append-only precedent: these mark alternative-cost casts (CR 601.2b
	// records how a spell was cast) read by the ETB/keyword machinery.
	FlagEvoked     // evoke: paid the evoke cost (CR 702)
	FlagDashed     // dash: paid the dash cost (CR 702)
	FlagOverloaded // overload cast (CR 702)
	FlagWarped     // warp cast: exile at next end step, may recast from exile (CR 702)
	// FlagBuyback returns the resolving spell to its owner's hand.
	FlagBuyback
	// FlagHarmonize and FlagSuspend exile the spell after it resolves.
	FlagHarmonize
	FlagSuspend
	// FlagEscaped marks a cast paid for with its Escape cost (CR 702.42a);
	// it survives onto the permanent, where the "sacrifice it unless it
	// escaped" ETB family and the escape-with-counters replacements read it
	// through the Card.Self+escaped spec.
	FlagEscaped
	// FlagMayPlay marks a cast made through a may-play-from-zone grant
	// (CR 401.5); the MayPlayLimit$ once-per-turn cap reads it from the log
	// (rules' mayPlaysThisTurn), the same CastInfo provenance marker the
	// Suspend flag is.
	FlagMayPlay
	// FlagKicked1/FlagKicked2 mark the and/or Kicker's two independent
	// optional costs ("Kicker {G} and/or {1}{U}", Forge's colon-separated
	// two-part Kicker:<a>:<b> line -- the Volver/Battlemage cycle family,
	// 18 corpus files): each records WHICH kicker was paid, so the
	// "Card.Self+kicked 1" / "kicked 2" trigger/replacement specs read the
	// specific part. Any part paid also sets FlagKicked, so every bare
	// "kicked" reader (the Condition$ Kicked gate, the spellScope
	// constraint) keeps meaning "some kicker was paid". Appended after the
	// enum's own append-only precedent.
	FlagKicked1
	FlagKicked2
	// FlagNoCounter marks a spell whose payment spent mana produced by an
	// AddsNoCounter$ mana ability (Cavern of Souls' creature-of-the-chosen
	// type mana, Boseiju's instant-or-sorcery mana): "that spell can't be
	// countered". It rides the pay-time CastInfo provenance marker (rules'
	// payCast ORs it into the same event the X value and mode flags ride) and
	// is read by the Counter primitive through the can't-be-countered gate.
	FlagNoCounter
	// FlagAdventure marks a cast of an Adventure card's Adventure spell face
	// (CR 714.3a); it is the provenance the resolution reader (spellRestZone)
	// uses to exile the spell into the adventure zone instead of the
	// graveyard. The zone's own provenance -- "this card sits in the
	// adventure zone" -- is log-derived (rules' adventureZoneAvailable)
	// because CastFlags reset on the very stack->exile move the resolution
	// makes. Appended per the enum's own append-only precedent.
	FlagAdventure
	// FlagReplicated marks a cast that paid its Replicate cost at least once
	// (CR 702.55a). The payment COUNT rides the same pay-time CastInfo's
	// Amount into state.Object.ReplicateTimes -- no carrier pairs {X} with
	// Replicate (measured), so the two never compete for the Amount field --
	// and the copy trigger's Count$ReplicatePaid reads it off the cast spell.
	// Appended per the enum's own append-only precedent.
	FlagReplicated
	// FlagConverged marks a cast whose pay-time CastInfo carries CR
	// 107.4f-family converge provenance: the Amount is the number of
	// distinct colours (WUBRG) of mana actually spent to cast the spell,
	// routed into Object.ConvergeColours. Emitted only for faces carrying a
	// Count$Converge SVar (rules/cast.go's faceWantsConverge), so a
	// non-converge cast stays byte-identical. Appended per the enum's own
	// append-only precedent.
	FlagConverged
	// FlagBestowed marks a cast paid for with the card's Bestow cost
	// (CR 702.114a): the spell was an Aura spell with enchant creature, and
	// the permanent that enters attached reverts to a creature when the
	// attachment ends. It is the provenance rules/stack.go's resolution
	// reader uses to substitute the synthesized Aura attach spell for the
	// face's (absent) spell ability. Appended per the enum's own
	// append-only precedent.
	FlagBestowed
	// FlagMultikicked marks a cast whose pay-time CastInfo carries CR
	// 702.43 multikicker provenance: the Amount is the number of times the
	// multikicker cost was paid, routed into Object.TimesKicked. Any
	// multikicked cast also sets FlagKicked (a multikicked cast IS a kicked
	// cast -- the bare predicate and the Condition$ Kicked gate keep
	// matching). Emitted only for a multikicked-mode cast with count > 0 or
	// a plain-Kicker cast mode whose face carries a Count$TimesKicked SVar
	// (rules/cast.go's faceWantsTimesKicked), so unrelated kicked casts stay
	// byte-identical. Appended per the enum's own append-only precedent.
	FlagMultikicked
	// FlagForetold marks a cast paid for with the card's Foretell cost
	// (CR 702.126a). The flag is SET TWICE in a foretold card's life, once
	// by each provenance marker: the {2} face-down hand exile (rules'
	// payCast foretell branch -- the action is not a cast, so only this
	// action ever sets it there) and the later foretell-cost cast from exile
	// (modeFlags("foretell_cast")), so the cast spell and the permanent it
	// becomes both carry it -- the stack->battlefield persistence is what
	// lets an ETB reader (Lupine Harbingers' CheckSVar$ WasForetold) and
	// Count$Foretold read it. An ordinary cast or any other way into exile
	// never sets it. Appended per the enum's own append-only precedent.
	FlagForetold
	// FlagManaSpent marks a cast whose pay-time CastInfo carries the TOTAL
	// mana actually spent to cast it (CR 601.2h's payment; task castprov1's
	// Count$CastTotalManaSpent capture, the FlagConverged pattern: the flag
	// routes the Amount into Object.ManaSpent instead of overwriting X).
	// Only a face whose SVar table reads the count (faceWantsCastSpend)
	// emits the event, so every unrelated cast stays byte-identical.
	// Appended per the enum's own append-only precedent.
	FlagManaSpent
	// FlagManaSnowSpent marks a cast whose pay-time CastInfo carries the
	// SNOW-unit part of the total mana spent to cast it (CR 107.4h; task
	// castfilter1's filtered Count$CastTotalManaSpent Snow capture, the
	// FlagManaSpent pattern: the flag routes the Amount into
	// Object.ManaSnowSpent instead of overwriting X or the unfiltered
	// total). It rides its own trailing pay-time CastInfo immediately after
	// FlagManaSpent's, so the two totals never share an event. Appended per
	// the enum's own append-only precedent.
	FlagManaSnowSpent
	// FlagManaTreasureSpent / FlagManaCaveSpent / FlagManaDesertSpent mark a
	// cast whose pay-time CastInfo carries the TREASURE-/CAVE-/DESERT-sourced
	// part of the total mana spent to cast it (task castfilter2's filtered
	// Count$CastTotalManaSpent <Type> captures, the FlagManaSnowSpent
	// pattern: the flag routes the Amount into its Object field instead of
	// overwriting X, the total or an earlier tag). Each rides its OWN
	// trailing pay-time CastInfo immediately after the previous tag's, so
	// the four totals never share an event, and events.Apply's CastInfo
	// switch checks the NEWEST flag first (Desert, Cave, Treasure, then
	// Snow, then the total) because the emission order is total, snow,
	// Treasure, Cave, Desert and every later event carries all earlier
	// flags. Appended per the enum's own append-only precedent.
	FlagManaTreasureSpent
	FlagManaCaveSpent
	FlagManaDesertSpent
	// FlagReplaceGraveyard marks a cast begun by a DB$ Play SA whose
	// ReplaceGraveyard$ Exile rider says the played spell must not rest in
	// the graveyard: "If that spell would be put into your graveyard this
	// turn, exile it instead" (Goblin Dark-Dwellers). The provenance of a
	// Play SA is cast-time (task replplay1), so the bit ORs into the same
	// pay-time CastInfo every other mode flag rides, and the resolution
	// reader spellRestZone (and the fizzle reader spellFizzleZone) uses it
	// to send the played card to exile. Appended per the enum's own
	// append-only precedent.
	FlagReplaceGraveyard
	// FlagAftermath marks a cast of a Split card's Aftermath alternate face
	// from the graveyard (CR 702.85a; the Flashback convention): the flag is
	// what the resolution reader (spellRestZone) and the fizzle reader
	// (spellFizzleZone) read to exile the card instead of the graveyard,
	// both on resolution and when countered. Appended per the enum's own
	// append-only precedent.
	FlagAftermath
	// FlagConspired marks a cast whose Conspire tap (CR 702.78a) was
	// actually paid: as the spell was cast, two untapped creatures the
	// caster controlled that shared a colour with it were tapped. The flag
	// is the provenance the Conspire keyword expansion's copy trigger reads
	// through Count$Conspired, so a DECLINED/plain cast (no tap paid) emits
	// no flag and resolves exactly like the plain cast. Appended per the
	// enum's own append-only precedent.
	FlagConspired
	// FlagMutated marks a spell cast for its Mutate cost (CR 702.140a). It is
	// the provenance rules/stack.go's resolution reader uses to merge the
	// spell's card into its target instead of moving it to the battlefield as
	// an ordinary permanent. FlagMutatedTop carries CR 702.140b's placement
	// choice (the mutating card goes on TOP of the target); its absence means
	// the mutating card goes UNDER. Both are set by the pay-time CastInfo the
	// cast's mutate-placement answer rides, so the choice is replay-derived.
	// Appended per the enum's own append-only precedent.
	FlagMutated
	FlagMutatedTop
	// FlagFused marks a Fuse cast (CR 702.101b) of a non-Room Split card:
	// one spell paid the combined mana cost of both halves and resolves both
	// halves' spell abilities in sequence. It is the provenance
	// rules/stack.go's resolution reader dispatches on to run BOTH faces
	// rather than the single Face().SpellAbility(). Appended per the enum's
	// own append-only precedent.
	FlagFused
)

// Object is any game object: a card in a zone, a permanent, or a spell on the
// stack. One struct keeps identity stable across zone changes.
type Object struct {
	ID         ObjID
	Card       *cards.Card
	FaceIdx    uint8
	Owner      PlayerID
	Controller PlayerID
	Zone       Zone

	Tapped     bool
	SummonSick bool
	Damage     int32
	Counters   []Counter

	// Zone-entry and damage history are derived exclusively in events.Apply
	// from MoveZone/Draw/PutOnStack and Damage. They persist until the next
	// TurnChange, so filters can answer Forge's ThisTurnEntered* and
	// wasDealtDamageThisTurn vocabulary without scanning a partial event log.
	// EnteredFrom is meaningful only while EnteredThisTurn is true.
	EnteredThisTurn        bool
	EnteredFrom            Zone
	WasDealtDamageThisTurn bool
	// The control-acquisition tuple (AcqTurn, AcqStep) records WHEN this
	// object last came under its current controller's control on the
	// battlefield: stamped by events.Apply on every battlefield ENTRY (Move,
	// a real CR 400.7 new-object zone change — a battlefield→battlefield
	// stay is not a new acquisition) and on every battlefield ControlChange.
	// kw:Echo's intervening-if (CR 702.35a) compares it against the
	// controller's Player.LastUpkeepTurn. Written ONLY inside events.Apply
	// so a live game and a replay derive it identically; Clone copies both
	// with the struct.
	AcqTurn int32
	AcqStep Step
	// ActivatedThisTurn counts the non-mana activated abilities whose
	// activation minted an AbilityPush with this source this turn
	// (events.Apply's AbilityPush case). Mana abilities never mint one (CR
	// 605.3a: they are structurally off the stack), so the count is exactly
	// the repeatable-ability churn a bot policy needs to bound its own
	// loop-shaped activations (Basalt Monolith's "{3}: Untap this artifact"
	// re-enabling its own tap forever) without ever misreading a human's
	// legal unlimited activations -- the count is advice, never a gate.
	ActivatedThisTurn int32

	// AttacksThisTurn counts the DeclareAttackers events this object has
	// attacked in this turn (events.Apply's DeclareAttackers case), reset in
	// TurnChange's per-object loop. Extra combats within one turn share
	// g.Turn and do NOT reset it, so a "attacks for the first time each
	// turn" trigger (rules/trigger_match.go attacksMatches' FirstAttack$)
	// reads count == 1 at fire time -- trigger matching runs on the FOLDED
	// event, so the event's own attack is already counted.
	AttacksThisTurn int32

	// ExertedThisTurn records CR 702.100a's exert election (task exert1):
	// the permanent was exerted this turn. Set only by events.Apply's Exert
	// case; reset in TurnChange's per-object loop (a per-turn fact) and
	// cleared with ExertSkipUntap when the permanent leaves the battlefield
	// (CR 400.7: a new object never carries the old object's exerted
	// status). The filter predicate notExertedThisTurn reads it, which is
	// what Combat Celebrant's IsPresent$ offer gate evaluates.
	ExertedThisTurn bool

	// ExertSkipUntap records CR 702.100b's other lifetime: an exerted
	// creature won't untap during its controller's NEXT untap step, a
	// window that spans the turn boundary (TurnChange fires between the
	// exerting turn's cleanup and the next untap step), so TurnChange does
	// NOT reset it. It is consumed at use, the regeneration-shield
	// expire-at-use precedent: the untap-step scan (rules/turn.go
	// finishUntapStep) skips the untap of a permanent carrying it and emits
	// an Exert event with Amount -1, whose fold clears the flag. Cleared
	// with ExertedThisTurn on leaving the battlefield. Untap effects are
	// unaffected: CR 702.100b names only the untap step, and the skip is
	// implemented in the turn scan, never in effects.TryUntap.
	ExertSkipUntap bool

	// preStackEntry* carries a card's entry history only while it is on the
	// stack. events.Apply captures it before PutOnStack overwrites the public
	// fields, then restores and clears it for CR 733.1's logged reverse move.
	// It is never a second source of truth for a completed zone change.
	PreStackEntryThisTurn bool
	PreStackEntryFrom     Zone
	// PreStackEnteredLen is the entry-list boundary before a proposed cast.
	// A CR 733.1 reverse restores that boundary, removing the proposal and
	// every reversible cost move it made without touching earlier casts.
	PreStackEnteredLen int
	HasPreStackEntry   bool

	// Incarnation advances whenever an object crosses the battlefield
	// boundary. ObjID is stable for the match, but a permanent that leaves and
	// returns is a new object under CR 400.7; delayed and keyword-triggered
	// actions snapshot this value when tied to that permanent incarnation.
	Incarnation uint32

	// Stack-only.
	Ability           *cards.SA
	Source            ObjID
	SourceIncarnation uint32
	Targets           []Target
	// Remembered carries a triggered ability's Ctx.Remembered from the
	// moment it was queued (rules.checkTriggers) through to resolution. An
	// ability object has no Face (Ruling F3) and therefore no card-script
	// route back to "the object that caused this trigger" once it is
	// sitting on the stack, so that has to be data on the object itself,
	// the same way Targets already is for a genuine chosen target. Task 20.
	Remembered []Target

	// Combat-only.
	IsAttacking bool
	Attacking   PlayerID
	BlockedBy   []ObjID
	// EncoreAttackTurn/Defender record "attacks that opponent this turn if
	// able" on an encore token. Zero Turn means no requirement; turns begin
	// at 1, so the zero value is unambiguous.
	EncoreAttackTurn     int32
	EncoreAttackDefender PlayerID

	// Goads holds CR 701.38 attack requirements. Relationships can have the
	// default next-turn lifetime, be permanent, or depend on source/control.
	Goads []GoadEffect

	// Timestamp orders continuous effects. Assigned from Game.Clock whenever
	// the object enters the battlefield.
	Timestamp uint32

	// Cast-time metadata. X and CastFlags matter while this object is a
	// spell on the stack and, once it resolves, on the permanent it becomes
	// (an ETB "if it was kicked" trigger needs to read them off the
	// permanent) -- events.Move resets both when the object leaves the
	// battlefield.
	X         int32
	CastFlags uint64
	// ReplicateTimes is CR 702.55a's count of replicate payments the cast
	// made, carried by the pay-time CastInfo's FlagReplicated Amount (the
	// X-overwrite guard: the flag routes the Amount here instead of into X).
	// It rides the same provenance window as X/CastFlags and resets
	// alongside them in events.Move.
	ReplicateTimes int32
	// ConvergeColours is the number of distinct colours (WUBRG) of mana
	// actually spent to cast the spell (CR 107.4f-family converge), carried
	// by the pay-time CastInfo's FlagConverged Amount. It rides the same
	// provenance window as X/CastFlags and resets alongside them in
	// events.Move; a copy of the spell was never cast and reads 0.
	ConvergeColours int32
	// TimesKicked is CR 702.43's count of times the spell's multikicker cost
	// was paid as it was cast, carried by the pay-time CastInfo's
	// FlagMultikicked Amount (the X-overwrite guard: the flag routes the
	// Amount here instead of into X). A plain-Kicker cast mode's count (1,
	// or 2 for a paid-both two-part Kicker) rides the same flag so the 11
	// legacy Count$TimesKicked carriers read real counts. It rides the same
	// provenance window as X/CastFlags and resets alongside them in
	// events.Move; a COPY of the spell was never kicked and reads 0 (the
	// same reading Count$ReplicatePaid documents).
	TimesKicked int32
	// Conspired is CR 702.78a's provenance that the spell's Conspire tap was
	// paid as it was cast, carried by the pay-time CastInfo's FlagConspired
	// (a bool, not a count: Conspire never copies more than once). It rides
	// the same provenance window as X/CastFlags and resets alongside them in
	// events.Move; a COPY of the spell was never cast and reads false (the
	// same reading Count$ReplicatePaid documents).
	Conspired bool
	// ManaSpent is the TOTAL mana actually spent to cast the spell (CR
	// 601.2h's payment -- the spent delta's pips summed over every slot),
	// carried by the pay-time CastInfo's FlagManaSpent Amount (the
	// X-overwrite guard: the flag routes the Amount here instead of into
	// X). Convoke contributions are taps and Delve exiles cards, so neither
	// rides the delta: a convoke-only cast's total spend is a real zero.
	// It rides the same provenance window as X/CastFlags and resets
	// alongside them in events.Move; a copy of the spell was never cast and
	// a cheated-in permanent reads 0.
	ManaSpent int32
	// ManaSnowSpent is the SNOW-unit part of ManaSpent: how many of the mana
	// units the cast's payment spent were produced by a Snow permanent (CR
	// 107.4h). It is carried by the pay-time CastInfo's FlagManaSnowSpent
	// Amount (the X-overwrite guard: the flag routes the Amount here instead
	// of into X), the filtered Count$CastTotalManaSpent Snow head's
	// provenance. Snow units are consumed alongside their pool slot
	// (resolveManaWith's parallel tally), so this never exceeds ManaSpent for
	// the same slot; a cast that spent no snow mana is a real 0. It rides the
	// same provenance window as ManaSpent and resets alongside it in
	// events.Move; a copy of the spell was never cast and a cheated-in
	// permanent reads 0.
	ManaSnowSpent int32
	// ManaTreasureSpent / ManaCaveSpent / ManaDesertSpent are the
	// TREASURE-/CAVE-/DESERT-sourced parts of ManaSpent: how many of the
	// mana units the cast's payment spent were produced by a permanent of
	// that type (task castfilter2, the ManaSnowSpent pattern). They are
	// carried by the pay-time CastInfo's FlagManaTreasureSpent /
	// FlagManaCaveSpent / FlagManaDesertSpent Amounts (the X-overwrite
	// guard: each flag routes its Amount here instead of into X, the total
	// or an earlier tag). Typed units are consumed after plain ones and
	// before snow (resolveManaWith's takeUnit order), so the typed splits
	// never exceed ManaSpent for the same slot and never overlap the snow
	// split; a cast that spent none of a tag is a real 0. They ride the
	// same provenance window as ManaSpent and reset alongside it in
	// events.Move; a copy of the spell was never cast and a cheated-in
	// permanent reads 0.
	ManaTreasureSpent int32
	ManaCaveSpent     int32
	ManaDesertSpent   int32
	// NotedNumber is the number a trigger's Execute$ body noted onto the
	// CARD (Lupine Harbingers' T:Mode$ ChangesZone | Destination$ Exile
	// trigger executing DB$ Pump | NoteNumber$ Count$YourTurns -- the
	// corpus's one NoteNumber$ carrier). events.NotedNumber carries it,
	// Count$NotedNumber reads it, and it resets with the X/CastFlags window
	// when the permanent leaves the battlefield: the note is made in exile
	// and consumed by the ETB machinery of the cast it later becomes, and a
	// fresh exile re-notes it.
	NotedNumber int32

	// Chosen* record answers to "as this enters/resolves, choose ..."
	// effects: a card name, a creature type, a number, a colour (the
	// K:ETBReplacement ChooseColor family -- Utopia Sprawl, Caged Sun,
	// Quirion Elves; the letter the mana path reads when a Produced$ Chosen
	// ability resolves). Reset alongside X/CastFlags when the object leaves
	// the battlefield.
	ChosenName   string
	ChosenType   string
	ChosenNumber int32
	ChosenColor  string
	// RiotChoice is set by the logged as-enters Riot choice. It survives the
	// hand/stack path and Move consumes it on battlefield entry.
	RiotChoice string
	// Protector is the CR 310.10 Siege protector: the opponent its
	// controller chose to protect this Battle as it entered. It is a property
	// of the battle (not a counter), recorded through a Choose "protector"
	// event so it is replay-derived, and reset when the object leaves the
	// battlefield (a re-entering battle is protected afresh). ProtectorValid
	// distinguishes "no protector chosen yet" from a real protector: seat 0
	// is a legal opponent, so a zero Protector alone is ambiguous.
	Protector      PlayerID
	ProtectorValid bool
	// LastNotedMana is the mana type the object's last RememberCostMana$
	// activation paid with (Jeweled Amulet: "note the type of mana spent to
	// pay this activation cost") — the colour letter(s) of the mana the
	// payment actually spent, in WUBRG order. Folded by events.Choose's
	// "noted-mana" marker; read by effMana's Produced$ "Special
	// LastNotedType". Empty when nothing has been noted (or the object left
	// and returned — a move clears it with the rest of the per-copy state).
	LastNotedMana string
	// IntrinsicKeywords are keyword choices that become part of this
	// permanent's characteristics (currently Riot's haste choice).
	IntrinsicKeywords []string
	// Chosen is the current card/player choice. It is distinct from
	// Remembered: Forge uses Player.Chosen for the most recent choice and
	// Player.IsRemembered for choices explicitly marked RememberChosen$.
	Chosen []Target

	// ChosenModes carries a modal spell's CR 601.2b announcement or a modal
	// triggered ability's CR 603.3c placement choice to resolution: the SVar
	// names of the chosen Choices$ sub-abilities, in execution order.
	// Resolution builds Ctx.Modes from this so effCharm runs exactly the
	// chosen modes instead of asking again. It is a cache maintained beside
	// the already-logged ModeChosen marker; replay re-poses and re-answers the
	// same decision through the identical code path, so it is not a second
	// source of truth. Nil when no modal announcement has been made.
	ChosenModes []string

	// Imprinted holds cards ImprintCards$ explicitly associated with this
	// object. It is distinct from ExiledCards: Forge's host card has separate
	// imprintedCards and exiledCards collections, and their consumers must not
	// make an ordinary exile satisfy an Imprinted selector. It is state
	// because later abilities (Chrome Mox) refer to it after the originating
	// resolution has ended.
	Imprinted []ObjID
	// ExiledCards holds cards this object exiled through ChangeZone (Forge's
	// hostCard.exiledCards). The association exists only while the card
	// remains in exile; events.Move removes it when the card leaves. It is
	// what DefinedCards$ ExiledWith consumes, not the Imprinted list above,
	// and it is distinct from the ExiledWith ObjID field below (that one is
	// the reverse relationship: what exiled THIS card).
	ExiledCards []ObjID

	// ExileReturn holds the cards this object exiled through ChangeZone's
	// Duration$ UntilHostLeavesPlay (the Oblivion Ring / Banisher Priest
	// pattern): each entry's From is the zone the card was exiled from, and
	// when this object leaves the battlefield the rules sweep returns every
	// entry whose object is still in exile to that zone under its owner's
	// control. Like ExiledCards it is a zone relationship, not an imprint:
	// events.Move prunes entries naming an object that left exile by any
	// other path, so a sweep never returns a card whose exile was some other
	// effect's business. Event-backed through the Imprint kind's
	// "until-host-leaves" Text discriminator.
	ExileReturn []ExileReturnEntry

	// MergedCards holds the cards stacked BENEATH a mutated permanent's top
	// card (CR 702.140d), top-of-pile first. Card/FaceIdx on the object always
	// describe the TOP card; each entry here is one card that mutated below it.
	// A merged-under card is not an independent permanent: its object is parked
	// in ZCeased (which has no membership list, so no battlefield scan sees it)
	// and only the pile's own departure moves it (events.Move), which is
	// CR 702.140e's "each card that's merged ... moves to its owner's
	// graveyard" -- and the same move for every other zone. TimesMutated is
	// CR 702.140f's count of how many times this permanent has mutated, read by
	// Count$TimesMutated.
	MergedCards  []MergedCard
	TimesMutated int32

	// AttachedTo is the permanent this Aura or Equipment is attached to; 0
	// means unattached. Reset whenever the object itself leaves the
	// battlefield (events.Move) -- an Aura or Equipment cannot stay
	// "attached" once it isn't a permanent.
	AttachedTo ObjID

	// ExiledWith is the object whose effect most recently put this card into
	// exile. events.Apply derives it from a MoveZone event's existing IDs
	// carrier (or, for Hideaway's face-down exile, the Counter/Amount
	// carrier), so Card.ExiledWithSource filters replay without ambient
	// state. Zero means no tracked exile provenance.
	ExiledWith ObjID
	// FaceDown records a face-down exile (CR 702.75 Hideaway). It is state,
	// rather than merely a Secret event flag, so later projections know not to
	// reveal the card to another player.
	FaceDown bool
	// Cloaked records the cloak variant of the face-down battlefield entry
	// (CR 708.5's cloak: a 2/2 creature with ward {2}, turn-face-up cost =
	// the card's mana cost). It folds from the MoveZone Counter value
	// "entered_cloaked" exactly as FaceDown folds from "entered_face_down"
	// -- no new event kind, no Event field change -- and is cleared wherever
	// FaceDown is (leaving the battlefield; a future turn-face-up path).
	Cloaked bool

	// FaceDownSetType is the face-down set type a ChangeZone FaceDownSetType$
	// named (Yedora's "Land & Forest", Missy's "Artifact & Creature &
	// Cyberman"), stored raw as Forge writes it. Empty means CR 708.5's plain
	// face: a vanilla 2/2 creature. While FaceDown and on the battlefield it
	// replaces the synthetic {Creature} type set in the layer-4 derivation.
	FaceDownSetType string
	// FaceDownPower/FaceDownToughness and FaceDownHasPT carry the folded
	// FaceDownPower$/FaceDownToughness$ pair (Magar's 3/3). HasPT reports
	// whether a pair was named; without it a face-down Creature is CR 708.5's
	// 2/2 and a non-Creature set type derives 0/0.
	FaceDownPower     int32
	FaceDownToughness int32
	FaceDownHasPT     bool

	// Paired is the permanent this Soulbond creature is paired with (CR 702.103):
	// a creature its controller may pair it with when either enters an the
	// battlefield, as long as the controller controls both. 0 means unpaired.
	// Reset whenever the object leaves the battlefield (events.Move).
	Paired ObjID

	// IsToken and IsCopy mark an object that only ever exists on the stack
	// or the battlefield (CR 111.7 tokens, CR 707.10 copies). A token copy
	// minted by Myriad (CR 702.109) or a "create a token copy" effect
	// (CR 706.2, DB$ CopyPermanent) carries BOTH and legitimately lives on
	// the battlefield. See Ephemeral.
	IsToken  bool
	IsCopy   bool
	IsMyriad bool

	// CopyFace is the CR 613.1a copy-effect basis for a permanent that became a
	// copy of another (DB$ Clone): while non-nil, Face() returns THIS face
	// instead of the object's own card face, so every read site -- name,
	// abilities, keywords, types, colours, P/T, mana production -- sees the
	// copied characteristics with no per-caller plumbing. It is set and cleared
	// ONLY inside events.Apply (the ClonePermanent fold and Move's
	// leaves-the-battlefield reset), so a live game and a replay derive it
	// identically. The clone's modifier parameters (AddTypes$/SetColor$/
	// AddKeywords$/SetPower$/SetToughness$) are separate layer-4/5/6/7
	// continuous effects registered by the primitive, so this face stays the
	// source's PRINTED face and the layer walk applies the exceptions in CR 613
	// order on top. nil on every object that is not a copy.
	CopyFace *cards.Face
	// CopyGainThisAbility records the clone's GainThisAbility$ True rider: the
	// synthetic CopyFace already carries the ORIGINAL object's abilities (and
	// SVar table) so the ability that produced the copy survives the copy.
	// Engine-runtime, rebuilt from the ClonePermanent event on replay like
	// CopyFace.
	CopyGainThisAbility bool

	// Unlocked marks one face of an Enchantment Room (CR 309): the door the
	// room was CAST as is unlocked from entry; DoorUnlock (the unlock
	// activation) flips this when the OTHER half's door is paid for. A
	// room's locked half's abilities are inactive; after the unlock both
	// halves' rules text is live (rules-side scans consult this field). Only
	// events.Apply writes it, so a replay rebuilds it.
	Unlocked bool
}

// MergedCard is one card stacked beneath a mutated permanent's top card
// (CR 702.140d). Obj names the card's parked object (in ZCeased); Card and
// FaceIdx are the card's identity, carried on the value so the ability scans
// can read it without a second object lookup and so a parked object that is
// somehow gone still leaves the card's abilities live.
type MergedCard struct {
	Obj     ObjID
	Card    *cards.Card
	FaceIdx uint8
}

// ExileReturnEntry is one ChangeZone Duration$ UntilHostLeavesPlay exile:
// the exiled object and the zone it was exiled from (which the return moves
// it back to; a battlefield-origin exile returns to the battlefield, a
// hand-origin one to the hand).
type ExileReturnEntry struct {
	Obj  ObjID
	From Zone
}

// BestowedAttached reports whether o is a card printed with Bestow that is
// currently attached to a permanent (CR 702.114e: while attached to a
// creature the bestowed permanent is an Aura with enchant creature, not a
// creature; unattached it is a creature again). It is derived from live
// state -- AttachedTo and the printed face -- so every replay and every read
// site derives the switch identically and no event field carries a marker.
// An unattached bestowed card, and any object printed without Bestow, is
// never "bestowed attached".
func (o *Object) BestowedAttached() bool {
	return o.AttachedTo != 0 && o.Face() != nil && o.Face().HasKeyword("Bestow")
}

func (o *Object) Face() *cards.Face {
	// CR 613.1a: a copy effect is the FIRST layer, so while one applies the
	// object's characteristics come from the copied face. Routing it here is
	// what makes every Face() reader in the tree see the copy by construction
	// (CR 707.2) rather than each call site having to ask the layer system.
	if o.CopyFace != nil {
		return o.CopyFace
	}
	if o.Card == nil || int(o.FaceIdx) >= len(o.Card.Faces) {
		return nil
	}
	return o.Card.Faces[o.FaceIdx]
}

// faceDownEffective reports whether this object's face is currently hidden by
// CR 708.5: it is FaceDown and on the battlefield. While so, every printed
// characteristic is replaced by the face-down set.
func (o *Object) faceDownEffective() bool {
	return o.FaceDown && o.Zone == ZBattlefield
}

// FaceDownTypeWords is the effective base type set of a face-down battlefield
// permanent (CR 708.5): {Creature} when no FaceDownSetType$ was folded, else
// the set type split on Forge's " & " join. A non-face-down object returns
// its printed types. It returns a fresh slice so a caller can append to it
// (the layer-4 walk does) without aliasing stored state.
func (o *Object) FaceDownTypeWords() []string {
	if !o.faceDownEffective() {
		if f := o.Face(); f != nil {
			return append([]string(nil), f.Types...)
		}
		return nil
	}
	set := strings.TrimSpace(o.FaceDownSetType)
	if set == "" {
		return []string{"Creature"}
	}
	parts := strings.Split(set, " & ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"Creature"}
	}
	return out
}

// EffectiveIsCreature reports whether this object is a creature right now,
// honouring CR 708.5: while a battlefield object is face down its PRINTED face
// does not exist, so creature-ness comes from the folded FaceDownSetType$
// (default Creature). Every printed-face "is this a creature" read that gates
// a creature rule (combat, the creature SBAs, convoke, protection) must go
// through here, or a manifested non-creature or a face-down set type that
// drops Creature reads the wrong answer.
func (o *Object) EffectiveIsCreature() bool {
	if o.faceDownEffective() {
		for _, w := range o.FaceDownTypeWords() {
			if w == "Creature" {
				return true
			}
		}
		return false
	}
	f := o.Face()
	return f != nil && f.IsCreature()
}

// EffectiveIsArtifact reports whether this object is an artifact right now,
// honouring CR 708.5 like EffectiveIsCreature: while a battlefield object is
// face down its PRINTED face does not exist, so a manifested or cloaked
// artifact reads its folded face-down type set (which never names Artifact
// today, but the fold, not the corpus, decides). The Improvise announcement
// and its offer-gate credit (rules/cast.go) are the readers; other artifact
// reads (e.g. the Affinity keyword's Count$Valid spec path) go through the
// ordinary filter grammar and do not call this.
func (o *Object) EffectiveIsArtifact() bool {
	if o.faceDownEffective() {
		for _, w := range o.FaceDownTypeWords() {
			if w == "Artifact" {
				return true
			}
		}
		return false
	}
	f := o.Face()
	return f != nil && f.IsArtifact()
}

// MergedFaceAt returns the face of the i-th card stacked beneath the top card
// (0 is the first under-card), or nil when i is out of range. It is the one
// accessor the ability scans use so a merged card's face is resolved the same
// way everywhere.
func (o *Object) MergedFaceAt(i int) *cards.Face {
	if i < 0 || i >= len(o.MergedCards) {
		return nil
	}
	mc := &o.MergedCards[i]
	if mc.Card == nil || int(mc.FaceIdx) >= len(mc.Card.Faces) {
		return nil
	}
	return mc.Card.Faces[mc.FaceIdx]
}

// Ephemeral reports whether this object has, right now, ceased to exist: a
// copy of a spell or ability once it has LEFT THE STACK (CR 707.10h -- a copy
// of a spell that has left the stack is a transient reference, not a real
// object), a token once it has left the battlefield (CR 111.7 -- so IsToken
// alone is not enough, a battlefield token is a perfectly real permanent),
// or an ability object (no card, Card == nil -- always ephemeral, since it
// never legitimately exists off the stack at all).
//
// The IsCopy half is therefore ZONE-AWARE, exactly like effects/filter.go's
// own guard: a battlefield object carrying IsCopy is a real permanent --
// Myriad (CR 702.109) and populate ("create a token copy", CR 706.2) mint
// their tokens as IsToken+IsCopy and legitimately keep them on the
// battlefield, and a copy of a permanent SPELL that has resolved onto the
// battlefield (CR 707.10g, token1's stack-entry clear) is likewise real.
// Only a copy that is neither on the stack (still a spell) nor on the
// battlefield (still a permanent) has ceased to exist.
//
// This build parks such objects in exile rather than deleting them, and
// callers (view.cardViews and any future zone-listing code) consult this
// single definition instead of re-deriving it, so the "copy off the stack
// and battlefield, token off the battlefield, or cardless" rule cannot drift
// between call sites.
func (o *Object) Ephemeral() bool {
	return (o.IsCopy && o.Zone != ZStack && o.Zone != ZBattlefield) ||
		(o.IsToken && o.Zone != ZBattlefield) || o.Card == nil
}

func (o *Object) Counter(kind string) int32 {
	for _, c := range o.Counters {
		if c.Kind == kind {
			return c.N
		}
	}
	return 0
}

// AddCounter adds n counters of a kind, creating the entry if needed. Counters
// never go negative: removing more than present clamps at zero.
func (o *Object) AddCounter(kind string, n int32) {
	for i := range o.Counters {
		if o.Counters[i].Kind == kind {
			o.Counters[i].N += n
			if o.Counters[i].N < 0 {
				o.Counters[i].N = 0
			}
			return
		}
	}
	if n > 0 {
		o.Counters = append(o.Counters, Counter{kind, n})
	}
}

// CloneDeep returns a value copy of o whose slice fields (Counters, Targets,
// Remembered, BlockedBy, Chosen, Goads, ChosenModes) are independently backed, so mutating
// the copy's slices can never alias o's -- everything else (Card, a shared
// pointer into the immutable compiled corpus, plus every scalar field) is
// correct as a plain value copy. This is the one definition of "deep-copy an
// Object": Game.Clone (the whole live arena), rules.Engine's own last-known-
// information capture (emit, before events.Emit mutates the live object)
// and Engine.Clone's copy of a pending trigger's Ctx.LKI all call this
// rather than each repeating the same four-line copy, so a future slice or
// map field added to Object needs updating in exactly one place to stay
// deep.
func (o *Object) CloneDeep() Object {
	c := *o
	c.Counters = append([]Counter(nil), o.Counters...)
	c.Targets = append([]Target(nil), o.Targets...)
	c.Remembered = append([]Target(nil), o.Remembered...)
	c.BlockedBy = append([]ObjID(nil), o.BlockedBy...)
	c.Chosen = append([]Target(nil), o.Chosen...)
	c.Goads = append([]GoadEffect(nil), o.Goads...)
	c.ChosenModes = append([]string(nil), o.ChosenModes...)
	c.IntrinsicKeywords = append([]string(nil), o.IntrinsicKeywords...)
	c.Imprinted = append([]ObjID(nil), o.Imprinted...)
	c.ExiledCards = append([]ObjID(nil), o.ExiledCards...)
	c.ExileReturn = append([]ExileReturnEntry(nil), o.ExileReturn...)
	c.MergedCards = append([]MergedCard(nil), o.MergedCards...)
	return c
}

// SacrificedInfoOf captures the last-known-information snapshot of the object
// id names at the instant it is about to be sacrificed -- power, toughness and
// mana value -- so a later Sacrificed$<Property> SVar can answer "the
// sacrificed creature's power" without reading a now-graveyard object. It
// must be called BEFORE the sacrifice's zone change: the object is still a
// battlefield permanent then, so its face and any +1/+1 counters (the
// layer-7d portion of its P/T, preserved because they have not yet been reset
// by Move) are live. A missing object or faceless object (an ability or stack
// wrapper) degrades to the zero snapshot rather than panicking.
//
// The power/toughness convention deliberately matches the engine's existing
// effects-side reads (effects/count.go's Count$CardPower/Count$CardToughness:
// face value plus P1P1, not the full layer-system Derived) so the new
// Sacrificed$ heads agree with their nearest existing analogue.
func SacrificedInfoOf(g *Game, id ObjID) SacrificedInfo {
	o := g.Obj(id)
	if o == nil || o.Face() == nil {
		return SacrificedInfo{Obj: id}
	}
	p := int32(o.Face().Power()) + o.Counter("P1P1")
	t := int32(o.Face().Toughness()) + o.Counter("P1P1")
	return SacrificedInfo{Obj: id, Power: p, Toughness: t, ManaValue: o.Face().Cmc()}
}
