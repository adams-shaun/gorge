package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// animateReplacements reads an Animate's Replacements$ (gorge
// animateGrant.replacements: one continuous replacement ability granted to
// the animated object for the animation's duration). The one supported shape
// is the self "if this permanent would leave the battlefield, exile it
// instead" line (Llanowar Greenwidow, Realmbreaker, Geth): the kernel carries
// it on the override record as Override.ExileOnLeave, so it rides the record's
// lifetime and dieZone consults it with the holder as the affected object
// (dirac/repl_counters.go exileOnLeaveN). The carriers' R: line names no
// Destination$, so gorge fires it for every departure: leave redirects a
// bounce or tuck to exile too (overrideLeaveZone), and a batch departure that
// bypasses leave stalls rather than diverge (fireLeave). Every other
// replacement shape refuses: the record has no general replacement field.
func (lc *lowerCtx) animateReplacements(ps *params, o *lOverride) error {
	v, ok := ps.get("Replacements")
	if !ok {
		return nil
	}
	name := strings.TrimSpace(v)
	if name == "" || strings.Contains(name, ",") {
		return refuse("Animate Replacements$ %q", v)
	}
	line, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok || line.Event != "Moved" {
		return refuse("Animate Replacements$ %q", name)
	}
	rp := newParams(line.Params)
	rp.consume("Event", "ReplaceWith")
	if rp.str("ActiveZones") != "Battlefield" || rp.str("Origin") != "Battlefield" ||
		strings.TrimSpace(rp.str("ValidCard")) != "Card.Self" {
		return refuse("Animate Replacements$ %q: not the self leave-to-exile line", name)
	}
	w := cards.ResolveSVar(lc.face.SVars, line.Params["ReplaceWith"])
	if w == nil {
		return refuse("Animate Replacements$ %q: no ReplaceWith$ body", name)
	}
	var rl lRepl
	done, err := lc.selfExile(rp, w, &rl)
	if err != nil {
		return err
	}
	if !done || rl.Kind != "ExileOnLeave" || rl.AnyZone || rl.Ctr != "" {
		return refuse("Animate Replacements$ %q", name)
	}
	if left := rp.unread(); len(left) > 0 {
		return refuse("Animate Replacements$ %q: unread %s$", name, left[0])
	}
	o.ExileOnLeave = true
	ps.consume("Replacements")
	return nil
}
