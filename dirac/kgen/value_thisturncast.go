package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// thisTurnCastValue lowers Count$ThisTurnCast_<spec> (the /Op suffix is
// already peeled) to kop.ExThisTurnCastSpec, the kernel's per-push cast
// ring. It follows gorge's evaluation exactly
// (effects/count_body.go evalCountBodyProvenance, then
// rules/host_cast_ledger.go spellsCastThisTurnMatching):
//
//   - a bare !CastSaSource in any alternative is stripped from every
//     alternative and excludes every push of the source object
//     (stripBareCastSaSource + SpellsCastThisTurnMatchingExcluding);
//   - the count is You-scoped (the caster is the controller) when the
//     stripped spec contains "You" anywhere (gorge's substring test);
//   - wasCastFromYourHand[ByYou] is answered per push from its origin zone
//     (castProvenanceAdmits on the ledger's cast view); both spellings are
//     You-scoped, so "by you" is the scope;
//   - CastSa Spell.ManaFromTreasure is answered per push from the cast's
//     spend facts (castProvenanceAdmits; gorge tags a Treasure's mana
//     TypedTreasure/TypedArtifactTreasure): the kernel records the tap of a
//     Treasure source during the spell's build (CastSpecTreasure). A spec
//     that carries the token on some alternatives only is refused, because
//     an alternative without it admits a cast the per-cast bit rejects;
//   - wasCastFromExile is answered per push from its origin zone
//     (castOriginAdmits on the ledger's cast view, CastSpecExile); the
//     bare wasCastFromGraveyard is the CastFlags-or-latest-cast-zone
//     predicate matchesSpec reads on the object now (CastSpecGrave,
//     ); each must ride every
//     alternative;
//   - the rest is matched against the pushed object as it is now, through
//     filter clauses both kernel matchers read exactly.
//
// Every other form refuses with a named reason.
func (lc *lowerCtx) thisTurnCastValue(spec string) (lValue, error) {
	aggregateTypes := false
	if stripped, ok := strings.CutSuffix(spec, "$CardTypes"); ok && !strings.Contains(stripped, "CastSa") {
		// the count-level $CardTypes aggregate
		// (gorge stripCastSourceAggregate then aggregateCastSourceProperty,
		// effects/count_agg.go: the DISTINCT card types among the matching
		// casts, each read where it now is).
		spec, aggregateTypes = stripped, true
	}
	if strings.Contains(spec, "$") {
		// !CastSaSource$<Prop> and the other count-level $<Prop>
		// aggregates over the matching objects (Call Forth the Tempest)
		return lValue{}, refuse("other:count ThisTurnCast %q: a $<Property> aggregate", spec)
	}
	// A non-MayPlay AffectedZone$ Stack static is refused by continuous
	// (lower_trigger.go: "AffectedZone %q without MayPlay"), so the count
	// cannot reach gorge's stackGrantCast grant walk (cascade.go) through a
	// lowered static; a MayPlay Stack static is a cast permission, not a
	// grant walk. The count itself is per push and exact in both.
	alts := strings.Split(spec, ",")
	bits := 0
	tre, ctrlTrig := 0, 0
	for i, alt := range alts {
		var had bool
		// CastSa Spell.ManaFromTreasure: the cast spent mana a Treasure
		// produced (gorge castProvenanceAdmits on the spend facts). gorge
		// strips the token from each alternative; a mixed spec admits a
		// cast that matches the alternative without the token, so the
		// kernel's per-cast bit cannot answer it.
		if alts[i], had = stripCastToken(alt, "CastSa Spell.ManaFromTreasure"); had {
			tre++
		}
		// +ControlledBy TriggeredActivator is
		// resolved by gorge's SpecContext.TriggerContext against the
		// resolving trigger's activating player (rules/host_cast_ledger.go).
		// Only a SpellCast trigger body for any activating player has that
		// player; every other context keeps the spaced-qualifier refusal.
		if alts[i], had = stripCastToken(alts[i], "ControlledBy TriggeredActivator"); had {
			ctrlTrig++
		}
		if alts[i], had = stripCastToken(alts[i], "!CastSaSource"); had {
			bits |= kop.CastSpecExclude
		}
		if _, pos := stripCastToken(alts[i], "CastSaSource"); pos {
			return lValue{}, refuse("other:count ThisTurnCast %q: positive CastSaSource", spec)
		}
	}
	switch {
	case tre == len(alts):
		bits |= kop.CastSpecTreasure
	case tre != 0:
		return lValue{}, refuse("other:count ThisTurnCast %q: treasure provenance on some alternatives only", spec)
	}
	switch {
	case ctrlTrig == len(alts):
		tr := (*lTrigger)(nil)
		if lc.ab != nil {
			tr = lc.ab.Trigger
		}
		if !lc.trigBody || lc.ab == nil || lc.ab.Latent || tr == nil || tr.Event != "SpellCast" || tr.Whose != whoAny {
			return lValue{}, refuse("other:count ThisTurnCast %q: ControlledBy TriggeredActivator outside a SpellCast trigger for any player", spec)
		}
		bits |= kop.CastSpecCtrlTrig
	case ctrlTrig != 0:
		return lValue{}, refuse("other:count ThisTurnCast %q: ControlledBy TriggeredActivator on some alternatives only", spec)
	}
	for i := range alts {
		alts[i] = strings.ReplaceAll(alts[i], "ControlledBy Player.Active", "ActivePlayerCtrl")
	}
	if strings.ContainsAny(strings.Join(alts, ","), " <>") {
		// ControlledBy <role>, named<X>, any other spaced qualifier
		return lValue{}, refuse("other:count ThisTurnCast %q: a spaced or bracketed qualifier", spec)
	}
	// gorge's youScoped reads the spec handed to the ledger: CastSaSource
	// stripped, the provenance tokens still in it ("wasCastFromYourHand"
	// itself says You)
	if strings.Contains(strings.Join(alts, ","), "You") {
		bits |= kop.CastSpecYou
	}
	hand, notHand := 0, 0
	for i, alt := range alts {
		var pos, neg bool
		var h bool
		alt, h = stripCastToken(alt, "wasCastFromYourHandByYou")
		pos = pos || h
		alt, h = stripCastToken(alt, "wasCastFromYourHand")
		pos = pos || h
		alt, h = stripCastToken(alt, "!wasCastFromYourHandByYou")
		neg = neg || h
		alt, h = stripCastToken(alt, "!wasCastFromYourHand")
		neg = neg || h
		if pos && neg {
			return lValue{}, refuse("other:count ThisTurnCast %q: hand and not-hand provenance on one alternative", spec)
		}
		if pos {
			hand++
		}
		if neg {
			notHand++
		}
		alts[i] = alt
	}
	// wasCastFromExile (gorge castOriginTokens: THIS push's origin is
	// exile, any caster) and the bare wasCastFromGraveyard (NOT an origin
	// token in gorge: the effects-side predicate matchesSpec reads on the
	// object as it is now, flashback/harmonize/jump-start/escape or a
	// latest cast from a graveyard).
	// Like the hand bits each must ride every alternative: an alternative
	// without it admits a cast the per-cast test rejects.
	for _, o := range []struct {
		tok string
		bit int
	}{{"wasCastFromExile", kop.CastSpecExile}, {"wasCastFromGraveyard", kop.CastSpecGrave}} {
		had := 0
		for i, alt := range alts {
			var h bool
			if alts[i], h = stripCastToken(alt, o.tok); h {
				had++
			}
		}
		switch {
		case had == len(alts):
			bits |= o.bit
		case had != 0:
			return lValue{}, refuse("other:count ThisTurnCast %q: %s on some alternatives only", spec, o.tok)
		}
	}
	rest := strings.Join(alts, ",")
	switch {
	case hand == len(alts):
		bits |= kop.CastSpecHand
	case notHand == len(alts):
		bits |= kop.CastSpecNotHand
	case hand != 0 || notHand != 0:
		return lValue{}, refuse("other:count ThisTurnCast %q: hand provenance on some alternatives only", spec)
	}
	if strings.Contains(rest, "wasCast") || strings.Contains(rest, "CastSa") {
		// the negated or You-spelled origin tokens, wasCastByYou, CastSa
		// spend and flag tokens: per-cast provenance the ring does not record
		return lValue{}, refuse("other:count ThisTurnCast %q: a cast-provenance token", spec)
	}
	f, err := lc.filter(rest)
	if err != nil {
		return lValue{}, err
	}
	if f.Players != 0 {
		return lValue{}, refuse("other:count ThisTurnCast %q: a player alternative", spec)
	}
	for _, c := range f.Clauses {
		if why := castRingClauseInexact(c); why != "" {
			return lValue{}, refuse("other:count ThisTurnCast %q: %s", spec, why)
		}
	}
	if aggregateTypes {
		bits |= kop.CastSpecTypes
	}
	return lValue{Kind: vThisTurnCastSpec, N: bits, F: f}, nil
}

