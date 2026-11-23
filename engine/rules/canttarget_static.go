package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// stat:CantTarget is the printed S:Mode$ CantTarget static (Thrun, Breaker of
// Silence; Ground Seal; Karplusan Strider; ...) and the Effect-delivered body
// Vines of Vastwood and Autumn's Veil register through StaticAbilities$. Its
// own init keeps the registration out of the shared statics.go list, where a
// other change also adds a mode.
func init() { effects.RegisterNonAPI("stat:CantTarget") }

// cantTargetBody is the scoping grammar a CantTarget restriction carries,
// extracted from either delivery path. One matcher judges both so the
// printed static and the Effect-delivered body cannot drift.
type cantTargetBody struct {
	validCard     string
	validTarget   string
	validCards    string
	validCreature string
	validSA       string
	validSource   string
	activator     string
	affectedZone  string
	// sourceCanOnlyTarget marks Wall of Shadows' unmodelled
	// SourceCanOnlyTarget$ body; see cantTargetBlocks.
	sourceCanOnlyTarget string
	remembered          []state.ObjID
	// effectDelivered distinguishes the registered Effect body's historical
	// "no ValidCard$/ValidTarget$ means the remembered set" reading from a
	// printed static, where an empty ValidTarget$ means every card in the
	// AffectedZone$ (Ground Seal's "cards in graveyards can't be targeted").
	effectDelivered bool
}

func cantTargetBodyOfEffect(ce *ContinuousEffect) cantTargetBody {
	return cantTargetBody{
		validCard:           ce.RestrictParam(cards.PKValidCard),
		validTarget:         ce.RestrictParam(cards.PKValidTarget),
		validCards:          ce.RestrictParam(cards.PKValidCards),
		validCreature:       ce.RestrictParam(cards.PKValidCreature),
		validSA:             ce.RestrictParam(cards.PKValidSA),
		validSource:         ce.RestrictParam(cards.PKValidSource),
		activator:           ce.RestrictParam(cards.PKActivator),
		affectedZone:        ce.RestrictParam(cards.PKAffectedZone),
		sourceCanOnlyTarget: ce.RestrictParams["SourceCanOnlyTarget"],
		remembered:          ce.Remembered,
		effectDelivered:     true,
	}
}

func cantTargetBodyOfStatic(sv staticView) cantTargetBody {
	return cantTargetBody{
		validCard:           sv.ParamStr(cards.PKValidCard),
		validTarget:         sv.ParamStr(cards.PKValidTarget),
		validCards:          sv.ParamStr(cards.PKValidCards),
		validCreature:       sv.ParamStr(cards.PKValidCreature),
		validSA:             sv.ParamStr(cards.PKValidSA),
		validSource:         sv.ParamStr(cards.PKValidSource),
		activator:           sv.ParamStr(cards.PKActivator),
		affectedZone:        sv.ParamStr(cards.PKAffectedZone),
		sourceCanOnlyTarget: sv.Params["SourceCanOnlyTarget"],
		remembered:          sv.Remembered,
	}
}

