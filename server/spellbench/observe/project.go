// Package observe builds the Section 6 observation for one seat from gorge's
//
// Ported from github.com/jackmaiorino/spellbench/engines/gorge/internal/observe
// at revision 1beea4c104911bba77a5672c644e570ed9afe28a (MIT).
// seat projection (view.Project, visibility "seat") completed from engine state.
package observe

import (
	"fmt"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/identity"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

type Projector struct {
	R     gamecfg.Reader
	IDs   *identity.Tracker
	Mulls [2]uint32
}

type State struct {
	PriorityHolder *state.PlayerID
	Known          []protocol.Known
}

var superTypes = map[string]bool{"basic": true, "legendary": true, "ongoing": true, "snow": true, "world": true}

func Visible(viewer state.PlayerID, o *state.Object) bool {
	switch o.Zone {
	case state.ZBattlefield, state.ZGraveyard, state.ZExile, state.ZStack, state.ZCommand:
		return true
	case state.ZHand:
		return o.Owner == viewer
	}
	return false
}

func MayLook(viewer state.PlayerID, o *state.Object) bool {
	looker := o.Controller
	if o.HasMayLook {
		looker = o.MayLookPlayer
	}
	return viewer == looker
}

func controller(o *state.Object) state.PlayerID {
	if o.Zone == state.ZBattlefield || o.Zone == state.ZStack {
		return o.Controller
	}
	return o.Owner
}

func (p *Projector) name(viewer state.PlayerID, o *state.Object) *string {
	if o.FaceDown && !MayLook(viewer, o) {
		return nil
	}
	var n string
	if o.Ability != nil {
		// An ability is named after its source, unless that source is face
		// down and the viewer may not look at it.
		if src := p.R.Board().Obj(o.Source); src != nil && src.Face() != nil && (!src.FaceDown || MayLook(viewer, src)) {
			n = src.Face().Name
		}
	} else if n = p.R.Facts(o.ID).Name; n == "" && o.Face() != nil {
		n = o.Face().Name
	}
	if n == "" {
		return nil
	}
	return &n
}

// omitted reports whether gorge's seat view leaves o out of its public zone,
// as view's cardViews does: a resolved ability or spell copy parked in exile,
// a token that left the battlefield, a phased-out permanent (CR 702.25b). The
// observation lists no such object, so it has no reference (Section 5.1). The
// view lists every stack object, abilities included.
func omitted(o *state.Object) bool {
	return o.Zone != state.ZStack && (o.Face() == nil || o.Ephemeral() || o.PhasedOut)
}

func (p *Projector) Ref(viewer state.PlayerID, id state.ObjID) (*protocol.ObjectRef, error) {
	o := p.R.Board().Obj(id)
	if o == nil || !Visible(viewer, o) || omitted(o) {
		return nil, nil
	}
	oid, err := p.IDs.VisibleID(viewer, id)
	if err != nil {
		return nil, err
	}
	return &protocol.ObjectRef{ObjectID: oid, CardName: p.name(viewer, o), OwnerSeat: Seat(o.Owner),
		ControllerSeat: Seat(controller(o)), Zone: o.Zone.String()}, nil
}

// LookRef references a hidden-zone object the open look shows to viewer. An
// object the viewer can see already has its visible id; a look id would give
// it two.
func (p *Projector) LookRef(viewer state.PlayerID, id state.ObjID) (protocol.ObjectRef, error) {
	o := p.R.Board().Obj(id)
	if o == nil || Visible(viewer, o) {
		return protocol.ObjectRef{}, fmt.Errorf("engine_contract_failure:look_ref_not_hidden %d", id)
	}
	oid, err := p.IDs.LookID(viewer, id)
	if err != nil {
		return protocol.ObjectRef{}, err
	}
	name := o.Face().Name
	return protocol.ObjectRef{ObjectID: oid, CardName: &name, OwnerSeat: Seat(o.Owner),
		ControllerSeat: Seat(o.Owner), Zone: o.Zone.String()}, nil
}

func (p *Projector) Characteristics(viewer state.PlayerID, o *state.Object) *protocol.Characteristics {
	if o.FaceDown && !MayLook(viewer, o) {
		if o.Zone != state.ZBattlefield && o.Zone != state.ZStack {
			return nil
		}
		two := int32(2)
		return &protocol.Characteristics{Supertypes: []string{}, Types: []string{"creature"}, Subtypes: []string{},
			Colors: []string{}, Power: &two, Toughness: &two, Keywords: []string{}}
	}
	d := p.R.Facts(o.ID)
	c := &protocol.Characteristics{Supertypes: []string{}, Types: []string{}, Subtypes: []string{},
		Colors: Colors(d.Colors), Keywords: Keywords(d.Keywords)}
	for _, t := range d.Types {
		n := Normalize(t)
		switch {
		case superTypes[n]:
			c.Supertypes = append(c.Supertypes, n)
		case slices.Contains(protocol.Vocab["card_type"], n):
			c.Types = append(c.Types, n)
		default:
			c.Subtypes = append(c.Subtypes, n)
		}
	}
	c.ManaValue = manaValue(o)
	if slices.Contains(c.Types, "creature") {
		pw, tg := d.Power, d.Toughness
		c.Power, c.Toughness = &pw, &tg
	}
	return c
}

// manaValue is CR 202.3's mana value: a transforming double-faced card uses
// its front face's cost on either face (CR 712.8e: Vector Glider is 2), and X
// counts as its announced value only while the spell is on the stack (CR
// 202.3e).
func manaValue(o *state.Object) uint32 {
	f := o.Face()
	if f == nil {
		return 0
	}
	cost := f.ManaCost
	if o.Card != nil && o.Card.AlternateMode == "DoubleFaced" && len(o.Card.Faces) > 0 {
		cost = o.Card.Faces[0].ManaCost
	}
	mv := max(0, botpolicy.CmcOf(cost))
	if o.Zone == state.ZStack && o.Ability == nil {
		mv += int32(strings.Count(cost, "X")) * max(0, o.X)
	}
	return uint32(mv)
}

// builder carries per-observation caches (the inverted block map of Task 12).
type builder struct {
	p        *Projector
	viewer   state.PlayerID
	blocking map[state.ObjID][]state.ObjID
}

func (p *Projector) Record(viewer state.PlayerID, id state.ObjID) (protocol.ObjectRecord, error) {
	return (&builder{p: p, viewer: viewer}).record(id)
}

func (b *builder) record(id state.ObjID) (protocol.ObjectRecord, error) {
	o := b.p.R.Board().Obj(id)
	ref, err := b.p.Ref(b.viewer, id)
	if err != nil {
		return protocol.ObjectRecord{}, err
	}
	if ref == nil {
		return protocol.ObjectRecord{}, fmt.Errorf("object %d is not visible to %s", id, Seat(b.viewer))
	}
	rec := protocol.ObjectRecord{ObjectRef: *ref, FaceDown: o.FaceDown, Token: o.IsToken, Copy: o.IsCopy,
		Characteristics: b.p.Characteristics(b.viewer, o)}
	if o.Zone == state.ZBattlefield {
		rec.Permanent, err = b.permanent(o)
	}
	// An exiled card's exiling source is a public zone relation (the card's
	// Object.ExiledWith), and a host rebuilding the board from the
	// observation needs it: a source whose mana or count reads "cards exiled
	// with it" (Ugin's Labyrinth's {C}{C}) otherwise loses the link. Name the
	// source only when the viewer can see it, the same visibility rule every
	// other reference obeys.
	if o.Zone == state.ZExile && o.ExiledWith != 0 {
		if src, serr := b.p.Ref(b.viewer, o.ExiledWith); serr == nil && src != nil {
			rec.ExiledBy = src
		}
	}
	return rec, err
}

func (b *builder) records(cvs []view.CardView) ([]protocol.ObjectRecord, error) {
	out := make([]protocol.ObjectRecord, 0, len(cvs))
	for _, cv := range cvs {
		r, err := b.record(cv.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func u32(v int32) uint32 { return uint32(max(0, v)) }

func (p *Projector) Observation(viewer state.PlayerID, st State) (obs protocol.Observation, err error) {
	p.IDs.BeginObservation(viewer)
	defer func() {
		if err == nil {
			p.IDs.EndObservation(viewer)
		}
	}()
	g := p.R.Board()
	v := view.Project(g, p.R.Chars(), viewer, nil)
	b := &builder{p: p, viewer: viewer}
	obs = protocol.Observation{Viewer: Seat(viewer), Turn: uint32(max(0, g.Turn)), PhaseStep: PhaseStep(g),
		Stack: []protocol.StackEntry{}, PendingTriggers: []protocol.PendingTrigger{}, Known: st.Known}
	if obs.Known == nil {
		obs.Known = []protocol.Known{}
	}
	// day_night is always present under the true flag. The protocol spells
	// CR 731.1's "neither" as "none"; state.DayNight's String is empty for it.
	dayNight := g.DayNight.String()
	if dayNight == "" {
		dayNight = "none"
	}
	obs.DayNight = &dayNight
	if v.Active != view.NoSeat {
		s := Seat(v.Active)
		obs.ActiveSeat = &s
	}
	if st.PriorityHolder != nil {
		s := Seat(*st.PriorityHolder)
		obs.PrioritySeat = &s
	}
	for i, pv := range v.Players {
		pl := g.Players[pv.ID]
		// The engine spells its player counter "POISON" (effects/counters.go's
		// effPoison; state.Player.Counter is an exact-match lookup).
		poison := u32(pl.Counter("POISON"))
		po := protocol.PlayerObs{Seat: Seat(pv.ID), Life: pv.Life, Poison: &poison,
			Counters:     map[string]uint32{},
			Designations: []string{},
			ManaPool: protocol.ManaPool{W: u32(pl.Pool[state.MW]), U: u32(pl.Pool[state.MU]), B: u32(pl.Pool[state.MB]),
				R: u32(pl.Pool[state.MR]), G: u32(pl.Pool[state.MG]), C: u32(pl.Pool[state.MC])},
			LandsPlayedThisTurn: u32(pl.LandsPlayed), MulligansTaken: p.Mulls[pv.ID],
			HandCount: uint32(pv.HandSize), LibraryCount: uint32(pv.LibrarySize)}
		// poison has its own field; every other player counter rides counters.
		// A zero entry is kept: state.Player.Counters retains one after a
		// counter is removed to zero, and view.PlayerView.Counters keeps the
		// key, so dropping it here would make the rebuild differ from native.
		// The poison field has no presence bit of its own, so a zero-valued
		// poison counter rides both: the field carries the count and counters
		// carries the key. A positive poison rides only the field, never both
		// routings at once.
		for _, c := range pl.Counters {
			if strings.EqualFold(c.Kind, "poison") && c.N > 0 {
				continue
			}
			po.Counters[Counter(c.Kind)] = u32(c.N)
		}
		if g.IsMonarch(pv.ID) {
			po.Designations = append(po.Designations, "monarch")
		}
		if g.IsInitiative(pv.ID) {
			po.Designations = append(po.Designations, "initiative")
		}
		var err error
		if pv.ID == viewer {
			if po.Hand, err = b.records(pv.Hand); err != nil {
				return obs, err
			}
		}
		for _, z := range []struct {
			dst *[]protocol.ObjectRecord
			src []view.CardView
		}{{&po.Battlefield, pv.Battlefield}, {&po.Graveyard, pv.Graveyard}, {&po.Exile, pv.Exile}, {&po.Command, pv.Command}} {
			if *z.dst, err = b.records(z.src); err != nil {
				return obs, err
			}
		}
		obs.Players[i] = po
	}
	return obs, b.stackAndPending(&obs, v)
}
