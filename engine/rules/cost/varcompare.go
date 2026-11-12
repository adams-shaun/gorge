package cost

import "strings"

// specCompareOps are the comparison suffix spellings of Forge's numeric
// filter predicates (cmcEQX, powerGEX, toughnessLTX, ...).
var specCompareOps = [...]string{"EQ", "NE", "LT", "LE", "GT", "GE"}

// termComparesVar reports whether one filter term is a numeric predicate
// whose right-hand side is exactly the name name ("cmcEQX" for X); a
// non-empty op restricts it to that one operator ("EQ").
func termComparesVar(term, name, op string) bool {
	rest, ok := strings.CutSuffix(term, name)
	if !ok || len(rest) < 3 {
		return false
	}
	got := rest[len(rest)-2:]
	if op != "" && got != op {
		return false
	}
	for _, o := range specCompareOps {
		if got == o {
			return true
		}
	}
	return false
}

// SpecComparesVar reports whether a filter spec carries a numeric predicate
// whose right-hand side is the name name: Sidisi, Regent of the Mire's
// Creature.Other+cmcEQX, Scrap Welder's Artifact.cmcGTX, Kozilek's
// Card.cmcEQX. A spec without the name's letter returns false without
// splitting anything (the one byte scan every caller pays).
func SpecComparesVar(spec, name string) bool { return SpecComparesVarOp(spec, name, "") }

// SpecComparesVarOp is SpecComparesVar restricted to one comparison operator
// ("EQ", "GT", ...; "" admits every one): Nahiri's Sacrifice's
// Creature.cmcEQX is SpecComparesVarOp(spec, "X", "EQ").
func SpecComparesVarOp(spec, name, op string) bool {
	if name == "" || !strings.Contains(spec, name) {
		return false
	}
	for alt := range strings.SplitSeq(spec, ",") {
		_, rest, found := strings.Cut(alt, ".")
		if !found {
			continue
		}
		for term := range strings.SplitSeq(rest, "+") {
			if termComparesVar(term, name, op) {
				return true
			}
		}
	}
	return false
}

// StripVarCompares removes every numeric predicate whose right-hand side is
// name from a filter spec, keeping each alternative's base and its other
// terms: Creature.Other+cmcEQX -> Creature.Other, Artifact.cmcGTX ->
// Artifact. It is the offer-time reading of a predicate whose right-hand
// side does not exist yet (an X the ability's targets fix, CR 601.2c before
// 601.2h): the same "judge it once the value is fixed" carve-out an
// X-bound ValidTgts$ gets before X is announced.
func StripVarCompares(spec, name string) string {
	if !SpecComparesVar(spec, name) {
		return spec
	}
	alts := strings.Split(spec, ",")
	for i, alt := range alts {
		base, rest, found := strings.Cut(alt, ".")
		if !found {
			continue
		}
		var keep []string
		for term := range strings.SplitSeq(rest, "+") {
			if !termComparesVar(term, name, "") {
				keep = append(keep, term)
			}
		}
		if len(keep) == 0 {
			alts[i] = base
		} else {
			alts[i] = base + "." + strings.Join(keep, "+")
		}
	}
	return strings.Join(alts, ",")
}

// SpecHasTerm reports whether some alternative of a filter spec carries the
// exact '+'-joined term term after its base ("ChosenType" in
// Creature.ChosenType).
func SpecHasTerm(spec, term string) bool {
	if !strings.Contains(spec, term) {
		return false
	}
	for alt := range strings.SplitSeq(spec, ",") {
		_, rest, found := strings.Cut(alt, ".")
		if !found {
			continue
		}
		for t := range strings.SplitSeq(rest, "+") {
			if t == term {
				return true
			}
		}
	}
	return false
}

// ReplaceTerm rewrites every exact occurrence of term in a filter spec's
// '+'-joined term lists to with ("" drops the term): Creature.ChosenType
// with "Elf" is Creature.Elf, with "" is Creature.
func ReplaceTerm(spec, term, with string) string {
	if !SpecHasTerm(spec, term) {
		return spec
	}
	alts := strings.Split(spec, ",")
	for i, alt := range alts {
		base, rest, found := strings.Cut(alt, ".")
		if !found {
			continue
		}
		var keep []string
		for t := range strings.SplitSeq(rest, "+") {
			switch {
			case t != term:
				keep = append(keep, t)
			case with != "":
				keep = append(keep, with)
			}
		}
		if len(keep) == 0 {
			alts[i] = base
		} else {
			alts[i] = base + "." + strings.Join(keep, "+")
		}
	}
	return strings.Join(alts, ",")
}
