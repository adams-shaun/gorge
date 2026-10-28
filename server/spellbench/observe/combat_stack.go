package observe

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// blockMap inverts the attackers' BlockedBy lists: blocker -> attackers it
// blocks. gorge keeps zero tombstones for removed blockers; they are skipped.
func (b *builder) blockMap() map[state.ObjID][]state.ObjID {
	if b.blocking == nil {
		b.blocking = map[state.ObjID][]state.ObjID{}
		g := b.p.E.G
		for i := range g.Objs {
			a := &g.Objs[i]
			if a.Zone == state.ZBattlefield && a.IsAttacking {
				for _, blk := range a.BlockedBy {
					if blk != 0 {
						b.blocking[blk] = append(b.blocking[blk], a.ID)
					}
				}
			}
		}
	}
	return b.blocking
}

// sameRef references id, or nil once id has changed zones since key was
// taken (Sections 5.1, 6.4 and 6.5: a reference to an object that left is
// null). known is false for a reference the tracker never recorded, which
// follows the current object.
func (b *builder) sameRef(id state.ObjID, key string, known bool) (*protocol.ObjectRef, error) {
	if known && key != b.p.IDs.Key(id) {
		return nil, nil
	}
	return b.p.Ref(b.viewer, id)
}

// looksBackTrigger reports whether ability's own card-text trigger is a
// leaves-the-battlefield trigger (Forge's Mode$ ChangesZone | Origin$
// Battlefield): gorge's own signal for CR 603.6d's look-back-in-time rule
// (rules/trigger_match.go's unexported looksBack). gorge does not export
// that match itself, so this mirrors its unexported findTriggerForAbility:
// walk src's current face's Triggers for the one whose Effect is ability,
// using only exported fields (Object.Face, Face.Triggers, Trigger.Effect).
//
// A trigger whose Origin is not the battlefield is not a leaves-the-
// battlefield trigger even when this particular move happens to end in a
// graveyard (CR 603.6c): a "put into a graveyard from anywhere" trigger such
// as Narcomoeba's fires from the graveyard it already occupies, so its
// source never left.
func looksBackTrigger(src *state.Object, ability *cards.SA) bool {
	f := src.Face()
	if f == nil {
		return false
	}
	for _, t := range f.Triggers {
		if t.Effect == ability {
			return t.Mode == "ChangesZone" && t.Params["Origin"] == "Battlefield"
		}
	}
	return false
}

func (b *builder) permanent(o *state.Object) (*protocol.Permanent, error) {
	pm := &protocol.Permanent{Tapped: o.Tapped, SummoningSick: o.SummonSick, Damage: u32(o.Damage),
		Counters: map[string]uint32{}, BlockedAttackers: []protocol.ObjectRef{},
		Statuses: statusNames(b.p.E.G, o), Chosen: chosen(o), ClassLevel: classLevel(o)}
	for _, c := range o.Counters {
		// gorge's own status markers (Shield, Deathtouched) ride an ordinary
		// counter for want of a status field; they are engine internals, never
		// one of Section 6.10's counters. A zero entry is kept: the engine
		// retains it after a counter is removed to zero and view.CardView
		// keeps the key.
		if !state.InternalCounterMarker(c.Kind) {
			pm.Counters[Counter(c.Kind)] += u32(c.N)
		}
	}
	if o.AttachedTo != 0 {
		r, err := b.p.Ref(b.viewer, o.AttachedTo)
		if err != nil {
			return nil, err
		}
		if r != nil {
			t := protocol.ObjectTarget(*r)
			pm.AttachedTo = &t
		}
	}
	if o.IsAttacking {
		pm.Attacking = true
		if o.AttackingBattle != 0 {
			key, known := b.p.IDs.AttackKey(o.ID)
			r, err := b.sameRef(o.AttackingBattle, key, known)
			if err != nil {
				return nil, err
			}
			if r != nil {
				t := protocol.ObjectTarget(*r)
				pm.AttackTarget = &t
			}
		} else {
			t := protocol.PlayerTarget(Seat(o.Attacking))
			pm.AttackTarget = &t
		}
	}
	attackers := b.blockMap()[o.ID]
	pm.Blocking = len(attackers) > 0 || b.p.IDs.Blocking(o.ID)
	for _, a := range attackers {
		r, err := b.p.Ref(b.viewer, a)
		if err != nil {
			return nil, err
		}
		if r != nil {
			pm.BlockedAttackers = append(pm.BlockedAttackers, *r)
		}
	}
	return pm, nil
}

