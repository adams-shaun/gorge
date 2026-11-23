package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// effectReplEmblem lowers a DB$ Effect with no Duration$ whose one
// ReplacementEffects$ line is the dies-to-exile rewrite (Cry of the
// Carnarium's DeathRep: Event$ Moved | ValidLKI$ Creature | Origin$
// Battlefield | Destination$ Graveyard | ReplaceWith$ Exile): gorge
// registers it as an effect-created replacement that lives until end of
// turn, the kernel's UntilEOT emblem carrying one ReplExileOnLeave
// (dirac/repl_exileleaveemb.go). No Duration$ is UntilEOT: gorge's
// no-Duration Effect lifetime (head comment of lower_damageprevent.go). The
// line lowers through lc.lowerRepl's exileOnLeave arm exactly as a printed
// R: line would and the appended record moves onto the emblem, so the
// filter, the plain ChangeZone body and the battlefield-origin gate read
// once, in the place that owns them.
//
// The controller-relative words of the filter read with the emblem's seat as
// the source controller at run time (repl_exileleaveemb.go). A candidate
// beside another matching dies-to-exile source stalls there; the CR 616.1
// order ask that names an emblem candidate is deliberately not implemented.
//
// It reports false, consuming nothing, for any other shape so effectPrevent
// keeps today's refusal name (the census classes do not churn).
func (lc *lowerCtx) effectReplEmblem(ps *params) (bool, error) {
	for k := range ps.m {
		if k != "ReplacementEffects" && !ignoredParams[k] {
			return false, nil
		}
	}
	name := strings.TrimSpace(ps.peek("ReplacementEffects"))
	if name == "" || strings.Contains(name, ",") {
		return false, nil
	}
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || r.Event != "Moved" {
		return false, nil
	}
	rp := newParams(r.Params)
	rp.consume("Event", "ReplaceWith")
	if !rp.is("Origin", "Battlefield") || !rp.is("Destination", "Graveyard") {
		return false, nil // any other origin (absent is from anywhere) is not this arm's shape
	}
	if az, has := rp.get("ActiveZones"); has && strings.TrimSpace(az) != "Battlefield" {
		return false, nil
	}
	vc, vl := rp.str("ValidCard"), rp.str("ValidLKI")
	if (vc == "") == (vl == "") {
		return false, nil // exactly one of the two
	}
	if left := rp.unread(); len(left) > 0 {
		return false, nil
	}
	w := cards.ResolveSVar(lc.face.SVars, r.Params["ReplaceWith"])
	if w == nil {
		// a bare ReplaceWith$ Exile names no SVar: the plain exile body the
		// card parser resolves the named SVar to
		w = &cards.SA{API: "ChangeZone", Params: map[string]string{"Defined": "ReplacedCard", "Destination": "Exile"}}
	}
	if w.Sub != nil {
		return false, nil
	}
	if ctr, _, bodyOK := exileBody(w); !bodyOK || ctr != "" {
		// a rider or counter body is the census shape class, not this arm's
		return false, nil
	}
	for i := range lc.out.Repls {
		// a face that already lowers its own dies-to-exile replacement keeps
		// today's refusal (lowerRepl would refuse the pair by name, and the
		// census class would churn)
		if lc.out.Repls[i].Kind == "ExileOnLeave" {
			return false, nil
		}
	}
	spec := vc
	if spec == "" {
		spec = vl
	}
	if _, err := lc.filter(spec); err != nil {
		// the spec does not compile: effectPrevent's own refusal stands
		return false, nil
	}
	at := len(lc.out.Repls)
	if err := lc.lowerRepl(cards.Repl{Event: r.Event, Params: r.Params, With: w}); err != nil {
		return true, err
	}
	rl := lc.out.Repls[at]
	lc.out.Repls = lc.out.Repls[:at]
	if rl.Kind != "ExileOnLeave" || rl.Cond.Kind != cNone || rl.AnyZone {
		return true, refuse("Effect replacement emblem shape")
	}
	ps.consume("ReplacementEffects")
	lc.out.Emblems = append(lc.out.Emblems, lEmblem{Repls: []lRepl{rl}, UntilEOT: true})
	lc.emit(lInstr{Op: "CreateEmblem", Obj: lRef{Kind: rYou}, C: len(lc.out.Emblems) - 1, Note: "the controller gets an emblem that exiles each creature that would die this turn"})
	return true, nil
}
