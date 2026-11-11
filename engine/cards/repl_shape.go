package cards

// replPrimitive is the registry label for one R: line: "repl:"+Event, or
// "repl:<Event>.OtherShape" for an event whose implemented family covers only
// one shape of the line (the api:Craft.OtherShape precedent), so registering
// the event name cannot certify a shape the engine leaves inert.
func replPrimitive(f *Face, r Repl) string {
	switch r.Event {
	case "BeginTurn":
		if !BeginTurnExtraTurnSkipShape(r) {
			return "repl:BeginTurn.OtherShape"
		}
	case "CopySpell":
		if _, ok := CopySpellExtraOf(f, r); !ok {
			return "repl:CopySpell.OtherShape"
		}
	}
	return "repl:" + r.Event
}

// BeginTurnExtraTurnSkipShape reports whether a BeginTurn line is the
// implemented skip-an-extra-turn shape (Trouble in Pairs, Stranglehold,
// Ugin's Nexus, Gerrard's Hourglass Pendant: ExtraTurn$ True | Skip$ True,
// no ReplaceWith$). Time Vault's normal-turn optional skip is not.
func BeginTurnExtraTurnSkipShape(r Repl) bool {
	_, hasWith := r.Param(PKReplaceWith)
	return r.ParamStr(PKExtraTurn) == "True" && r.ParamStr(PKSkip) == "True" &&
		r.With == nil && !hasWith
}

// CopySpellExtraOf parses an R:Event$ CopySpell line of the Twinning Staff
// shape -- ActiveZones$ Battlefield | ValidSpell$ Spell | ReplaceWith$ <S>,
// <S> = DB$ ReplaceEffect | VarName$ Amount | VarValue$ <V>, and
// <V> = ReplaceCount$Amount/Plus.N with N >= 1 -- returning N. Any other body
// reports false (the line is then not registered: repl:CopySpell.OtherShape).
func CopySpellExtraOf(f *Face, r Repl) (int, bool) {
	if f == nil || r.Event != "CopySpell" || r.With == nil || r.With.API != "ReplaceEffect" {
		return 0, false
	}
	if r.ParamStr(PKActiveZones) != "Battlefield" || r.ParamStr(PKValidSpell) != "Spell" {
		return 0, false
	}
	if r.With.ParamStr(PKVarName) != "Amount" {
		return 0, false
	}
	body, ok := f.SVars[r.With.ParamStr(PKVarValue)]
	const prefix = "ReplaceCount$Amount/Plus."
	if !ok || len(body) <= len(prefix) || body[:len(prefix)] != prefix {
		return 0, false
	}
	n := 0
	for _, d := range body[len(prefix):] {
		if d < '0' || d > '9' || n > 1000 {
			return 0, false
		}
		n = n*10 + int(d-'0')
	}
	return n, n >= 1
}