// stripCastToken is gorge's StripPredicateToken: it removes the exact
// predicate tok from alternative alt's "+" chain after the base.
func stripCastToken(alt, tok string) (string, bool) {
	base, preds, ok := strings.Cut(alt, ".")
	if !ok {
		return alt, false
	}
	parts := strings.Split(preds, "+")
	out := parts[:0]
	had := false
	for _, p := range parts {
		if p == tok {
			had = true
			continue
		}
		out = append(out, p)
	}
	if !had {
		return alt, false
	}
	if len(out) == 0 {
		return base, true
	}
	return base + "." + strings.Join(out, "+"), true
}

// castRingClauseInexact names why a clause cannot be matched exactly by
// both the kernel's perm matcher (a pushed object now on the battlefield)
// and its card matcher (on the stack or in another zone), or "" when it
// can. Card types, colours, subtypes, the controller (owner off the
// battlefield and stack), the mana value, hasXCost (false face down on
// both sides, CR 708.2/708.4), Historic and ActivePlayerCtrl read the same
// object facts on both sides, and so do withFlying / withoutFlying (gorge
// objectHasKeyword over the bound Derived keywords: the perm matcher's kwGen
// on the battlefield, the card matcher's printed keywords elsewhere, none face
// down); and Legendary (gorge hasType: the
// perm matcher's superGen reads the current types, the card matcher the
// printed ones); every other flag and comparison is
// refused. Self and Other would read the pushed object as the source
// (gorge's specCtx(ev.Obj)), so they are refused too.
func castRingClauseInexact(c lClause) string {
	switch {
	case c.Flags&^(fWithFlying|fWithoutFlying|fLegendary) != 0:
		return "a filter flag"
	case c.TypesAll != 0:
		return "an all-of type clause"
	case c.Types&tEmblem != 0:
		return "an emblem alternative"
	case c.BasePermanent:
		return "a Permanent base"
	case c.Owner != whoAny:
		return "an owner selector"
	case c.Ctrl != whoAny && c.Ctrl != whoYou && c.Ctrl != whoOpp:
		return "a controller role"
	case c.CmpStat != cmpNone && c.CmpStat != cmpHasX && c.CmpStat != cmpHistoric && c.CmpStat != cmpActiveCtrl && (c.CmpStat != cmpCMC || c.CmpX != ""):
		return "a comparison other than a constant mana value"
	}
	return ""
}
