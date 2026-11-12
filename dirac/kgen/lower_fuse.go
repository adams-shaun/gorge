package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Fuse (CR 702.102): a split card printed with fuse may be cast from the
// hand as both halves at once, one fused split spell (gorge's "fuse" cast:
// rules legal_walk_hand.go, split.go fusedSplitFaces, fuseCost and
// resolveFused). Each half still lowers as the card's own face (the front
// and face:<card>#1, cast alone as before); the fused spell lowers as one
// more pool entry, fuse:<card>, the front's CardDef.FuseDef:
//
//   - its record is the halves combined (CR 702.102b, 709.4d): the mana
//     costs summed (CR 702.102c; gorge's fuseCost), the colours and the
//     types joined, so a mana value, colour or type read of the spell on
//     the stack sees both halves (gorge state.SplitHalves with FlagFused);
//   - its one spell ability is the left half's, the right half's chain
//     lowered after it into the same program (CR 702.102d: the left half's
//     instructions, then the right's). The target slots follow in gorge's
//     target-stage order (stage 0 the left half's, stage 1 the right's), and
//     each half's chain reads its own face's SVars and only its own target
//     slots, as gorge's runFusedHalves gives each half a fresh context.
//
// It shows the front (Face 0) and is only ever on the stack: the kernel
// turns the hand card to it for the cast and back to the front as it leaves
// (dirac/face_fuse.go). Refused by name: halves of different card types
// (gorge's fusedTimingOK reads each half's speed, the kernel the fused
// record's), an {X}, hybrid or Phyrexian cost on either half (gorge sums the
// printed costs, which those shapes do not add), a right half with a
// keyword or a line beyond its one spell ability (the fused record reads
// only the left half's lines), and a half whose chain targets below its
// root: gorge's fused cast announces only each half's root targets and asks
// a SubAbility$ link's at resolution (rules target_unique_chain.go
// castSubAskLinks returns no link for a fuse cast), where CR 601.2c chooses
// every target on cast, as the fused record's slots do (Double Jump //
// Flying Kick's "target creature an opponent controls").

const fuseKeyPrefix = "fuse:"

// fuseKey is the pool name of card's fused split spell.
func fuseKey(card string) string { return fuseKeyPrefix + card }

// fuseCard reports whether c is a split card printed with fuse (on either
// half: gorge's fusedSplitFaces reads both). castableFaceLayout keeps a
// Room or an aftermath half out of FaceSplit beside it.
func fuseCard(c *cards.Card) bool {
	return c.AlternateMode == cards.AlternateSplit && len(c.Faces) == 2 && c.Faces[0] != nil && c.Faces[1] != nil &&
		(c.Faces[0].HasKeyword("Fuse") || c.Faces[1].HasKeyword("Fuse"))
}

// fuseLayoutRefusal names why fuse card c's fused spell cannot lower, or
// "" when it can.
func fuseLayoutRefusal(c *cards.Card) string {
	f0, f1 := c.Faces[0], c.Faces[1]
	if f0.IsInstant() != f1.IsInstant() || f0.IsSorcery() != f1.IsSorcery() || !f0.IsInstant() && !f0.IsSorcery() {
		return "Fuse over halves that are not both instants or both sorceries"
	}
	for _, f := range c.Faces {
		mc, err := parsePrintedManaCost(f.ManaCost)
		if err != nil || mc.None || mc.X || len(mc.Hybrid) > 0 || len(mc.Twobrid) > 0 || len(mc.Phyrexian) > 0 {
			return "Fuse with an {X}, hybrid, Phyrexian or absent half cost"
		}
	}
	for _, k := range f1.Keywords {
		if cards.KeywordHead(k) != "Fuse" {
			return "Fuse with a keyword on the right half"
		}
	}
	if len(f1.Abilities) != 1 || f1.SpellAbility() == nil || len(f1.Triggers) != 0 || len(f1.Statics) != 0 || len(f1.Repls) != 0 {
		return "Fuse with a right half line beyond its spell ability"
	}
	return ""
}

