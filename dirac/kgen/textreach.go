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
// in its value is never read behaviourally. KeywordLine is the raw K: line the
// parser keeps as its dedup tag (the parsed params carry the behaviour);
// NeedsToPlay is deck-building metadata no game read touches.
func displayParam(k string) bool {
	switch k {
	case "SpellDescription", "TriggerDescription", "StackDescription", "Description", "PrecostDesc", "CostDesc", "TgtPrompt",
		"KeywordLine", "NeedsToPlay":
		return true
	}
	return false
}

// adamantGateOf reports the colour words of a value's Count$Adamant_<n>.<word>
// gates and the value with those words removed: the gate's colour is read live
// at the kernel's ExAdamant (kw_evoke.go exAdamant, the composed colour-word
// colour of the source face), so its word is modelled; the value's rest scans
// as the caller's surface does. ok is false for a value with no gate.
func adamantGateOf(v string) (words []string, blank string, ok bool) {
	if !strings.Contains(v, "Adamant_") {
		return nil, "", false
	}
	rest := []rune(v)
	cut := map[int]bool{}
	for i := 0; i < len(v); {
		at := strings.Index(v[i:], "Adamant_")
		if at < 0 {
			break
		}
		i += at + len("Adamant_")
		for i < len(v) && v[i] >= '0' && v[i] <= '9' {
			i++
		}
		if i >= len(v) || v[i] != '.' {
			continue
		}
		i++
		start := i
		for i < len(v) && isLetterByte(v[i]) {
			i++
		}
		for j := start; j < i; j++ {
			cut[j] = true
		}
		if start < i {
			words = append(words, v[start:i])
		}
	}
	if len(words) == 0 {
		return nil, "", false
	}
	b := make([]rune, 0, len(rest))
	for i, r := range rest {
		if !cut[i] {
			b = append(b, r)
		}
	}
	return words, string(b), true
}

// isLetterByte reports whether b is an ASCII letter (the colour words are).
func isLetterByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// kwChoiceProtOf reports the colour words of a KWChoice$ list's "Protection
// from <colour>" terms and the list with those terms removed: the grant the
// choice lowers (OpGrantKeyword) reads the term's colour by the resolving
// entry's colour-word colour (dirac/static_colorwords.go cwProtGrant), so the
// word is modelled; the list's rest scans as the caller's surface does. ok is
// false when no term names a colour.
func kwChoiceProtOf(v string) (words []string, rest string, ok bool) {
	for _, term := range strings.Split(v, ",") {
		t := strings.TrimSpace(term)
		c, after, found := strings.Cut(strings.ToLower(t), "protection from ")
		if found && c == "" {
			if w := strings.TrimSpace(after); w != "" {
				switch w {
				case "white", "blue", "black", "red", "green":
					words = append(words, w)
					continue
				}
			}
		}
		rest = strings.TrimSpace(rest + "," + term)
	}
	rest = strings.TrimPrefix(rest, ",")
	return words, rest, len(words) != 0
}

// battlefieldPumpZone reports a Pump's PumpZone$ absent or Battlefield: the
// zones whose KWChoice$ grant the kernel's OpGrantKeyword rewrites.
func battlefieldPumpZone(z string) bool {
	return z == "" || z == "Battlefield"
}

// permKindOf is the leading kind token of an Affected$ spec: "Creature.Blue"
// reads Creature, "Card.Black" Card.
func permKindOf(v string) string {
	at := strings.IndexAny(v, ".+ ")
	if at < 0 {
		at = len(v)
	}
	return v[:at]
}

// permAffectedKinds are the leading kinds a Continuous static's Affected$ spec
// may name for the kernel to read the spec's colour clauses on the static's
// source face (matchPermOne rewrites them; a card or spell spec's read is a
// matchCard read the model does not rewrite).
var permAffectedKinds = map[string]bool{
	"Creature": true, "Artifact": true, "Enchantment": true, "Land": true,
	"Permanent": true, "Planeswalker": true, "Battle": true,
	"Plains": true, "Island": true, "Swamp": true, "Mountain": true, "Forest": true,
}

