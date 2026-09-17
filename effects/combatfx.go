package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("Tap", effTap)
	Register("TapAll", effTapAll)
	Register("UntapAll", effUntapAll)
	Register("Pump", effPump)
	Register("PumpAll", effPumpAll)
	Register("Animate", effAnimate)
	Register("Protection", effProtection)
}

// effTapAll is Forge's TapAllEffect (78 raw corpus lines, 75 files): the
// battlefield walk is the DEFINED players' when the script names one (or
// targets), every living seat's otherwise; ValidCards$ filters it (default
// "Permanent"). RememberTapped$ is Forge's clear-then-add contract, exactly
// like SacrificeAll's RememberSacrificed$: the resolution's Remembered set
// is REPLACED by the cards this primitive tapped (Forge clears the host
// card's list before computing the victim list, then adds one entry per
// card in it -- tapped or not, every listed card is remembered).
// TapperController$ hands the tap provenance to each card's own controller
// (Forge's per-card tapper), the resolving controller otherwise.
func effTapAll(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	spec := sa.Params["ValidCards"]
	if spec == "" {
		spec = "Permanent"
	}
	remember := strings.EqualFold(sa.Params["RememberTapped"], "True")
	if remember {
		c.Remembered = nil
		clearEventRemembered(h, c)
	}
	tapper := c.Controller
	perCardTapper := strings.TrimSpace(sa.Params["TapperController"]) != ""
	players := allPlayersFor(h, c, sa)
	for _, p := range players {
		ids := append([]state.ObjID(nil), g.Zone(state.ZBattlefield, p)...)
		for _, id := range ids {
			if !MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				continue
			}
			if remember {
				c.Remembered = append(c.Remembered, state.Target{Obj: id})
				eventRemember(h, c, id)
			}
			o := g.Obj(id)
			if o == nil || o.Zone != state.ZBattlefield || o.Tapped {
				continue
			}
			tap := tapper
			if perCardTapper {
				tap = o.Controller
			}
			h.EmitTap(id, tap, false)
		}
	}
}

// effUntapAll is Forge's UntapAllEffect (126 raw corpus lines, 125 files):
// the same battlefield walk as effTapAll, filtered by ValidCards$ (Forge's
// default is no filter at all -- the whole battlefield -- so "Permanent" is
// the equivalent default here). RememberUntapped$ remembers ONLY the cards
// that actually untapped (Forge adds inside the untapped branch), and the
// resolution's Remembered set is extended, not replaced (UntapAll has no
// clear-remembered step). ControllerUntaps$ hands the per-card controller
// the untap provenance, the resolving controller otherwise.
func effUntapAll(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	spec := sa.Params["ValidCards"]
	if spec == "" {
		spec = "Permanent"
	}
	remember := strings.EqualFold(sa.Params["RememberUntapped"], "True")
	untapper := c.Controller
	perCardUntapper := strings.TrimSpace(sa.Params["ControllerUntaps"]) != ""
	players := allPlayersFor(h, c, sa)
	for _, p := range players {
		ids := append([]state.ObjID(nil), g.Zone(state.ZBattlefield, p)...)
		for _, id := range ids {
			if !MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				continue
			}
			o := g.Obj(id)
			if o == nil || o.Zone != state.ZBattlefield || !o.Tapped {
				continue
			}
			untap := untapper
			if perCardUntapper {
				untap = o.Controller
			}
			h.Emit(events.Event{Kind: events.Untap, Obj: id, Player: untap})
			if remember {
				c.Remembered = append(c.Remembered, state.Target{Obj: id})
				eventRemember(h, c, id)
			}
		}
	}
}