// statusNames projects the permanent status designations the engine knows:
// the view.CardFlag bits view.cardFlags folds, plus the CR 701.38 goad the
// view has no bit for (upstream SpellBench carries it as "goaded"). The order
// is fixed so an observation is deterministic; the neutral builder maps the
// names back to CardFlag bits.
func statusNames(g *state.Game, o *state.Object) []string {
	out := []string{}
	add := func(on bool, name string) {
		if on {
			out = append(out, name)
		}
	}
	add(o.Suspected, "suspected")
	add(o.Renowned, "renowned")
	add(o.Monstrous, "monstrous")
	add(o.Harnessed, "harnessed")
	add(o.Solved, "solved")
	add(o.Cloaked, "cloaked")
	add(len(o.Goads) > 0, "goaded")
	add(int(o.Controller) < len(g.Players) && o.ID != 0 && g.Players[o.Controller].RingBearer == o.ID, "ring_bearer")
	add(o.CastFlags&(state.FlagMorphed|state.FlagMegamorphed) != 0, "morphed")
	add(o.CastFlags&state.FlagDisguised != 0, "disguised")
	add(o.DoorUnlocked(0), "left_door")
	add(o.DoorUnlocked(1), "right_door")
	return out
}

// chosen is Section 6.6's chosen characteristics, in the upstream goldens'
// {"kind","value"} shape. A permanent may record more than one independent
// choice (a name and a type, say), so each is its own entry.
func chosen(o *state.Object) []map[string]string {
	out := []map[string]string{}
	add := func(kind, value string) {
		if value != "" {
			out = append(out, map[string]string{"kind": kind, "value": value})
		}
	}
	add("card_name", o.ChosenName)
	add("creature_type", o.ChosenType)
	add("color", o.ChosenColor)
	if o.ChosenNumber > 0 {
		out = append(out, map[string]string{"kind": "number", "value": fmt.Sprint(o.ChosenNumber)})
	}
	return out
}

// classLevel is a Class permanent's CR 716 designation. class_level is nullable
// (V8): a non-Class permanent has no level, while a Class at its base level
// answers 1, exactly as state.Object.ClassLevel() does.
func classLevel(o *state.Object) *uint32 {
	f := o.Face()
	if f == nil {
		return nil
	}
	isClass := false
	for _, t := range f.Types {
		if Normalize(t) == "class" {
			isClass = true
			break
		}
	}
	if !isClass {
		return nil
	}
	n := uint32(o.ClassLevel())
	return &n
}

var stackKinds = map[state.StackObjKind]string{state.StackKindSpell: "spell",
	state.StackKindActivated: "activated_ability", state.StackKindTriggered: "triggered_ability"}

