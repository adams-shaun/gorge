package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
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
	g := h.Game()
	// Defined$ ValidStack <spec>: every stack object matching the spec. It is
	// a prefix, not a whole-value case, because the spec rides in the same
	// parameter after one space ("ValidStack Spell.OppCtrl,...").
	if spec, ok := strings.CutPrefix(sa.Params["Defined"], "ValidStack"); ok {
		return validStackTargets(g, strings.TrimSpace(spec), c)
	}
	switch sa.Params["Defined"] {
	case "":
		// Forge's rule: an ability that names targets acts on them; one that
		// names none acts on its source. A sub-ability that wants its
		// parent's targets says so explicitly (Defined$ Targeted /
		// ParentTarget), which every script in the corpus does.
		if _, targeted := sa.Params["ValidTgts"]; targeted {
			return copyTargets(c.Targets)
		}
		return []state.Target{{Obj: c.Source}}
	case "You":
		return []state.Target{{Player: c.Controller, IsPlayer: true}}
	case "Self", "Parent":
		return []state.Target{{Obj: c.Source}}
	case "Remembered":
		return copyTargets(c.Remembered)
	case "ChosenCard", "ChosenPlayer":
		// ChooseCard/ChoosePlayer bind the current resolution's most recent
		// choice here. This is deliberately distinct from Remembered: Forge
		// only copies the answer there when RememberChosen$ is set. A later,
		// independently resolving ability reads the same event-backed choice
		// from its source permanent.
		if c.ChosenValid || len(c.Chosen) > 0 {
			return copyTargets(c.Chosen)
		}
		if o := g.Obj(c.Source); o != nil {
			return copyTargets(o.Chosen)
		}
		return nil
	case "Player.IsRemembered":
		return playersOf(c.Remembered)
	case "Player.Chosen":
		return playersOf(c.Chosen)
	case "RememberedController":
		return controllersOf(g, c.Remembered)
	case "RememberedOwner":
		return ownersOf(g, c.Remembered)
	case "TargetedController", "TargetedPlayer":
		return controllersOf(g, c.Targets)
	case "ChosenController":
		return controllersOf(g, c.Chosen)
	case "Targeted", "ParentTarget":
		return copyTargets(c.Targets)
	case "TriggeredCard", "TriggeredCardLKICopy", "TriggeredNewCardLKICopy",
		"TriggeredSpellAbility", "TriggeredAttacker", "TriggeredSource":
		// M1 does not model LKI copies, new-object identity or the
		// ability-vs-card distinction separately: every one of these forms
		// names the same Remembered object entry a trigger captured.
		return objectsOf(c.Remembered)
	case "DelayTriggerRememberedLKI", "RememberedLKI", "TriggeredAttackerLKICopy":
		// A delayed trigger's Execute$ (Flickerwisp's TrigBounce) resolves its
		// referent through Defined$ DelayTriggerRememberedLKI: the object(s)
		// the delayed trigger captured at registration, which rules pushes
		// onto the fired ability's Remembered. DelayTriggerRememberedLKI and
		// the other LKI spellings are the same Remembered object set.
		return objectsOf(c.Remembered)
	case "Imprinted", "ImprintedController":
		// The RepeatEach iteration's current subject (Forge's UseImprinted$):
		// Heroism pumps/remembers it, Stench of Evil deals its damage to its
		// controller. Zero outside a loop iteration resolves to nothing —
		// the imprint-pile spelling (Mirrorworks' exiled-with-imprint list)
		// has no engine state yet and stays a known approximation.
		if c.RepeatSubject.IsPlayer {
			return []state.Target{{Player: c.RepeatSubject.Player, IsPlayer: true}}
		}
		if o := g.Obj(c.RepeatSubject.Obj); o != nil {
			return []state.Target{{Obj: c.RepeatSubject.Obj}}
		}
		return nil
	case "ReplacedCard":
		// The card a replacement is acting on (Rest in Peace shape: the R: line
		// intercepts a "would go to the graveyard" Move, ReplaceWith$ needs to
		// name the object the replaced event was about). "Replaced" is set only
		// on a replacement's own context, so outside a replacement -- and for a
		// replaced object that has since ceased to exist -- Defined falls back to
		// nil (nothing to act on) rather than the chosen targets.
		if c.Replaced != 0 && g.Obj(c.Replaced) != nil {
			return []state.Target{{Obj: c.Replaced}}
		}
		return nil
	case "TriggeredDefendingPlayer":
		if out := oneTriggerPlayer(c.DefendingPlayer); out != nil {
			return out
		}
		return playersOf(c.Remembered)
	case "TriggeredPlayer":
		if out := oneTriggerPlayer(c.TriggerPlayer); out != nil {
			return out
		}
		return playersOf(c.Remembered)
	case "TriggeredAttackingPlayer":
		return oneTriggerPlayer(c.AttackingPlayer)
	case "TriggeredAttackedTarget":
		return oneTriggerPlayer(c.AttackedTarget)
	case "TriggeredActivator":
		return oneTriggerPlayer(c.TriggerActivator)
	case "TriggeredCardController":
		if p, ok := TriggeredCardController(g, c.TriggerContext, c.Remembered); ok {
			return []state.Target{{Player: p, IsPlayer: true}}
		}
		return nil
	case "Equipped", "Enchanted", "AttachedTo":
		// The corpus spells this three ways depending on whether the source
		// is Equipment, an Aura, or a generic script; all three name the
		// same field (Task 14 wires its producer).
		if o := g.Obj(c.Source); o != nil && o.AttachedTo != 0 && g.Obj(o.AttachedTo) != nil {
			return []state.Target{{Obj: o.AttachedTo}}
		}
		return nil
	case "Opponent", "Player.Opponent", "Player.Other":
		var out []state.Target
		for _, p := range g.AliveFrom(c.Controller) {
			if p != c.Controller {
				out = append(out, state.Target{Player: p, IsPlayer: true})
			}
		}
		return out
	case "Player":
		var out []state.Target
		for _, p := range g.AliveFrom(c.Controller) {
			out = append(out, state.Target{Player: p, IsPlayer: true})
		}
		return out
	}
	// Forge joins independent Defined selectors with " & " to name all of
	// them (Karazikar's "TriggeredAttackingPlayer & You" is the corpus
	// example), not their set intersection. Resolve each known selector in
	// script order so player effects act on both players deterministically.
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
	// Any Defined$ form M1 does not model falls back to the chosen targets
	// rather than silently acting on nothing.
	return copyTargets(c.Targets)
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
func copyTargets(s []state.Target) []state.Target {
	return append([]state.Target(nil), s...)
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
