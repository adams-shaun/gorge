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

// textWordsOf is the CardDef.TextWords of a card: every word of
// textWordNames that gorge's CR 612.2 rewrite (cards.SubstituteTextWord over
// Face.WithTextChanges' reach: Oracle text, type line, keywords, SVars, and
// every ability, trigger, static and replacement parameter) would change in
// some face. A LockInText$ ability and a locked parameter key are scanned
// too, so the mask is a superset of what gorge rewrites: a stall where gorge
// changes nothing is safe, a missed word is not.
func textWordsOf(c *cards.Card) uint16 {
	var m uint16
	scan := func(s string) {
		if s == "" {
			return
		}
		low := strings.ToLower(s)
		for i, w := range textWordNames {
			if m&(1<<i) != 0 || !strings.Contains(low, w) {
				continue
			}
			if cards.SubstituteTextWord(s, w, "\x00") != s {
				m |= 1 << i
			}
		}
	}
	params := func(p map[string]string) {
		for _, v := range p {
			scan(v)
		}
	}
	var sa func(a *cards.SA, depth int)
	sa = func(a *cards.SA, depth int) {
		if a == nil || depth > 16 {
			return
		}
		params(a.Params)
		scan(a.Line)
		sa(a.Sub, depth+1)
	}
	for _, f := range c.Faces {
		if f == nil {
			continue
		}
		scan(f.Oracle)
		for _, t := range f.Types {
			scan(t)
		}
		for _, k := range f.Keywords {
			scan(k)
		}
		params(f.SVars)
		for _, a := range f.Abilities {
			sa(a, 0)
		}
		for i := range f.Triggers {
			params(f.Triggers[i].Params)
			sa(f.Triggers[i].Effect, 0)
		}
		for i := range f.Statics {
			params(f.Statics[i].Params)
		}
		for i := range f.Repls {
			params(f.Repls[i].Params)
			sa(f.Repls[i].With, 0)
		}
	}
	return m
}

// changeText lowers one DB$ ChangeText branch of a GenericChoice (Mind
// Bend) to OpExt ExtChangeText over the choice's one battlefield target.
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
	if ps.str("Defined") != "ParentTarget" {
		return refuse("ChangeText Defined$ %q", ps.str("Defined"))
	}
	obj, err := lc.defined("ParentTarget")
	if err != nil {
		return err
	}
	if obj.Kind != rTarget {
		return refuse("ChangeText of %s", obj)
	}
	t := lc.ab.Targets[obj.N]
	if t.Zone != zBattlefield || !t.Max.isConst() || t.Max.N != 1 || t.F == nil || t.F.Players != 0 {
		return refuse("ChangeText of a target slot other than one battlefield object")
	}
	lc.out.HasChangeText = true
	lc.emit(lInstr{Op: "ChangeText", Obj: obj, C: int(kind)})
	return nil
}
