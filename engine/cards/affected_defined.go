package cards

import (
	"strconv"
	"strings"
)

// affectedDefinedFilter maps an `AffectedDefined$` value onto the filter
// property the legacy `Affected$` form carried for the same object.
var affectedDefinedFilter = map[string]string{
	"Self":            "Self",
	"Enchanted":       "EnchantedBy",
	"Equipped":        "EquippedBy",
	"AttachedBy Self": "AttachedBy",
}

// NormalizeAffectedDefined rewrites a static's AffectedDefined key (plus an
// optional Affected type filter) into the single legacy Affected filter, the
// type with the defined object as a property, that every static reader in
// this module already understands, and drops the AffectedDefined key.
//
// Upstream Forge moved ~2,600 static lines to the AffectedDefined form
// between FORGE_REF 95f04e8 and fb4d809 (an Affected filter of creatures
// equipped by the source became an AffectedDefined of the equipped object
// with a separate Creature type filter). The mapping here
// reproduces the old line exactly for every pair measured across the two
// corpora. A value it does not know, or an Affected$ with comma
// alternatives, is left untouched, so the param census still reports it
// rather than the static quietly changing scope.
func NormalizeAffectedDefined(p map[string]string) {
	// Card-Forge acaef15177 spelled a Spacecraft's STATION N+ static
	// `AffectedDefined$ Self | Station$ <N>`: the effects hold while the source
	// has at least N charge counters. Read it as the Affected$
	// Card.counters_GE<N>_CHARGE spelling it replaced (which the rewrite below
	// turns into Card.Self+counters_GE<N>_CHARGE), so gorge's static walk and
	// dirac's kgen see one form. Only a Continuous static on its own source
	// with a literal threshold 1..255 is read; any other shape keeps its
	// Station key, which no reader takes, so the param census and kgen still
	// report it.
	if n, err := strconv.Atoi(p["Station"]); err == nil && n >= 1 && n <= 255 && p["Station"][0] >= '1' && p["Station"][0] <= '9' &&
		p["Mode"] == "Continuous" && p["AffectedDefined"] == "Self" && p["Affected"] == "" {
		p["Affected"] = "Card.counters_GE" + p["Station"] + "_CHARGE"
		delete(p, "Station")
	}
	defined, ok := p["AffectedDefined"]
	if !ok {
		return
	}
	prop, known := affectedDefinedFilter[strings.TrimSpace(defined)]
	if !known {
		return
	}
	affected := strings.TrimSpace(p["Affected"])
	if strings.Contains(affected, ",") {
		return
	}
	if affected == "" {
		affected = "Card"
	}
	typ, props, _ := strings.Cut(affected, ".")
	out := typ + "." + prop
	if props != "" {
		out += "+" + props
	}
	p["Affected"] = out
	delete(p, "AffectedDefined")
}

// parseStaticParams is parseParams for a static body: the one place every
// static reader in this package gets its params, so the AffectedDefined$
// rewrite cannot be missed by one of them.
func parseStaticParams(val string) map[string]string {
	p := parseParams(val)
	NormalizeAffectedDefined(p)
	return p
}
