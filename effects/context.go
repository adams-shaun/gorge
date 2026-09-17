package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Defined resolves a Defined$ parameter to concrete targets. With no Defined$
// at all, Forge's own rule applies: an ability that declares ValidTgts$ (it
// has real targets to name) acts on the chosen ones; an ability with no
// ValidTgts$ acts on its own source (R-10's default).
//
// Every return here is a defensive copy, never a slice sharing a backing
// array with Ctx.Targets or Ctx.Remembered: Ctx is threaded by pointer through
// Resolve, so a caller that filters the returned slice in place (the ordinary
// out := s[:0]; for range append(out, ...) idiom) must not be able to corrupt
// state a later effect in the same Sub chain still relies on.
func Defined(h Host, c *Ctx, sa *cards.SA) []state.Target {
	if ts, ok := knownDefinedTargets(h, c, sa.Params["Defined"]); ok {
		return ts
	}
	// Keep Defined's historical per-member fallback for a mixed known/unknown
	// expression. knownDefinedTargets is deliberately stricter for callers
	// that need a fail-closed fetch-list classification, not a new public
	// contract for ordinary effects.
	if sa.Params["Defined"] == "Imprinted" || sa.Params["Defined"] == "ImprintedLKI" {
		// Ordinary (non-fetch-list) Imprinted resolution: the source's
		// Imprinted association. knownDefinedTargets deliberately does NOT
		// recognise this selector (TestImprintedDefinedLibraryFetchFailsClosed
		// pins a hidden-library Imprinted fetch as a fail-closed no-op, since
		// Imprinted has no persisted library-position context), so this stays
		// scoped to Defined's own broader fallback contract.
		g := h.Game()
		if o := g.Obj(c.Source); o != nil {
			out := make([]state.Target, 0, len(o.Imprinted))
			for _, id := range o.Imprinted {
				if g.Obj(id) != nil {
					out = append(out, state.Target{Obj: id})
				}
			}
			return out
		}
		return nil
	}
	if strings.Contains(sa.Params["Defined"], " & ") {
		var out []state.Target
		for _, part := range strings.Split(sa.Params["Defined"], " & ") {
			copy := *sa
			copy.Params = make(map[string]string, len(sa.Params))
			for k, v := range sa.Params {
				copy.Params[k] = v
			}
			copy.Params["Defined"] = strings.TrimSpace(part)
			out = append(out, Defined(h, c, &copy)...)
		}
		return out
	}
	// Forge's rule: an ability that names targets acts on them; one that
	// names none acts on its source. A sub-ability that wants its
	// parent's targets says so explicitly (Defined$ Targeted /
	// ParentTarget), which every script in the corpus does.
	if _, targeted := sa.Params["ValidTgts"]; targeted {
		return copyTargets(c.Targets)
	}
	return []state.Target{{Obj: c.Source}}
}

// knownDefinedTargets resolves a Defined$ form only when every selector in it
// is modelled. Unlike Defined, it never falls back to the source or chosen
// targets: callers such as a hidden-library ChangeZone need to distinguish an
// actual object fetch list from an unrecognised selector. Forge's " & " joins
// independent selectors, not their intersection, so known members are joined
// in script order. One unknown member makes the whole expression unknown --
// the fail-closed direction.
func knownDefinedTargets(h Host, c *Ctx, spec string) ([]state.Target, bool) {
	g := h.Game()
	// Defined$ ValidStack <spec>: every stack object matching the spec. It is
	// a prefix, not a whole-value case, because the spec rides in the same
	// parameter after one space ("ValidStack Spell.OppCtrl,...").
	if stackSpec, ok := strings.CutPrefix(spec, "ValidStack"); ok {
		return validStackTargets(g, strings.TrimSpace(stackSpec), c), true
	}
	// Defined$ Remembered.<spec>: the subset of the resolution's Remembered
	// objects matching <spec>, evaluated as a Card filter (Regrowth-shaped
	// "each card exiled this way" follow-ups that narrow Remembered by type
	// or predicate rather than acting on the whole set).
	if filterSpec, ok := strings.CutPrefix(spec, "Remembered."); ok {
		var out []state.Target
		for _, t := range c.Remembered {
			if !t.IsPlayer {
				if o := g.Obj(t.Obj); o != nil && MatchesObjectCtx(g, "Card."+filterSpec, o, c.SpecContext(c.Controller)) {
					out = append(out, t)
				}
			}
		}
		return out, true
	}
	if ts, ok := definedSpec(h, c, spec); ok {
		return ts, true
	}
	if !strings.Contains(spec, " & ") {
		return nil, false
	}
	var out []state.Target
	for _, part := range strings.Split(spec, " & ") {
		ts, ok := knownDefinedTargets(h, c, strings.TrimSpace(part))
		if !ok {
			return nil, false
		}
		out = append(out, ts...)
	}
	return out, true
}

