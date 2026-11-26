package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// api:ChangeText (CR 612, gorge engine/effects/changetext.go): Mind Bend's
// two Choices$ bodies, "change a colour word" (ChangeColorWord$ Choose
// Choose) and "change a basic land type" (ChangeTypeWord$
// ChooseBasicLandType ChooseBasicLandType). The kernel holds no per-permanent
// text, so the lowering models the change only where it cannot matter: the
// target's face never reads the "from" word (CardDef.TextWords, below), and
// the kernel stalls (StallTextChange) when it does. Every other ChangeText
// spelling refuses.

// textWordNames are the ten words a lowered change can name, in
// CardDef.TextWords bit order: the five colours (WUBRG) then the five basic
// land types (Plains Island Swamp Mountain Forest, WUBRG order).
var textWordNames = [10]string{"white", "blue", "black", "red", "green", "plains", "island", "swamp", "mountain", "forest"}

// textWordColourBits is the mask of the five colour words in textWordNames:
// the bits a StatColorWords rewrite reaches (its basic land type words stay).
const textWordColourBits = 0x1f

// textWordsOf is the CardDef.TextWords of a card: every word of
// textWordNames that gorge's CR 612.2 rewrite (cards.SubstituteTextWord over
// Face.WithTextChanges' reach: Oracle text, type line, keywords, SVars, and
// every ability, trigger, static and replacement parameter) would change in
// some face. A LockInText$ ability and a locked parameter key are scanned
// too, so the mask is a superset of what gorge rewrites: a stall where gorge
// changes nothing is safe, a missed word is not.
func textWordsOf(c *cards.Card) uint16 {
	return textWordsMasks(c).words
}

// displayParam reports whether an ability parameter key only carries text for
// the client's display (gorge reads it in engine/view alone), so a colour word
// in its value is never read behaviourally.
func displayParam(k string) bool {
	switch k {
	case "SpellDescription", "TriggerDescription", "StackDescription", "Description", "PrecostDesc", "CostDesc", "TgtPrompt":
		return true
	}
	return false
}

// textWordsMasks is a card's TextWords beside CWUnmodelled: the colour words
// (bits 0..4) that sit on a surface the kernel's colour-word model does not
// read (dirac/static_colorwords.go rewrites target-spec filters, printed
// protection, and an Animate activation's word-list Colors$, whose override
// record carries the colour-word colour frozen as the ability was put on the
// stack -- static_colorwords.go cwFreeze, ovCw -- and stalls a face whose
// rewritten word also sits on any
// other surface). The modelled surfaces are the display-only Oracle text, the
// printed Protection keywords (a Protection spelling rides CardDef.ProtColors;
// any other keyword's colour word is unmodelled), the display-only description
// parameters (displayParam), a script line's text beside its parsed parameters
// (the line's own words are inert; every read goes through Params), the
// ValidTgts$ parameter of a spell, triggered-ability or replacement body (a
// kop.TargetSpec, read through the kernel's target funnel), and an Animate
// activation's (AB$) word-list Colors$. Every other
// surface - the type line, the SVars, and every other parameter of any
// ability, trigger, static and replacement - marks its colour words
// unmodelled. A basic land type word is
// never a colour-word bite, so the unmodelled mask holds colour bits only.
func textWordsMasks(c *cards.Card) (tw struct{ words, unmodelled uint16 }) {
	scan := func(s string, modelled int8) {
		if s == "" {
			return
		}
		low := strings.ToLower(s)
		for i, w := range textWordNames {
			// a word already seen on a modelled surface still marks the
			// unmodelled mask when it recurs on an unmodelled one
			seen := tw.words&(1<<i) != 0 && (modelled != 0 || i >= 5 || tw.unmodelled&(1<<i) != 0)
			if seen || !strings.Contains(low, w) {
				continue
			}
			if cards.SubstituteTextWord(s, w, "\x00") != s {
				tw.words |= 1 << i
				if modelled == 0 && i < 5 {
					tw.unmodelled |= 1 << i
				}
			}
		}
	}
	params := func(p map[string]string, modelled int8) {
		for k, v := range p {
			if displayParam(k) {
				scan(v, 2)
				continue
			}
			scan(v, modelled)
		}
	}
	var sa func(a *cards.SA, depth int, modelled int8)
	sa = func(a *cards.SA, depth int, modelled int8) {
		if a == nil || depth > 16 {
			return
		}
		for k, v := range a.Params {
			keyModelled := modelled
			if keyModelled != 2 && k == "ValidTgts" {
				keyModelled = 1
			}
			if keyModelled != 2 && k == "Colors" && a.API == "Animate" && a.Kind == "AB" {
				// an Animate activation's word-list Colors$: the override record
				// carries the colour-word colour frozen as the ability was put
				// on the stack (static_colorwords.go cwFreeze, ovCw), so the
				// words are read (dirac/static_layerwalk.go layer 5, chars_gen
				// colorsGen) and never stall. A spell's (SP$) or a static's
				// stays unmodelled: no frozen colour reaches its record.
				keyModelled = 1
			}
			if displayParam(k) {
				keyModelled = 2
			}
			scan(v, keyModelled)
		}
		scan(a.Line, 2) // the line beside its parsed parameters: words only
		sa(a.Sub, depth+1, modelled)
	}
	for _, f := range c.Faces {
		if f == nil {
			continue
		}
		scan(f.Oracle, 1)
		for _, t := range f.Types {
			scan(t, 0)
		}
		for _, k := range f.Keywords {
			if strings.HasPrefix(k, "Protection") {
				scan(k, 1)
			} else {
				scan(k, 0)
			}
		}
		params(f.SVars, 0)
		for _, a := range f.Abilities {
			sa(a, 0, 0)
		}
		for i := range f.Triggers {
			params(f.Triggers[i].Params, 0)
			sa(f.Triggers[i].Effect, 0, 0)
		}
		for i := range f.Statics {
			params(f.Statics[i].Params, 0)
		}
		for i := range f.Repls {
			params(f.Repls[i].Params, 0)
			sa(f.Repls[i].With, 0, 0)
		}
	}
	tw.unmodelled &= textWordColourBits
	return tw
}

