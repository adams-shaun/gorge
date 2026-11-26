package kgen

import "strings"

// "you may play/cast <cards> from the top of your
// library" (MayPlay$ True | AffectedZone$ Library, TopLibrary filters).
//
// gorge's may-play walk offers exactly one library card: lib[0] of the
// PRIORITY player's own library (engine/rules/legal_mayplay.go), then
// matches it against Affected$ like any other zone, where TopLibrary is
// always true. So the word is implied by the zone and is stripped before
// lc.filter; the kernel offers only the top card (tryLibTop). Every comma
// alternative must carry TopLibrary (else a deeper library card would
// match in gorge's static walk) and a YouCtrl / YouOwn word (an opponent's
// top card is never offered, and the shape stays refused).
//
// MayPlayLimit$ 1 (Assemble the Players, Johann) lowers WITH
// StOncePerTurn: gorge caps a limited static once per turn PER STATIC
// (engine/rules/mayplay.go mayPlayKeyLimitReached), the same as
// Forge and XMage, so a second, different top card is not played through
// the same static that turn.

// libTopAffected strips TopLibrary from each alternative of a library
// permission's Affected$.
func libTopAffected(aff string) (string, error) {
	alts := strings.Split(aff, ",")
	for i, alt := range alts {
		dot := strings.IndexByte(alt, '.')
		if dot < 0 {
			return "", refuse("MayPlay Library %q", aff)
		}
		words := strings.Split(alt[dot+1:], "+")
		kept := words[:0:0]
		top, mine := false, false
		for _, w := range words {
			switch w {
			case "TopLibrary":
				top = true
				continue
			case "YouCtrl", "YouOwn":
				mine = true
			}
			kept = append(kept, w)
		}
		if !top || !mine {
			return "", refuse("MayPlay Library %q", aff)
		}
		alts[i] = alt[:dot]
		if len(kept) > 0 {
			alts[i] += "." + strings.Join(kept, "+")
		}
	}
	return strings.Join(alts, ","), nil
}

// libTopRefusals refuses the riders a library permission does not model.
func libTopRefusals(sp *params) error {
	for _, k := range []string{"MayPlayIgnoreType", "MayPlayWithoutManaCost", "MayPlayIgnoreColor", "RaiseCost"} {
		if _, ok := sp.get(k); ok {
			return refuse("MayPlay Library with %s$", k)
		}
	}
	return nil
}

// libTopAfterStack folds a library permission's ValidAfterStack$ spell
// qualifier (Thundermane Dragon's Spell.Creature+powerGE4) into the stripped
// Affected$ alternatives. gorge asks the qualifier of every non-land card the
// permission offers (engine/rules/mayplay.go: the spell view of a card still
// in its origin zone) and never of a land (CR 305.1), so an alternative that
// excludes lands with nonLand is narrowed by the qualifier's words; one that
// admits lands is refused (Glarb's Land.TopLibrary alternative), as is a
// qualifier that is not one Spell.<words> alternative.
func libTopAfterStack(sp *params, aff string) (string, error) {
	q, ok := sp.get("ValidAfterStack")
	if !ok {
		return aff, nil
	}
	words, found := strings.CutPrefix(q, "Spell.")
	if !found || words == "" || strings.ContainsAny(words, ", ") {
		return "", refuse("MayPlay Library with ValidAfterStack$ %q", q)
	}
	alts := strings.Split(aff, ",")
	for i, alt := range alts {
		if !strings.Contains("+"+alt[strings.IndexByte(alt, '.')+1:]+"+", "+nonLand+") {
			return "", refuse("MayPlay Library with ValidAfterStack$ beside an alternative that admits lands: %q", aff)
		}
		alts[i] = alt + "+" + words
	}
	sp.consume("ValidAfterStack")
	return strings.Join(alts, ","), nil
}

// libTopAffectedOK reports whether libTopAffected would accept aff: every
// comma alternative carries TopLibrary and a YouCtrl / YouOwn word. It is
// the look-permission shape check (the solved Case of the Locked Hothouse's
// MayLookAt$ You beside its MayPlay$ True), so the look covers exactly the
// cards the permission does; the Affected$ itself is not rewritten here.
func libTopAffectedOK(aff string) bool {
	_, err := libTopAffected(aff)
	return err == nil
}

// libTopLook lowers a Continuous static's MayLookAt$ grant, the view-only
// top-of-library look (and, through faceDownLookOnly and handRevealOnly, the
// other two view-only looks). It returns the same error shape as the rest of
// the lowering: a refusal names the shape it does not model.
func (lc *lowerCtx) libTopLook(sp *params, st *lStatic, aff string) error {
	ml, _ := sp.get("MayLookAt")
	az, _ := sp.get("AffectedZone")
	// "Players play with the top card of their libraries revealed"
	// (Field of Dreams, Lantern of Insight, Wizened Snitches): the
	// Affected$ spec carries no controller conjunct, so the grant covers
	// every player's own top card -- gorge's MayLookAt$ Player branch
	// (layers.go) emits it with MayLookAtAll and the library-top read
	// (layers_static.go LibraryTopRevealed) matches the top card of the
	// reading player's own library, which the TopLibrary predicate pins.
	libReveal := aff == "Card.TopLibrary" && ml == "Player"
	// the solved Case of the Locked Hothouse's look: every alternative
	// of the permission's Affected$ carries TopLibrary and a YouCtrl/
	// YouOwn word, so the look covers exactly the cards the permission
	// does
	topLib := libTopAffectedOK(aff)
	libLook := (libReveal || topLib) && az == "Library" && (ml == "You" || ml == "True" || ml == "Player")
	if libLook && !sp.is("MayPlay", "True") {
		if ez, ok := sp.get("EffectZone"); ok && ez != "Battlefield" {
			return refuse("MayLookAt EffectZone %q", ez)
		}
		sp.consume("AffectedZone")
		st.Kind = "" // view-only: lower to nothing
		return nil
	}
	if faceDownLookOnly(sp.m) || handRevealOnly(sp.m) { // lower_handreveal.go
		sp.consume("AffectedZone")
		st.Kind = "" // view-only: lower to nothing
		return nil
	}
	// Future Sight / Magus of the Future, and the solved Case of the
	// Locked Hothouse: the library-top look beside MayPlay$ True on the
	// same line. gorge's static scan emits the look as its own
	// view-only ContinuousEffect (layers.go's MayLookAt branch copies
	// base, which carries no MayPlay) next to the MayPlay grant, so the
	// line is the Courser of Kruphix pair on one line: the look lowers
	// to nothing and the permission takes the MayPlay path below
	// (mayPlayStatic, the same library-top rules).
	if libLook {
		if sp.is("MayPlay", "True") {
			// the permission lowers on through mayPlayStatic, which reads
			// the AffectedZone$ itself (the library-top rules)
			sp.consume("MayLookAt")
			_, err := lc.mayPlayStatic(sp, st, aff)
			return err
		}
		sp.consume("AffectedZone")
		st.Kind = "" // view-only: lower to nothing
		return nil
	}
	return refuse("MayLookAt shape")
}
