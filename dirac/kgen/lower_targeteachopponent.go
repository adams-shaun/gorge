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
// This reduction holds ONLY in a two-player game (the kernel is two-player):
// every alternative is OppCtrl, so every candidate has the one opponent as
// its controller -- max is 1 when any candidate exists and 0 otherwise, min is
// 0, and the single group is the whole set. That is exactly TargetMin$ 0 |
// TargetMax$ 1, which the kernel already lowers. The which-opponent ask
// (opponentPick) is posed only with two or more living opponents, so it never
// arises here. Any other shape refuses.
//
// "Each other player" (SVar PlayerCountOther$Amount, Kaya, Spirits' Justice's
// -2) and YouDontCtrl are the same two-player identities as "opponents" and
// OppCtrl (filter.go maps YouDontCtrl to whoOpp). Lowered Max 1 over that
// candidate set equals gorge's distinct-owner count (targets_ask.go:127-148):
// one owner group, so the same single ask over the same candidates.
// twoSeat-at-caller: forEachOpp, below, marks the card.
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
	oneEach := mx == "OneEach" && (strings.TrimSpace(lc.face.SVars["OneEach"]) == "PlayerCountOpponents$Amount" || lc.otherOneEach(ps, f, t.Zone))
	if !oneEach && (!t.Max.isConst() || t.Max.N != 1) {
		return refuse("TargetsForEachPlayer$ shape %q: TargetMax$ %q", v, mx)
	}
	lc.twoSeat("one target for each opponent read as one target")
	t.Max = cst(1)
	return nil
}
