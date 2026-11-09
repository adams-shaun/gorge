package kgen

import (
	"fmt"
	"strings"
)

// Remembered persisting on the source (remembered-persist):
// gorge's Remembered also lives on the SOURCE object (effects eventRemember,
// read back through sourceRemembered), so Journey to Nowhere's leaves
// trigger returns the creature its enters trigger exiled, and Mesmeric
// Fiend's returns the card its enters trigger took. The kernel's remembered
// set is per resolution (resolve.go), so this one card shape is lowered with
// CardDef.RemPersist: the enters trigger's remembered objects are kept in
// the kernel's exile links (gen.links, keyed by the source's battlefield
// incarnation; zone_rememberpersist.go) when its resolution ends with the
// source still that incarnation, and the leaves trigger of that incarnation
// starts its resolution with them as its remembered set. A source that left
// before its enters trigger resolved has a new incarnation, which no leaves
// trigger reads (CR 610.3c's "O-ring trick": the card stays exiled).
//
// The shape, exactly: two triggers and nothing else that reads or writes
// Remembered. T0: Mode$ ChangesZone, Origin$ Any, Destination$ Battlefield,
// ValidCard$ Card.Self, whose body remembers (RememberTargets$ or
// RememberChanged$); T1: Mode$ ChangesZone, Origin$ Battlefield,
// Destination$ Any or Graveyard (the dies form, Hoarding Dragon),
// ValidCard$ Card.Self, whose body is ChangeZone Defined$
// Remembered Origin$ Exile to the battlefield or hand, optionally followed
// by Cleanup ClearRemembered$ True — or Unidentified Hovership's manifest
// form: the body is ManifestDread DefinedPlayer$ RememberedOwner (Amount$
// 1, one dread), optionally followed by the same Cleanup. Every other
// ability on the face (Crew 1) must be remembered-clean: it neither reads
// nor writes the set, so its resolutions neither spend nor rewrite what
// the pair carries (its Keep re-keeps the seeded set unchanged).
func (lc *lowerCtx) remPersistShape() bool {
	f := lc.face
	if len(f.Statics) != 0 || len(f.Repls) != 0 || len(f.Triggers) != 2 {
		return false
	}
	for _, a := range f.Abilities {
		// a raw-script scan, fail-closed: any Remember* key or value in
		// the ability or its sub-abilities refuses
		for s := a; s != nil; s = s.Sub {
			for k, v := range s.Params {
				if strings.Contains(k, "Remember") || strings.Contains(v, "Remember") {
					return false
				}
			}
		}
	}
	t0, t1 := &f.Triggers[0], &f.Triggers[1]
	if t0.Mode != "ChangesZone" || t1.Mode != "ChangesZone" {
		return false
	}
	is := func(m map[string]string, k, v string) bool { return strings.TrimSpace(m[k]) == v }
	if !is(t0.Params, "Origin", "Any") || !is(t0.Params, "Destination", "Battlefield") || !is(t0.Params, "ValidCard", "Card.Self") ||
		!is(t1.Params, "Origin", "Battlefield") || !(is(t1.Params, "Destination", "Any") || is(t1.Params, "Destination", "Graveyard")) || !is(t1.Params, "ValidCard", "Card.Self") {
		return false
	}
	w := t0.Effect
	if w == nil || w.Sub != nil || !(is(w.Params, "RememberTargets", "True") || is(w.Params, "RememberChanged", "True")) {
		return false
	}
	r := t1.Effect
	switch {
	case r != nil && r.API == "ChangeZone":
		if !is(r.Params, "Defined", "Remembered") || !is(r.Params, "Origin", "Exile") ||
			!(is(r.Params, "Destination", "Battlefield") || is(r.Params, "Destination", "Hand")) {
			return false
		}
	case r != nil && r.API == "ManifestDread":
		// the leave trigger manifests dread for the remembered card's
		// owner (its ETB exiled it), then clears the set; SubAbility$
		// stays in Params beside the resolved Sub
		if !is(r.Params, "DefinedPlayer", "RememberedOwner") {
			return false
		}
		for k := range r.Params {
			switch k {
			case "DefinedPlayer", "SubAbility":
			default:
				return false
			}
		}
	case r != nil && r.API == "Token":
		// the leave trigger's token for the exiled card's owner, sized by
		// its mana value (Skyclave Apparition): only if the card is still
		// exiled with the source (ConditionPresent$ Card.ExiledWithSource
		// over the remembered card), the one script, the owner and the X/X
		// sides and nothing else
		if !is(r.Params, "TokenOwner", "RememberedOwner") || !is(r.Params, "ConditionDefined", "Remembered") ||
			!is(r.Params, "ConditionPresent", "Card.ExiledWithSource") {
			return false
		}
		for k := range r.Params {
			switch k {
			case "TokenScript", "TokenOwner", "ConditionDefined", "ConditionPresent", "TokenPower", "TokenToughness", "SubAbility":
			default:
				return false
			}
		}
	default:
		return false
	}
	if c := r.Sub; c != nil && (c.API != "Cleanup" || c.Sub != nil || !is(c.Params, "ClearRemembered", "True") || len(c.Params) != 1) {
		return false
	}
	for k, v := range w.Params {
		if strings.Contains(v, "Remembered") || k == "Defined" && strings.Contains(v, "Remember") {
			return false
		}
	}
	return true
}

