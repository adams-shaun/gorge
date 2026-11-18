package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// The PlayerCount comparison family: Forge PlayerProperty spellings that
// compare a candidate player's board or life with the resolving
// controller's ("you"), read as the property of a PlayerCount<Group>$ head
// (Birth of the Imperium's HasPropertyhasFewerCreaturesInPlayThanYou, Stoic
// Farmer's HasPropertywithMoreLandsThanYou, Great Unclean One's
// HasPropertylifeLTCount$YourLifeTotal, Sudden Salvation's
// HasPropertycontrolsPermanent.IsRemembered) and as the group token of the
// bare PlayerCountProperty head (Oreskos Explorer's
// PlayerCountPropertywithAtLeast1MoreLandsThanYou$Amount). The comparison
// players are fixed at evaluation time (CR 608.2h): "you" is the count's
// resolving controller throughout, never the counted member.
//
// One parametric parser (parsePlayerThanYouCmp) is the single grammar; the
// count arms below route through it, so a property cannot resolve on one
// group and fail closed on its sibling — the class the shared
// playerCountCondition dispatch closes. Every unrecognized spelling reports
// ok=false (unresolvable, the fail-closed verdict a PlayerCount arm gives an
// unmodelled property), never a fabricated zero.

type playerThanYouKindCode uint16

const (
	// hasFewer<Type>sIn<Play|Yard>ThanYou — the restrictions.go grammar,
	// shared from hasFewerThanYouSpec/hasFewerThanYouHolds.
	playerThanYouFewerTypeIn playerThanYouKindCode = iota + 1
	// withMore<Type>sThanYou — strictly more battlefield objects of Type.
	playerThanYouMoreType
	// withAtLeast<N>More<Type>sThanYou — at least N more.
	playerThanYouAtLeastNMoreType
	// lifeLTCount$<expr> — a life total below the nested count expression's
	// value.
	playerThanYouLifeLTCount
	// controlsPermanent[.<spec>] — controls a battlefield permanent matching
	// the object spec (bare: any permanent).
	playerThanYouControlsPermanent
)

// playerThanYouCmp is one parsed comparison. The hasFewer family keeps its
// raw property (prop) — its parse lives in restrictions.go and is shared
// verbatim; the board-count forms carry the singular permanent-type word
// (typ, from the shared permanentTypeWords vocabulary — the corpus's
// measured population is Lands/Creatures only, and a word outside the
// vocabulary is unmodelled, so its head stays unresolvable and
// unregistered); lifeLTCount the nested count expression; controlsPermanent
// the full object spec.
type playerThanYouCmp struct {
	kind playerThanYouKindCode
	typ  string
	prop string
	n    int32
	rhs  string
	spec string
}

// parsePlayerThanYouCmp parses ONE comparison property (no HasProperty
// prefix — the count arms strip it). Unrecognized spellings report
// ok=false; the caller's arm then hands the head to the next dispatch arm,
// where it falls to the unresolvable verdict as before.
func parsePlayerThanYouCmp(prop string) (playerThanYouCmp, bool) {
	switch {
	case strings.HasPrefix(prop, "hasFewer"):
		if _, _, ok := hasFewerThanYouSpec(prop); !ok {
			return playerThanYouCmp{}, false
		}
		return playerThanYouCmp{kind: playerThanYouFewerTypeIn, prop: prop}, true
	case strings.HasPrefix(prop, "withAtLeast"):
		body, ok := strings.CutSuffix(prop, "ThanYou")
		if !ok {
			return playerThanYouCmp{}, false
		}
		rest, ok := strings.CutPrefix(body, "withAtLeast")
		if !ok {
			return playerThanYouCmp{}, false
		}
		i := strings.Index(rest, "More")
		if i <= 0 {
			return playerThanYouCmp{}, false
		}
		n, err := strconv.ParseInt(rest[:i], 10, 32)
		if err != nil || n < 1 {
			return playerThanYouCmp{}, false
		}
		typ, ok := playerThanYouPluralType(rest[i+len("More"):])
		if !ok {
			return playerThanYouCmp{}, false
		}
		return playerThanYouCmp{kind: playerThanYouAtLeastNMoreType, typ: typ, n: int32(n)}, true
	case strings.HasPrefix(prop, "withMore"):
		body, ok := strings.CutSuffix(prop, "ThanYou")
		if !ok {
			return playerThanYouCmp{}, false
		}
		typ, ok := playerThanYouPluralType(strings.TrimPrefix(body, "withMore"))
		if !ok {
			return playerThanYouCmp{}, false
		}
		return playerThanYouCmp{kind: playerThanYouMoreType, typ: typ}, true
	case strings.HasPrefix(prop, "lifeLTCount$"):
		rhs := strings.TrimPrefix(prop, "lifeLTCount$")
		if strings.TrimSpace(rhs) == "" {
			return playerThanYouCmp{}, false
		}
		return playerThanYouCmp{kind: playerThanYouLifeLTCount, rhs: rhs}, true
	case strings.HasPrefix(prop, "controlsPermanent"):
		spec := strings.TrimPrefix(prop, "controlsPermanent")
		objSpec := "Permanent"
		if rest, has := strings.CutPrefix(spec, "."); has {
			if strings.TrimSpace(rest) == "" {
				return playerThanYouCmp{}, false
			}
			objSpec = "Permanent." + rest
		} else if spec != "" {
			return playerThanYouCmp{}, false
		}
		// An unread object spec would match nothing and read as a fabricated
		// "controls none" — the census keeps the property unmodelled so the
		// head falls to the unresolvable verdict instead.
		if len(UnknownPredicates(objSpec)) != 0 {
			return playerThanYouCmp{}, false
		}
		return playerThanYouCmp{kind: playerThanYouControlsPermanent, spec: objSpec}, true
	}
	return playerThanYouCmp{}, false
}

