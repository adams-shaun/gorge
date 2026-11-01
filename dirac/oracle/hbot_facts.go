package oracle

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// HBotFacts is HBotFactsSet over the "fdn" set.
func HBotFacts(reg *cards.Registry, tbl *kop.Table, names map[string]string) *hbot.Facts {
	return HBotFactsSet(reg, tbl, names, fdnSet())
}

// HBotFactsSet builds the kernel heuristic bot's per-table facts
// (dirac/search/hbot.Facts) from gorge's compiled cards: what gorge's
// heuristic bot reads off a card's strings and an option's facts, read
// once into plain numbers. names maps a kernel card name to gorge's (as
// NewSBTCardsSet).
func HBotFactsSet(reg *cards.Registry, tbl *kop.Table, names map[string]string, set cardset.Set) *hbot.Facts {
	sc := NewSBTCardsSet(reg, tbl, names, set)
	f := hbot.NewFacts(tbl)
	for i := range tbl.Cards {
		gc := sc.card(dirac.CardID(i)).gc
		if gc == nil {
			continue
		}
		face := gc.Faces[0]
		c := &f.Cards[i]
		c.CMC = int8(botpolicy.CmcOf(face.ManaCost))
		c.Creature = face.IsCreature()
		c.Basic, c.Instant = false, false
		for _, t := range face.Types {
			if strings.EqualFold(t, "Basic") {
				c.Basic = true
			}
			if strings.EqualFold(t, "Instant") {
				c.Instant = true
			}
		}
		c.Counter = false
		if sa := face.SpellAbility(); sa != nil && sa.API == "Counter" {
			c.Counter = true
		}
		sa := face.SpellAbility()
		c.Trick = effects.CastCombatTrick(sa)
		c.FaceDmg = 0
		if effects.CastRemovalKind(sa) == "damage" {
			if d := effects.CastRemovalDamage(sa); d > 0 {
				switch effects.CastRemovalTargets(sa) {
				case "any", "player":
					c.FaceDmg = int16(min(d, 32767))
				}
			}
		}
		mp := face.ManaProduction()
		c.Produces = [6]int8{}
		for k := 0; k < 6; k++ {
			c.Produces[k] = int8(min(mp.Colour[k], 127))
		}
		c.ProdAny, c.ProdReflected, c.ProdIndet = mp.Any, mp.Reflected, mp.Indeterminate
		c.SpellTgt = hbotTgtChain(face, face.SpellAbility())
		cd := &tbl.Cards[i]
		for j := range cd.Abilities {
			ab := &c.Abilities[j]
			ka := &cd.Abilities[j]
			var sa *cards.SA
			if ka.Kind == kop.AbSpell {
				sa = face.SpellAbility()
			} else if gi, ok := sc.gorgeAbility(dirac.CardID(i), uint8(j)); ok {
				sa = sc.gorgeSA(dirac.CardID(i), gi)
			}
			if sa == nil {
				continue
			}
			ab.Tgt = hbotTgtChain(face, sa)
			if ka.Kind != kop.AbActivated && ka.Kind != kop.AbMana {
				continue
			}
			ab.Cost = hbotLabelCost(face.Name + ": " + sa.ParamStr(cards.PKSpellDescription))
			ab.Attach = sa.API == "Attach"
			if kw := hbotPureGrant(sa); len(kw) > 0 {
				ab.GrantOK = true
				for _, k := range kw {
					ab.Grant |= hbotKw(k)
				}
			}
			ab.Boon = hbotBoonOnly(hbotStaticModes(sa, face.SVars))
			if ka.Cost.Flags&kop.CostLoyalty != 0 {
				ab.Loyal = true
				ab.LUltimate = strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKUltimate)), "True")
				ab.LRemove = hbotRemovesPermanent(sa)
				cost := sa.ParamStr(cards.PKCost)
				ab.LDelta, ab.LX = hbotLoyaltyDelta(cost)
			}
		}
	}
	return f
}

// hbotLabelCost is botpolicy's abilityCost: the label's first digit run,
// else 20.
func hbotLabelCost(label string) int16 {
	var cur int32
	have := false
	for i := 0; i < len(label); i++ {
		c := label[i]
		if c >= '0' && c <= '9' {
			if have {
				cur = cur*10 + int32(c-'0')
			} else {
				cur = int32(c - '0')
				have = true
			}
			if cur > 30000 {
				cur = 30000
			}
		} else if have {
			return int16(cur)
		}
	}
	if have {
		return int16(cur)
	}
	return 20
}

