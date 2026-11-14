package kgen

import "strings"

// splitProtGrant takes the "Protection:<spec>" terms out of a Continuous
// static's AddKeyword$ list (Sword of Wealth and Power's "Protection:Instant
// & Protection:Sorcery"): the rest of the list and one filter holding every
// quality's clauses (protection from each is its own ability, CR 702.16g,
// so a source matching any of them is protected from). gorge reads the
// derived keyword through protectionQuality and sourceHasQuality, a filter
// over the source; the kernel reads kop.Static.ProtFrom the same way it
// reads a printed CardDef.ProtFrom. Only plain type and colour terms
// lower, and only in the bare "Protection:<spec>" form: a subtype term would need the matchup's subtype remap, and a
// controller or player term (Player.Opponent) is a different rule.
//
// Protection:Card.ChosenColor:chosenColor (Ward Sliver, Pentarch Ward,
// Benevolent Blessing) is the host's chosen colour, st.ProtChosen, from any
// host; its exception field becomes st.ProtExc (chosenColorGrant).
func (lc *lowerCtx) splitProtGrant(kw string, st *lStatic) (string, error) {
	var rest []string
	var out *lFilter
	terms := 0
	hostOK := lc.out.Types&(tArtifact|tEnchantment) != 0
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		head, param, ok := strings.Cut(k, ":")
		if !ok || head != "Protection" {
			rest = append(rest, k)
			continue
		}
		terms++
		if q, _, _ := strings.Cut(param, ":"); strings.TrimSpace(q) == "Card.ChosenColor" {
			if err := lc.chosenColorGrant(param, terms == 1 && !strings.Contains(kw, "&"), st); err != nil {
				return "", refuse("keyword %q: %v", k, err)
			}
			continue
		}
		// Only an Equipment's or an Aura's grant is lockstep-verified (Sword of
		// Wealth and Power; Spirit Mantle, Unquestioned Authority).
		if !hostOK {
			return "", refuse("keyword %q: a granted protection from a non-artifact host", k)
		}
		// A trailing description or exception field (an Aura's "doesn't
		// remove CARDNAME", CR 702.16n and p) is not modelled by the
		// kernel's enforcement sites, so such a grant stays refused.
		if strings.Contains(param, ":") {
			return "", refuse("keyword %q: protection with a description or exception field", k)
		}
		spec := strings.TrimSpace(param)
		f, err := lc.filter(spec)
		if err == nil {
			err = plainQuality(f)
		}
		if err == nil {
			for _, c := range f.Clauses {
				if c.Subtype != "" || c.NotSubtype != "" {
					err = refuse("quality %s names a subtype", f)
				}
			}
		}
		if err != nil {
			return "", refuse("keyword %q: %v", k, err)
		}
		if out == nil {
			out = f
		} else {
			out = &lFilter{Clauses: append(append([]lClause(nil), out.Clauses...), f.Clauses...)}
		}
	}
	st.ProtFrom = out
	return strings.Join(rest, " & "), nil
}

// chosenColorGrant lowers one Protection:Card.ChosenColor:<text>[:<exception>
// [:SBA]] term. The exception (CR 702.16 "this effect doesn't remove ...")
// is spared only at the kernel's Aura state-based check, so it lowers only
// when it is the sole protection term, and only in the two forms whose
// excepted source no other protection check can name: the host itself
// (Card.CardUID_HostCardUID, Pentarch Ward) and, with the SBA flag, the
// host's controller's attachments (<spec>.ControlledBy
// Player.PlayerUID_HostCardControllerUID, Benevolent Blessing).
func (lc *lowerCtx) chosenColorGrant(param string, sole bool, st *lStatic) error {
	f := strings.Split(param, ":")
	if len(f) < 2 || f[1] != "chosenColor" || len(f) > 4 {
		return refuse("protection with a description or exception field")
	}
	st.ProtChosen = true
	if len(f) == 2 {
		return nil
	}
	sba := len(f) == 4
	if sba && f[3] != "SBA" || !sole {
		return refuse("a protection exception beside another protection term")
	}
	var clauses []string
	for _, c := range strings.Split(f[2], ",") {
		c = strings.TrimSpace(c)
		switch {
		case c == "Card.CardUID_HostCardUID" && !sba:
			clauses = append(clauses, "Card.Self")
		case sba && strings.HasSuffix(c, ".ControlledBy Player.PlayerUID_HostCardControllerUID"):
			spec := strings.TrimSuffix(c, ".ControlledBy Player.PlayerUID_HostCardControllerUID")
			if strings.Contains(spec, ".") {
				return refuse("protection exception %q", c)
			}
			if spec == "Equipment" {
				spec = "Card.Equipment" // a subtype the filter grammar spells off Card
			}
			if strings.Contains(spec, ".") {
				spec += "+YouCtrl"
			} else {
				spec += ".YouCtrl"
			}
			clauses = append(clauses, spec)
		default:
			return refuse("protection exception %q", c)
		}
	}
	exc, err := lc.filter(strings.Join(clauses, ","))
	if err != nil {
		return err
	}
	st.ProtExc = exc
	return nil
}