func (b *builder) stackAndPending(obs *protocol.Observation, v view.View) error {
	g := b.p.E.G
	for _, sv := range v.Stack {
		o := g.Obj(sv.ID)
		ref, err := b.p.Ref(b.viewer, sv.ID)
		if err != nil {
			return err
		}
		kind, ok := stackKinds[o.StackKind]
		if ref == nil || !ok || (o.Ability != nil) == (o.StackKind == state.StackKindSpell) {
			// Never a partial stack (Section 9.5): the game halts instead.
			return fmt.Errorf("engine_contract_failure:stack_entry %d", sv.ID)
		}
		se := protocol.StackEntry{ObjectRef: *ref, StackKind: kind, FaceDown: o.FaceDown, Copy: o.IsCopy,
			Targets: []*protocol.TargetRef{}}
		if o.Ability != nil {
			src := g.Obj(o.Source)
			key, known := b.p.IDs.SourceKey(o.ID)
			// Section 6.5: an ability's source is null once that source has
			// left. The tracker's recorded key, not gorge's SourceIncarnation,
			// decides (gorge leaves SourceIncarnation 0 on an ordinary ability
			// while its permanent sits at incarnation 1, so an incarnation
			// comparison would null live sources). A key mismatch means the
			// source changed zones again after the ability was put on the
			// stack: that alone covers an activated ability (a cycled or
			// sacrificed source moves before its ability is even pushed, per
			// the tracker's own pre-cost key capture, so its key is already
			// stale by the time it is recorded) and a triggered ability whose
			// source moves after being put on the stack.
			//
			// It misses a triggered ability created under CR 603.6d's
			// look-back-in-time rule: a genuine leaves-the-battlefield trigger
			// (dies, sacrificed, exiled, bounced) is created only once its
			// source has already left, so the key recorded for it already
			// matches wherever the source landed and never goes stale on its
			// own. looksBackTrigger reads the same signal gorge's own matcher
			// uses for that rule: when the ability is that shape and the
			// source is no longer on the battlefield, in any zone, the source
			// has left too. (Sources taken from a hand or library are nulled
			// by visibility instead. A graveyard-resident trigger such as
			// Narcomoeba's, put into a graveyard from the library, is not a
			// leaves-the-battlefield trigger even though the move ends in a
			// graveyard, CR 603.6c: its Origin is not the battlefield, so its
			// source never left where it now sits.)
			gone := known && key != b.p.IDs.Key(o.Source)
			if !gone && src != nil && o.StackKind == state.StackKindTriggered &&
				looksBackTrigger(src, o.Ability) && src.Zone != state.ZBattlefield {
				gone = true
			}
			if src != nil && !gone {
				if se.Source, err = b.p.Ref(b.viewer, o.Source); err != nil {
					return err
				}
			}
		} else {
			se.Characteristics = b.p.Characteristics(b.viewer, o)
		}
		for i, t := range o.Targets {
			if t.IsPlayer {
				pt := protocol.PlayerTarget(Seat(t.Player))
				se.Targets = append(se.Targets, &pt)
				continue
			}
			key, known := b.p.IDs.TargetKey(o.ID, i)
			r, err := b.sameRef(t.Obj, key, known)
			if err != nil {
				return err
			}
			if r == nil {
				se.Targets = append(se.Targets, nil)
			} else {
				ot := protocol.ObjectTarget(*r)
				se.Targets = append(se.Targets, &ot)
			}
		}
		se.Modes, se.XValue = modesAndX(o)
		obs.Stack = append(obs.Stack, se)
	}
	for _, pt := range b.p.E.PendingTriggers() {
		src := g.Obj(pt.Source)
		if src != nil && !Visible(b.viewer, src) {
			continue // Section 6.6: a trigger from a hidden, unrevealed source is omitted
		}
		entry := protocol.PendingTrigger{ControllerSeat: Seat(pt.Controller), Optional: pt.Optional}
		if src != nil {
			var err error
			if entry.Source, err = b.p.Ref(b.viewer, pt.Source); err != nil {
				return err
			}
			entry.SourceName = b.p.name(b.viewer, src)
		}
		obs.PendingTriggers = append(obs.PendingTriggers, entry)
	}
	return nil
}

func modesAndX(o *state.Object) ([]uint32, *uint32) {
	f := o.Face()
	sa := o.Ability
	if sa == nil && f != nil {
		sa = f.SpellAbility()
	}
	var modes []uint32
	if sa != nil && sa.Params["Choices"] != "" {
		modes = []uint32{}
		choices := strings.Split(sa.Params["Choices"], ",")
		for _, m := range o.ChosenModes {
			for i, c := range choices {
				if strings.TrimSpace(c) == m {
					modes = append(modes, uint32(i))
				}
			}
		}
	}
	var x *uint32
	if f != nil && o.Ability == nil && strings.Contains(f.ManaCost, "X") {
		v := u32(o.X)
		x = &v
	}
	return modes, x
}