// allPlayersFor scopes an All primitive to its Defined$ players or (when it
// has targets but no explicit Defined$) its chosen player targets.  Forge's
// TargetRestrictions supplies ValidTgts$ as the latter form (Mana Short and
// Early Harvest); falling back to every battlefield is only correct when the
// SA has neither selector.
func allPlayersFor(h Host, c *Ctx, sa *cards.SA) []state.PlayerID {
	g := h.Game()
	if strings.TrimSpace(sa.Params["Defined"]) == "" {
		if _, targeted := sa.Params["ValidTgts"]; !targeted {
			return g.AliveFrom(0)
		}
	}
	seen := map[state.PlayerID]bool{}
	var out []state.PlayerID
	for _, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		if int(p) < len(g.Players) && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// effTap taps each Defined$ permanent. The tapper is the resolving ability's
// controller unless Tapper$ names a player (Forge TapEffect). ETB$ True is the
// "enters tapped" replacement body (804 corpus files, every ETBTapped land):
// the permanent is given its entry state and does not become tapped (CR
// 603.2e), so EmitTap marks it and no Taps trigger runs.
func effTap(h Host, c *Ctx, sa *cards.SA) {
	tapper := c.Controller
	if spec := strings.TrimSpace(sa.Params["Tapper"]); spec != "" {
		sub := *sa
		sub.Params = map[string]string{"Defined": spec}
		if ps := Defined(h, c, &sub); len(ps) > 0 {
			tapper = PlayerOf(h, c, ps[0])
		}
	}
	entering := strings.EqualFold(sa.Params["ETB"], "True")
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			continue
		}
		h.EmitTap(o.ID, tapper, entering)
	}
}

// effPump, effPumpAll, effAnimate and effProtection are all continuous
// effects: they build a state.ContinuousEffect and hand it to the Host,
// which is rules.Engine.AddContinuous, so the CR 613 layer system Task 19
// built actually has a caller (Task 19c). Every one of them is a temporary
// grant from a resolving spell or ability, never a permanent's own printed
// static, so they always set UntilEOT: true -- the layer system's existing
// EndOfTurnCleanup already drops those on schedule, and Engine.active()
// never checks the source's battlefield presence for an UntilEOT effect
// (Giant Growth outlives the instant that cast it), so nothing else needs
// to change for these to expire correctly.
//
// Each one scopes its effect to exactly the object it targets with
// Affects: "Card.Self" and Source: <that object's ID> -- not the resolving
// ability's own source -- reusing the same Self-predicate pattern Task 19's
// own lord-effect tests already established (layers_test.go), rather than
// inventing a new filter form.
func effPump(h Host, c *Ctx, sa *cards.SA) {
	att := Num(h, c, sa, "NumAtt", 0)
	def := Num(h, c, sa, "NumDef", 0)
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		registerPumpEffects(h, c, o.ID, att, def, sa)
	}
}

// effPumpAll bakes in the affected set at resolution time (CR 611.2c: such an
// effect applies only to the permanents matching the filter when the ability
// resolves, not to ones that start matching later), so it registers one
// continuous effect per matching object -- found by walking AliveFrom(0)'s
// fixed APNAP seat order and each seat's battlefield zone slice in its
// existing registration order, never a map -- rather than one shared
// filter-based effect that Derived would re-evaluate against the battlefield
// forever.
func effPumpAll(h Host, c *Ctx, sa *cards.SA) {
	att := Num(h, c, sa, "NumAtt", 0)
	def := Num(h, c, sa, "NumDef", 0)
	spec := sa.Params["ValidCards"]
	if spec == "" {
		spec = "Creature"
	}
	g := h.Game()
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				registerPumpEffects(h, c, id, att, def, sa)
			}
		}
	}
}

// durationTiming maps a Pump/PumpAll Duration$ value to its expiry: an
// explicitly indefinite shape ("Permanent", CR 611.2a) becomes a Permanent
// effect that outlives its source and every cleanup; "UntilEndOfCombat"
// (CR 511.2) becomes a combat-phase-scoped effect, dropped when the
// end-of-combat step ends; every other value ("", "UntilEndOfTurn", ...)
// keeps the historic UntilEOT default, dropped by end-of-turn cleanup.
// The one-sentence rule: a resolution-created one-shot pump honours the
// duration its script declared instead of being forced to end of turn.
func durationTiming(dur string) (permanent bool, untilEOT bool) {
	switch strings.ToLower(strings.TrimSpace(dur)) {
	case "permanent":
		return true, false
	case "untilendofcombat":
		return false, false
	default:
		return false, true
	}
}