// definedSpec resolves one RECOGNISED Defined$ value. The bool distinguishes
// "this spec names an object reference this build models" from "unknown
// spec": Defined's public contract keeps the chosen-targets fallback for
// anything unmodelled, but a caller that must not guess (damage.go's
// DamageSource$ resolution, damage.go's ValidPlayers$ resolution) reads the
// bool and fails closed to its own conservative default instead of silently
// redirecting at the chosen targets.
func definedSpec(h Host, c *Ctx, spec string) ([]state.Target, bool) {
	g := h.Game()
	switch spec {
	case "":
		return nil, false
	case "Self", "Parent", "EffectSource", "OriginalHost":
		// EffectSource/OriginalHost name the ability's own source object --
		// the permanent that pushed the resolving ability, or the card that
		// originally generated it before any copies. newDamageRider unwraps
		// an ability stack object to that source afterwards, so handing
		// back the raw c.Source here is the same object every other
		// source-defaulting path yields.
		return []state.Target{{Obj: c.Source}}, true
	case "You":
		return []state.Target{{Player: c.Controller, IsPlayer: true}}, true
	case "TopOfLibrary", "BottomOfLibrary":
		// Library order is top-first. These selectors name one known card, not
		// a player whose whole library should be searched; hidden-origin
		// ChangeZone therefore consumes the returned identity as its fetch list.
		lib := g.Zone(state.ZLibrary, c.Controller)
		if len(lib) == 0 {
			return nil, true
		}
		i := 0
		if spec == "BottomOfLibrary" {
			i = len(lib) - 1
		}
		return []state.Target{{Obj: lib[i]}}, true
	case "Remembered":
		return copyTargets(c.Remembered), true
	case "ChosenCard", "ChosenPlayer":
		// ChooseCard/ChoosePlayer bind the current resolution's most recent
		// choice here. This is deliberately distinct from Remembered: Forge
		// only copies the answer there when RememberChosen$ is set. A later,
		// independently resolving ability reads the same event-backed choice
		// from its source permanent.
		if c.ChosenValid || len(c.Chosen) > 0 {
			return copyTargets(c.Chosen), true
		}
		if o := g.Obj(c.Source); o != nil {
			return copyTargets(o.Chosen), true
		}
		return nil, true
	case "Player.IsRemembered":
		return playersOf(c.Remembered), true
	case "Player.Chosen":
		return playersOf(c.Chosen), true
	case "RememberedController":
		return controllersOf(g, c.Remembered), true
	case "RememberedOwner":
		return ownersOf(g, c.Remembered), true
	case "TargetedController", "TargetedPlayer":
		return controllersOf(g, c.Targets), true
	case "ChosenController":
		return controllersOf(g, c.Chosen), true
	case "Targeted", "ParentTarget", "ParentTargeted", "ThisTargetedCard":
		return copyTargets(c.Targets), true
	case "TriggeredCard", "TriggeredCardLKICopy", "TriggeredNewCardLKICopy",
		"TriggeredSpellAbility", "TriggeredAttacker", "TriggeredAttackerLKICopy",
		"TriggeredTargetLKICopy", "DelayTriggerRemembered",
		"DelayTriggerRememberedLKI", "RememberedLKI":
		// M1 does not model LKI copies, new-object identity or the
		// ability-vs-card distinction separately: every one of these forms
		// names the same Remembered object entry a trigger captured.
		return objectsOf(c.Remembered), true
	case "TriggeredTarget":
		// The object or player that received the triggering event. Spiteful
		// Shadows uses this as a DamageSource$: the enchanted creature, not the
		// Aura whose trigger is resolving, deals the reflected damage. Preserve
		// the target's kind here; callers that require an object (the damage
		// rider) already reject player entries rather than guessing. When the
		// causing event's mode did not capture a TriggerTarget (a hand-built
		// context or an Attached-mode trigger the referent walk does not
		// model), fall back to the chosen targets -- Defined's pre-branch
		// convention for a trigger selector whose provenance was not recorded.
		if c.TriggerTarget.Obj != 0 || c.TriggerTarget.IsPlayer {
			return []state.Target{c.TriggerTarget}, true
		}
		return copyTargets(c.Targets), true
	case "TriggeredSource":
		// The damage source the causing event recorded (pg2's
		// TriggerContext.TriggerSource): a DamageDone execute's "that source
		// deals ..." reading. Prefer the event role when the firing trigger
		// captured one -- for a DamageDone trigger Remembered holds the
		// DAMAGED object, so the old objectsOf fallback names the recipient,
		// not the dealer. No corpus card uses Defined$ TriggeredSource (the
		// 6 DamageSource$ TriggeredSource lines are the only users), and the
		// fallback keeps a non-trigger context behaving exactly as before.
		if c.TriggerSource != 0 {
			return []state.Target{{Obj: c.TriggerSource}}, true
		}
		return objectsOf(c.Remembered), true
	case "TriggeredSourceController", "TriggeredTargetController":
		// The controller of the source/target the causing event recorded:
		// Flameblade Angel's and Harsh Justice's "deals 1 damage to that
		// source's controller", Greatbow Doyen's "to that creature's
		// controller". The role is preferred when the trigger captured one
		// (a DamageDone trigger's Remembered is the DAMAGED object, whose
		// controller is exactly wrong for the source form); the fallback --
		// Remembered[0]'s controller -- is deciderFromSpec's convention for
		// the same two spellings on OptionalDecider$ lines, so both reads of
		// one spelling agree wherever the role is absent.
		ref := c.TriggerSource
		if spec == "TriggeredTargetController" {
			if c.TriggerTarget.Obj != 0 || c.TriggerTarget.IsPlayer {
				if c.TriggerTarget.IsPlayer {
					return []state.Target{{Player: c.TriggerTarget.Player, IsPlayer: true}}, true
				}
				ref = c.TriggerTarget.Obj
			} else if len(c.Remembered) > 0 {
				ref = c.Remembered[0].Obj
			}
		} else if ref == 0 && len(c.Remembered) > 0 {
			ref = c.Remembered[0].Obj
		}
		if o := g.Obj(ref); o != nil {
			return []state.Target{{Player: o.Controller, IsPlayer: true}}, true
		}
		return nil, true
	case "ReplacedCard":
		// The card a zone-change replacement is acting on. Outside such a
		// replacement (or after the object ceased to exist), resolve nothing.
		if c.Replaced != 0 && g.Obj(c.Replaced) != nil {
			return []state.Target{{Obj: c.Replaced}}, true
		}
		return nil, true
	case "ReplacedTarget":
		// Damage replacements may affect either an object or a player. Preserve
		// that distinction rather than deriving a player through object zero.
		if c.ReplacementTarget.IsPlayer {
			if int(c.ReplacementTarget.Player) < len(g.Players) {
				return []state.Target{c.ReplacementTarget}, true
			}
			return nil, true
		}
		if c.ReplacementTarget.Obj != 0 && g.Obj(c.ReplacementTarget.Obj) != nil {
			return []state.Target{c.ReplacementTarget}, true
		}
		return nil, true
	case "ReplacedSource":
		if c.ReplacementSource != 0 && g.Obj(c.ReplacementSource) != nil {
			return []state.Target{{Obj: c.ReplacementSource}}, true
		}
		return nil, true
	case "ReplacedSourceController":
		if o := g.Obj(c.ReplacementSource); o != nil && int(o.Controller) < len(g.Players) {
			return []state.Target{{Player: o.Controller, IsPlayer: true}}, true
		}
		return nil, true
	case "ReplacedTargetController":
		if c.ReplacementTarget.IsPlayer {
			return []state.Target{c.ReplacementTarget}, true
		}
		if o := g.Obj(c.ReplacementTarget.Obj); o != nil && int(o.Controller) < len(g.Players) {
			return []state.Target{{Player: o.Controller, IsPlayer: true}}, true
		}
		return nil, true
	case "TriggeredDefendingPlayer":
		if out := oneTriggerPlayer(c.DefendingPlayer); out != nil {
			return out, true
		}
		return playersOf(c.Remembered), true
	case "TriggeredPlayer":
		if out := oneTriggerPlayer(c.TriggerPlayer); out != nil {
			return out, true
		}
		return playersOf(c.Remembered), true
	case "TriggeredAttackingPlayer":
		if out := oneTriggerPlayer(c.AttackingPlayer); out != nil {
			return out, true
		}
		return nil, true
	case "TriggeredAttackedTarget":
		if out := oneTriggerPlayer(c.AttackedTarget); out != nil {
			return out, true
		}
		return nil, true
	case "TriggeredActivator":
		if out := oneTriggerPlayer(c.TriggerActivator); out != nil {
			return out, true
		}
		return nil, true
	case "TriggeredCardController":
		if p, ok := TriggeredCardController(g, c.TriggerContext, c.Remembered); ok {
			return []state.Target{{Player: p, IsPlayer: true}}, true
		}
		return nil, true
	case "Equipped", "Enchanted", "AttachedTo":
		// The corpus spells this three ways depending on whether the source
		// is Equipment, an Aura, or a generic script; all three name the
		// same field (Task 14 wires its producer).
		if o := g.Obj(c.Source); o != nil && o.AttachedTo != 0 && g.Obj(o.AttachedTo) != nil {
			return []state.Target{{Obj: o.AttachedTo}}, true
		}
		return nil, true
	case "Opponent", "Player.Opponent", "Player.Other":
		var out []state.Target
		for _, p := range g.AliveFrom(c.Controller) {
			if p != c.Controller {
				out = append(out, state.Target{Player: p, IsPlayer: true})
			}
		}
		return out, true
	case "Player":
		var out []state.Target
		for _, p := range g.AliveFrom(c.Controller) {
			out = append(out, state.Target{Player: p, IsPlayer: true})
		}
		return out, true
	}
	// Any Defined$ form this build does not model falls back to the chosen
	// targets rather than silently acting on nothing (the caller decides via
	// the bool whether that fallback is acceptable).
	return nil, false
}

