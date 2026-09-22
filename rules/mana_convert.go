package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

// stat:ManaConvert (CR 608.2i, "spend mana as though it were mana of any
// color"): the S:Mode$ ManaConvert statics. This file owns the grammar and
// the one per-payment conversion set; rules/mana.go's resolveMana consults
// it through the *manaConv parameter on the payment call sites, so an
// offered cost (castable / manaAbilityPayable / the mana window / the X ask)
// and the cost actually charged can never disagree about what the payer may
// spend.
//
// The conversion is a fact about the PAYER'S mana for ONE payment, never
// about the cost: "you may spend white mana as though it were red mana"
// leaves the {R} pip exactly what it was and lets the pool's white mana pay
// it. That is why the model below annotates pool colours, not pips.

// manaConv is the colour-conversion set one payment is resolved under. The
// zero value converts nothing, which is exactly the behaviour of a game with
// no ManaConvert static on the battlefield -- every pre-existing call site
// degenerates to today's exact-colour match, so no golden game can move
// merely because the machinery exists.
type manaConv struct {
	// wild[i] is true when the payer's mana of colour i (manaLetters order)
	// may be spent as mana of any COLOUR (CR 608.2i's "any color"): any
	// coloured pip or hybrid pip may be paid with it. Colourless-specific
	// {C} pips are NOT covered -- {C} is not a colour (CR 107.4c) -- unless
	// wildC is also set (the AnyType->AnyType wording, "mana of any type").
	wild [6]bool
	// wildC extends wild to the colourless-specific {C} pips. Set by the
	// AnyType->AnyType conversion ("mana of any type can be spent..."); the
	// corpus carries it on two script lines.
	wildC bool
	// to[i][j] is true when the payer's mana of colour i may be spent as
	// though it were mana of colour j (the White->Red wording). It is a
	// specific mapping, consulted only when the exact-colour match and wild
	// both fail.
	to [6][6]bool
	// onlyC[i] is the <-C RESTRICTION ("you may spend other mana only as
	// though it were colorless mana", the nonWhite<-C wording, one corpus
	// occurrence): the payer's mana of colour i may pay ONLY a
	// colourless-specific {C} pip or generic, never a coloured pip -- not
	// even its own colour's pip, which is the whole point of the
	// restriction.
	onlyC [6]bool
}

// empty reports whether the conversion would change any pip match. The
// payment sites skip the conversion path entirely on an empty conv so the
// pure resolveMana (and with it every pre-existing game) is byte-identical.
func (m *manaConv) empty() bool {
	if m.wildC {
		return false
	}
	for i := range m.wild {
		if m.wild[i] || m.onlyC[i] {
			return false
		}
		for j := range m.to[i] {
			if m.to[i][j] {
				return false
			}
		}
	}
	return true
}

// manaColourFrom maps a ManaConversion$ "from" word to the pool-colour
// indexes it names: a colour name or letter is that colour; "AnyType" is all
// six (any type includes colourless); "non<name>" is every colour except the
// named one. An unknown word names nothing -- a conversion this build cannot
// parse is silently inert rather than guessed wide.
func manaColourFrom(word string) []int {
	name := strings.TrimSpace(word)
	complement := false
	if rest, ok := strings.CutPrefix(name, "non"); ok && rest != "" {
		name, complement = rest, true
	}
	var base []int
	switch strings.ToLower(name) {
	case "anytype":
		base = []int{state.MW, state.MU, state.MB, state.MR, state.MG, state.MC}
	case "w", "white":
		base = []int{state.MW}
	case "u", "blue":
		base = []int{state.MU}
	case "b", "black":
		base = []int{state.MB}
	case "r", "red":
		base = []int{state.MR}
	case "g", "green":
		base = []int{state.MG}
	case "c", "colorless":
		base = []int{state.MC}
	default:
		return nil
	}
	if !complement {
		return base
	}
	excl := map[int]bool{}
	for _, i := range base {
		excl[i] = true
	}
	var out []int
	for i := range manaLetters {
		if !excl[i] {
			out = append(out, i)
		}
	}
	return out
}

// manaColourTo maps a ManaConversion$ "to" word to the pip it makes
// acceptable: a colour name or letter is that pip; "AnyColor" is the wild
// (any colour, not {C}) grant; "AnyType" is wild plus the {C} pips. The
// second return reports whether the word resolved at all, so an unknown
// conversion target leaves the static inert rather than granting everything.
func applyManaConversionTo(conv *manaConv, from []int, word string) bool {
	name := strings.ToLower(strings.TrimSpace(word))
	switch name {
	case "anycolor":
		for _, i := range from {
			conv.wild[i] = true
		}
	case "anytype":
		for _, i := range from {
			conv.wild[i] = true
		}
		conv.wildC = true
	case "c", "colorless":
		// "as though it were colorless mana" as a GRANT: mana of the "from"
		// colours may additionally pay {C} pips. Not a corpus shape today,
		// but the same grammar the restriction side uses; kept for symmetry.
		for _, i := range from {
			conv.to[i][state.MC] = true
		}
	case "w", "white":
		for _, i := range from {
			conv.to[i][state.MW] = true
		}
	case "u", "blue":
		for _, i := range from {
			conv.to[i][state.MU] = true
		}
	case "b", "black":
		for _, i := range from {
			conv.to[i][state.MB] = true
		}
	case "r", "red":
		for _, i := range from {
			conv.to[i][state.MR] = true
		}
	case "g", "green":
		for _, i := range from {
			conv.to[i][state.MG] = true
		}
	default:
		return false
	}
	return true
}

