package kgen

import (
	"fmt"
	"math/bits"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The relation and keyword filter words (T13b): each is a kernel FilterWord
// literal (dirac/filter_*.go), emitted through the clause's spill program
// (filter_spill.go), so a filter that lowers today keeps its struct form and
// only a filter that refused gains a program.

// fwArm lowers one filter property p of the clause.
type fwArm func(c *lClause, p string) error

// fwArms maps a property spelling to its arm; filter.go's property switch
// asks it last, before "unknown property".
var fwArms = map[string]fwArm{}

func init() {
	fwName[kop.FwWithKeyword] = "FwWithKeyword"
	fwName[kop.FwChosenCtrl] = "FwChosenCtrl"
	fwName[kop.FwEnchantedControllerCtrl] = "FwEnchantedControllerCtrl"
	fwName[kop.FwSharesCr] = "FwSharesCr"
	fwName[kop.FwSharesName] = "FwSharesName"
	fwName[kop.FwSameName] = "FwSameName"
	fwName[kop.FwSameManaType] = "FwSameManaType"

	// with<K>, without<K> and !with<K> over a printed keyword the kernel keeps
	// a record of (dirac/filter_withkeyword.go printedKeyword); gorge registers
	// all three spellings (engine/effects/filter.go's keyword registry), under
	// the compact key — and, for the names with spaces, the spaced spellings
	// too (keywordPredicateFor's normalization makes "with Doctor's companion"
	// and "withDoctor'scompanion" one keyword to gorge, so both lower here).
	for name, id := range fwKeywordWords {
		spellings := []string{name}
		if spaced, ok := fwKeywordSpaced[name]; ok {
			spellings = append(spellings, spaced, " "+spaced)
		}
		for _, spell := range spellings {
			for _, p := range [...]struct {
				spell string
				neg   bool
			}{{"with" + spell, false}, {"without" + spell, true}, {"!with" + spell, true}} {
				id, neg := id, p.neg
				fwArms[p.spell] = func(c *lClause, _ string) error {
					return c.fwLit(lSpillLit{W: kop.FwWithKeyword, Neg: neg, N: int(id)})
				}
			}
		}
	}
	// the referent words (T13b-2): gorge's two-token
	// sharesCreatureTypeWith/sharesNameWith <ref> (sharesTypeArg's vocabulary,
	// resolved by sharesTypeReferents), each a program literal carrying its
	// referent in R. The referents the kernel keeps no live object of stay
	// refused (fwShareRefs/fwNameRefs name only what the handler reads):
	// Convoked (the kernel records the count, never the creatures), and
	// Sacrificed/Enchanted (not in gorge's vocabulary).
	for _, fam := range []struct {
		w    kop.FilterWord
		head string
		refs map[string]kop.Ref
	}{
		{kop.FwSharesCr, "sharesCreatureTypeWith", fwShareRefs},
		{kop.FwSharesName, "sharesNameWith", fwNameRefs},
	} {
		w, head := fam.w, fam.head
		for ref, r := range fam.refs {
			ref, r := ref, r
			for _, pre := range [...]struct {
				spell string
				neg   bool
			}{{"", false}, {"!", true}} {
				neg := pre.neg
				fwArms[pre.spell+head+" "+ref] = func(c *lClause, _ string) error {
					return c.fwLit(lSpillLit{W: w, Neg: neg, R: r})
				}
			}
		}
	}
	// withFlash reads the KwMask bit the kernel already keeps (the printed mask
	// for a card), the struct's PredHasKeyword form, which refuses Flash on the
	// battlefield. withHexproof is not lowered, with or without: gorge matches
	// by keyword head, so it also matches Hexproof:<spec>, which the plain
	// KwHexproof bit cannot see (TestWithHexproofStaysRefused).
	flashBit := bits.TrailingZeros32(uint32(kop.KwFlash))
	fwArms["withFlash"] = func(c *lClause, _ string) error { return c.setCmp(cmpHasKw, 0, flashBit, "", "") }
	for _, p := range [...]string{"withoutFlash", "!withFlash"} {
		fwArms[p] = func(c *lClause, _ string) error { return c.setCmp(cmpLacksKw, 0, flashBit, "", "") }
	}
	// ChosenCtrl: controlled by the player the source chose as it entered.
	fwArms["ChosenCtrl"] = func(c *lClause, _ string) error {
		return c.fwLit(lSpillLit{W: kop.FwChosenCtrl})
	}
	// EnchantedControllerCtrl: controlled by the controller of the permanent
	// the source is attached to. gorge refuses the '!' of it too (an absent
	// link inverts to nothing), so only the positive spelling lowers.
	fwArms["EnchantedControllerCtrl"] = func(c *lClause, _ string) error {
		return c.fwLit(lSpillLit{W: kop.FwEnchantedControllerCtrl})
	}
}

// fwKeywordWords are the printed-keyword words lowered to FwWithKeyword, by
// gorge's compact keyword name (spaces removed): only keywords the word's
// kernel read answers as gorge does. Disturb, Mutate and Phasing lower onto
// the word's fail-closed default: their K: line refuses the whole face, so no
// lowered card can print them, and printedKeyword's default false is exact
// over every lowered card. Flanking reads CardDef.Flanking, set by the
// printed bare K:Flanking's lowering.
var fwKeywordWords = map[string]kop.KwID{
	"Toxic": kop.KwToxic, "Horsemanship": kop.KwHorsemanship, "Morph": kop.KwMorph, "Megamorph": kop.KwMegamorph,
	"Suspend": kop.KwSuspend, "Cascade": kop.KwCascade, "Convoke": kop.KwConvoke, "Foretell": kop.KwForetell,
	"Madness": kop.KwMadness, "Multikicker": kop.KwMultikicker,
	// T13b-2's records: the keywords with no KwMask bit. Cycling, Ward,
	// Enchant, Unearth, Decayed, Persist and Doctor's companion carry their
	// own cold CardDef field (dirac/filter_withkeyword.go printedKeyword).
	"Cycling": kop.KwCycling, "Ward": kop.KwPrintedWard, "Enchant": kop.KwEnchant,
	"Unearth": kop.KwUnearth, "Decayed": kop.KwDecayed, "Persist": kop.KwPersist,
	"Doctor'scompanion": kop.KwDoctorsCompanion,
	// Flanking (CardDef.Flanking) and the three whose printed face no lowered
	// card can carry
	"Flanking": kop.KwFlanking, "Disturb": kop.KwDisturb, "Mutate": kop.KwMutate, "Phasing": kop.KwPhasing,
}

// fwKeywordSpaced are the compact names that also read in a spaced spelling
// (gorge keywordPredicateFor's normalization removes every space of the
// token): the corpus writes both.
var fwKeywordSpaced = map[string]string{"Doctor'scompanion": "Doctor's companion"}

// fwShareRefs maps the sharesCreatureTypeWith referent spellings gorge reads
// (sharesTypeArg's vocabulary, engine/effects/filter_attachment.go) to the
// Ref the program literal carries, for the referents the kernel keeps live:
// Targeted (the reading context entry's object targets, every slot — the
// handler iterates them all, so the Ref's slot span is not a bound), Self,
// RememberedCard (the entry's remembered object, gorge's first card entry),
// Remembered and RememberedLKI (gorge's whole-set spellings over the SAME
// sc.Remembered list; the kernel keeps the set, g.rem, so the read is exact
// where RememberedCard names only its first entry), Imprinted (the source's
// first imprinted card, RefImprinted's read), and Convoked (the creatures
// that convoked the resolving spell's cast, CR 702.66; the cast's stack
// entry holds their set, dirac/kw_convokelist.go, where the count rode
// stackEntry.prov bits 3-7 alone). Commander, TriggeredAttacker and
// TriggeredCard keep their own lowerings (cmpSharesCrCmdr, W7SharesCrTrigObj);
// Sacrificed and Enchanted stay refused, gorge's sharesTypeArg never
// accepting them.
var fwShareRefs = map[string]kop.Ref{
	"Targeted": kop.RefTargets0123, "Self": kop.RefSelf,
	"RememberedCard": kop.RefRemembered, "Imprinted": kop.RefImprinted,
	"Remembered": kop.RefRememberedSet, "RememberedLKI": kop.RefRememberedSet,
	"Convoked": kop.RefConvoked,
}

// fwNameRefs is fwShareRefs for sharesNameWith: Targeted, RememberedCard and
// the triggering object (TriggeredCard, TriggeredCardLKICopy). Remembered and
// Self keep their struct lowerings (cmpSameNameRem, cmpSameNameSelf), the one
// Valid <filter> carrier (Permanent.Legendary) its W7SharesNameLegend one,
// and the general Valid <filter> form lowers as prop's own tail arm
// (filter.go fwSharesNameValid), the inner filter riding the literal's F.
var fwNameRefs = map[string]kop.Ref{
	"Targeted": kop.RefTargets0123, "RememberedCard": kop.RefRemembered,
	"TriggeredCard": kop.RefTriggerObj, "TriggeredCardLKICopy": kop.RefTriggerObj,
}

// fwLit adds one filter-word literal to the clause's program, moving a
// comparison already in the Pred slot into it first (as spillCmp does). A
// comparison with no word, or a full program, refuses.
func (c *lClause) fwLit(l lSpillLit) error {
	switch c.CmpStat {
	case cmpNone:
		c.CmpStat = cmpSpill
	case cmpSpill:
	default:
		first, ok := cmpLit(c.CmpStat, c.CmpOp, c.CmpN, c.CmpX, c.CmpCounter)
		if !ok {
			return fmt.Errorf("filter word %s beside a comparison with no word", fwName[l.W])
		}
		c.pushSpill(first)
		c.CmpStat, c.CmpOp, c.CmpN, c.CmpX, c.CmpCounter = cmpSpill, 0, 0, "", ""
	}
	if !c.pushSpill(l) {
		return fmt.Errorf("filter word %s: the clause's program is full", fwName[l.W])
	}
	return nil
}

// fwArmOrUnknown lowers property p by its arm, or is the unknown-property
// error: the tail of lClause.prop's switch. The prefix gate names the word
// families the arm table lowers: with<K> (and its without/!with spellings),
// the two-token share words, the two link words (T13b), and T13c's zone,
// printed-layout and base-P/T words, the base compares parameterised by the
// bound. The general sharesNameWith Valid <filter> form (fwSharesNameValid)
// is answered first: its exact one-word carrier filterWordPred2 answered
// before this arm was reached.
func (c *lClause) fwArmOrUnknown(p string, knownSub func(string) bool) error {
	if inner, neg, ok := sharesNameValidArg(p); ok {
		return c.fwSharesNameValid(inner, neg, knownSub)
	}
	if strings.HasPrefix(p, "with") || strings.HasPrefix(p, "!with") ||
		strings.HasPrefix(p, "sharesCreatureTypeWith") || strings.HasPrefix(p, "!sharesCreatureTypeWith") ||
		strings.HasPrefix(p, "sharesNameWith") || strings.HasPrefix(p, "!sharesNameWith") ||
		p == "sameName" || p == "!sameName" ||
		p == "ChosenCtrl" || p == "EnchantedControllerCtrl" ||
		p == "suspended" || p == "TopLibrary" || p == "DoubleFaced" ||
		p == "Modal" || p == "singleTarget" {
		if arm, ok := fwArms[p]; ok {
			return arm(c, p)
		}
	}
	for _, pre := range [...]string{"basePower", "baseToughness"} {
		if _, ok := strings.CutPrefix(p, pre); ok {
			return c.basePTArm(p, pre) // filter_words_zoneprov.go: the layer-7b base P/T compare
		}
	}
	return fmt.Errorf("unknown property %q", p)
}

// sharesNameValidArg splits the general sharesNameWith Valid <filter> word
// (gorge sharesNameValidBattlefield, engine/effects/filter_attachment.go):
// the inner filter is what survives in the token, the spec splitters having
// cut the outer '+' and ',' before the word was read. The zone-suffixed
// ValidExile/ValidGraveyard spellings are a different, unmodelled referent
// and stay unknown.
func sharesNameValidArg(p string) (inner string, neg, ok bool) {
	if rest, has := strings.CutPrefix(p, "!"); has {
		neg, p = true, rest
	}
	inner, ok = strings.CutPrefix(p, "sharesNameWith Valid ")
	if !ok {
		return "", neg, false
	}
	inner = strings.TrimSpace(inner)
	if inner == "" || strings.ContainsAny(inner, "+,") {
		// gorge never sees an inner with a '+' or ',': the token would have
		// been cut first; an empty inner is an unbound referent
		return "", neg, false
	}
	return inner, neg, true
}

// fwSharesNameValid lowers the general form: the word carries the inner
// filter as its nested-filter operand, lowered over the battlefield the
// sweep reads (kop.FLit.F; kernel fwSharesName's Valid arm). The parse keeps
// the inner filter on the spill literal; spillProg interns it.
func (c *lClause) fwSharesNameValid(inner string, neg bool, knownSub func(string) bool) error {
	ic, err := parseClause(inner, knownSub)
	if err != nil {
		return fmt.Errorf("sharesNameWith Valid %q: %w", inner, err)
	}
	return c.fwLit(lSpillLit{W: kop.FwSharesName, Neg: neg, Inner: &lFilter{Clauses: []lClause{ic}}})
}