// objectsOf returns Remembered's object entries (IsPlayer false) as a fresh
// slice -- never aliasing Ctx.Remembered, for the reason copyTargets' own
// doc comment gives.
func objectsOf(ts []state.Target) []state.Target {
	var out []state.Target
	for _, t := range ts {
		if !t.IsPlayer {
			out = append(out, t)
		}
	}
	return out
}

// playersOf returns Remembered's player entries (IsPlayer true) as a fresh
// slice.
func oneTriggerPlayer(t state.Target) []state.Target {
	if !t.IsPlayer {
		return nil
	}
	return []state.Target{t}
}

func playersOf(ts []state.Target) []state.Target {
	var out []state.Target
	for _, t := range ts {
		if t.IsPlayer {
			out = append(out, t)
		}
	}
	return out
}

func controllersOf(g *state.Game, ts []state.Target) []state.Target {
	return relatedPlayers(g, ts, false)
}

func ownersOf(g *state.Game, ts []state.Target) []state.Target {
	return relatedPlayers(g, ts, true)
}

func relatedPlayers(g *state.Game, ts []state.Target, owner bool) []state.Target {
	seen := map[state.PlayerID]bool{}
	var out []state.Target
	for _, t := range ts {
		p := t.Player
		if !t.IsPlayer {
			o := g.Obj(t.Obj)
			if o == nil {
				continue
			}
			p = o.Controller
			if owner {
				p = o.Owner
			}
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, state.Target{Player: p, IsPlayer: true})
		}
	}
	return out
}

