package rules

import (
	"fmt"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// "Loses all abilities" (CR 613.1f), beyond keywords.
//
// A layer-6 RemoveAbilities effect (Forge's RemoveAllAbilities$ True: the
// Witness Protection / Frogify / Darksteel Mutation Aura statics, Humility,
// the one-shot Animate deliveries of Turn to Frog and its family) removes
// EVERY ability of the permanents it applies to -- triggered, activated, mana
// and static abilities as well as the keywords the layer-6 keyword walk
// already clears. Two halves implement that, and both read the same effects:
//
//   - the affected permanent's PRINTED abilities do not exist
//     (printedAbilitiesGone, the gate every printed-face scan shares with CR
//     708.8's face-down rule), and an ability another effect GRANTED it
//     survives only when that grant is not older than the removal
//     (grantLostTo: layer 6 applies in timestamp order, so a later grant is
//     applied on top of the removal and an earlier one is removed with the
//     rest);
//   - the continuous effects the permanent's own static abilities generate
//     stop applying. That half lives inside active() (pruneLostSourceAbilities):
//     every reader of the list -- the layer walks, the granted-ability and
//     granted-trigger collectors, the may-play and land-drop rules
//     modifications -- then sees the list without them, with no per-reader
//     gate to forget.
//
// CR 613.6 bounds the second half: an effect that started to apply in an
// earlier layer keeps applying even though the ability generating it is
// removed in layer 6. The static scan records each static ability's first
// layer on the effects it emits (state.ContinuousEffect.SourceAbilityLayer):
// one beginning in layers 1-5 is never dropped; a REMOVER beginning in layer
// 6 is dropped only by a removal with an EARLIER timestamp (two removers that
// can remove each other's abilities are a dependency loop, CR 613.8b, so
// timestamp order decides); any other effect beginning in layer 6 depends on
// the removal of its source's abilities (CR 613.8a) and is dropped by any
// removal, as is one beginning in layer 7 or a rules modification that is no
// layer effect at all.
//
// CR 305.7 feeds the same two halves from layer 4: a land whose subtype an
// applied effect set to basic land types (state.SetsBasicLandSubtypes: Blood
// Moon, Spreading Seas) loses every ability its rules text generates. That
// loss never removes an ability another effect GRANTED the land -- it is part
// of the layer-4 effect, before any layer-6 grant -- so abilityLoss reports
// lost with a zero timestamp and grantLostTo's "removal newer than the grant"
// stays false. The land's own layer-6+ statics stop through the same pruner;
// ones that began in layers 1-5 (a self-setter like Multiversal Passage's)
// keep applying (CR 613.6).

// ruleModAbilityLayer is the SourceAbilityLayer of a static-generated rules
// modification that is not a layer effect (Layer zero: MayPlay,
// AdjustLandPlays, a trigger or SVar grant): past every real layer, so any
// removal of the source's abilities ends it.
const ruleModAbilityLayer = LPT + 1

// markSourceAbilityLayer stamps the effects one static ability emitted with
// the first layer that ability applies in (see the file comment).
func markSourceAbilityLayer(emitted []ContinuousEffect) {
	first := ruleModAbilityLayer
	for i := range emitted {
		if l := emitted[i].Layer; l != 0 && l < first {
			first = l
		}
	}
	for i := range emitted {
		if emitted[i].Layer == 0 {
			emitted[i].SourceAbilityLayer = ruleModAbilityLayer
		} else {
			emitted[i].SourceAbilityLayer = first
		}
	}
}

// removalApplies reports whether the ability-removing effect r applies to the
// battlefield permanent or graveyard card o, whose finished layer-4 type list
// is types. It is the layer-6 walk's own applicability test (derivedCompute):
// the shared matcher, then the AffectedZone$ qualifier.
func (e *Engine) removalApplies(r *ContinuousEffect, o *state.Object, types []string) bool {
	if o.Zone != state.ZBattlefield && o.Zone != state.ZGraveyard {
		return false
	}
	if types == nil {
		// Bound even when empty, as derivedCompute binds it: a nil list is
		// SpecContext's "unbound, read the printed face".
		types = []string{}
	}
	if !e.matchesWithChars(r, o.ID, types, nil, 0) {
		return false
	}
	if r.AffectedZone != "" && !r.MayPlay {
		if zones, all, ok := effects.ParseZones(r.AffectedZone); !ok || (!all && !slices.Contains(zones, o.Zone)) {
			return false
		}
	}
	return true
}

// sourceLoss is one static source's verdict in pruneLostSourceAbilities:
// whether some live removal applies to it, and the earliest such timestamp.
type sourceLoss struct {
	id    state.ObjID
	lost  bool
	first uint32
}

// pruneLostSourceAbilities drops from active()'s freshly assembled list the
// static-generated effects whose source has lost all abilities (see the file
// comment). list is in CR 613 order, so its RemoveAbilities effects are met in
// timestamp order; a remover that an earlier one silenced is not live and
// removes nothing. The survivors keep their order. Called only when the list
// holds a remover at all, so a board with none pays nothing.
//
// It runs inside activeBuild, before the list is published: applicability is
// read through typeCharacteristicsActive over this same list (layer 4 is
// settled before layer 6, and no layer-4 effect is ever dropped here), never
// through Derived.
func (e *Engine) pruneLostSourceAbilities(list []ContinuousEffect) []ContinuousEffect {
	var removerBuf [8]int
	removers := removerBuf[:0]
	landSetters := false
	for i := range list {
		if list[i].Layer == LAbilities && list[i].RemoveAbilities {
			removers = append(removers, i)
		}
		if state.SetsBasicLandSubtypes(&list[i]) {
			landSetters = true
		}
	}
	if len(removers) == 0 && !landSetters {
		return list
	}
	var deadBuf [8]bool
	dead := deadBuf[:0]
	if len(removers) > len(deadBuf) {
		dead = make([]bool, 0, len(removers))
	}
	dead = dead[:len(removers)]
	// earliest returns the earliest timestamp among the live removers
	// list[removers[:upto]] that apply to source.
	earliest := func(source state.ObjID, upto int) (uint32, bool) {
		o := e.G.Obj(source)
		if o == nil || o.Zone != state.ZBattlefield || o.Face() == nil {
			return 0, false
		}
		var types []string
		typed := false
		for k := 0; k < upto; k++ {
			if dead[k] {
				continue
			}
			r := &list[removers[k]]
			if !typed {
				types, typed = e.typeCharacteristicsActive(list, source, 0), true
			}
			if e.removalApplies(r, o, types) {
				// Timestamp order: the first hit is the earliest.
				return r.Timestamp, true
			}
		}
		return 0, false
	}
	// landSet: CR 305.7 applies to source (read off list before compaction).
	landSet := func(source state.ObjID) bool {
		if !landSetters {
			return false
		}
		o := e.G.Obj(source)
		if o == nil || o.Zone != state.ZBattlefield || o.Face() == nil {
			return false
		}
		_, set := e.typesAndLandSetActive(list, source)
		return set
	}
	// A remover that is itself a static ability beginning in layer 6 is
	// silenced by an EARLIER live remover applying to its source; it is
	// never silenced by itself or a later one (CR 613.6).
	for k, ri := range removers {
		r := &list[ri]
		if r.SourceAbilityLayer != LAbilities {
			continue
		}
		if ts, ok := earliest(r.Source, k); ok && ts < r.Timestamp {
			dead[k] = true
		} else if landSet(r.Source) {
			dead[k] = true
		}
	}
	var lossBuf [8]sourceLoss
	losses := lossBuf[:0]
	lossOf := func(source state.ObjID) (uint32, bool) {
		for i := range losses {
			if losses[i].id == source {
				return losses[i].first, losses[i].lost
			}
		}
		ts, ok := earliest(source, len(removers))
		if !ok && landSet(source) {
			ok = true
		}
		losses = append(losses, sourceLoss{id: source, lost: ok, first: ts})
		return ts, ok
	}
	// Every verdict is decided before the list is compacted: lossOf reads
	// list (the removers by their original index, and layer 4 through
	// typeCharacteristicsActive), so compacting while deciding would hand a
	// later source's check a shifted entry -- a Witness Protection that
	// silenced one Gnarlid Colony then "removed" a second Colony's abilities
	// through whatever grant had slid into the remover's slot.
	var dropBuf [64]bool
	drops := dropBuf[:0]
	if len(list) > len(dropBuf) {
		drops = make([]bool, 0, len(list))
	}
	drops = drops[:len(list)]
	next := 0
	for i := range list {
		ce := &list[i]
		drop := false
		isRemover := next < len(removers) && removers[next] == i
		switch {
		case isRemover:
			drop = dead[next]
		case ce.SourceAbilityLayer < LAbilities:
			// A registered effect (zero), or a static ability that began
			// applying before layer 6.
		default:
			// A layer-6 effect that is not itself a remover (a keyword grant:
			// Gnarlid Colony's "creatures you control with a +1/+1 counter
			// have trample") depends on a removal of its source's abilities
			// (CR 613.8a: the removal changes its existence), so the removal
			// applies first whatever the timestamps and the effect never
			// begins -- the same as an effect beginning in layer 7.
			_, drop = lossOf(ce.Source)
		}
		if isRemover {
			next++
		}
		drops[i] = drop
	}
	kept := list[:0]
	for i := range list {
		ce := &list[i]
		if !drops[i] {
			if len(kept) != i {
				kept = append(kept, *ce)
			} else {
				kept = kept[:i+1]
			}
		}
	}
	clear(list[len(kept):])
	return kept
}

// abilityLoss reports whether a live "loses all abilities" effect applies to
// the battlefield permanent or graveyard card o (the graveyard arm serves
// Yixlid Jailer's AffectedZone$ Graveyard removal, read through the pre-move
// look-back for a card leaving the graveyard), and the latest such effect's
// timestamp. It reads active()'s published list, in which every remover still
// present is live (pruneLostSourceAbilities dropped the silenced ones). A
// board with no remover answers from the per-build summary alone.
//
// During a layer scan (active() mid-build) it stands down and reports no
// loss, the same stand-down matchesSpec's Derived bind takes.
func (e *Engine) abilityLoss(o *state.Object) (uint32, bool) {
	if o == nil || o.Face() == nil || (o.Zone != state.ZBattlefield && o.Zone != state.ZGraveyard) {
		return 0, false
	}
	// The board-wide early outs (mid-build stand-down, the sticky proof, a
	// list with no remover), answered once per active() build
	// (abilityloss_memo.go).
	m := &e.lossMemo
	act, live := m.boardLive(e)
	if !live {
		return 0, false
	}
	// Memoized per object too, except while a derivation guard is live;
	// verify mode recomputes every hit. The memo is battlefield-only: a
	// graveyard read is answered through the pre-move look-back clone, which
	// shares the live object's ObjID, so caching its graveyard answer under
	// that id would answer for the live object's new zone.
	memo := o.Zone == state.ZBattlefield && e.derivedMemoUsable()
	gen := m.gen
	var hit abilityLossEnt
	if memo && int(o.ID) < len(m.ents) && m.ents[o.ID].gen == gen {
		if hit = m.ents[o.ID]; !abilityLossMemoVerify {
			return hit.ts, hit.lost
		}
	}
	var types []string
	typed := false
	var latest uint32
	lost := false
	if m.landSetters {
		// CR 305.7: a land whose subtype an effect set loses its rules-text
		// abilities, but never an ability another effect granted it -- so
		// this loss leaves latest alone (grantLostTo's "removal newer than
		// the grant" stays false for it).
		types, lost = e.typesAndLandSetActive(act, o.ID)
		typed = true
	}
	for i := range act {
		r := &act[i]
		if r.Layer != LAbilities || !r.RemoveAbilities {
			continue
		}
		if !typed {
			types, typed = e.typeCharacteristicsActive(act, o.ID, 0), true
		}
		if e.removalApplies(r, o, types) {
			lost = true
			if r.Timestamp > latest {
				latest = r.Timestamp
			}
		}
	}
	if hit.gen != 0 && (hit.ts != latest || hit.lost != lost) {
		panic(fmt.Sprintf("rules: ability-loss memo stale for obj %d at build %d: cached (%d, %v), fresh (%d, %v)",
			o.ID, m.seq, hit.ts, hit.lost, latest, lost))
	}
	// The walk is a pure read, but a nested read inside it could in
	// principle rebuild active(); boardLive would then have moved to a new
	// generation, and an answer read off the old list is not recorded.
	if memo && m.gen == gen && m.seq == e.activeBuildSeq {
		m.store(o.ID, gen, latest, lost, cap(e.G.Objs)+1)
	}
	return latest, lost
}

// printedAbilitiesLost reports whether o's printed abilities are removed by a
// "loses all abilities" effect.
func (e *Engine) printedAbilitiesLost(o *state.Object) bool {
	_, lost := e.abilityLoss(o)
	return lost
}

// printedAbilitiesGone is the gate every printed-face scan shares: the
// object's printed triggered, activated, mana and static abilities do not
// exist while it is face down (CR 708.8, faceDownPrintedHides) or while a
// "loses all abilities" effect applies to it (CR 613.1f).
func (e *Engine) printedAbilitiesGone(o *state.Object) bool {
	return e.faceDownPrintedHides(o) || e.printedAbilitiesLost(o)
}

// grantLostTo reports whether an ability the continuous effect ce grants to o
// is removed by a "loses all abilities" effect applying to o: layer 6 applies
// in timestamp order, so the grant survives exactly when no such removal is
// newer than it (a tie applies the removal first, compareContinuousPtr).
func (e *Engine) grantLostTo(ce *ContinuousEffect, o *state.Object) bool {
	ts, lost := e.abilityLoss(o)
	return lost && ts > ce.Timestamp
}

// abilityLossProof is the sticky proof that active() can hold no "loses all
// abilities" effect, so the gates above answer without building active() --
// the trigger walk and the replacement dispatch run once per emitted event,
// and almost no match carries such an effect (trigger_grantfree.go's
// reasoning and shape). A remover reaches active() only from a registered
// effect (AddContinuous, or a direct registry write a fixture makes) or from
// the static scan, every face of which is a face of some object: its card's,
// its CopyFace (built from another object's face) or a merged card's. So
// while no registered effect removes abilities and no object's faces name
// RemoveAllAbilities$ on a static or in an SVar body (a static granted from
// one), there is none. Objects are checked as they are appended; any hit ends
// the proof for the rest of the match. Clone and the look-back observer copy
// it.
type abilityLossProof struct {
	seen    bool
	objs    int // e.G.Objs[:objs] checked
	contPtr *ContinuousEffect
	contLen int // e.continuous header last checked
}

// abilityLossPossible reports whether some active() entry may be a "loses
// all abilities" effect (false only while the proof holds).
func (e *Engine) abilityLossPossible() bool {
	pr := &e.lossProof
	if pr.seen {
		return true
	}
	if n := len(e.continuous); n != pr.contLen || (n > 0 && &e.continuous[0] != pr.contPtr) {
		for i := range e.continuous {
			if e.continuous[i].RemoveAbilities || state.SetsBasicLandSubtypes(&e.continuous[i]) {
				pr.seen = true
				return true
			}
		}
		pr.contLen, pr.contPtr = n, nil
		if n > 0 {
			pr.contPtr = &e.continuous[0]
		}
	}
	for ; pr.objs < len(e.G.Objs); pr.objs++ {
		o := &e.G.Objs[pr.objs]
		if faceMayRemoveAbilities(o.CopyFace) {
			pr.seen = true
			return true
		}
		if o.Card != nil {
			for _, f := range o.Card.Faces {
				if faceMayRemoveAbilities(f) {
					pr.seen = true
					return true
				}
			}
		}
	}
	return false
}

func faceMayRemoveAbilities(f *cards.Face) bool {
	if f == nil {
		return false
	}
	for i := range f.Statics {
		st := &f.Statics[i]
		if st.HasParam(cards.PKRemoveAllAbilities) || st.HasParam(cards.PKRemoveLandTypes) {
			return true
		}
	}
	for _, v := range f.SVars {
		if strings.Contains(v, "RemoveAllAbilities") {
			return true
		}
	}
	return false
}