// registerPumpEffects is Pump and PumpAll's sole per-object registration
// path. It also owns their sole KW$ read, so neither effect can reimplement
// Forge's keyword-list grammar: both always use cards.SplitKeywordList. It
// creates a layer-7c modification for a nonzero stat change and a separate
// layer-6 grant for any keywords, since Derived applies each layer
// independently. Skipping a zero/empty half avoids polluting
// Engine.continuous with an effect that would never do anything.
func registerPumpEffects(h Host, c *Ctx, id state.ObjID, att, def int32, sa *cards.SA) {
	kws := cards.SplitKeywordList(sa.Params["KW"])
	permanent, untilEOT := durationTiming(sa.Params["Duration"])
	if att != 0 || def != 0 {
		h.AddContinuous(state.ContinuousEffect{
			Source: id, Affects: "Card.Self", Controller: c.Controller,
			Layer: state.LPT, Sub: state.SubModify,
			AddPower: att, AddToughness: def,
			Duration: sa.Params["Duration"], Permanent: permanent, UntilEOT: untilEOT,
		})
	}
	if len(kws) > 0 {
		h.AddContinuous(state.ContinuousEffect{
			Source: id, Affects: "Card.Self", Controller: c.Controller,
			Layer: state.LAbilities, AddKeywords: kws,
			Duration: sa.Params["Duration"], Permanent: permanent, UntilEOT: untilEOT,
		})
	}
}

// effAnimate does not require the target to already be on the battlefield --
// Forge's own Animate targets a creature card in a graveyard as often as a
// permanent -- so unlike Pump it has no zone guard beyond "the object still
// exists". Setting base P/T is layer 7b (SubSet), which is why it uses
// SetPower/SetToughness/HasSet rather than Pump's AddPower/AddToughness: a
// later set must overwrite an earlier one regardless of timestamp order
// (TestSetBeforeModifyRegardlessOfTimestamp already locks that in), where an
// Add would incorrectly stack.
//
// The P/T effect is only registered when Power$ or Toughness$ is actually
// present: roughly two thirds of the corpus's own DB$ Animate calls (e.g.
// Kitesail Larcenist, Kami of Industry) use it purely to grant a type or
// keyword change and never mention Power$/Toughness$ at all. Num's zero
// default would otherwise turn every one of those into "becomes a 0/0",
// silently killing the very creature the card was granting an ability to.
func effAnimate(h Host, c *Ctx, sa *cards.SA) {
	_, hasPower := sa.Params["Power"]
	_, hasToughness := sa.Params["Toughness"]
	pw := Num(h, c, sa, "Power", 0)
	tf := Num(h, c, sa, "Toughness", 0)
	types := strings.Fields(strings.ReplaceAll(sa.Params["Types"], ",", " "))
	// Colors$ names the colour set the animated object carries; with
	// OverwriteColors$ True it REPLACES the object's colours (the manland
	// family -- Celestial Colonnade's "white and blue" -- where the land's
	// printed colourlessness must not survive), without it the colours are
	// ADDED. Both are layer-5 grants, normalised to WUBRG letters here so
	// "All" (every colour) and "Colorless" (an overwrite to the empty set)
	// never leak their words downstream. A value colorLetters cannot fully
	// parse (the corpus's "ChosenColor" family, which asks its controller for
	// a colour) fails closed: colorsGrant is false, the grant is NOT
	// registered and a Note says so, so the object keeps its printed colours
	// instead of the parse's empty prefix being overwritten over them. For
	// the same reason "Colorless" without OverwriteColors$ -- an add of the
	// empty set, a no-op whose corpus lines (raging_spirit) intend "becomes
	// colourless" -- is noted and skipped rather than silently registering a
	// dead effect.
	colorsRaw := strings.TrimSpace(sa.Params["Colors"])
	colors, colorsOK := colorLetters(sa.Params["Colors"])
	overwrite := colorsRaw != "" && strings.EqualFold(strings.TrimSpace(sa.Params["OverwriteColors"]), "True")
	colorsGrant := colorsRaw != "" && colorsOK && (len(colors) > 0 || overwrite)
	// Keywords$ is a "&"-separated keyword list (Celestial Colonnade's
	// "Flying & Vigilance"), the same grammar Pump's KW$ uses.
	kws := cards.SplitKeywordList(sa.Params["Keywords"])
	// RemoveCreatureTypes$ True strips the object's creature-type subtypes
	// (Mishra's Factory's land base carries none, but an animated creature or
	// planeswalker face does) before this animation's own Types$ apply.
	removeCreatureTypes := strings.EqualFold(strings.TrimSpace(sa.Params["RemoveCreatureTypes"]), "True")
	// Abilities$ names the SVar bodies (comma-separated, on THIS face's table)
	// the animated object gains -- Urza's Saga's chapters ("CARDNAME gains
	// '{T}: Add {C}'.") are the corpus's flagship shape. The grant is a
	// layer-6 ability grant (CR 613.1f): rules' grantedAbilities resolves the
	// names back through the SOURCE face's SVar table, so the name travels,
	// never a parsed copy. Duration$ Permanent makes the grant last while the
	// object is on the battlefield (the source-presence lifetime, which is
	// also what the object's own text obeys); any other Duration -- the
	// corpus's animate-a-land-for-a-turn lines -- keeps the ordinary
	// until-end-of-turn lifetime.
	var abilities []string
	for _, nm := range strings.Split(sa.Params["Abilities"], ",") {
		if nm = strings.TrimSpace(nm); nm != "" {
			abilities = append(abilities, nm)
		}
	}
	permanent := strings.EqualFold(strings.TrimSpace(sa.Params["Duration"]), "Permanent")
	if colorsRaw != "" && !colorsOK {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "Animate Colors$ " + colorsRaw + " is not implemented; colours unchanged"})
	} else if colorsRaw != "" && !colorsGrant {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "Animate Colors$ Colorless without OverwriteColors$ is not implemented; colours unchanged"})
	}
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil {
			continue
		}
		if hasPower || hasToughness {
			h.AddContinuous(state.ContinuousEffect{
				Source: o.ID, Affects: "Card.Self", Controller: c.Controller,
				Layer: state.LPT, Sub: state.SubSet,
				SetPower: pw, SetToughness: tf, HasSet: true, UntilEOT: true,
			})
		}
		if len(types) > 0 || removeCreatureTypes {
			h.AddContinuous(state.ContinuousEffect{
				Source: o.ID, Affects: "Card.Self", Controller: c.Controller,
				Layer: state.LType, AddTypes: types, RemoveCreatureTypes: removeCreatureTypes, UntilEOT: true,
			})
		}
		if colorsGrant {
			h.AddContinuous(state.ContinuousEffect{
				Source: o.ID, Affects: "Card.Self", Controller: c.Controller,
				Layer: state.LColor, AddColors: colors, OverwriteColors: overwrite,
				Duration: sa.Params["Duration"], Permanent: permanent, UntilEOT: !permanent,
			})
		}
		if len(kws) > 0 {
			h.AddContinuous(state.ContinuousEffect{
				Source: o.ID, Affects: "Card.Self", Controller: c.Controller,
				Layer: state.LAbilities, AddKeywords: kws,
				Duration: sa.Params["Duration"], Permanent: permanent, UntilEOT: !permanent,
			})
		}
		if len(abilities) > 0 {
			h.AddContinuous(state.ContinuousEffect{
				Source: o.ID, Affects: "Card.Self", Controller: c.Controller,
				Layer: state.LAbilities, AddAbilities: abilities,
				UntilEOT: !permanent,
			})
		}
	}
}

