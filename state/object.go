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
	FlagKicked uint8 = 1 << iota // CR 601.2b: paid an optional additional cost
	FlagSurged
	FlagFlashback
	FlagMiracle
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
	HasPreStackEntry      bool

	// Stack-only.
	Ability *cards.SA
	Source  ObjID
	Targets []Target
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

	// Timestamp orders continuous effects. Assigned from Game.Clock whenever
	// the object enters the battlefield.
	Timestamp uint32

	// Cast-time metadata. X and CastFlags matter while this object is a
	// spell on the stack and, once it resolves, on the permanent it becomes
	// (an ETB "if it was kicked" trigger needs to read them off the
	// permanent) -- events.Move resets both when the object leaves the
	// battlefield.
	X         int32
	CastFlags uint8

	// Chosen* record answers to "as this enters/resolves, choose ..."
	// effects: a card name, a creature type, a number. Reset alongside X/
	// CastFlags when the object leaves the battlefield.
	ChosenName   string
	ChosenType   string
	ChosenNumber int32

	// ChosenModes carries a modal spell's CR 601.2b announcement or a modal
	// triggered ability's CR 603.3c placement choice to resolution: the SVar
	// names of the chosen Choices$ sub-abilities, in execution order.
	// Resolution builds Ctx.Modes from this so effCharm runs exactly the
	// chosen modes instead of asking again. It is a cache maintained beside
	// the already-logged ModeChosen marker; replay re-poses and re-answers the
	// same decision through the identical code path, so it is not a second
	// source of truth. Nil when no modal announcement has been made.
	ChosenModes []string

	// AttachedTo is the permanent this Aura or Equipment is attached to; 0
	// means unattached. Reset whenever the object itself leaves the
	// battlefield (events.Move) -- an Aura or Equipment cannot stay
	// "attached" once it isn't a permanent.
	AttachedTo ObjID

	// IsToken and IsCopy mark an object that only ever exists on the stack
	// or the battlefield (CR 111.7 tokens, CR 707.10 copies). See Ephemeral.
	IsToken bool
	IsCopy  bool
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
// Remembered, BlockedBy, ChosenModes) are independently backed, so mutating
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
	c.ChosenModes = append([]string(nil), o.ChosenModes...)
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