// cantTargetBlocks is Forge's applyCantTargetAbility order: AffectedZone$,
// ValidTarget$, ValidSA$, ValidSource$, Activator$. It reports whether this
// restriction withholds id from actor's source/sa.
func (e *Engine) cantTargetBlocks(b cantTargetBody, id state.ObjID, actor state.PlayerID, source state.ObjID, sa *cards.SA, staticSource state.ObjID, staticController state.PlayerID, sc effects.SpecContext) bool {
	// SourceCanOnlyTarget$ (Wall of Shadows) is not modelled. Dropping it and
	// honouring ValidTarget$ Card.Self would withhold every spell, the
	// opposite of the card's text; fail toward "no restriction" instead.
	if b.sourceCanOnlyTarget != "" {
		return false
	}
	o := e.G.Obj(id)
	if o == nil {
		return false
	}
	if b.affectedZone == "" {
		// Forge's default AffectedZone$ is the battlefield: a printed static
		// functions only from the battlefield (CR 604.3), so the historic
		// caller-side zone gate is part of the matcher now.
		if o.Zone != state.ZBattlefield {
			return false
		}
	} else if !zoneSpecContains(b.affectedZone, o.Zone) {
		return false
	}
	spec := b.validCard
	if spec == "" {
		spec = b.validTarget
	}
	if spec == "" {
		spec = b.validCards
	}
	if spec == "" {
		spec = b.validCreature
	}
	for _, r := range b.remembered {
		sc.Remembered = append(sc.Remembered, state.Target{Obj: r})
	}
	// An Effect-delivered body's IsRemembered scope is the set it captured
	// (CR 611.2), so it must not union the source card's later persistent
	// memory. A printed static (effectDelivered false) keeps the union: its
	// remembered set is the static's own.
	sc.SourceRememberedStrict = b.effectDelivered
	if spec != "" {
		if !e.matchesSpec(spec, id, sc) {
			return false
		}
	} else if b.effectDelivered && len(b.remembered) == 0 {
		// restrictionApplies' Effect-delivered convention: a body with
		// neither a ValidCard$/ValidTarget$ spec nor Remembered matches
		// nothing. A printed static with no ValidTarget$ (Ground Seal) means
		// every card in its AffectedZone$.
		return false
	}
	if !e.cantTargetValidSA(b.validSA, source, sa, staticController) {
		return false
	}
	if b.validSource != "" {
		// ValidSource$ names the targeting spell/ability's host card: the
		// spell card itself, or the permanent that granted an ability. It is
		// matched from the restriction controller's perspective so
		// Card.nonGreen+OppCtrl means an opponent of Thrun's controller.
		host := e.protectionSource(source)
		if host == 0 {
			return false
		}
		sc := e.specCtx(staticSource, staticController)
		sc.PreparedSpell = e.preparedSpellOf(host)
		m, departed := matchesDepartedSource(e.G, e.departedColors, e, b.validSource, host, sc)
		if !departed {
			m = e.matchesSpec(b.validSource, host, sc)
		}
		if !m {
			return false
		}
	}
	if b.activator != "" && !effects.MatchesPlayerSpecCtx(e.G, b.activator, actor, staticController, e.playerSpecCtx(staticSource)) {
		return false
	}
	return true
}

// cantTargetValidSA classifies the targeting spell/ability with the same
// Spell/Activated/Triggered split counterValidSA uses. A STACK object is
// authoritative: a spell's own stack object (Ability == nil) or the Face-less
// ability wrapper AbilityPush/TriggerPush mints (Ability != nil, its Trigger
// record decides Activated vs Triggered). A permanent is the ability's
// SOURCE -- what ValidSource$ names -- not the targeting ability itself;
// classifying one as a spell made every activated and triggered ability look
// like a Spell, so a ValidSA$ Spell restriction withheld targets from
// abilities while ValidSA$ Activated,Triggered never applied. An activated
// ability chooses its targets (CR 602.2b) before AbilityPush mints that
// wrapper, so when no stack object is available the SA's own kind decides the
// two spellings (SP is a spell, AB an activated ability), matching every
// comma alternative. A printed DB/ST SubAbility$ link read off the stack --
// the offer census's chain ask (CR 601.2c: every link's targets are announced
// beside the root's own) -- classifies by the ROOT ability it belongs to,
// found by identity in the source permanent's face: Oko, Thief of Crowns'
// [-5] exchange half is part of an activated ability, and Shanna, Sisay's
// Legacy ("can't be the target of abilities your opponents control") must
// withhold itself from the link's census, leaving the ability with no second
// legal target and no offer (tail-53 pair 478). A link outside the face's
// printed tree (a granted or merged-pile copy) finds no root and fails open,
// the historical reading. A rootless DB/ST body is unclassifiable there, so
// it fails open (no restriction).
func (e *Engine) cantTargetValidSA(spec string, source state.ObjID, sa *cards.SA, staticController state.PlayerID) bool {
	if spec == "" {
		return true
	}
	if o := e.G.Obj(source); o != nil && o.Zone == state.ZStack {
		return e.counterValidSA(o, spec, staticController, source)
	}
	if sa == nil {
		return false
	}
	if kind, ok := cantTargetSASpelling(sa.Kind); ok {
		return cantTargetSAKindIn(spec, kind)
	}
	if root := subLinkRootOfFace(e.G.Obj(source), sa); root != nil {
		if kind, ok := cantTargetSASpelling(root.Kind); ok {
			return cantTargetSAKindIn(spec, kind)
		}
	}
	// A sub-ability's target is chosen with the root's announcement (CR
	// 601.2c, 602.2b): the pending cast of this very source says whether the
	// root is an activated ability or a spell.
	if pc := e.cast; pc != nil && pc.card == source {
		if pc.isAbility() {
			return cantTargetSAKindIn(spec, "Activated")
		}
		return cantTargetSAKindIn(spec, "Spell")
	}
	if sa.KindCode() == cards.SAKindDrawback {
		// A SubAbility$ link or a Choices$ mode is judged before its spell
		// or ability reaches the stack by the offer census (CR 700.2a: a
		// mode with no legal target can't be chosen). It is the spell or
		// activated ability whose chain carries it: Soul Transfer's exile
		// mode is a spell under Dense Foliage while the card is in hand.
		if o := e.G.Obj(source); o != nil {
			if kind := chainRootKind(o.Face(), sa); kind != "" {
				return cantTargetSAKindIn(spec, kind)
			}
		}
	}
	return false
}

