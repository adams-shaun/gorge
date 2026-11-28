package rules

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// untapReplacementSources walks all zones from which an UntapOtherPlayer
// static can apply. The order is stable and also admits the command-zone
// plane shape in the corpus. A phased-out permanent is treated as though it
// does not exist (CR 702.26b), so its static is no source.
func (e *Engine) untapReplacementSources(each func(id state.ObjID)) {
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if o := e.G.Obj(id); o != nil && o.PhasedOut {
				continue
			}
			each(id)
		}
		for _, id := range e.G.Zone(state.ZCommand, p) {
			each(id)
		}
	}
}

// hasUntapStepChoice recognizes Forge's bare keyword line verbatim. It is
// intentionally a characteristic check, not a card-name list: all 45 corpus
// carriers, including future cards with the same wording, take this path.
func hasUntapStepChoice(o *state.Object) bool {
	if o == nil || o.Face() == nil {
		return false
	}
	for _, k := range o.Face().Keywords {
		if strings.TrimSpace(k) == "You may choose not to untap CARDNAME during your untap step." {
			return true
		}
	}
	return false
}

// untapAdjustCap reads the active player's UntapAdjust grant: Forge spells the
// player keyword `UntapAdjust:<filter>:<N>` (Static Orb's `Affected$ Player`
// static), so at most N permanents matching <filter> untap during that
// player's untap step. The grant reaches this reader through playerKeywords,
// the same player-keyword surface shroud and hexproof grants use. With more
// than one grant the smallest N is the effective cap.
func (e *Engine) untapAdjustCap(p state.PlayerID) (spec string, n int, ok bool) {
	for _, kw := range e.playerKeywords(p) {
		head, rest, found := strings.Cut(strings.TrimSpace(kw), ":")
		if !found || kwHeadOf(head).ID != kwhUntapAdjust.ID {
			continue
		}
		filter, rawN, found := strings.Cut(rest, ":")
		if !found {
			continue
		}
		filter = strings.TrimSpace(filter)
		limit, err := strconv.Atoi(strings.TrimSpace(rawN))
		if filter == "" || err != nil || limit < 0 {
			continue
		}
		if !ok || limit < n {
			spec, n, ok = filter, limit, true
		}
	}
	return spec, n, ok
}

// untapAdjustCandidates returns the tapped permanents of p's battlefield that
// this untap step would untap and that the cap's filter counts. The gates
// mirror finishUntapStep's scan: a phased-out, exerted, one-shot-locked,
// denied or stun-counter-replaced permanent does not untap, so it neither
// consumes the cap nor appears in the election.
func (e *Engine) untapAdjustCandidates(p state.PlayerID, spec string) []state.ObjID {
	ctx := e.specCtx(0, p)
	var out []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if o == nil || o.PhasedOut || !o.Tapped || o.UntapChoice == "keep" {
			continue
		}
		if o.ExertSkipUntap || o.CantUntapNextStep || e.untapDenied(id) {
			continue
		}
		if o.Counter("STUN") > 0 && !e.UntapBlocked(id) {
			continue
		}
		if !e.matchesSpec(spec, id, ctx) {
			continue
		}
		out = append(out, id)
	}
	return out
}

// untapAdjustElection poses the cap election when the active player controls
// more tapped permanents than the cap lets untap. Options are the permanents
// that may stay tapped, in battlefield order; the answer folds one Choose
// "keep" per selected object so the resumed scan (and replay) skips it.
func (e *Engine) untapAdjustElection(spec string, limit int) bool {
	p := e.G.Active
	cands := e.untapAdjustCandidates(p, spec)
	if len(cands) <= limit {
		return false
	}
	keep := len(cands) - limit
	opts := make([]decision.Option, len(cands))
	for i, id := range cands {
		label := "Keep tapped"
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name != "" {
			label = "Keep " + o.Face().Name + " tapped"
		}
		opts[i] = decision.Option{Index: i, Kind: "keep_tapped", Obj: id, Label: label}
	}
	e.untapResume = &untapStep{next: 0, capDecided: true}
	e.choosing = chooseUntapCap
	e.ask(&decision.Decision{Player: p, Kind: decision.KChoose, Min: keep, Max: keep,
		Prompt: "Choose permanents to keep tapped", Options: opts})
	return true
}

