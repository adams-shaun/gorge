package state

import "github.com/adams-shaun/gorge/cards"

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
	FlagKicked uint16 = 1 << iota // CR 601.2b: paid an optional additional cost
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
	CastFlags uint16

	// Chosen* record answers to "as this enters/resolves, choose ..."
	// effects: a card name, a creature type, a number. Reset alongside X/
	// CastFlags when the object leaves the battlefield.
	ChosenName   string
	ChosenType   string
	ChosenNumber int32
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

	// AttachedTo is the permanent this Aura or Equipment is attached to; 0
	// means unattached. Reset whenever the object itself leaves the
	// battlefield (events.Move) -- an Aura or Equipment cannot stay
	// "attached" once it isn't a permanent.
	AttachedTo ObjID

	// ExiledWith is the object whose effect most recently put this card into
	// exile. events.Apply derives it from a MoveZone event's existing IDs
	// carrier, so Card.ExiledWithSource filters replay without ambient state.
	ExiledWith ObjID

	// IsToken and IsCopy mark an object that only ever exists on the stack
	// or the battlefield (CR 111.7 tokens, CR 707.10 copies). See Ephemeral.
	IsToken bool
	IsCopy  bool

	// Unlocked marks one face of an Enchantment Room (CR 309): the door the
	// room was CAST as is unlocked from entry; DoorUnlock (the unlock
	// activation) flips this when the OTHER half's door is paid for. A
	// room's locked half's abilities are inactive; after the unlock both
	// halves' rules text is live (rules-side scans consult this field). Only
	// events.Apply writes it, so a replay rebuilds it.
	Unlocked bool
}

func (o *Object) Face() *cards.Face {
	if o.Card == nil || int(o.FaceIdx) >= len(o.Card.Faces) {
		return nil
	}
	return o.Card.Faces[o.FaceIdx]
}

// Ephemeral reports whether this object has, right now, ceased to exist: a
// copy of a spell or ability (CR 707.10, gone the moment it leaves the
// stack), a token (CR 111.7, gone once it leaves the battlefield -- so
// IsToken alone is not enough, a token on the battlefield is a perfectly
// real permanent), or an ability object (no card, Card == nil -- always
// ephemeral, since it never legitimately exists off the stack at all).
// This build parks such objects in exile rather than deleting them, and
// callers (view.cardViews and any future zone-listing code) consult this
// single definition instead of re-deriving it, so the "copy, or token off
// the battlefield, or cardless" rule cannot drift between call sites.
func (o *Object) Ephemeral() bool {
	return o.IsCopy || (o.IsToken && o.Zone != ZBattlefield) || o.Card == nil
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
	c.Imprinted = append([]ObjID(nil), o.Imprinted...)
	c.ExiledCards = append([]ObjID(nil), o.ExiledCards...)
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