// staticSAKindMatches reports whether a ValidSA$ value on a ManaConvert
// static admits this kind of payment. The value is Forge's comma-separated
// OR list of "<kind>[.<qualifier>]" ("Spell", "Activated", "Spell.MayPlaySource",
// "Spell,Activated"); an absent value admits both. An alternative whose kind
// is neither Spell nor Activated (a Loyalty/other qualifier this build
// cannot evaluate) is skipped rather than guessed: a conversion the engine
// cannot scope never grants, the fail-closed direction every other
// restriction-class reader here uses.
func staticSAKindMatches(validSA string, ability bool) bool {
	v := strings.TrimSpace(validSA)
	if v == "" {
		return true
	}
	for _, alt := range strings.Split(v, ",") {
		kind := strings.TrimSpace(alt)
		if i := strings.IndexByte(kind, '.'); i >= 0 {
			kind = kind[:i]
		}
		switch kind {
		case "Spell":
			if !ability {
				return true
			}
		case "Activated":
			if ability {
				return true
			}
		}
	}
	return false
}

// manaConversion composes the conversion set for one payment. Printed
// statics are collected from every EffectZone-admitted source zone (including
// Command), while Effect-delivered SVar statics are read from the active
// continuous-effect registry. Optional grants are returned separately so the
// cast flow can make a real election instead of silently applying them.
func (e *Engine) manaConversion(p state.PlayerID, id state.ObjID, ability bool) manaConv {
	mandatory, optional := e.manaConversionParts(p, id, ability)
	mergeManaConv(&mandatory, optional)
	return mandatory
}

func (e *Engine) manaConversionParts(p state.PlayerID, id state.ObjID, ability bool) (manaConv, manaConv) {
	var mandatory, optional manaConv
	apply := func(sv staticView) {
		if vp, ok := sv.Params["ValidPlayer"]; ok &&
			!effects.MatchesPlayerSpec(e.G, vp, p, sv.Controller) {
			return
		}
		if vc, ok := sv.Params["ValidCard"]; ok && vc != "" &&
			!effects.MatchesSpecCtx(e.G, vc, id, e.specCtx(sv.Source, p)) {
			return
		}
		if vsa, ok := sv.Params["ValidSA"]; ok && !staticSAKindMatches(vsa, ability) {
			return
		}
		dst := &mandatory
		if strings.EqualFold(strings.TrimSpace(sv.Params["Optional"]), "True") {
			dst = &optional
		}
		for _, tok := range strings.Fields(sv.Params["ManaConversion"]) {
			if from, to, ok := strings.Cut(tok, "->"); ok {
				if froms := manaColourFrom(from); froms != nil {
					applyManaConversionTo(dst, froms, to)
				}
				continue
			}
			if from, ok := strings.CutSuffix(tok, "<-C"); ok {
				if froms := manaColourFrom(from); froms != nil {
					for _, i := range froms {
						dst.onlyC[i] = true
					}
				}
			}
		}
	}
	for pi, p := range e.G.AliveFrom(0) {
		for _, z := range staticSourceZones {
			if z == state.ZStack && pi > 0 {
				continue
			}
			for _, oid := range e.G.Zone(z, p) {
				o := e.G.Obj(oid)
				if o == nil || o.Face() == nil || (z == state.ZBattlefield && e.faceDownPrintedHides(o)) {
					continue
				}
				for si, sn := 0, o.PileStaticCount(); si < sn; si++ {
					pst, ok := o.PileStaticAt(si)
					if !ok || pst.Static.Mode != "ManaConvert" || !effectZoneOK(pst.Static.Params["EffectZone"], o.Zone) {
						continue
					}
					apply(staticView{Source: oid, Controller: o.Controller, Params: pst.Static.Params, SVars: pst.Face.SVars})
				}
			}
		}
	}
	for _, ce := range e.active() {
		if ce.CostStaticMode == "ManaConvert" {
			apply(staticView{Source: ce.Source, Controller: ce.Controller,
				Params: ce.CostStaticParams, SVars: ce.CostStaticSVars})
		}
	}
	return mandatory, optional
}

func mergeManaConv(dst *manaConv, src manaConv) {
	for i := range dst.wild {
		dst.wild[i] = dst.wild[i] || src.wild[i]
		dst.onlyC[i] = dst.onlyC[i] || src.onlyC[i]
		for j := range dst.to[i] {
			dst.to[i][j] = dst.to[i][j] || src.to[i][j]
		}
	}
	dst.wildC = dst.wildC || src.wildC
}