// changeText lowers one DB$ ChangeText branch of a GenericChoice (Mind
// Bend) to OpExt ExtChangeText over the choice's one battlefield target, or
// a direct SP$/DB$ ChangeText over its own "target spell or permanent"
// (Magical Hack, Sleight of Mind, a Spectral Shift mode:
// changeTextOwnTarget).
// Duration$ Permanent or absent both lower: the change is only ever a no-op
// here, so its length is never read. Any other parameter (ForbiddenNewTypes$,
// a literal word, ChooseCreatureType) refuses.
func (lc *lowerCtx) changeText(ps *params) error {
	var kind int16
	switch cw, tw := ps.str("ChangeColorWord"), ps.str("ChangeTypeWord"); {
	case cw == "Choose Choose" && tw == "":
		kind = kop.ChangeTextColour
	case tw == "ChooseBasicLandType ChooseBasicLandType" && cw == "":
		kind = kop.ChangeTextLand
	default:
		return refuse("ChangeText of words %q %q", cw, tw)
	}
	if d := ps.str("Duration"); d != "" && d != "Permanent" {
		return refuse("ChangeText Duration$ %s", d)
	}
	if lc.ab.Kind != akSpell {
		return refuse("ChangeText in a %d ability", lc.ab.Kind)
	}
	var obj lRef
	if _, own := ps.get("ValidTgts"); own {
		var err error
		if obj, err = lc.changeTextOwnTarget(ps); err != nil {
			return err
		}
	} else {
		if ps.str("Defined") != "ParentTarget" {
			return refuse("ChangeText Defined$ %q", ps.str("Defined"))
		}
		var err error
		if obj, err = lc.defined("ParentTarget"); err != nil {
			return err
		}
		if obj.Kind != rTarget {
			return refuse("ChangeText of %s", obj)
		}
		t := lc.ab.Targets[obj.N]
		if t.Zone != zBattlefield || !t.Max.isConst() || t.Max.N != 1 || t.F == nil || t.F.Players != 0 {
			return refuse("ChangeText of a target slot other than one battlefield object")
		}
	}
	lc.out.HasChangeText = true
	lc.emit(lInstr{Op: "ChangeText", Obj: obj, C: int(kind)})
	return nil
}

// changeTextOwnTarget is the direct spelling's own target: exactly
// ValidTgts$ Card with TgtZone$ Stack,Battlefield and no Defined$, i.e. one
// permanent or one spell other than this one (kop.TgtPermOrSpell; gorge
// effChangeText applies the change to the one chosen target, CR 612). Any
// other target shape (a filter, a count, a single zone) refuses.
func (lc *lowerCtx) changeTextOwnTarget(ps *params) (lRef, error) {
	if vt := ps.str("ValidTgts"); vt != "Card" {
		return lRef{}, refuse("ChangeText ValidTgts$ %q", vt)
	}
	if tz := ps.str("TgtZone"); tz != "Stack,Battlefield" {
		return lRef{}, refuse("ChangeText TgtZone$ %q", tz)
	}
	for _, k := range []string{"Defined", "TargetMin", "TargetMax", "TargetUnique", "TargetType"} {
		if _, ok := ps.get(k); ok {
			return lRef{}, refuse("ChangeText with ValidTgts$ and %s$", k)
		}
	}
	ps.consume("TgtPrompt")
	f, err := lc.filter("Card")
	if err != nil {
		return lRef{}, err
	}
	if f.Players != 0 {
		return lRef{}, refuse("ChangeText ValidTgts$ Card names players")
	}
	return lRef{Kind: rTarget, N: lc.addTarget(lTarget{F: f, Zone: zBattlefield, PermOrSpell: true})}, nil
}
