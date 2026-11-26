package kgen

// targetedPlayerGraveyard lowers TargetedPlayer$ValidGraveyard <spec>
// (Corpse Augur, The Mouth of Sauron): gorge evalPlayerRefProperty counts
// the spec over the graveyards with the targeted player as the filter's
// You, so a YouOwn clause is the targeted player's own cards there. Each
// clause must name YouOwn and no controller; it becomes the OwnedBy
// Targeted role (whoTgtPlayer, kop.F2TgtPlayerCtrl read on the card's
// owner off the battlefield), exact when the ability has one player slot.
func (lc *lowerCtx) targetedPlayerGraveyard(spec string) (lValue, error) {
	players := 0
	for _, t := range lc.ab.Targets {
		if t.F != nil && t.F.Players != 0 {
			players++
		}
	}
	if players != 1 {
		return lValue{}, refuse("TargetedPlayer$ValidGraveyard beside %d player slots", players)
	}
	f, err := lc.filter(spec)
	if err != nil {
		return lValue{}, err
	}
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return lValue{}, refuse("TargetedPlayer$ValidGraveyard %s", spec)
	}
	out := &lFilter{}
	for _, c := range f.Clauses {
		if c.Owner != whoYou || c.Ctrl != whoAny {
			return lValue{}, refuse("TargetedPlayer$ValidGraveyard %s: only YouOwn clauses", spec)
		}
		c.Owner = whoTgtPlayer
		out.Clauses = append(out.Clauses, c)
	}
	return lValue{Kind: vCountGraveyard, F: out}, nil
}