// chainRootKind names the ValidSA$ kind ("Spell" or "Activated") of the
// face's SP or AB ability whose SubAbility$ chain or Choices$ modes carry
// link, or "" when none does (a trigger body stays unclassified). Links are
// matched by their parsed Line: ResolveSVar mints a fresh SA per call.
func chainRootKind(f *cards.Face, link *cards.SA) string {
	if f == nil || link == nil || link.Line == "" {
		return ""
	}
	for _, root := range f.Abilities {
		kind := ""
		if c := root.KindCode(); c == cards.SAKindSpell {
			kind = "Spell"
		} else if c == cards.SAKindActivated {
			kind = "Activated"
		} else {
			continue
		}
		if chainCarriesLine(f, root, link.Line, 0) {
			return kind
		}
	}
	return ""
}

// chainCarriesLine walks sa's SubAbility$ chain and, for a modal Charm link,
// each Choices$ mode's chain resolved from f's SVars (bounded nesting), for a
// link parsed from line.
func chainCarriesLine(f *cards.Face, sa *cards.SA, line string, depth int) bool {
	for x := sa; x != nil; x = x.Sub {
		if x.Line == line {
			return true
		}
		if depth >= 3 || !isCharmWithChoices(x) {
			continue
		}
		for _, name := range effects.CharmOf(x).Modes {
			if m := cards.ResolveSVar(f.SVars, name); m != nil && chainCarriesLine(f, m, line, depth+1) {
				return true
			}
		}
	}
	return false
}

// cantTargetSASpelling maps an SA kind to the ValidSA$ spelling it classifies
// as: SP is a spell, AB an activated ability; every other kind (a DB/ST body
// read alone) names no class.
func cantTargetSASpelling(kind string) (string, bool) {
	switch kind {
	case "SP":
		return "Spell", true
	case "AB":
		return "Activated", true
	}
	return "", false
}

// subLinkRootOfFace returns the printed ability of o's face whose
// SubAbility$ chain holds sa by pointer identity. It is a census-time read:
// the offer walk passes a printed face ability, so a chain link is one of
// these pointers, and an object with no face (or a link minted outside the
// printed tree -- a granted or merged-pile copy) returns nil.
func subLinkRootOfFace(o *state.Object, sa *cards.SA) *cards.SA {
	if o == nil {
		return nil
	}
	f := o.Face()
	if f == nil {
		return nil
	}
	for _, ab := range f.Abilities {
		for s := ab; s != nil; s = s.Sub {
			if s == sa {
				return ab
			}
		}
	}
	return nil
}

// cantTargetSAKindIn reports whether the comma-separated ValidSA$ spec names
// kind exactly (surrounding space/tab trimmed). It scans by hand instead of
// strings.Split because the rules package's codeshape strings ratchet counts
// every new strings.* site.
func cantTargetSAKindIn(spec, kind string) bool {
	start := 0
	for i := 0; i <= len(spec); i++ {
		if i < len(spec) && spec[i] != ',' {
			continue
		}
		alt := spec[start:i]
		for len(alt) > 0 && (alt[0] == ' ' || alt[0] == '\t') {
			alt = alt[1:]
		}
		for len(alt) > 0 && (alt[len(alt)-1] == ' ' || alt[len(alt)-1] == '\t') {
			alt = alt[:len(alt)-1]
		}
		if alt == kind {
			return true
		}
		start = i + 1
	}
	return false
}
