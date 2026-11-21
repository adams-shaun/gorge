package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/effects"
)

// protectionCardTypes is gorge's effChooseType Card option list
// (effects type_choices.go chooseCardTypes, the alphabetical card-type
// vocabulary), the order the kernel's cardTypeChoice ask and gorge's
// "CardType" protection offers share.
var protectionCardTypes = effects.TypeChoiceLabels("Card", "", "")

// protection lowers the Protection effect (gorge effProtection: the target or
// Defined$ permanent gains "protection from <quality>" until end of turn,
// grantProtectionUntilEOT).
//
// Gains$ Choice, Choices$ AnyColor (Mother of Runes, Gods Willing, Aven
// Liberator, Benevolent Bodyguard, Emerge Unscathed, Shelter, Alseid of
// Life's Bounty): the controller chooses the colour as this resolves. The
// kernel already prices both halves for the chosen-colour sweeps (Akroma's
// Blessing): ChooseColorRes asks the colour, then ColorGrant C bit 2 marks
// the one target.
//
// A fixed Gains$ (Crimson Acolyte's red, Tower of the Magistrate's Artifact,
// Grotesque Hybrid's green,white) asks nothing: it is the same until-end-of-turn
// override record a Pump's KW$ Protection from <quality> lowers to
// (splitProtFromKW, kop.Override.ProtFrom), one clause per comma-separated
// quality (CR 702.16g; gorge splits the list the same way). The target is
// either the ValidTgts$ target or a Defined$ permanent (Keeper of Kookus,
// Self).
//
// A Choices$ list beyond AnyColor lowers as its own ask when every token
// names a quality the kernel's quality filters read: ExtProtectionChoice
// poses gorge's offer list (colors, colorless, card types — Giver of Runes'
// "AnyColor,colorless", Apostle's Blessing's "AnyColor,Artifact", Pippin,
// Guard of the Citadel's "CardType"), and one OvProtChoice override record
// per option grants the answered quality (act_protectionchoice.go). A
// single-offer list never asks (gorge askProtectionChoice's one-offer
// return); it is the fixed grant protectionFixed lowers. Gains$
// ChosenColor (Floating Shield: the sacrificed source's colour), a Gains$
// Defined$ reference (Opal Titan's Duration$ Permanent grant) and a
// Duration$ refuse by name.
func (lc *lowerCtx) protection(ps *params) error {
	for _, k := range [...]string{"Duration", "Optional"} {
		if _, ok := ps.get(k); ok {
			return refuse("Protection %s$", k)
		}
	}
	gains := strings.TrimSpace(ps.str("Gains"))
	if !strings.EqualFold(gains, "Choice") {
		return lc.protectionFixed(ps, gains)
	}
	choices := strings.TrimSpace(ps.str("Choices"))
	qs, fs, err := lc.protectionChoiceQualities(choices)
	if err != nil {
		return err
	}
	if _, ok := ps.get("Defined"); ok {
		return refuse("Protection Gains$ Choice with Defined$")
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	if len(qs) == 1 {
		// a one-offer Choices$ never asks: the fixed grant
		lc.emitOverride(obj, &lOverride{ProtFrom: fs[0]}, "EOT")
		return nil
	}
	if obj.Kind != rTarget && obj.Kind != rSelf {
		return refuse("Protection without its own target")
	}
	if lc.progHas("ChooseNumberMid") {
		// the answer shares res.bad's bit 0x20 and res.chosen with the
		// mid-resolution number (dirac resProtChoice / resChosenNumber)
		return refuse("Protection Gains$ Choice after a ChooseNumber in one resolution")
	}
	ps.consume("AILogic")
	lc.emit(lInstr{Op: "ProtectionChoiceRes", B: cst(len(qs)),
		Note: "the controller chooses the quality to gain protection from"})
	start := len(lc.out.overrides)
	for _, f := range fs {
		lc.out.overrides = append(lc.out.overrides, &lOverride{ProtFrom: f})
	}
	in := lInstr{Op: "Override", Obj: obj, CS: "EOT|protChoice=" + strconv.Itoa(len(qs)), C: start}
	in.Note = overrideString(lc.out.overrides[start])
	lc.emit(in)
	return nil
}

// protectionColourWord is a WUBRG letter as gorge's protectionColorWord
// spells it (the offer list's labels; the oracle matches options by the
// canonical encoding, the bot by gorge's label).
func protectionColourWord(l string) string {
	switch l {
	case "W":
		return "white"
	case "U":
		return "blue"
	case "B":
		return "black"
	case "R":
		return "red"
	case "G":
		return "green"
	}
	return ""
}

// protectionColourLetter is tok as a WUBRG letter (a letter, or the colour's
// full word), "" otherwise — the same tokens gorge colourLetter admits.
func protectionColourLetter(tok string) string {
	switch strings.ToUpper(tok) {
	case "W", "WHITE":
		return "W"
	case "U", "BLUE":
		return "U"
	case "B", "BLACK":
		return "B"
	case "R", "RED":
		return "R"
	case "G", "GREEN":
		return "G"
	}
	return ""
}

// protectionChoiceQualities mirrors gorge combatfx.go protectionChoiceOffers:
// the qualities Choices$ lets the controller pick from, in list order, each
// with the protection filter that grants it. AnyColor is the five colours;
// colorless and a named colour stand for themselves; "CardType" is the
// card-type vocabulary itself (Pippin, Guard of the Citadel), the same
// alphabetical list gorge's effChooseType Card category and the kernel's
// cardTypeChoice ask over; a named card type (Apostle's Blessing's
// "Artifact") is itself.
func (lc *lowerCtx) protectionChoiceQualities(choices string) ([]string, []*lFilter, error) {
	if choices == "" {
		return nil, nil, refuse("Protection Choices$ %q: no qualities listed", choices)
	}
	var qs []string
	var fs []*lFilter
	add := func(q string, f *lFilter, err error) error {
		if err != nil {
			return err
		}
		qs = append(qs, q)
		fs = append(fs, f)
		return nil
	}
	one := func(l string) error {
		return add(protectionColourWord(l), &lFilter{Clauses: []lClause{{Colors: l}}}, nil)
	}
	quality := func(q string) error {
		spec := q
		if effects.IsColorlessWord(q) {
			spec = "Card.Colorless" // the filter grammar's no-colour property
		}
		f, err := lc.qualityFilter(spec)
		if err != nil {
			return refuse("Protection Choices$ %q: quality %q does not lower", choices, q)
		}
		return add(q, f, nil)
	}
	for tok := range strings.SplitSeq(choices, ",") {
		tok = strings.TrimSpace(tok)
		switch {
		case strings.EqualFold(tok, "AnyColor"):
			for _, l := range []string{"W", "U", "B", "R", "G"} {
				if err := one(l); err != nil {
					return nil, nil, err
				}
			}
		case effects.IsColorlessWord(tok):
			if err := quality("colorless"); err != nil {
				return nil, nil, err
			}
		case protectionColourLetter(tok) != "":
			if err := one(protectionColourLetter(tok)); err != nil {
				return nil, nil, err
			}
		case strings.EqualFold(tok, "CardType"):
			for _, ct := range protectionCardTypes {
				if err := quality(ct); err != nil {
					return nil, nil, err
				}
			}
		default:
			if t := protectionCardType(tok); t != "" {
				if err := quality(t); err != nil {
					return nil, nil, err
				}
				continue
			}
			return nil, nil, refuse("Protection Choices$ %q: unknown quality %q", choices, tok)
		}
	}
	return qs, fs, nil
}

// protectionCardType is the card type a Choices$ token names (gorge
// protectionCardType's match over the same vocabulary), "" otherwise.
func protectionCardType(tok string) string {
	for _, t := range protectionCardTypes {
		if strings.EqualFold(t, tok) {
			return t
		}
	}
	return ""
}

// protectionFixed lowers a Protection effect whose Gains$ names its qualities
// (red; Artifact; green,white) to one end-of-turn override record carrying the
// quality filter.
func (lc *lowerCtx) protectionFixed(ps *params, gains string) error {
	var q *lFilter
	switch {
	case strings.EqualFold(gains, "ChosenColor"), strings.EqualFold(gains, "TargetedCardColor"), strings.HasPrefix(gains, "Defined "):
		return refuse("Protection Gains$ %q reads the source or a triggering card at resolution (not lowered)", gains)
	}
	for _, w := range strings.Split(gains, ",") {
		w = strings.TrimSpace(w)
		if w == "" || strings.ContainsAny(w, " .$") {
			return refuse("Protection Gains$ %q (a fixed colour or card type lowers)", gains)
		}
		if c := protColourLetters[strings.ToLower(w)]; c != "" {
			q = appendProtClause(q, &lFilter{Clauses: []lClause{{Colors: c}}})
			continue
		}
		f, err := lc.qualityFilter(w)
		if err != nil {
			return refuse("Protection Gains$ %q: %v", gains, err)
		}
		q = appendProtClause(q, f)
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	ps.consume("AILogic")
	lc.emitOverride(obj, &lOverride{ProtFrom: q}, "EOT")
	return nil
}