// copyTargets returns a defensive copy of s: same elements, independent
// backing array. A nil s yields nil, not an empty-but-non-nil slice, so
// Defined's observable results are unchanged for every input — only the
// aliasing is fixed.
// eventRemember records one remembered card on the resolution's source with
// the event-backed Choose entry Forge's host.addRemembered writes. The
// ctx-level list a chained SubAbility reads is the caller's job; this is the
// persistent half -- the source object's event-backed Remembered list, which
// survives the resolution and is what Card.IsRemembered and
// Count$RememberedSize read later (Forge's host card remembered list).
func eventRemember(h Host, c *Ctx, id state.ObjID) {
	if c.Source == 0 {
		return
	}
	h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "remembered", IDs: []state.ObjID{id}})
}

// clearEventRemembered mirrors Forge host.clearRemembered.  Rider primitives
// call it before replacing their ctx set, so Count$RememberedSize and a later
// resolution observe exactly the same persistent set as the current chain.
// imprint records Forge's host.addImprintedCards. TargetedSource names the
// source card of a targeted stack ability when one exists; ordinary targets
// are themselves cards. The one shared resolver is used by every API so a
// future ImprintCards$ rider cannot be accidentally skipped by its primitive.
func imprint(h Host, c *Ctx, sa *cards.SA) {
	if c.Source == 0 || strings.TrimSpace(sa.Params["ImprintCards"]) == "" {
		return
	}
	var ids []state.ObjID
	for _, t := range Defined(h, c, &cards.SA{Params: map[string]string{"Defined": sa.Params["ImprintCards"]}}) {
		if t.IsPlayer {
			continue
		}
		id := t.Obj
		if sa.Params["ImprintCards"] == "TargetedSource" {
			if o := h.Game().Obj(id); o != nil && o.Source != 0 {
				id = o.Source
			}
		}
		if h.Game().Obj(id) != nil {
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 {
		h.Emit(events.Event{Kind: events.Imprint, Obj: c.Source, IDs: ids})
	}
}

func clearEventRemembered(h Host, c *Ctx) {
	if c.Source != 0 {
		if o := h.Game().Obj(c.Source); o != nil && len(o.Remembered) > 0 {
			h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "clear-remembered"})
		}
	}
}

