package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// effectGraveRepl lowers an Effect's ReplacementEffects$ line that exiles
// every card put into a graveyard from anywhere (Yawgmoth's Will, Gaea's
// Will and Magus of the Will's GraveToExile: Event$ Moved | ActiveZones$
// Command | Destination$ Graveyard | ValidCard$ Card.!token+YouOwn |
// ReplaceWith$ Exile): the kernel's ReplExileOnLeave with Repl.C 1, the
// any-zone rewrite the printed cards (Rest in Peace) lower, carried by the
// same emblem as the Effect's statics (an emblem's abilities are its
// statics and its replacement abilities; repl_exileleaveemb.go walks both
// forms). ActiveZones$ Command is the resolving effect's own location, the
// source the kernel's emblems stand in for, so the line consumes it.
//
// effectStaticEOT calls this after its statics lower and attaches the
// returned replacement to the emblem it emits; a line of any other shape
// reads nothing and keeps today's named unread refusal (a shape is never
// silently dropped).
func (lc *lowerCtx) effectGraveRepl(ps *params) (*lRepl, error) {
	name, ok := ps.m["ReplacementEffects"]
	if !ok {
		return nil, nil
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, ",") {
		return nil, nil
	}
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || r.Event != "Moved" {
		return nil, nil
	}
	rp := newParams(r.Params)
	rp.consume("Event", "ReplaceWith")
	if az, has := rp.get("ActiveZones"); has {
		if az != "Command" {
			return nil, nil // Battlefield/Stack spellings are other arms' shapes
		}
		// the resolving effect sits in the command zone, the source the
		// kernel's emblems stand in for: the gate is always open, and
		// exileOnLeave reads the key back, so it is dropped, not marked read
		delete(rp.m, "ActiveZones")
	}
	if rp.str("Destination") != "Graveyard" {
		return nil, nil
	}
	switch o := rp.str("Origin"); o {
	case "", "All", "Any":
	default:
		return nil, nil // an explicit origin is the battlefield-origin form
	}
	vc, vl := rp.str("ValidCard"), rp.str("ValidLKI")
	if vc == "" || vl != "" {
		// exactly a card spec: a ValidLKI scope (Mavinda's remembered spell)
		// is not this arm's shape
		return nil, nil
	}
	if left := rp.unread(); len(left) > 0 {
		return nil, nil
	}
	w := cards.ResolveSVar(lc.face.SVars, r.Params["ReplaceWith"])
	if w == nil {
		// a bare ReplaceWith$ Exile names no SVar: the plain exile body the
		// card parser resolves the named SVar to (effectReplEmblem's arm)
		w = &cards.SA{API: "ChangeZone", Params: map[string]string{"Defined": "ReplacedCard", "Destination": "Exile"}}
	}
	if w.Sub != nil {
		return nil, nil
	}
	ctr, _, bodyOK := exileBody(w)
	if !bodyOK || ctr != "" {
		// a rider or counter body is the census shape class, not this arm's
		return nil, nil
	}
	rl := lRepl{}
	done, err := lc.exileOnLeave(rp, w, &rl)
	if err != nil {
		return nil, err
	}
	if !done || !rl.AnyZone || rl.Kind != "ExileOnLeave" || rl.Ctr != "" {
		return nil, nil
	}
	ps.consume("ReplacementEffects")
	return &rl, nil
}
