package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
)

// proliferateTrigger lowers Mode$ Proliferate ("whenever you proliferate";
// gorge trigmatch/proliferate.go proliferateMatches on the completed
// action's marker): ValidPlayer$ against the proliferating player. A
// ValidCard$ or FirstTime$ refuses, and so does a line outside the
// battlefield (Voidwing Hybrid's graveyard return: the kernel scans the
// battlefield only).
func (lc *lowerCtx) proliferateTrigger(tp *params, tr *lTrigger) error {
	for _, k := range []string{"ValidCard", "FirstTime"} {
		if _, ok := tp.m[k]; ok {
			return refuse("Proliferate trigger %s$", k)
		}
	}
	if tr.Zone != zNone && tr.Zone != zBattlefield {
		return refuse("Proliferate trigger outside the battlefield")
	}
	w, ok := whoOfPlayer(tp.str("ValidPlayer"))
	if !ok {
		return refuse("Proliferate ValidPlayer %q", tp.str("ValidPlayer"))
	}
	tr.Event, tr.Whose = "Proliferate", w
	return nil
}

// tokenCreatedOnce lowers Mode$ TokenCreatedOnce ("whenever you create one
// or more tokens"; gorge): ValidToken$ against each created token,
// ValidPlayer$ against its creator, the line latched once per token batch
// (one resolution: kernel TrigTokenCreated with Tf2Batch). OnlyFirst$ (Akim,
// the Soaring Wind) refuses: gorge reads it as an implicit once-per-turn
// limit on the line and matches any creator, while Forge fires only for the
// named player's first token creation of the turn. A body reading the batch
// refuses (the queued instance carries only the first token).
func (lc *lowerCtx) tokenCreatedOnce(tp *params, tr *lTrigger, t cards.Trigger) error {
	if _, ok := tp.m["OnlyFirst"]; ok {
		return refuse("TokenCreatedOnce OnlyFirst$")
	}
	if tr.Zone != zNone && tr.Zone != zBattlefield {
		return refuse("TokenCreatedOnce outside the battlefield")
	}
	if v, ok := tp.get("ValidToken"); ok {
		f, err := lc.filter(v)
		if err != nil {
			return err
		}
		if f.Players != 0 {
			return refuse("TokenCreatedOnce ValidToken %s", f)
		}
		tr.F = f
	}
	if v, ok := tp.get("ValidPlayer"); ok {
		w, ok := whoOfPlayer(v)
		if !ok || v == "" {
			return refuse("TokenCreatedOnce ValidPlayer %q", v)
		}
		tr.Whose = w
	}
	if lc.saReadsBatch(t.Effect) {
		return refuse("TokenCreatedOnce body reads the batch")
	}
	tr.Event, tr.Batch = "TokenCreated", true
	return nil
}

// scrySurveilTrigger lowers Mode$ Scry / Mode$ Surveil ("whenever you scry
// or surveil"; gorge trigmatch cards.go scryMatches / surveilMatches):
// ValidPlayer$ against the scrying or surveilling player. ValidCard$,
// FirstTime$ and ToBottom$ refuse, and so does a line outside the
// battlefield (the kernel scans the battlefield only).
func (lc *lowerCtx) scrySurveilTrigger(tp *params, tr *lTrigger, mode string) error {
	for _, k := range []string{"ValidCard", "FirstTime", "ToBottom"} {
		if _, ok := tp.m[k]; ok {
			return refuse("%s trigger %s$", mode, k)
		}
	}
	if tr.Zone != zNone && tr.Zone != zBattlefield {
		return refuse("%s trigger outside the battlefield", mode)
	}
	w, ok := whoOfPlayer(tp.str("ValidPlayer"))
	if !ok {
		return refuse("%s ValidPlayer %q", mode, tp.str("ValidPlayer"))
	}
	tr.Event, tr.Whose = mode, w
	return nil
}