// lowerFusedEntry lowers fuse key k: card's fused split spell.
func lowerFusedEntry(p *pool, k string) *lCard {
	out := &lCard{Name: k}
	name, ok := strings.CutPrefix(k, fuseKeyPrefix)
	c := p.byName[name]
	lc := &lowerCtx{p: p, name: k, card: c, out: out, mode: -1, known: p.knownSubtype}
	if !ok || c == nil || !fuseCard(c) || castableFaceLayout(c) != kop.FaceSplit {
		lc.refuseNode("card", "fuse key %q", k)
		return out
	}
	if r := fuseLayoutRefusal(c); r != "" {
		lc.refuseNode("K:Fuse", "%s", r)
		return out
	}
	lc.face = c.Faces[0]
	out.printed = lc.face.Name
	out.FaceMode, out.Face, out.FusedSpell = kop.FaceSplit, 0, true
	out.Faces = faceNames(name, c, kop.FaceSplit)
	lc.fuseAlt = c.Faces[1]
	lc.lowerFace() // the left half, then lowerFusedHalf for the right
	return out
}

// lowerFusedHalf folds the right half into the fused entry lowerFace has
// lowered from the left half: its record (CR 709.4d) and its spell chain
// after the left half's in the one spell ability (CR 702.102d).
func (lc *lowerCtx) lowerFusedHalf() {
	alt := lc.fuseAlt
	lc.fuseAlt = nil
	out := lc.out
	mc, err := parsePrintedManaCost(alt.ManaCost)
	if err != nil {
		lc.refuseNode("ManaCost", "%v", err)
		return
	}
	out.Cost = addManaCost(out.Cost, mc)
	for _, t := range alt.Types {
		if _, ok := typeBits[t]; ok {
			continue // the same card types as the left half (fuseLayoutRefusal)
		}
		if !strings.Contains(" "+strings.Join(out.Subtypes, " ")+" ", " "+t+" ") {
			out.Subtypes = append(out.Subtypes, t)
		}
	}
	out.Colors = joinColors(out.Colors, colorsOf(alt))
	for _, body := range alt.SVars {
		if b, ok := strings.CutPrefix(body, "Count$"); ok && strings.EqualFold(strings.TrimSpace(b), "Converge") || strings.Contains(body, "Count$CastTotalManaSpent") {
			out.SpendRead = true // as lowerFace reads the left half's
		}
	}
	var spell *lAbility
	for _, a := range out.Abilities {
		if a.Kind == akSpell {
			spell = a
		}
	}
	sa := alt.SpellAbility()
	if spell == nil || sa == nil {
		lc.refuseNode("card", "fused spell without both halves' spell abilities")
		return
	}
	lc.guard("A[0] "+sa.Kind+"$ "+sa.API+" (fused right half)", func() error {
		saved := lc.face
		defer func() { lc.face = saved }()
		lc.face = alt // its SVars
		lc.ab = spell
		// a fresh chain: the right half reads only its own target slots
		// (gorge runFusedHalves' per-half Ctx), as lowerAbility starts one
		lc.chainT, lc.xPaidAb, lc.colGrant, lc.colTgt, lc.remTgt = nil, nil, false, lRef{}, 0
		return lc.chain(sa, newParams(sa.Params))
	})
	for _, t := range spell.Targets {
		if !t.Decl || t.SubDecl || t.AtRes {
			lc.refuseNode("K:Fuse", "Fuse with a SubAbility$ link's target: gorge's fused cast asks it at resolution (castSubAskLinks), CR 601.2c announces it on cast")
			return
		}
	}
}

// joinColors is the WUBRG-ordered union of two colour strings.
func joinColors(a, b string) string {
	var out strings.Builder
	for i := 0; i < 5; i++ {
		if c := "WUBRG"[i]; strings.IndexByte(a, c) >= 0 || strings.IndexByte(b, c) >= 0 {
			out.WriteByte(c)
		}
	}
	return out.String()
}
