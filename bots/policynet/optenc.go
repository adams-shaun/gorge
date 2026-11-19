package policynet

import (
	"sync"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// OptionEncoder encodes the options of one decision against one view: it
// resolves the view's cards ONCE (the card index EncodeOption reads and the
// entity references FeaturesEntity reads) instead of once per option, into
// dense slices it keeps across Reset calls. Its output is exactly
// EncodeOptionWith's for the same view, seat and option.
//
// The view must not change between Reset and the last Encode: the index
// holds pointers into its zone lists.
type OptionEncoder struct {
	v    *view.View
	seat state.PlayerID
	// refs is cardIndex's resolution in its first-match order; pos maps an
	// ObjID to 1 + its index in refs (0 = not visible), kept all-zero
	// between uses: Reset clears exactly the entries the last use set.
	refs []cardRef
	pos  []int32
	// ent maps an ObjID to 1 + its State.Cards index (entityCards' order),
	// built on the first FeaturesEntity Encode after a Reset; entOrder is
	// that order's distinct ids (the entries ent set).
	ent      []int32
	entOrder []state.ObjID
	entOK    bool
	// ids are the refs' ObjIDs, for Reset's clearing: the view the refs
	// point into may have been re-projected in place since.
	ids     []state.ObjID
	avail   float32
	availOK bool
}

// cardRef is one view-resolved card: its CardView (in the view), which
// zone class it sits in (zoneIndex, or -1 for command/commanders/stack),
// and whether the zone list is the viewer's own.
type cardRef struct {
	cv   *view.CardView
	zone int
	mine bool
}

// maxDenseObjID bounds the dense ObjID index; an id past it (never seen in
// a real game) is resolved by a scan of refs instead.
const maxDenseObjID = 1 << 20

// Reset points the encoder at v for seat, resolving its cards in
// cardIndex's fixed order (my hand, my battlefield, my graveyard, my
// exile, then every other battlefield, graveyard and exile in seat order,
// my command zone and commanders, the stack) -- first match wins.
func (x *OptionEncoder) Reset(v *view.View, seat state.PlayerID) {
	for _, id := range x.ids {
		if int(id) < len(x.pos) {
			x.pos[id] = 0
		}
	}
	for _, id := range x.entOrder {
		if int(id) < len(x.ent) {
			x.ent[id] = 0
		}
	}
	clear(x.refs)
	x.refs, x.ids, x.entOrder, x.entOK, x.availOK = x.refs[:0], x.ids[:0], x.entOrder[:0], false, false
	x.v, x.seat = v, seat
	for i := range v.Players {
		pv := &v.Players[i]
		if pv.ID == seat {
			x.add(pv.Hand, zoneHand, true)
			x.add(pv.Battlefield, zoneBattlefield, true)
			x.add(pv.Graveyard, zoneGraveyard, true)
			x.add(pv.Exile, zoneExile, true)
		}
	}
	for i := range v.Players {
		pv := &v.Players[i]
		if pv.ID == seat {
			continue
		}
		x.add(pv.Battlefield, zoneBattlefield, false)
		x.add(pv.Graveyard, zoneGraveyard, false)
		x.add(pv.Exile, zoneExile, false)
	}
	for i := range v.Players {
		pv := &v.Players[i]
		if pv.ID == seat {
			x.add(pv.Command, -1, true)
			x.add(pv.Commanders, -1, true)
		}
	}
	for i := range v.Stack {
		if sv := &v.Stack[i]; sv.Card != nil {
			x.addOne(sv.Card, -1, sv.Card.Controller == seat)
		}
	}
}

func (x *OptionEncoder) add(cards []view.CardView, zone int, mine bool) {
	for i := range cards {
		x.addOne(&cards[i], zone, mine)
	}
}

func (x *OptionEncoder) addOne(cv *view.CardView, zone int, mine bool) {
	id := cv.ID
	if _, ok := x.lookup(id); ok {
		return
	}
	if int(id) < maxDenseObjID {
		if int(id) >= len(x.pos) {
			x.pos = growInt32(x.pos, int(id)+1)
		}
		x.pos[id] = int32(len(x.refs) + 1)
	}
	x.refs = append(x.refs, cardRef{cv: cv, zone: zone, mine: mine})
	x.ids = append(x.ids, id)
}

// lookup is the card index's read: the first-resolved card with id.
func (x *OptionEncoder) lookup(id state.ObjID) (*cardRef, bool) {
	if int(id) < maxDenseObjID {
		if int(id) < len(x.pos) {
			if p := x.pos[id]; p > 0 {
				return &x.refs[p-1], true
			}
		}
		return nil, false
	}
	for i := range x.refs {
		if x.refs[i].cv.ID == id {
			return &x.refs[i], true
		}
	}
	return nil, false
}

// growInt32 extends s (all-zero beyond its length) to n zero entries.
func growInt32(s []int32, n int) []int32 {
	if n <= cap(s) {
		return s[:n]
	}
	t := make([]int32, n, n+n/2)
	copy(t, s)
	return t
}

// availableTotal is availableTotal(*x.v, x.seat), computed once per Reset.
func (x *OptionEncoder) availableTotal() float32 {
	if !x.availOK {
		x.avail, x.availOK = availableTotal(*x.v, x.seat), true
	}
	return x.avail
}

// entRef is entityOptionRefs' lookup: 1 + id's State.Cards index (the
// entity order: my battlefield, every other battlefield in seat order, my
// hand, the stack; first match wins), 0 when id is no entity.
func (x *OptionEncoder) entRef(id state.ObjID) int32 {
	if id == 0 {
		return 0
	}
	if !x.entOK {
		x.buildEnt()
	}
	if int(id) < maxDenseObjID {
		if int(id) < len(x.ent) {
			return x.ent[id]
		}
		return 0
	}
	for i, e := range x.entOrder {
		if e == id {
			return int32(i + 1)
		}
	}
	return 0
}

// buildEnt resolves the entity order's distinct ids into entOrder (and the
// dense ent table), skipping a repeated id as entityCards does.
func (x *OptionEncoder) buildEnt() {
	x.entOK = true
	v, seat := x.v, x.seat
	var me *view.PlayerView
	for i := range v.Players {
		if v.Players[i].ID == seat {
			me = &v.Players[i]
		}
	}
	if me != nil {
		x.entAdd(me.Battlefield)
	}
	for i := range v.Players {
		if v.Players[i].ID == seat {
			continue
		}
		x.entAdd(v.Players[i].Battlefield)
	}
	if me != nil {
		x.entAdd(me.Hand)
	}
	for i := range v.Stack {
		if sv := &v.Stack[i]; sv.Card != nil {
			x.entAddOne(sv.Card.ID)
		}
	}
}

func (x *OptionEncoder) entAdd(cards []view.CardView) {
	for i := range cards {
		x.entAddOne(cards[i].ID)
	}
}

func (x *OptionEncoder) entAddOne(id state.ObjID) {
	if int(id) < maxDenseObjID {
		if int(id) >= len(x.ent) {
			x.ent = growInt32(x.ent, int(id)+1)
		}
		if x.ent[id] != 0 {
			return
		}
		x.ent[id] = int32(len(x.entOrder) + 1)
	} else {
		for _, e := range x.entOrder {
			if e == id {
				return
			}
		}
	}
	x.entOrder = append(x.entOrder, id)
}

// Encode is EncodeOptionWith(fs, *v, seat, d, o, idx, n) for the view and
// seat of the last Reset.
func (x *OptionEncoder) Encode(fs FeatureSet, d decision.Kind, o decision.Option, idx, n int) Option {
	opt := x.encodeV1(d, o, idx, n)
	if fs == FeaturesV1 {
		return opt
	}
	if decKindIndex(d) < 0 {
		// Remove the colliding v1 "other" bit. EncodeOption emits the
		// decision-kind slot before the card-type block, so the FIRST slot
		// at that row is the decision kind's; a real Land's own type bit, set
		// later, survives.
		opt.Slots = dropSlot(opt.Slots, uint16(decKindOffset+len(decision.Kinds)))
		opt.Hashed = append(opt.Hashed, Feature{Row: hashID("mz|deckind|other"), Value: 1})
	}
	if o.Obj != 0 {
		if ref, ok := x.lookup(o.Obj); ok && ref.cv.SpellAPI != "" {
			opt.Hashed = append(opt.Hashed, Feature{Row: hashID("mz|api|" + ref.cv.SpellAPI), Value: 1})
		}
	}
	if fs.Entity() {
		rel := o.Attacker
		if rel == 0 {
			rel = o.Battle
		}
		opt.EntA, opt.EntB = x.entRef(o.Obj), x.entRef(rel)
	}
	return opt
}

var optionEncoders = sync.Pool{New: func() any { return new(OptionEncoder) }}

// withEncoder runs f with a pooled encoder Reset to (v, seat).
func withEncoder[T any](v *view.View, seat state.PlayerID, f func(*OptionEncoder) T) T {
	x := optionEncoders.Get().(*OptionEncoder)
	x.Reset(v, seat)
	r := f(x)
	x.v = nil
	clear(x.refs)
	optionEncoders.Put(x)
	return r
}

// EncodeOptionsWith encodes every option of a decision of kind d against
// one view: dst[i] = EncodeOptionWith(fs, *v, seat, d, opts[i], i,
// len(opts)), with the view's cards resolved once. dst is reused when it
// has the capacity.
func EncodeOptionsWith(fs FeatureSet, v *view.View, seat state.PlayerID, d decision.Kind, opts []decision.Option, dst []Option) []Option {
	if cap(dst) < len(opts) {
		dst = make([]Option, len(opts))
	}
	dst = dst[:len(opts)]
	x := optionEncoders.Get().(*OptionEncoder)
	x.Reset(v, seat)
	for i := range opts {
		dst[i] = x.Encode(fs, d, opts[i], i, len(opts))
	}
	x.v = nil
	clear(x.refs)
	optionEncoders.Put(x)
	return dst
}

// ViewOmit is the lean-projection mask (view.ProjectLeanInto) of the view
// parts fs's encoders never read: the own-library list, the opponent
// archetype posterior, ability and effective costs, the copied decision, the
// own deck manifest and the potential actions. A set that reads any of them
// (FeaturesMZOwnLib reads the deck manifest; the diagnostic sets are not
// played) omits nothing, so its projection is ProjectInto's.
func (fs FeatureSet) ViewOmit() view.Omit {
	switch fs {
	case FeaturesV1, FeaturesMZ, FeaturesEntity, FeaturesMZGrave, FeaturesPod:
		return view.OmitLibrary | view.OmitArchetype | view.OmitAbilityCosts | view.OmitEffectiveCost |
			view.OmitDecision | view.OmitOwnDeck | view.OmitPotential
	}
	return 0
}
