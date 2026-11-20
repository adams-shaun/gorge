package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// splitProtGrant takes the "Protection:<spec>" terms out of a Continuous
// static's AddKeyword$ list (Sword of Wealth and Power's "Protection:Instant
// & Protection:Sorcery"): the rest of the list and one filter holding every
// quality's clauses (protection from each is its own ability, CR 702.16g,
// so a source matching any of them is protected from). gorge reads the
// derived keyword through protectionQuality and sourceHasQuality, a filter
// over the source; the kernel reads kop.Static.ProtFrom the same way it
// reads a printed CardDef.ProtFrom. Only plain type, colour, subtype and
// multicolored terms lower (qualityFilter), from any host: a controller or
// player term (Player.Opponent) is a different rule. An exception field
// (a Ward Aura's "doesn't remove CARDNAME", Spectra Ward's "doesn't remove
// Auras") becomes st.ProtExc (protException).
//
// Protection:Card.ChosenColor:chosenColor (Ward Sliver, Pentarch Ward,
// Benevolent Blessing) is the host's chosen colour, st.ProtChosen, from any
// host; its exception field becomes st.ProtExc (chosenColorGrant).
func (lc *lowerCtx) splitProtGrant(kw string, st *lStatic) (string, error) {
	var rest []string
	var out *lFilter
	terms := 0
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		if c, ok := protColourKW(k); ok {
			// On every host the colour spellings lower to colour-mask clauses of
			// the same quality filter, not to kop keyword bits (KwMask has no
			// free bits for white, blue or green): one ability per term, so a
			// source matching any clause is protected from (CR 702.16g; Pristine
			// Angel's each color on a creature).
			//
			// Protection from black and from red keep their bits on every
			// host (Akroma's Memorial, as before the colour clauses): gorge
			// spells the granted keyword on the creature, and the feature
			// encoder reads it from the bit (feat.KeywordNames), which a
			// quality clause does not carry.
			if c == "B" || c == "R" {
				rest = append(rest, k)
				continue
			}
			terms++
			out = appendProtClause(out, &lFilter{Clauses: []lClause{{Colors: c}}})
			continue
		}
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
		// A description after the spec is part of the quality, not a rule:
		// gorge protectionQuality reads only the first field. An exception
		// field (a Ward Aura's "doesn't remove CARDNAME", Spectra Ward's
		// "doesn't remove Auras", CR 702.16) rides st.ProtExc, read only at the
		// kernel's Aura state-based check (protException).
		spec, desc, _ := strings.Cut(param, ":")
		if _, exc, hasExc := strings.Cut(desc, ":"); hasExc {
			if err := lc.protException(exc, terms == 1 && !strings.Contains(kw, "&"), st); err != nil {
				return "", refuse("keyword %q: %v", k, err)
			}
		}
		f, err := lc.qualityFilter(strings.TrimSpace(spec))
		if err != nil {
			return "", refuse("keyword %q: %v", k, err)
		}
		out = appendProtClause(out, f)
	}
	st.ProtFrom = out
	return strings.Join(rest, " & "), nil
}

// protColourKW maps a "Protection from <q>" keyword term to its colours (the
// printed spellings' protColourLetters; "each color" is any coloured source).
// Anything else is not a colour term.
func protColourKW(k string) (string, bool) {
	w, ok := strings.CutPrefix(k, "Protection from ")
	if !ok {
		return "", false
	}
	c := protColourLetters[w]
	return c, c != ""
}

// appendProtClause merges f's clauses into out (one ability per term, so the
// clauses are alternatives: a source matching any of them is protected from).
func appendProtClause(out, f *lFilter) *lFilter {
	if out == nil {
		return f
	}
	return &lFilter{Clauses: append(append([]lClause(nil), out.Clauses...), f.Clauses...)}
}

// qualityFilter lowers one protection quality spec: the plain object
// characteristics plainQualityOf reads (no controller, owner, comparison or
// state terms); a subtype term lowers as every filter's does (the matchup's
// subtype remap renumbers every table filter, quality filters included).
func (lc *lowerCtx) qualityFilter(spec string) (*lFilter, error) {
	f, err := lc.filter(spellTypeQuality(spec))
	if err == nil {
		err = plainQualityOf(f, true)
	}
	return f, err
}

// spellTypeQuality reads a quality's Spell.Instant and Spell.Sorcery terms
// (Devoted Caretaker's "instant spells and sorcery spells") as the card types
// themselves: an instant or sorcery is never a permanent, so the source of
// damage, a block or a target is a spell of that type exactly when it is a
// card of it, which is what the kernel's quality match reads (the filter
// grammar has no Spell head outside a ReduceCost ValidTarget$). Any other
// Spell term is left for the filter to refuse.
func spellTypeQuality(spec string) string {
	if !strings.Contains(spec, "Spell.") {
		return spec
	}
	ts := strings.Split(spec, ",")
	for i, t := range ts {
		if t = strings.TrimSpace(t); t == "Spell.Instant" || t == "Spell.Sorcery" {
			t = strings.TrimPrefix(t, "Spell.")
		}
		ts[i] = t
	}
	return strings.Join(ts, ",")
}