// textWordsMasks is a card's TextWords beside CWUnmodelled: the colour words
// (bits 0..4) that sit on a surface the kernel's colour-word model does not
// read (dirac/static_colorwords.go rewrites target-spec filters, printed
// protection, an Animate activation's word-list Colors$ -- whose override
// record carries the colour-word colour frozen as the ability was put on the
// stack, static_colorwords.go cwFreeze, ovCw -- a Continuous static's
// Affected$ filter (matchPermOne reads it on the static's source face), a
// Count$Adamant_<n>.<colour> gate's colour (kw_evoke.go exAdamant reads the
// spend of the composed colour live), a granted
// protection keyword inside a Pump KWChoice$ list (OpGrantKeyword rewrites the
// colour by the resolving entry's colour-word colour, cwProtGrant), and stalls
// a face whose rewritten word also sits on any other surface). The modelled
// surfaces are the display-only Oracle text, the printed Protection keywords
// (a Protection spelling rides CardDef.ProtColors; any other keyword's colour
// word is unmodelled), the display-only description
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
	var s twScanner
	for _, f := range c.Faces {
		if f != nil {
			s.face(f)
		}
	}
	tw.words = s.words
	tw.unmodelled = s.unmodelled & textWordColourBits
	return tw
}

// twScanner accumulates textWordsMasks' two masks over a card's surfaces.
type twScanner struct{ words, unmodelled uint16 }

// scan marks the colour/type words of s: modelled is 0 for an unmodelled
// surface, 1 for a modelled one and 2 for display-only text.
func (t *twScanner) scan(s string, modelled int8) {
	if s == "" {
		return
	}
	low := strings.ToLower(s)
	for i, w := range textWordNames {
		// a word already seen on a modelled surface still marks the
		// unmodelled mask when it recurs on an unmodelled one
		seen := t.words&(1<<i) != 0 && (modelled != 0 || i >= 5 || t.unmodelled&(1<<i) != 0)
		if seen || !strings.Contains(low, w) {
			continue
		}
		if cards.SubstituteTextWord(s, w, "\x00") != s {
			t.words |= 1 << i
			if modelled == 0 && i < 5 {
				t.unmodelled |= 1 << i
			}
		}
	}
}

// scanSplit scans a gate's read words as modelled and its rest as rest does.
func (t *twScanner) scanSplit(words []string, rest string, restModelled int8) {
	for _, w := range words {
		t.scan(w, 1)
	}
	t.scan(rest, restModelled)
}

// adamant scans a value's Count$Adamant_ gates; false when v has none.
func (t *twScanner) adamant(v string, modelled int8) bool {
	w, blank, ok := adamantGateOf(v)
	if ok {
		t.scanSplit(w, blank, modelled)
	}
	return ok
}

// kwChoice scans a KWChoice$ list's "Protection from <colour>" terms; false
// when no term names a colour.
func (t *twScanner) kwChoice(v string, modelled int8) bool {
	w, rest, ok := kwChoiceProtOf(v)
	if ok {
		t.scanSplit(w, rest, modelled)
	}
	return ok
}

// params scans a parameter map, skipping the key skip.
func (t *twScanner) params(p map[string]string, modelled int8, skip string) {
	for k, v := range p {
		if k != "" && k == skip {
			continue
		}
		switch {
		case displayParam(k):
			t.scan(v, 2)
		case t.adamant(v, modelled):
		case k == "KWChoice" && t.kwChoice(v, modelled):
		default:
			t.scan(v, modelled)
		}
	}
}

// saKeyModelled is the surface class of one ability parameter.
func saKeyModelled(a *cards.SA, k string, modelled int8) int8 {
	if modelled == 2 {
		return modelled
	}
	if k == "ValidTgts" {
		return 1
	}
	if k == "Colors" && a.API == "Animate" && a.Kind == "AB" {
		// an Animate activation's word-list Colors$: the override record
		// carries the colour-word colour frozen as the ability was put
		// on the stack (static_colorwords.go cwFreeze, ovCw), so the
		// words are read (dirac/static_layerwalk.go layer 5, chars_gen
		// colorsGen) and never stall. A spell's (SP$) or a static's
		// stays unmodelled: no frozen colour reaches its record.
		return 1
	}
	return modelled
}