// remPersistSaga is a Saga whose later chapter reads what an earlier chapter
// remembered (The Measure of a Man: chapter I exiles up to one target with
// RememberChanged$, chapter III returns the remembered card from exile and
// clears the set). The two are linked abilities (CR 607.2a): gorge's chapter
// trigger resolves with an empty remembered set and no capture
// (rules/saga.go checkChapterTriggers, TriggerContext.Chapter), so the
// reading chapter's Defined$ Remembered reads the Saga's persistent list
// (effects chapterLinkedRemembered -> sourceRemembered), which a battlefield
// re-entry clears (CR 400.7).
// The kernel carries it on the links as the enters/leaves pair does
// (dirac/zone_rememberpersist.go), split by ability: the writing chapters keep
// their finished set (keep), only the reading chapters start with it (seed),
// so a chapter between them neither spends nor rewrites it.
//
// The shape, exactly: every ability is a chapter and no static or
// replacement reads Remembered; a writing chapter writes the set only by an
// Exile that remembers what it moved and reads no Remembered; a reading
// chapter writes nothing, reads Remembered only as a return from exile of
// the remembered objects, ends with ClearRemembered, and comes after every
// writing chapter. Any other read (a count, a filter, a chapter that both
// writes and reads) refuses.
func (lc *lowerCtx) remPersistSaga() (seed, keep uint32, ok bool) {
	if lc.out.SagaN == 0 || len(lc.out.Abilities) == 0 || len(lc.out.Abilities) > 32 {
		return 0, 0, false
	}
	for _, s := range lc.out.Statics {
		if kremReads(fmt.Sprint(s.F)) || kremReads(fmt.Sprint(s.Cond)) {
			return 0, 0, false
		}
	}
	for _, r := range lc.out.Repls {
		for _, in := range r.Prog {
			if kremReads(in.String()) {
				return 0, 0, false
			}
		}
	}
	lastW, firstR := 0, 1<<30
	for _, a := range lc.out.Abilities {
		if a.Name != "Chapter" || a.Latent || a.Gate == nil || a.Index != int(a.Index&31) {
			return 0, 0, false
		}
		var heads []string
		if t := a.Trigger; t != nil {
			heads = append(heads, fmt.Sprint(t.F), fmt.Sprint(t.F2), fmt.Sprint(t.Cond))
		}
		heads = append(heads, fmt.Sprint(a.ActCond))
		for _, t := range a.Targets {
			heads = append(heads, fmt.Sprint(t.F))
		}
		for _, h := range heads {
			if kremReads(h) {
				return 0, 0, false
			}
		}
		w, r := false, false
		for i, in := range a.Prog {
			switch {
			case kremWrites(in):
				if in.Op != "Exile" || !strings.Contains(in.CS, "Remember") || kremReads(in.String()) {
					return 0, 0, false
				}
				w = true
			case in.Op == "ClearRemembered":
				if !r || i != len(a.Prog)-1 {
					return 0, 0, false
				}
			case kremReads(in.String()):
				if in.Op != "Return" || in.Obj.Kind != rRemembered || !strings.Contains(in.CS, "from=Exile") || in.F != nil {
					return 0, 0, false
				}
				r = true
			}
		}
		switch {
		case w && r:
			return 0, 0, false
		case w:
			keep |= 1 << a.Index
			lastW = max(lastW, a.Gate.Level)
		case r:
			if a.Prog[len(a.Prog)-1].Op != "ClearRemembered" {
				return 0, 0, false
			}
			seed |= 1 << a.Index
			firstR = min(firstR, a.Gate.Level)
		}
	}
	if seed == 0 || keep == 0 || firstR <= lastW {
		return 0, 0, false
	}
	return seed, keep, true
}
