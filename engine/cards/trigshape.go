// Trigger-line shape helpers: the trigger-line grammar words the CR 605.1b
// triggered-mana-ability machinery (trig:ManaAdded, Caged Sun) reads, spelled
// once here where the vocabulary lives, so every reader compares the same
// spelling. parseParams trims every parameter value at load, so these read
// the stored values directly.

package cards

// saAbilityPrefix is the ValidSA$ SpellAbility. spec prefix.
const saAbilityPrefix = "SpellAbility."

// StaticTrue reports whether the trigger line's Static$ parameter reads true,
// Forge's marker for a CR 605.1b triggered mana ability (Caged Sun's two
// `T:Mode$ ManaAdded | Static$ True` lines, the TapsForMana mana doublers').
// The two spellings are the corpus's; face_hasmanaability.go reads the same
// pair.
func (t *Trigger) StaticTrue() bool {
	v := t.ParamStr(PKStatic)
	return v == "True" || v == "true"
}

// ProducedChosenColour reports whether the trigger line's Produced$ names the
// trigger source's own as-enters colour ("ChosenColor", Caged Sun's lines).
func (t *Trigger) ProducedChosenColour() bool {
	return t.ParamStr(PKProduced) == "ChosenColor"
}

// PlayerIsYou reports whether the trigger line's Player$ binds the event to
// the trigger source's own controller ("You", Caged Sun's lines).
func (t *Trigger) PlayerIsYou() bool {
	return t.ParamStr(PKPlayer) == "You"
}

// ManaAbilityMark reads Forge's ValidSA$ SpellAbility.ManaAbility mark: it
// reports whether v names that mark and, when it does, whether it asks for a
// mana ability (true) or a non-mana ability (".!ManaAbility", false). Any
// other ValidSA$ shape is not the mark: present is false.
func ManaAbilityMark(v string) (present, wantMana bool) {
	if len(v) <= len(saAbilityPrefix) || v[:len(saAbilityPrefix)] != saAbilityPrefix {
		return false, false
	}
	v = v[len(saAbilityPrefix):]
	negated := false
	if len(v) > 0 && v[0] == '!' {
		negated, v = true, v[1:]
	}
	if v != "ManaAbility" {
		return false, false
	}
	return true, !negated
}

// ProducedIsChosen reports whether a DB$ Mana body's Produced$ names the
// producing source's own as-enters colour ("Chosen", the body spelling of the
// trigger line's "ChosenColor"; mana_production.go's token grammar).
func ProducedIsChosen(v string) bool {
	return v == "Chosen" || v == "ChosenColor"
}

// manaAddedLineKeys are the trigger-line parameters the CR 605.1b fold
// (pay.PaymentPlanFoldableManaAdded) accepts on a Mode$ ManaAdded line:
// Mode, the ValidSource$ Land / ValidSA$ SpellAbility.ManaAbility and
// Produced$ ChosenColor / Player$ You gates, the Secondary$ rider, the
// Execute$ body link and the presentation text. ValidSource$/ValidSA$
// themselves are NOT vetted by the classifier: the planner matches the line
// through the real matcher against a synthetic ManaAdd under the mana-ability
// facts, which evaluates them exactly as the resolution will.
var manaAddedLineKeys = [...]ParamKey{
	PKMode, PKValidSource, PKValidSA, PKProduced, PKPlayer, PKExecute,
	PKStatic, PKTriggerZones, PKSecondary, PKTriggerDescription,
}

// ManaAddedLineKeysAdmitted reports whether every key of the trigger line's
// Params is one of manaAddedLineKeys (the closed world counted as the total
// minus the listed keys' hits; a key with no ParamKey constant is always
// foreign).
func (t *Trigger) ManaAddedLineKeysAdmitted() bool {
	foreign := len(t.Params)
	for _, k := range manaAddedLineKeys {
		if t.HasParam(k) {
			foreign--
		}
	}
	return foreign == 0
}

// manaAddedBodyKeys are the parameters the same fold accepts on the line's
// DB$ Mana body (its Produced$/Amount$, the Defined$ its reader vets, the
// presentation text); the parser keeps API$ out of Params.
var manaAddedBodyKeys = [...]ParamKey{
	PKProduced, PKAmount, PKDefined, PKSpellDescription, PKStackDescription,
}

// ManaBodyKeysAdmitted is ManaAddedLineKeysAdmitted over an ability body.
func ManaBodyKeysAdmitted(sa *SA) bool {
	foreign := len(sa.Params)
	for _, k := range manaAddedBodyKeys {
		if sa.HasParam(k) {
			foreign--
		}
	}
	return foreign == 0
}