// sa scans an ability, its Line and its sub-ability chain.
func (t *twScanner) sa(a *cards.SA, depth int, modelled int8) {
	if a == nil || depth > 16 {
		return
	}
	for k, v := range a.Params {
		km := saKeyModelled(a, k, modelled)
		if displayParam(k) {
			t.scan(v, 2)
			continue
		}
		if t.adamant(v, km) {
			continue
		}
		// a Pump KWChoice$'s "Protection from <colour>" term: the
		// choice's grant (OpGrantKeyword) reads the term's colour by
		// the resolving entry's colour-word colour (dirac
		// static_colorwords.go cwProtGrant), so the word is read. A
		// stack-scoped pump's grant is dropped by the kernel's spell
		// arm (interp.go grantSpellKw), so it stays unmodelled.
		if km != 2 && k == "KWChoice" && a.API == "Pump" && battlefieldPumpZone(a.Params["PumpZone"]) && t.kwChoice(v, km) {
			continue
		}
		t.scan(v, km)
	}
	t.scan(a.Line, 2) // the line beside its parsed parameters: words only
	t.sa(a.Sub, depth+1, modelled)
}

// keyword scans one printed keyword line.
func (t *twScanner) keyword(k string) {
	switch {
	case strings.HasPrefix(k, "Protection"):
		t.scan(k, 1)
	case strings.Contains(k, "$"):
		// a parameterized keyword line beside its parsed expansion: the
		// behavioural colour words ride the expansion's parsed surfaces
		// (the SVars, Repls, Triggers and Statics the expander built,
		// scanned with the face), the line's own words are inert, as an
		// ability's Line beside its Params
		t.scan(k, 2)
	default:
		t.scan(k, 0)
	}
}

// static scans one static ability's parameters.
func (t *twScanner) static(st map[string]string) {
	skip := ""
	if st["Mode"] == "Continuous" && permAffectedKinds[permKindOf(st["Affected"])] &&
		(st["AffectedZone"] == "" || st["AffectedZone"] == "Battlefield") {
		// a Continuous static over the battlefield permanents: the
		// kernel reads the Affected$ spec's colour clauses on the
		// static's source face (matchPermOne rewrites them, the
		// static's source slot), so the words are read
		skip = "Affected"
		t.scan(st["Affected"], 1)
	}
	if st["Mode"] == "AlternativeCost" && strings.Contains(st["Cost"], "ExileFromHand<") &&
		(st["ValidSA"] == "" || strings.Contains(st["ValidSA"], "Self")) {
		// a self-carried alternative cost: gorge reads it from the
		// cast card's printed face in its cast zone (statics_altcost.go
		// alternativeCosts' face walk over o.Face() of the card being
		// cast, at beginCastWith -- a hand, graveyard or exile card
		// carries no colour-word substitutions), and the parsed cost
		// is frozen for the pick (cast_asks.go exAsk), so the words
		// are read but never rewritten. An external grant (a
		// battlefield source's Cost$, ValidSA$ without Self) reads the
		// source's rewritten text and stays unmodelled until a kernel
		// rewrite exists.
		skip = "Cost"
		t.scan(st["Cost"], 2)
	}
	t.params(st, 0, skip)
}

// face scans every surface of one card face.
func (t *twScanner) face(f *cards.Face) {
	t.scan(f.Oracle, 1)
	for _, ty := range f.Types {
		t.scan(ty, 0)
	}
	for _, k := range f.Keywords {
		t.keyword(k)
	}
	t.params(f.SVars, 0, "")
	for _, a := range f.Abilities {
		t.sa(a, 0, 0)
	}
	for i := range f.Triggers {
		t.params(f.Triggers[i].Params, 0, "")
		t.sa(f.Triggers[i].Effect, 0, 0)
	}
	for i := range f.Statics {
		t.static(f.Statics[i].Params)
	}
	for i := range f.Repls {
		t.params(f.Repls[i].Params, 0, "")
		t.sa(f.Repls[i].With, 0, 0)
	}
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