// playerThanYouPluralType reads the plural type word between the comparison
// verb and ThanYou ("Lands" -> Land, "Creatures" -> Creature) and admits it
// only from the CR 205.2 permanent-type vocabulary the battlefield counts
// read. An unknown type word is unmodelled.
func playerThanYouPluralType(word string) (string, bool) {
	singular, ok := strings.CutSuffix(word, "s")
	if !ok || !permanentTypeWords[singular] {
		return "", false
	}
	return singular, true
}

// playerThanYouHolds evaluates one parsed comparison for the candidate
// member against you. Counts read after continuous effects (CR 613.1d,
// 205.1) through the type predicate the object filters use, on the
// candidate's own battlefield; the comparison players are fixed at
// evaluation time (CR 608.2h). An out-of-range seat or an unreadable
// nested count reports known=false — unresolvable, never a fake verdict.
func playerThanYouHolds(h Host, g *state.Game, c *Ctx, cmp playerThanYouCmp, member, you state.PlayerID, sc SpecContext, depth int) (holds, known bool) {
	if int(member) >= len(g.Players) || int(you) >= len(g.Players) || member < 0 || you < 0 {
		return false, false
	}
	switch cmp.kind {
	case playerThanYouFewerTypeIn:
		return hasFewerThanYouHolds(g, cmp.prop, member, you, sc)
	case playerThanYouMoreType, playerThanYouAtLeastNMoreType:
		mine := unlessDefenderTypeCount(g, state.ZBattlefield, member, cmp.typ, sc)
		yours := unlessDefenderTypeCount(g, state.ZBattlefield, you, cmp.typ, sc)
		if cmp.kind == playerThanYouMoreType {
			return mine > yours, true
		}
		return mine >= yours+cmp.n, true
	case playerThanYouLifeLTCount:
		// The threshold is the nested count expression's value in the
		// resolving context ("you" is c.Controller, the resolver — Great
		// Unclean One's YourLifeTotal). The depth carries the caller's, so a
		// crafted self-nesting rhs dies at evalCountExprOK's maxCountDepth
		// guard instead of recursing.
		v, ok := evalCountExprOK(h, c, cmp.rhs, depth+1)
		if !ok {
			return false, false
		}
		return g.Players[member].Life < v, true
	case playerThanYouControlsPermanent:
		// Sudden Salvation: "each opponent who controls one or more of those
		// permanents" — the spec's IsRemembered predicate reads the
		// resolution's remembered set (Ctx.Remembered via sc), so the member
		// counts exactly when it controls one of the permanents the
		// resolution returned.
		for _, id := range g.Zone(state.ZBattlefield, member) {
			if MatchesObjectCtx(g, cmp.spec, g.Obj(id), sc) {
				return true, true
			}
		}
		return false, true
	}
	return false, false
}