// hbotPureGrant is gorge rules' pureGrantKeywords: a Pump with no
// sub-ability, defined on itself (or untargeted), adding keywords and
// nothing else.
func hbotPureGrant(ab *cards.SA) []string {
	if ab == nil || ab.API != "Pump" || ab.Sub != nil {
		return nil
	}
	switch strings.TrimSpace(string(effects.DefinedRefOf(ab).Raw)) {
	case "Self":
	case "":
		if effects.TargetsOf(ab).Has(effects.TgtValidPresent) {
			return nil
		}
	default:
		return nil
	}
	pp := effects.PumpOf(ab)
	if pp.NumAtt.Present || pp.NumDef.Present {
		return nil
	}
	if strings.Contains(pp.Grant.KWText, ":") {
		return nil
	}
	var out []string
	for _, p := range cards.SplitKeywordList(pp.Grant.KWText) {
		out = append(out, cards.KeywordHead(p))
	}
	return out
}

// hbotKw is the kop keyword bit of a gorge keyword head (0 when the kernel
// has none).
func hbotKw(k string) kop.KwMask {
	switch strings.ToLower(k) {
	case "flying":
		return kop.KwFlying
	case "reach":
		return kop.KwReach
	case "first strike":
		return kop.KwFirstStrike
	case "double strike":
		return kop.KwDoubleStrike
	case "deathtouch":
		return kop.KwDeathtouch
	case "lifelink":
		return kop.KwLifelink
	case "trample":
		return kop.KwTrample
	case "vigilance":
		return kop.KwVigilance
	case "haste":
		return kop.KwHaste
	case "menace":
		return kop.KwMenace
	case "defender":
		return kop.KwDefender
	case "flash":
		return kop.KwFlash
	case "hexproof":
		return kop.KwHexproof
	case "indestructible":
		return kop.KwIndestructible
	case "prowess":
		return kop.KwProwess
	case "shroud":
		return kop.KwShroud
	}
	return 0
}