func effProtection(h Host, c *Ctx, sa *cards.SA) {
	gains := resolveGains(sa.Params["Gains"], sa.Params["Choices"])
	if gains == "" {
		return
	}
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		h.AddContinuous(state.ContinuousEffect{
			Source: o.ID, Affects: "Card.Self", Controller: c.Controller,
			Layer: state.LAbilities, AddKeywords: []string{"Protection from " + gains},
			UntilEOT: true,
		})
	}
}

// resolveGains turns Protection's Gains$ parameter into a concrete quality
// string ("red", "artifacts", ...). Gains$ Choice (Mother of Runes: "Gains$
// Choice | Choices$ AnyColor") names no chooser this build has -- a real
// player choice is Task 20's job, the same simplification effCharm and
// effVote already apply to Choices$ elsewhere in this package -- so it
// resolves deterministically instead of asking: AnyColor (the only Choices$
// value this corpus uses here) becomes white, the fixed first-of-WUBRG
// default; anything else takes the first comma-separated entry, matching
// effCharm/effVote's own "first choice" convention.
func resolveGains(gains, choices string) string {
	if !strings.EqualFold(gains, "Choice") {
		return gains
	}
	if strings.EqualFold(choices, "AnyColor") {
		return "white"
	}
	return strings.TrimSpace(strings.SplitN(choices, ",", 2)[0])
}