func copyTargets(s []state.Target) []state.Target {
	return append([]state.Target(nil), s...)
}

// moveZoneEvent preserves an exile's source provenance in MoveZone's existing
// IDs carrier. All effect primitives that move a card into exile use this one
// constructor, so ExiledWithSource is derived from the logged move rather than
// a live-only side table.
func moveZoneEvent(c *Ctx, id state.ObjID, from, to state.Zone) events.Event {
	ev := events.Event{Kind: events.MoveZone, Obj: id, From: from, To: to}
	if to == state.ZExile && exileProvenanceNeeded(c) {
		ev.IDs = []state.ObjID{c.Source}
	}
	return ev
}

// exileProvenanceNeeded avoids changing every ordinary exile event merely
// because it shares the movement primitive. A source needs the association
// only when its own compiled script later names ExiledWithSource; testing the
// immutable SVar table makes that decision stable through replay.
func exileProvenanceNeeded(c *Ctx) bool {
	if c == nil || c.Source == 0 {
		return false
	}
	for _, body := range c.SVars {
		if strings.Contains(body, "ExiledWithSource") {
			return true
		}
	}
	return false
}

// PlayerOf resolves a target to a player: an explicit player target, or the
// controller of a targeted object.
func PlayerOf(h Host, c *Ctx, t state.Target) state.PlayerID {
	if t.IsPlayer {
		return t.Player
	}
	if o := h.Game().Obj(t.Obj); o != nil {
		return o.Controller
	}
	return c.Controller
}

// validStackToken is one comma-separated token of a Defined$ ValidStack spec.
// The kind half is state.StackKindToken -- the SAME kind/controller/card-type
// grammar target legality's TargetType$ census parses (rules delegates to
// state; effects cannot import rules, which is exactly why the grammar lives
// in state) -- plus the two qualifiers only a ValidStack spec carries:
//
//   - "Other" (Reverse the Polarity's, Swift Silence's `Spell.Other`): the
//     object is not the resolving ability's own source object. Same
//     relative-to-source reading the card-spec grammar's Other predicate
//     gives ValidTgts$.
//   - "sharesNameWith <card-spec>" (Grimoire Thief's
//     `Spell.sharesNameWith ExiledWithSource`): the object's face name is
//     the name of some card matching <card-spec>, evaluated with the
//     ordinary object matcher. An inner spec this build cannot resolve
//     (ExiledWithSource needs exile provenance it does not track) matches no
//     card, so the name set is empty and the token admits nothing -- the
//     fail-closed direction, never widened.
type validStackToken struct {
	kt             state.StackKindToken
	other          bool
	sharesNameWith string
}