// splitProtSpecAll takes the "Protection:<spec>" terms out of a PumpAll's KW$
// list into one end-of-turn override record's ProtFrom (splitProtFromKW over
// those terms only: the colour spellings keep pumpAll's own routes). nil when
// the list has none.
func (lc *lowerCtx) splitProtSpecAll(kw string) (string, *lOverride, error) {
	var rest, prot []string
	for _, k := range strings.Split(kw, "&") {
		if k = strings.TrimSpace(k); strings.HasPrefix(k, "Protection:") {
			prot = append(prot, k)
		} else if k != "" {
			rest = append(rest, k)
		}
	}
	if len(prot) == 0 {
		return kw, nil, nil
	}
	o := &lOverride{}
	if _, err := lc.splitProtFromKW(strings.Join(prot, " & "), o); err != nil {
		return "", nil, err
	}
	return strings.Join(rest, " & "), o, nil
}

// splitProtFromKW takes the protection terms out of a granted KW$ list (a
// Pump's, a PumpAll UntilYourNextTurn's or an Animate's Keywords$) into the grant's quality
// filter o.ProtFrom: "Protection from <colour>" and "Protection from each
// color" lower to colour-mask clauses, and "Protection:<spec>[:<desc>]"
// lowers through lc.filter (kop.Override.ProtFrom, read beside a Continuous
// static's ProtFrom). A Player.Opponent quality lowers to a Ctrl=opponent
// clause: gorge judges a player-relative quality against the source's
// controller, with the protected permanent's controller as the "you" the
// relation names (protection.go's player-relative arm, which only a live
// source object has), which the kernel's quality match reads off the clause. Every other
// player or controller term still refuses, as does an exception field, a
// subtype term and a value term. The rest of the list returns for
// pumpKeywords.
func (lc *lowerCtx) splitProtFromKW(kw string, o *lOverride) (string, error) {
	var rest []string
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		if c, ok := protColourKW(k); ok {
			o.ProtFrom = appendProtClause(o.ProtFrom, &lFilter{Clauses: []lClause{{Colors: c}}})
			continue
		}
		head, param, ok := strings.Cut(k, ":")
		if !ok || head != "Protection" {
			rest = append(rest, k)
			continue
		}
		spec, desc, _ := strings.Cut(param, ":")
		if _, _, hasExc := strings.Cut(desc, ":"); hasExc {
			return "", refuse("keyword %q: protection with a description or exception field", k)
		}
		if spec == "Player.Opponent" {
			o.ProtFrom = appendProtClause(o.ProtFrom, &lFilter{Clauses: []lClause{{Ctrl: uint8(kop.CtrlOpp)}}})
			continue
		}
		f, err := lc.qualityFilter(strings.TrimSpace(spec))
		if err == nil {
			for _, c := range f.Clauses {
				// a mana-value or counter bound on an override record reads
				// without the spell's announced X (protSpellXMV is the printed
				// and static readers' spell arm), so such a quality refuses
				if c.CmpStat != 0 || c.CmpX != "" || c.CmpCounter != "" {
					err = refuse("quality %s reads a value", f)
				}
			}
		}
		if err != nil {
			return "", refuse("keyword %q: %v", k, err)
		}
		o.ProtFrom = appendProtClause(o.ProtFrom, f)
	}
	return strings.Join(rest, " & "), nil
}

// chosenColorGrant lowers one Protection:Card.ChosenColor:<text>[:<exception>
// [:SBA]] term (the exception: protException).
func (lc *lowerCtx) chosenColorGrant(param string, sole bool, st *lStatic) error {
	f := strings.Split(param, ":")
	if len(f) < 2 || f[1] != "chosenColor" || len(f) > 4 {
		return refuse("protection with a description or exception field")
	}
	st.ProtChosen = true
	if len(f) == 2 {
		return nil
	}
	return lc.protException(strings.Join(f[2:], ":"), sole, st)
}

// protException lowers a granted protection's exception field,
// "<clauses>[:SBA]" (CR 702.16 "this effect doesn't remove ..."). The
// exception is spared only at the kernel's Aura state-based check, so it
// lowers only when it is the sole protection term, and only in the forms
// whose excepted source no other protection check can name: the host itself
// (Card.CardUID_HostCardUID, Pentarch Ward and the Ward Auras), every Aura
// (Aura, Spectra Ward) and, with the SBA flag, the host's controller's
// attachments (<spec>.ControlledBy Player.PlayerUID_HostCardControllerUID,
// Benevolent Blessing). Anything else refuses by name.
func (lc *lowerCtx) protException(exc string, sole bool, st *lStatic) error {
	fs := strings.Split(exc, ":")
	sba := len(fs) == 2
	if len(fs) > 2 || sba && fs[1] != "SBA" || !sole {
		return refuse("a protection exception beside another protection term, or with an unread flag")
	}
	var clauses []string
	for _, c := range strings.Split(fs[0], ",") {
		c = strings.TrimSpace(c)
		switch {
		case c == "Card.CardUID_HostCardUID" && !sba:
			clauses = append(clauses, "Card.Self")
		case c == "Aura":
			clauses = append(clauses, "Card.Aura")
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
	exf, err := lc.filter(strings.Join(clauses, ","))
	if err != nil {
		return err
	}
	st.ProtExc = exf
	return nil
}