// hbotStaticModes is gorge rules' staticModesFromSVars.
func hbotStaticModes(sa *cards.SA, svars map[string]string) []string {
	if sa == nil || sa.API != "Effect" || svars == nil {
		return nil
	}
	names := strings.FieldsFunc(effects.EffectOf(sa).StaticAbilities, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	var out []string
	for _, name := range names {
		body := strings.TrimSpace(svars[name])
		if body == "" {
			continue
		}
		statics, ok := cards.ParseStaticLines(body)
		if !ok {
			continue
		}
		for _, st := range statics {
			if st.Mode != "" {
				out = append(out, st.Mode)
			}
		}
	}
	return out
}

// hbotBoonOnly is botpolicy's boonStaticsOnly.
func hbotBoonOnly(statics []string) bool {
	if len(statics) == 0 {
		return false
	}
	for _, m := range statics {
		if m != "CantBlockBy" && m != "CanAttackDefender" {
			return false
		}
	}
	return true
}

// hbotRemovesPermanent is gorge rules' abilityRemovesPermanent (the
// battlefield-origin ChangeZone family read off the destination).
func hbotRemovesPermanent(ab *cards.SA) bool {
	for sa, depth := ab, 0; sa != nil && depth < 32; sa, depth = sa.Sub, depth+1 {
		switch sa.API {
		case "Destroy", "DestroyAll", "DealDamage", "DamageAll", "Sacrifice", "SacrificeAll", "Fight":
			return true
		case "ChangeZone":
			cz := effects.ChangeZoneOf(sa)
			if strings.Contains(strings.ToLower(sa.ParamStr(cards.PKOrigin)), "battlefield") && cz.DestinationKnown &&
				!strings.EqualFold(cz.Destination.String(), "battlefield") {
				return true
			}
		}
	}
	return false
}

// hbotLoyaltyDelta reads a loyalty cost's net change: +N AddCounter, -N
// SubCounter, X for an announced SubCounter<X/LOYALTY>.
func hbotLoyaltyDelta(cost string) (int8, bool) {
	var d int
	x := false
	for _, part := range strings.Fields(cost) {
		var sign int
		switch {
		case strings.HasPrefix(part, "AddCounter<"):
			sign = 1
		case strings.HasPrefix(part, "SubCounter<"):
			sign = -1
		default:
			continue
		}
		body := part[strings.IndexByte(part, '<')+1:]
		n, kind, ok := strings.Cut(strings.TrimSuffix(body, ">"), "/")
		if !ok || !strings.EqualFold(kind, "LOYALTY") {
			continue
		}
		if n == "X" {
			x = true
			continue
		}
		v := 0
		for _, ch := range n {
			if ch < '0' || ch > '9' {
				v = -1
				break
			}
			v = v*10 + int(ch-'0')
		}
		if v >= 0 {
			d += sign * v
		}
	}
	return int8(d), x
}

// hbotTgtChain is the targeted SAs of sa's chain (a charm's chosen modes'
// chains carry their mode index), each as gorge's TargetEffect facts
// (sbtTargetEffect) and the ValidTgts$ fit (sbtSlotFits).
func hbotTgtChain(f *cards.Face, sa *cards.SA) []hbot.TgtEff {
	var out []hbot.TgtEff
	add := func(x *cards.SA, mode int8) {
		if _, ok := x.Params["ValidTgts"]; !ok {
			if _, ok := x.Params["TgtPrompt"]; !ok {
				return
			}
		}
		out = append(out, hbotTgtEff(f, x, mode))
	}
	for x := sa; x != nil; x = x.Sub {
		if x.API == "Charm" || x.API == "GenericChoice" {
			for i, name := range strings.Split(x.Params["Choices"], ",") {
				if i >= 8 {
					break
				}
				for m := cards.ResolveSVar(f.SVars, strings.TrimSpace(name)); m != nil; m = m.Sub {
					add(m, int8(i))
				}
			}
			continue
		}
		add(x, -1)
	}
	return out
}

func hbotTgtEff(f *cards.Face, x *cards.SA, mode int8) hbot.TgtEff {
	te := sbtTargetEffect(x)
	e := hbot.TgtEff{Mode: mode, Dmg: -1}
	switch te.API {
	case "DealDamage":
		e.API = hbot.ApiDamage
	case "DamageAll":
		e.API = hbot.ApiDamageAll
	case "Counter":
		e.API = hbot.ApiCounter
	case "Attach":
		e.API = hbot.ApiAttach
	default:
		e.API = hbot.ApiOther
	}
	e.DmgK = -1
	if te.Damage != nil && te.Damage.Amount != nil {
		e.Dmg = int16(min(*te.Damage.Amount, 30000))
		e.DmgK = e.Dmg
	} else if te.Damage != nil {
		// gorge evaluates a dynamic amount at the ask: the kicked count
		// and the announced X are the shapes the pool uses
		amt := strings.TrimSpace(effects.DamageAmount(x).Text)
		body := strings.TrimSpace(f.SVars[amt])
		switch {
		case amt == "X" && (body == "" || body == "Count$xPaid"):
			e.DmgX = true
		case strings.HasPrefix(body, "Count$Kicked."):
			parts := strings.Split(strings.TrimPrefix(body, "Count$Kicked."), ".")
			if len(parts) == 2 {
				k, err1 := strconv.Atoi(parts[0])
				u, err2 := strconv.Atoi(parts[1])
				if err1 == nil && err2 == nil {
					e.Dmg, e.DmgK = int16(u), int16(k)
				}
			}
		}
	}
	if te.Removal != nil {
		switch te.Removal.Kind {
		case "exile":
			e.Remove = hbot.RmExile
		case "command":
			e.Remove = hbot.RmCommand
		case "library":
			e.Remove = hbot.RmLibrary
		case "graveyard":
			e.Remove = hbot.RmGraveyard
		case "destroy":
			e.Remove = hbot.RmDestroy
		case "sacrifice":
			e.Remove = hbot.RmSacrifice
		case "bounce":
			e.Remove = hbot.RmBounce
		}
	}
	if te.API == "Attach" {
		e.AttachPol = hbotAttachPolarity(effects.AttachAILogic(f))
	}
	e.Boon = hbotBoonOnly(hbotStaticModes(x, f.SVars))
	e.Fit = hbotFit(x.Params["ValidTgts"])
	return e
}

// hbotAttachPolarity is botpolicy's attachPolarity.
func hbotAttachPolarity(logic string) int8 {
	switch logic {
	case "Pump", "Animate", "Curiosity":
		return 1
	case "Curse", "GainControl", "ChangeType", "KeepTapped", "HighestEvaluation":
		return -1
	}
	return 0
}

// hbotFit is the controller fit sbtSlotFits reads off ValidTgts$.
func hbotFit(valid string) uint8 {
	if strings.TrimSpace(valid) == "" {
		return hbot.FitMine | hbot.FitOpp | hbot.FitPlayerMine | hbot.FitPlayerOpp
	}
	var fit uint8
	for _, alt := range strings.Split(valid, ",") {
		alt = strings.TrimSpace(alt)
		typ, quals, _ := strings.Cut(alt, ".")
		mine, opp := true, true
		for _, q := range strings.Split(quals, "+") {
			switch q {
			case "YouCtrl", "YouOwn", "You":
				opp = false
			case "OppCtrl", "OppOwn", "Opponent":
				mine = false
			}
		}
		switch typ {
		case "Player":
			if mine {
				fit |= hbot.FitPlayerMine
			}
			if opp {
				fit |= hbot.FitPlayerOpp
			}
		case "Opponent":
			fit |= hbot.FitPlayerOpp
		case "Any":
			if mine {
				fit |= hbot.FitPlayerMine | hbot.FitMine
			}
			if opp {
				fit |= hbot.FitPlayerOpp | hbot.FitOpp
			}
		default:
			if mine {
				fit |= hbot.FitMine
			}
			if opp {
				fit |= hbot.FitOpp
			}
		}
	}
	return fit
}