// validStackTokens parses a Defined$ ValidStack spec into its tokens. A
// token whose base names no stack kind is skipped (the same non-stack-token
// rule the TargetType$ census applies); a spec with no usable token at all
// degrades to Spell-only -- the narrow default, never a widened one.
func validStackTokens(spec string) []validStackToken {
	spellOnly := validStackToken{kt: state.StackKindToken{Kinds: [3]bool{state.StackKindSpell: true}}}
	var toks []validStackToken
	for _, t := range strings.Split(spec, ",") {
		kt, ok := state.StackKindTokenOf(t)
		if !ok {
			continue
		}
		tok := validStackToken{kt: kt}
		_, rest, _ := strings.Cut(strings.TrimSpace(t), ".")
		for _, q := range strings.Split(rest, ".") {
			q = strings.TrimSpace(q)
			if inner, is := strings.CutPrefix(q, "sharesNameWith"); is {
				tok.sharesNameWith = strings.TrimSpace(inner)
				continue
			}
			if q == "Other" {
				tok.other = true
			}
		}
		toks = append(toks, tok)
	}
	if len(toks) == 0 {
		return []validStackToken{spellOnly}
	}
	return toks
}

// validStackTargets resolves a Defined$ ValidStack spec to the stack objects
// it names, at resolution time, in stack order (the stack zone's arena
// order -- the same enumeration the target census uses). Kind membership and
// controller qualifiers go through state.StackKindAdmits, so this arm cannot
// drift from what target legality offers; Other and sharesNameWith are the
// ValidStack-only qualifiers validStackToken carries.
func validStackTargets(g *state.Game, spec string, c *Ctx) []state.Target {
	toks := validStackTokens(spec)
	// One name set per distinct sharesNameWith inner spec, built before any
	// admission test so a card's position on the stack cannot order anything.
	// Distinct specs are collected in token order and matched cards are
	// walked in arena order -- no map range reaches a target list.
	var nameSets map[string]map[string]bool
	for _, tok := range toks {
		if tok.sharesNameWith == "" {
			continue
		}
		if nameSets == nil {
			nameSets = map[string]map[string]bool{}
		}
		if _, done := nameSets[tok.sharesNameWith]; done {
			continue
		}
		names := map[string]bool{}
		sc := c.SpecContext(c.Controller)
		for i := range g.Objs {
			o := &g.Objs[i]
			if o.Face() == nil {
				continue
			}
			if MatchesObjectCtx(g, tok.sharesNameWith, o, sc) {
				names[o.Face().Name] = true
			}
		}
		nameSets[tok.sharesNameWith] = names
	}
	var out []state.Target
	for _, oid := range g.Zone(state.ZStack, 0) {
		o := g.Obj(oid)
		if o == nil {
			continue
		}
		if !validStackAdmits(toks, state.StackKindOf(g, o), o, o.Controller, c.Controller, c.Source, nameSets) {
			continue
		}
		out = append(out, state.Target{Obj: oid})
	}
	return out
}

// validStackAdmits reports whether any token admits the stack object o.
// Token semantics are OR, the same as the TargetType$ census; each token's
// kind/controller half is state.StackKindAdmits on that one token, and the
// ValidStack-only qualifiers narrow it further.
func validStackAdmits(toks []validStackToken, k state.StackObjKind, o *state.Object,
	controller, you state.PlayerID, source state.ObjID, nameSets map[string]map[string]bool) bool {
	for _, tok := range toks {
		if !state.StackKindAdmits([]state.StackKindToken{tok.kt}, k, o, controller, you) {
			continue
		}
		if tok.other && o.ID == source {
			continue
		}
		if tok.sharesNameWith != "" {
			f := o.Face()
			if f == nil || !nameSets[tok.sharesNameWith][f.Name] {
				continue
			}
		}
		return true
	}
	return false
}