// countGroupPlayerThanYou counts the group members whose parsed comparison
// holds against the resolving controller. An empty group resolves a
// lifeLTCount threshold ONCE against the resolving context first (the same
// honesty playerCountCondition applies to a per-member SVar RHS): an empty
// group would otherwise never reach the nested read and launder an
// unreadable threshold into a legitimate-looking (0, true).
func countGroupPlayerThanYou(h Host, g *state.Game, c *Ctx, cmp playerThanYouCmp, group []state.PlayerID, depth int) (int32, bool) {
	if cmp.kind == playerThanYouLifeLTCount && len(group) == 0 {
		if _, ok := evalCountExprOK(h, c, cmp.rhs, depth+1); !ok {
			return 0, false
		}
	}
	sc := c.SpecContext(c.Controller)
	var n int32
	for _, m := range group {
		holds, known := playerThanYouHolds(h, g, c, cmp, m, c.Controller, sc, depth)
		if !known {
			return 0, false
		}
		if holds {
			n++
		}
	}
	return n, true
}

// playerCountThanYou answers the PlayerCount<group>$HasProperty<cmp>
// comparison family (the class of Stoic Farmer, Birth of the Imperium, Great
// Unclean One, Sudden Salvation): each group member whose board/life
// comparison against the resolving controller holds counts 1. The shared
// dispatch is what keeps the family from resolving on one group and failing
// closed on its sibling: every group arm that carries a property dispatch
// routes its HasProperty head here (before the playerCountExtreme
// fallthrough), so the parser above is the ONE grammar for all of them. An
// unrecognized property reports (0, false) — unresolvable, never a fake
// zero; the /Op count suffix (Birth of the Imperium's /Times.2) is peeled
// upstream by evalCountExprOK's generic cut and applied by applyCountOp
// here only when a caller reaches this helper with it still attached.
func playerCountThanYou(h Host, g *state.Game, c *Ctx, group []state.PlayerID, prop, arg string, depth int) (int32, bool) {
	if strings.TrimSpace(arg) != "" {
		return 0, false
	}
	base, op, hasOp := strings.Cut(strings.TrimSpace(prop), "/")
	base = strings.TrimSpace(base)
	cmp, ok := parsePlayerThanYouCmp(strings.TrimPrefix(base, "HasProperty"))
	if !ok {
		return 0, false
	}
	n, ok := countGroupPlayerThanYou(h, g, c, cmp, group, depth)
	if !ok {
		return 0, false
	}
	if hasOp {
		n = applyCountOp(n, op)
	}
	return n, true
}

// evalCountBodyPlayerProperty is the bare PlayerCountProperty<Group>$
// <Property> arm of evalCountBodyPlayer: the group token is ITSELF a
// comparison property (Oreskos Explorer's and Surveyor's Scope's
// PlayerCountPropertywithAtLeast<N>MoreLandsThanYou$Amount — Forge spells
// the property in the group slot), the counted group is every living player
// (the printed text reads "players", and a player cannot out-land or
// out-creature themself, so the self member never counts either way), and
// the property is Forge's Amount: 1 per qualifying member. The
// PlayerCountPropertyYou$ arm upstream claims every You-group spelling, so
// only comparison tokens reach here; anything else reports (0, false, true)
// — claimed and unresolvable, the fail-closed verdict.
func evalCountBodyPlayerProperty(h Host, c *Ctx, g *state.Game, head, arg string, depth int) (int32, bool, bool) {
	rest, ok := strings.CutPrefix(head, "PlayerCountProperty")
	if !ok {
		return 0, false, false
	}
	group, prop, hasProp := strings.Cut(rest, "$")
	// The property is Forge's Amount (1 per qualifying member) — read
	// through the cov3 per-player-property table that owns the word, never a
	// fresh string compare.
	if !hasProp || cov3PlayerPropCodes.Code(strings.TrimSpace(prop)) != cov3PlayerPropAmount || strings.TrimSpace(arg) != "" {
		return 0, false, true
	}
	cmp, ok := parsePlayerThanYouCmp(strings.TrimSpace(group))
	if !ok {
		return 0, false, true
	}
	n, ok := countGroupPlayerThanYou(h, g, c, cmp, g.AliveFrom(0), depth)
	return n, ok, true
}
