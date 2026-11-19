package kgen

import "strings"

// forEachOpp reads TargetsForEachPlayer$ True over an opponents-only target
// filter (Celebrate the Mountain-king, Lich's Relic,
// Omega, Heartless Evolution, The Theorist's -2).
//
// gorge: TargetsForEachPlayer$ True sets TgtForEachPlayer
// (engine/effects/params/targets.go:272-274) and TargetMax$ OneEach is the
// literal spelling TgtMaxOneEach (:230-232). The ask
// (engine/effects/targets_ask.go:127-150; rules' oneEachTargetBounds is the
// same) respells max as the number of DISTINCT CONTROLLERS among the
// candidates, then clamps it to the candidate count, and groups the options
// one per controller (:275-299).
//
// The kernel reads it for any number of seats (CR 601.2c, 115.1): a literal
// TargetMax$ 1 is a plain "up to one" slot (gorge's max is min(1, distinct),
// one group's single pick), and TargetMax$ OneEach is the kernel's
// different-controllers slot (TgfDiffController) bounded at the most
// opponents a game seats (oppOneEachMax): diffCtrlCap clamps the live ask to
// the distinct controllers among the candidates, exactly gorge's respelled
// max, and multiCands offers no candidate whose controller a pick of the
// slot already has (gorge's Option.Group exclusivity), seat-exact. In a two-player game
// that is the single opponent's one group, as before. The which-opponent
// ask (opponentPick) is not posed for this shape.
//
// "Each other player" (SVar PlayerCountOther$Amount, Kaya, Spirits' Justice's
// -2) and YouDontCtrl are opponents and OppCtrl in a game without teams
// (filter.go maps YouDontCtrl to whoOpp), so they lower the same way.
//
// otherOneEach reports the accepted PlayerCountOther$ shape, whose bounds the
// caller sets directly (value.go knows no PlayerCountOther head, and that
// alias is deliberately not added). It requires the same opponent-permanent
// filter forEachOpp does, so any other shape still reaches the value reads and
// refuses with their message.
func (lc *lowerCtx) otherOneEach(ps *params, f *lFilter, zone int) bool {
	fe, _ := ps.get("TargetsForEachPlayer")
	mn, _ := ps.get("TargetMin")
	mx, _ := ps.get("TargetMax")
	return strings.TrimSpace(fe) == "True" && strings.TrimSpace(mn) == "0" && strings.TrimSpace(mx) == "OneEach" &&
		strings.TrimSpace(lc.face.SVars["OneEach"]) == "PlayerCountOther$Amount" && oppPermFilter(f, zone)
}

// oppPermFilter: the filter targets permanents only, every alternative OppCtrl.
func oppPermFilter(f *lFilter, zone int) bool {
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 || zone != zBattlefield {
		return false
	}
	for _, c := range f.Clauses {
		if c.Ctrl != whoOpp {
			return false
		}
	}
	return true
}

func (lc *lowerCtx) forEachOpp(ps *params, f *lFilter, t *lTarget) error {
	v, ok := ps.get("TargetsForEachPlayer")
	if !ok {
		return nil
	}
	if strings.TrimSpace(v) != "True" {
		return refuse("TargetsForEachPlayer$ shape %q", v)
	}
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 || t.Zone != zBattlefield {
		return refuse("TargetsForEachPlayer$ shape %q: target filter is not opponent-controlled permanents", v)
	}
	if lc.livingEach(ps, f, t) {
		return nil
	}
	for _, c := range f.Clauses {
		if c.Ctrl != whoOpp {
			return refuse("TargetsForEachPlayer$ shape %q: a non-OppCtrl alternative", v)
		}
	}
	if mn, ok := ps.get("TargetMin"); !ok || strings.TrimSpace(mn) != "0" || !t.Min.isConst() || t.Min.N != 0 {
		return refuse("TargetsForEachPlayer$ shape %q: TargetMin$ is not 0", v)
	}
	mx, _ := ps.get("TargetMax")
	mx = strings.TrimSpace(mx)
	oneEach := lc.oppOneEach(ps) || lc.otherOneEach(ps, f, t.Zone)
	if !oneEach && (!t.Max.isConst() || t.Max.N != 1) {
		return refuse("TargetsForEachPlayer$ shape %q: TargetMax$ %q", v, mx)
	}
	if !oneEach {
		t.Max = cst(1)
		return nil
	}
	if t.Unique || t.SameCtrl || t.SharedCrType || t.DistinctName || t.Divided != 0 || t.ParentPlus1 != 0 {
		return refuse("TargetsForEachPlayer$ OneEach beside another set constraint")
	}
	t.DiffCtrl = true
	t.Max = cst(oppOneEachMax)
	return nil
}

// oppOneEachMax bounds a OneEach slot: one target for each opponent, at most
// the three opponents of a four-seat game (dirac MaxSeats-1 <= kop.MaxTargets).
const oppOneEachMax = 3

// oppOneEach reports TargetsForEachPlayer$ True | TargetMax$ OneEach over the
// opponents' count (SVar OneEach PlayerCountOpponents$Amount).
func (lc *lowerCtx) oppOneEach(ps *params) bool {
	fe, _ := ps.get("TargetsForEachPlayer")
	mx, _ := ps.get("TargetMax")
	return strings.TrimSpace(fe) == "True" && strings.TrimSpace(mx) == "OneEach" &&
		strings.TrimSpace(lc.face.SVars["OneEach"]) == "PlayerCountOpponents$Amount"
}

// livingEach reads "for each player, up to one target permanent that player
// controls" (TargetMax$ OneEach, SVar OneEach PlayerCountPlayers$Amount, TargetMin$
// 0: Nils, Discipline Enforcer) over a battlefield permanent filter of any
// controller. gorge sizes the ask at the number of distinct controllers among
// the candidates and groups the options one per controller
// (engine/effects/targets_ask.go: TgtForEachPlayer, Option.Group), so the
// choice is a set of at most one permanent per controller. That is the
// kernel's different-controllers slot (TgfDiffController: no two chosen
// objects share a controller) whose bound is the living players' count, an
// "up to" the distinct controllers can never exceed; livingBound (lower.go's
// addTarget) reads that count as the two seats of the two-player kernel
// (twoSeat-at-caller: livingBound marks the card).
func (lc *lowerCtx) livingEach(ps *params, f *lFilter, t *lTarget) bool {
	mn, _ := ps.get("TargetMin")
	mx, _ := ps.get("TargetMax")
	if strings.TrimSpace(mx) != "OneEach" || strings.TrimSpace(mn) != "0" || !t.Min.isConst() || t.Min.N != 0 ||
		strings.TrimSpace(lc.face.SVars["OneEach"]) != "PlayerCountPlayers$Amount" || !oneEachPerms(f, t.Zone) {
		return false
	}
	t.DiffCtrl = true
	return true
}

// oneEachPerms: the filter targets battlefield permanents only.
func oneEachPerms(f *lFilter, zone int) bool {
	return f != nil && f.Players == 0 && len(f.Clauses) > 0 && zone == zBattlefield
}
