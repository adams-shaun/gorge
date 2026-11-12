package rules

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// cost_announce_bound.go is the announce-bound cost filter: a Sac part whose
// spec compares a mana value against an X the CASTER announces -- Nahiri's
// Sacrifice's "sacrifice an artifact or creature with mana value X"
// (Sac<1/Creature.cmcEQX;Artifact.cmcEQX>, Announce$ X on the spell's own SA,
// SVar:X:Count$xPaid, no printed {X}).
//
// CR 601.2b announces X before CR 601.2h pays costs, so the sacrificed
// permanent is chosen at the announced value. Before the announcement the
// comparison is judged open (StripVarCompares): any creature or artifact
// makes the offer legal because some X equals its mana value. The X ask
// (announceSacXAsk) offers exactly the mana values a candidate has, and once
// it is answered (pc.xDone) the spec is matched with X bound to pc.x.

// costAnnounceVarName is the variable name an announce-bound cost spec is
// read for: the one corpus carrier spells it X.
const costAnnounceVarName = "X"

// trimParam is the defensive trim on a param or SVar read: the corpus parser
// trims at the boundary (parseParams trims keys and values, the line reader
// right-trims each line), but a hand-built face need not, and an untrimmed
// read would silently miss the announce-bound path. It is bytes.TrimSpace,
// not strings.TrimSpace: the no-strings ratchet
// (internal/codeshape/strings_ratchet_test.go) never rises, and this read
// sits on the per-candidate spec walk.
func trimParam(s string) string { return string(bytes.TrimSpace([]byte(s))) }

// costSpecAnnounceBound reports whether spec, read for source's cost,
// compares cmcEQX against an X the source's spell ability announces. Only
// EQ is bound: it is the one operator the carrier uses, and the exact value
// enumeration in announceSacXAsk (the candidates' own mana values) holds for
// EQ only. A face with a printed {X} announces through the mana path and is
// left alone.
func (e *Engine) costSpecAnnounceBound(spec string, source state.ObjID) bool {
	if !costvocab.SpecComparesVarOp(spec, costAnnounceVarName, "EQ") {
		return false
	}
	o := e.G.Obj(source)
	if o == nil || o.Face() == nil {
		return false
	}
	f := o.Face()
	sa := f.SpellAbility()
	if sa == nil || trimParam(sa.ParamStr(cards.PKAnnounce)) != costAnnounceVarName {
		return false
	}
	if trimParam(f.SVars[costAnnounceVarName]) != "Count$xPaid" {
		return false
	}
	return pay.FaceCostOf(f).Cost.X == 0
}

// matchAnnounceBoundCostSpec is payer.MatchesSpecFrom for an announce-bound
// spec: once the pending cast of source has announced X, the comparison is
// bound to it; before that (the offer gate, the planner) it is open.
func (e *Engine) matchAnnounceBoundCostSpec(spec string, id state.ObjID, you state.PlayerID, source state.ObjID) bool {
	if pc := e.cast; pc != nil && pc.card == source && !pc.isAbility() && pc.xDone {
		sc := e.specCtx(source, you)
		prev := sc.Resolve
		x := pc.x
		sc.Resolve = func(name string) (int32, bool) {
			if name == costAnnounceVarName {
				return x, true
			}
			return prev(name)
		}
		return e.matchesSpec(spec, id, sc)
	}
	return e.matchesSpecFrom(costvocab.StripVarCompares(spec, costAnnounceVarName), id, you, source)
}

// announceSacXAsk poses the X announcement for a cast whose Sac part is
// announce-bound: the options are the distinct mana values the part's
// candidates have (never a value no permanent can pay). Reports whether it
// asked or aborted the cast.
func (e *Engine) announceSacXAsk(pc *pendingCast) bool {
	if pc.isAbility() {
		return false
	}
	for _, part := range pc.cost.Sac {
		if !e.costSpecAnnounceBound(part.Spec, pc.card) {
			continue
		}
		stripped := part
		stripped.Spec = costvocab.StripVarCompares(part.Spec, costAnnounceVarName)
		seen := map[int32]bool{}
		var vals []int32
		for _, oid := range pay.SacrificeCostCandidates(asPayer(e), pc.player, pc.card, stripped, false) {
			o := e.G.Obj(oid)
			if o == nil || o.Face() == nil {
				continue
			}
			if mv := o.Face().ManaValue(); !seen[mv] {
				seen[mv] = true
				vals = append(vals, mv)
			}
		}
		if len(vals) == 0 {
			e.abortCast(pc, "announce sacrifice cost no longer payable; cast aborted", true)
			return true
		}
		sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
		e.askXChoice(pc, vals)
		return true
	}
	return false
}

// askXChoice poses the "Choose a value for X" decision over vals (ascending)
// for the pending cast: the Shoal alternative-cost arm and the announce-bound
// sacrifice share it.
func (e *Engine) askXChoice(pc *pendingCast, vals []int32) {
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Choose a value for X", Source: pc.card}
	for _, x := range vals {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "x",
			Label: fmt.Sprintf("X = %d", x), Amount: int(x)})
	}
	e.choosing = chooseCast
	e.ask(d)
}