// answerUntapAdjust records Static Orb's cap election: each chosen option is a
// permanent the player decided to keep tapped. It folds one Choose "keep" per
// object before resuming the scan at index 0, marking the continuation so the
// phase-in and day/night actions do not run twice.
func (e *Engine) answerUntapAdjust(chosen []decision.Option) {
	e.choosing = chooseNone
	for _, o := range chosen {
		e.emit(events.Event{Kind: events.Choose, Obj: o.Obj, Counter: "untap", Text: "keep"})
	}
	e.untapResume = &untapStep{next: 0, capDecided: true}
	if e.finishUntapStep(0) {
		e.finishEnteredStep()
	}
}

// untapTurnPermanent is the one turn-step caller of effects.TryUntap. The
// effect helper also owns ability untaps, so stun counters replace either
// kind of untap identically (CR 122.1d).
func (e *Engine) untapTurnPermanent(subject state.ObjID) {
	effects.TryUntap(e, subject)
}

// UntapBlocked reports whether a CantHappen Untap replacement would stop id from
// becoming untapped at this point in the turn.
func (e *Engine) UntapBlocked(id state.ObjID) bool {
	return e.gameEventCantHappen("Untap", events.Event{Kind: events.Untap, Obj: id})
}

// staticPresentHolds implements a static's IsPresent$/PresentCompare$ gate.
// The default compare is presence. It deliberately uses the same
// deterministic zone matcher as trigger intervening-if conditions
// (PresentZone$, the battlefield by default).
func (e *Engine) staticPresentHolds(st cards.Static, source state.ObjID) bool {
	if !e.classBandGateHolds(st.ParamStr(cards.PKClassBand), source) {
		return false
	}
	spec, ok := st.Param(cards.PKIsPresent)
	if !ok {
		return true
	}
	n, known := presentCountInZones(e, st.ParamStr(cards.PKPresentZone), spec, source, e.controllerOf(source))
	if !known {
		return false
	}
	cmp := strings.TrimSpace(st.ParamStr(cards.PKPresentCompare))
	if cmp == "" {
		return n > 0
	}
	return comparePresent(n, e.presentCompareFor(cmp, source, e.controllerOf(source)))
}

// untapOtherStaticsMatch reports whether subject untaps during a foreign
// player's untap step under a matching UntapOtherPlayer static. ValidCard
// defaults to Card.Self; IsPresent$/PresentCompare$ gates are checked before
// the subject filter (Quest for Renewal's four quest counters shape).
func (e *Engine) untapOtherStaticsMatch(subject state.ObjID) bool {
	matched := false
	e.untapReplacementSources(func(id state.ObjID) {
		if matched {
			return
		}
		o := e.G.Obj(id)
		// CR 708.2a: a face-down source (a manifested Seedborn Muse) has
		// no printed statics, nor one that lost all abilities.
		if o == nil || o.Face() == nil || e.printedAbilitiesGone(o) {
			return
		}
		for si, sn := 0, o.PileStaticCount(); si < sn; si++ {
			pst, ok := o.PileStaticAt(si)
			if !ok {
				continue
			}
			st := pst.Static
			if isRoom(o) && pst.Face == o.Face() && !o.DoorUnlocked(int(o.FaceIdx)) {
				continue
			}
			if st.Mode != "UntapOtherPlayer" || !e.staticPresentHolds(st, id) {
				continue
			}
			spec := st.ParamStr(cards.PKValidCard)
			if spec == "" {
				spec = "Card.Self"
			}
			if e.matchesSpecFrom(spec, subject, e.controllerOf(id), id) {
				matched = true
				return
			}
		}
	})
	if matched {
		return true
	}
	// The granted half: an AddStaticAbility$ grant of a Mode$
	// UntapOtherPlayer body (Urban Burgeoning's "Enchanted land has 'Untap
	// this land during each other player's untap step'"). The granted
	// static's source is its host, so ValidCard$ Card.Self names the host,
	// under the same IsPresent$ gate.
	for _, sv := range e.grantedStatics("UntapOtherPlayer") {
		st := cards.Static{Mode: "UntapOtherPlayer", Params: sv.Params}
		if !e.staticPresentHolds(st, sv.Source) {
			continue
		}
		spec := st.ParamStr(cards.PKValidCard)
		if spec == "" {
			spec = "Card.Self"
		}
		if e.matchesSpecFrom(spec, subject, sv.Controller, sv.Source) {
			return true
		}
	}
	return false
}
