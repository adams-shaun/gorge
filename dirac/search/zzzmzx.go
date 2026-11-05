package search

import "github.com/adams-shaun/gorge/dirac"

// zzzmzx.go: the MageZero like-for-like priority keys, each off by default
// and the search exactly as without it.
//
//   - AltCands (:altcands): the special-action and alternative-cast
//     kinds priorityCands otherwise leaves out of its vocabulary (plot,
//     bestow, sneak, a Room's unlock, turning a face-down permanent up) are
//     candidates beside the casts. XMage's MCTS root offers them (Nyxborn
//     Hydra's bestow 1.48 a game, Highway Robbery's plot 1.82); off, the
//     dirac never searches them.
//   - CastDedup (:castdedup): a cast-like candidate (AkCast,
//     AkCastCasualty and the AltCands kinds) whose card, mode and target
//     repeat an earlier candidate's is dropped, so two copies of a card in
//     hand are one candidate. XMage's duplicate-state check merges them
//     (dirac 2.8-15.5 duplicate decisions a seat-game, XMage 0-1.7).
//     Activations stay one per permanent: two permanents of one card can
//     differ (tapped, counters).

// altKind reports whether k is one of AltCands' kinds.
func altKind(k dirac.ActionKind) bool {
	switch k {
	case dirac.AkPlot, dirac.AkBestow, dirac.AkSneak, dirac.AkWebSling, dirac.AkUnlock, dirac.AkTurnFaceUp:
		return true
	}
	return false
}

// castLike reports whether CastDedup merges candidates of kind k.
func castLike(k dirac.ActionKind) bool {
	return k == dirac.AkCast || k == dirac.AkCastCasualty || altKind(k)
}

// castDup reports whether a repeats the card, mode and target of a
// candidate already in pt (CastDedup).
func castDup(s *dirac.State, pt *point, a dirac.Action) bool {
	if !castLike(a.Kind()) {
		return false
	}
	c := s.ObjCard(dirac.Obj(a.Source()))
	for j := 0; j < pt.n; j++ {
		if pt.off[j+1]-pt.off[j] != 1 {
			continue
		}
		b := dirac.Action(pt.acts[pt.off[j]])
		if b.Kind() == a.Kind() && b.Mode() == a.Mode() && b.Target() == a.Target() &&
			s.ObjCard(dirac.Obj(b.Source())) == c {
			return true
		}
	}
	return false
}
