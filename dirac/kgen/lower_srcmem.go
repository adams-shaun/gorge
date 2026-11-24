package kgen

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The per-source Remembered (CardDef.SrcMem; dirac/zone_srcmem.go,
// zone_srcmemrem.go). gorge's Remembered also lives on the SOURCE object
// (effects eventRemember, read back through sourceRemembered), so a leaves
// trigger returns what an enters trigger exiled (Hypnox) and a watch trigger
// reads the token an enters trigger copied (Dance of Many). The kernel's set
// is per resolution; a SrcMem card keeps one record per remembered object in
// the cold srcMem store, stamped with the source's ver, and
//
//   - a self-leaves trigger that reads Remembered starts with the records of
//     the incarnation it fired from (SmSeed bit = Abilities index);
//   - a trigger's own ValidCard$ IsRemembered reads the resolution's set
//     united with the source's live records (gorge anyRemembered).
//
// The shape is fail-closed, all of:
//
//   - every Remember* param on the face is in srcMemWriters, with its writer
//     in a T: (default zone or Battlefield) or an A: of the permanent, never
//     an SP$; no Forget* or LKI word anywhere;
//   - a chain that writes neither reads nor clears (the writer's own
//     resolution reads nothing the store holds);
//   - a chain that reads does so only as Defined$ Remembered / DirectRemembered
//     in a self-leaves trigger (Mode$ ChangesZone, Origin$ Battlefield,
//     ValidCard$ Card.Self, Destination$ Any or Graveyard), optionally closed
//     by Cleanup ClearRemembered$ True;
//   - a watch trigger (leaves, Destination$ Any) reads Remembered only as its
//     own ValidCard$ <type>.IsRemembered;
//   - no static, replacement or other trigger parameter names Remembered, so
//     Remembered$ counts, RememberedController/Owner and Player.IsRemembered
//     refuse.
//
// A card with no reader (Cloudshift's one-chain flicker) or whose reader also
// writes (Grindstone's loop) is not this shape and keeps its old route.
var srcMemWriters = map[[2]string]string{
	{"ChangeZone", "RememberChanged"}:    "effects/zone_change.go effChangeZone",
	{"ChangeZone", "RememberTargets"}:    "effects/parent_targets.go rememberChosenTargets",
	{"ChangeZoneAll", "RememberChanged"}: "effects/zone_changeall.go effChangeZoneAll",
	{"CopyPermanent", "RememberTokens"}:  "effects/copypermanent.go effCopyPermanent",
	{"Token", "RememberTokens"}:          "effects/token.go applyTokenMintRiders",
}

// srcMemWatchValid is a watch trigger's ValidCard$: one clause ending in
// IsRemembered.
var srcMemWatchValid = regexp.MustCompile(`^[A-Za-z]+\.IsRemembered$`)

// smChain is what a chain of abilities does with the remembered set.
type smChain struct{ w, r, clr bool }

// srcMemNamesRemember reports whether a param text names a remembered set:
// the word itself, or a card SVar whose text does (except skip, the link to
// the next chain node).
func (lc *lowerCtx) srcMemNamesRemember(k, v, skip string) bool {
	if strings.Contains(k, "Remember") || strings.Contains(v, "Remember") {
		return true
	}
	for _, tok := range strings.FieldsFunc(v, func(r rune) bool {
		return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
	}) {
		if tok == skip {
			continue
		}
		if sv, ok := lc.face.SVars[tok]; ok && strings.Contains(sv, "Remember") {
			return true
		}
	}
	return false
}

// srcMemChain classifies one ability chain (the SA and its Sub links).
func (lc *lowerCtx) srcMemChain(sa *cards.SA) (c smChain, ok bool) {
	for n := sa; n != nil; n = n.Sub {
		for k, v := range n.Params {
			if strings.Contains(k, "Forget") || strings.Contains(v, "Forget") || strings.Contains(k, "LKI") || strings.Contains(v, "LKI") {
				return c, false
			}
			v = strings.TrimSpace(v)
			switch {
			case strings.HasPrefix(k, "Remember"):
				// the writer params; a ChangeZone writer names its own
				// objects (no Hidden$ search or choice)
				if _, w := srcMemWriters[[2]string{n.API, k}]; !w || v != "True" {
					return c, false
				}
				if n.API == "ChangeZone" {
					if _, hidden := n.Params["Hidden"]; hidden {
						return c, false
					}
					_, tgts := n.Params["ValidTgts"]
					_, def := n.Params["Defined"]
					if !tgts && !def {
						return c, false
					}
				}
				c.w = true
			case k == "Defined" && (v == "Remembered" || v == "DirectRemembered"):
				c.r = true
			case n.API == "Cleanup" && k == "ClearRemembered" && v == "True" && len(n.Params) == 1:
				c.clr = true
			case k == "ValidTgts" && n == sa && sa.Kind == "AB" && srcMemTgtValue(v):
				// an activated ability's own target filter reads the remembered
				// set without a seed: the target-time source is threaded in
				// (srcMemTgtReader); a trigger's or a sub-ability's stays refused
			case lc.srcMemNamesRemember(k, v, n.Params["SubAbility"]):
				return c, false
			}
		}
	}
	return c, true
}

// srcMemSelfLeaves: Mode$ ChangesZone, Origin$ Battlefield, ValidCard$
// Card.Self, Destination$ Any or Graveyard.
func srcMemSelfLeaves(t *cards.Trigger) bool {
	p := t.Params
	d := strings.TrimSpace(p["Destination"])
	return t.Mode == "ChangesZone" && strings.TrimSpace(p["Origin"]) == "Battlefield" &&
		strings.TrimSpace(p["ValidCard"]) == "Card.Self" && (d == "Any" || d == "Graveyard")
}

// srcMemWatch: a leaves trigger (Destination$ Any) whose ValidCard$ is
// <type>.IsRemembered.
func srcMemWatch(t *cards.Trigger) bool {
	p := t.Params
	return t.Mode == "ChangesZone" && strings.TrimSpace(p["Origin"]) == "Battlefield" &&
		strings.TrimSpace(p["Destination"]) == "Any" && srcMemWatchValid.MatchString(strings.TrimSpace(p["ValidCard"]))
}

// srcMemShape reports whether the card is the SrcMem shape, and its seed:
// the abilities (bit = Abilities index) whose resolution starts with the
// source's kept set.
func (lc *lowerCtx) srcMemShape() (seed uint32, ok bool) {
	kind, ok := lc.srcMemScript()
	if !ok || !lc.srcMemLoweredClean() {
		return 0, false
	}
	return lc.srcMemSeed(kind)
}

// srcMemScript is srcMemShape's script half: the face is clean, a writer and
// a reader are classified. A writer being lowered asks it before the lowered
// half exists (RememberTargets$, lower_zone.go); the card's end check
// (refuseCrossAbilityRemembered) still asks srcMemShape whole.
func (lc *lowerCtx) srcMemScript() (kind []uint8, ok bool) {
	if !lc.srcMemFaceClean() {
		return nil, false
	}
	writer, ok := lc.srcMemAbilities()
	if !ok {
		return nil, false
	}
	kind, tw, reader, ok := lc.srcMemTriggers()
	if !ok || !(writer || tw) || !reader && !lc.srcMemTgtReader() {
		return nil, false
	}
	return kind, true
}

// srcMemTgtReader reports a direct ValidTgts$ that reads the remembered set
// next to ExiledWithSource. That target's source is threaded in
// (matchCardTgt -> matchCardExwTgt, zone_exiledwithtgt.go), so the kernel
// reads the source's live records; a target filter without ExiledWithSource
// gets no source there and stays refused.
func (lc *lowerCtx) srcMemTgtReader() bool {
	for _, a := range lc.face.Abilities {
		if srcMemTgtValue(a.Params["ValidTgts"]) {
			return true
		}
	}
	return false
}

// srcMemTgtValue is a target filter that reads the remembered set next to
// ExiledWithSource: the two words together are the shape whose source is
// threaded into the matcher.
func srcMemTgtValue(v string) bool {
	v = strings.TrimSpace(v)
	return strings.Contains(v, "IsRemembered") && strings.Contains(v, "ExiledWithSource")
}

// srcMemWriterOK reports whether the card is the SrcMem shape by its script,
// so a writer's remembered set is kept on the source for its readers.
func (lc *lowerCtx) srcMemWriterOK() bool {
	_, ok := lc.srcMemScript()
	return ok
}

// srcMemFaceClean: no static or replacement names a remembered set.
func (lc *lowerCtx) srcMemFaceClean() bool {
	for _, s := range lc.face.Statics {
		for k, v := range s.Params {
			if lc.srcMemNamesRemember(k, v, "") {
				return false
			}
		}
	}
	for _, r := range lc.face.Repls {
		for k, v := range r.Params {
			if lc.srcMemNamesRemember(k, v, "") {
				return false
			}
		}
	}
	return true
}

// srcMemAbilities checks the A:/SP$ chains: a writer sits in an activated
// ability of the permanent; none reads or clears.
func (lc *lowerCtx) srcMemAbilities() (writer, ok bool) {
	for _, a := range lc.face.Abilities {
		c, ok := lc.srcMemChain(a)
		if !ok || c.r || c.clr || c.w && a.Kind != "AB" {
			return false, false
		}
		writer = writer || c.w
	}
	return writer, true
}

// srcMemTriggers classifies each face trigger: kind[i] is 1 for a self-leaves
// reader, 2 for a watch trigger, 0 otherwise.
func (lc *lowerCtx) srcMemTriggers() (kind []uint8, writer, reader, ok bool) {
	kind = make([]uint8, len(lc.face.Triggers))
	for i := range lc.face.Triggers {
		t := &lc.face.Triggers[i]
		watch := srcMemWatch(t)
		for k, v := range t.Params {
			if k == "TriggerDescription" || k == "Execute" || watch && k == "ValidCard" {
				continue
			}
			if lc.srcMemNamesRemember(k, v, "") {
				return nil, false, false, false
			}
		}
		var c smChain
		if t.Effect != nil {
			if c, ok = lc.srcMemChain(t.Effect); !ok {
				return nil, false, false, false
			}
		}
		switch {
		case c.w:
			if c.r || c.clr || watch {
				return nil, false, false, false
			}
			if z := strings.TrimSpace(t.Params["TriggerZones"]); z != "" && z != "Battlefield" {
				return nil, false, false, false
			}
			writer = true
		case c.r:
			if !srcMemSelfLeaves(t) || watch {
				return nil, false, false, false
			}
			kind[i] = 1
			reader = true
		case watch:
			kind[i] = 2
			reader = true
		case c.clr:
			return nil, false, false, false
		}
	}
	return kind, writer, reader, true
}

// srcMemLoweredClean: no lowered static or replacement reads Remembered.
func (lc *lowerCtx) srcMemLoweredClean() bool {
	for _, s := range lc.out.Statics {
		if kremReads(fmt.Sprint(s.F)) || kremReads(fmt.Sprint(s.Cond)) {
			return false
		}
	}
	for _, r := range lc.out.Repls {
		for _, in := range r.Prog {
			if kremReads(in.String()) {
				return false
			}
		}
	}
	return true
}

// srcMemSeed walks the lowered abilities: one that reads Remembered without
// writing it must be a classified trigger (kind), a self-leaves reader reading
// in its program only (its bit joins the seed) or a watch trigger reading in
// its filter only.
func (lc *lowerCtx) srcMemSeed(kind []uint8) (seed uint32, ok bool) {
	for _, a := range lc.out.Abilities {
		var head []string
		if a.Trigger != nil {
			head = append(head, fmt.Sprint(a.Trigger.F2), fmt.Sprint(a.Trigger.Cond))
		}
		head = append(head, fmt.Sprint(a.ActCond))
		tgtReads := false
		for _, t := range a.Targets {
			head = append(head, fmt.Sprint(t.F))
			tgtReads = tgtReads || kremReads(fmt.Sprint(t.F))
		}
		writes, progReads, headReads := false, false, false
		for _, in := range a.Prog {
			writes = writes || kremWrites(in)
			progReads = progReads || kremReads(in.String())
		}
		for _, l := range head {
			headReads = headReads || kremReads(l)
		}
		filterReads := a.Trigger != nil && kremReads(fmt.Sprint(a.Trigger.F))
		if !headReads && !progReads && !filterReads {
			continue
		}
		if a.Trigger == nil {
			// an activated target filter's remembered read: the source's
			// live records are read at target time (srcMemTgtReader), so it
			// needs no seed; a read in the activation condition, the program
			// or a latent copy is another shape
			if writes || progReads || !tgtReads || kremReads(fmt.Sprint(a.ActCond)) || a.Latent || a.Kind != akActivated {
				return 0, false
			}
			continue
		}
		if writes || headReads || a.Latent || !a.OriginOK || a.Origin&0x80 == 0 {
			return 0, false
		}
		i := a.Origin & 0x7f
		if i >= len(kind) {
			return 0, false
		}
		switch {
		case kind[i] == 1 && !filterReads && a.Index < 32:
			seed |= 1 << a.Index
		case kind[i] == 2 && !progReads:
		default:
			return 0, false
		}
	}
	return seed, true
}
