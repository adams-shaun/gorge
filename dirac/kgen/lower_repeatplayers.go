package kgen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// repeatEachPlayers lowers api:RepeatEach over players (gorge
// effRepeatEach with RepeatPlayers$ Player / Player.Opponent / Opponent:
// repeatPlayers lists them from the resolving controller, AliveFrom), its
// RepeatSubAbility$ body once per player with the player bound as
// Remembered (cc.Remembered = base + subject). Lowered as
// OpRepeatEachPlayer (C&1: the controller first) whose body reads the
// player as RefEach: the body may name it only as Defined$ / TokenOwner$ / UnlessPayer$
// Remembered or Player.IsRemembered on a player op (repeatPlayerOps), choose no target
// and read or write no other remembered set. Any other selector or rider
// (DamageMap$, ChangeZoneTable$, RepeatOptionalForEachPlayer$, ...)
// refuses unread.
func (lc *lowerCtx) repeatEachPlayers(ps *params) error {
	var who lRef
	switch rp := ps.str("RepeatPlayers"); rp {
	case "Player":
		who = lRef{Kind: rEachPlayer}
	case "Player.Opponent", "Opponent":
		who = lRef{Kind: rEachOpp}
	case "Targeted":
		// gorge repeatPlayers' Targeted arm (choose_control.go): add the
		// resolution's targets' players, the loop then reading them in
		// AliveFrom(controller) order. The kernel reads them off the
		// ability's target slot (slotPlayers), controller first (C&1)
		var err error
		if who, err = lc.repeatEachTargetedSlot(); err != nil {
			return err
		}
	case "":
		return lc.repeatEachCards(ps) // lower_repeatcards.go
	default:
		return refuse("RepeatEach RepeatPlayers %q", rp)
	}
	if who.Kind == rEachOpp && ps.is("ChangeZoneTable", "True") {
		// gorge repeateach.go: the loop's zone changes reach Mode$
		// ChangesZoneAll once, after the loop, as one batch. Two players:
		// one opponent, one iteration, so the bracket is the iteration's
		// own (Adeline)
		ps.consume("ChangeZoneTable")
		lc.twoSeat("ChangeZoneTable over the opponents read as one iteration's batch")
	}
	name := ps.str("RepeatSubAbility")
	body := cards.ResolveSVar(lc.face.SVars, name)
	if body == nil {
		return refuse("RepeatEach RepeatSubAbility %q", name)
	}
	if ok, err := lc.repeatChooseGreatest(who, body); ok || err != nil {
		return err // lower_modecost.go
	}
	if ok, err := lc.repeatExileLand(who, ps, body); ok || err != nil {
		return err // lower_repeatexilechoice.go
	}
	if ok, err := lc.repeatBreach(who, ps, body); ok || err != nil {
		return err // lower_repeatgravechoice.go
	}
	defer func(o bool) { lc.repeatOpp = o }(lc.repeatOpp)
	lc.repeatOpp = who.Kind == rEachOpp
	for n := body; n != nil; n = n.Sub {
		var bad []string
		for k, v := range n.Params {
			v = strings.TrimSpace(v)
			if (k == "Defined" || k == "TokenOwner" || k == "UnlessPayer" || k == "DefinedPlayer") && (v == "Remembered" || v == "Player.IsRemembered") || k == "SubAbility" {
				continue
			}
			if k == "TokenAttacking" && v == tokenAttackingRemembered && who.Kind == rEachOpp {
				continue // the repeated opponent (lower_tokenattacking.go tokenAttacking)
			}
			if strings.Contains(k, "Remember") || strings.Contains(v, "Remember") ||
				strings.Contains(lc.face.SVars[v], "Remember") && !lc.eachPlSVarOK(lc.face.SVars[v]) || k == "ValidTgts" {
				bad = append(bad, k)
			}
		}
		if len(bad) > 0 {
			sort.Strings(bad)
			k := bad[0]
			return refuse("RepeatEach body %s$ %q", k, strings.TrimSpace(n.Params[k]))
		}
	}
	at, nt := len(lc.ab.Prog), len(lc.ab.Targets)
	lc.emit(lInstr{Op: "RepeatEach", Obj: who, C: 1})
	lc.repeatPl++
	err := lc.chain(body, nil)
	lc.repeatPl--
	if err != nil {
		return err
	}
	if len(lc.ab.Targets) != nt {
		return refuse("RepeatEach body with a target")
	}
	for i := at + 1; i < len(lc.ab.Prog); i++ {
		in := &lc.ab.Prog[i]
		if in.Obj.Kind == rRemembered {
			if !repeatPlayerOps[in.Op] {
				return refuse("RepeatEach body op %s on the repeated player", in.Op)
			}
			in.Obj = lRef{Kind: rEach}
		}
	}
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	return nil
}

// repeatEachTargetedSlot resolves RepeatPlayers$ Targeted's object: the
// ability's only target slot, and a player one (a TgtPlayer slot: no card
// clauses, a Players selector). gorge would repeat over every target's
// player (PlayerOf reads a card target's controller), so any other slot
// shape -- none, more than one, a card slot -- refuses by name.
func (lc *lowerCtx) repeatEachTargetedSlot() (lRef, error) {
	if len(lc.ab.Targets) != 1 {
		return lRef{}, refuse("RepeatEach Targeted: %d target slots", len(lc.ab.Targets))
	}
	t := &lc.ab.Targets[0]
	if t.Zone != 0 || len(t.F.Clauses) != 0 || t.F.Players == 0 {
		return lRef{}, refuse("RepeatEach Targeted: the target slot is not a player target")
	}
	return lRef{Kind: rTarget}, nil
}

// repeatPlayerOps are the ops whose Obj the kernel resolves as a player
// (playerRef / players: RefEach is the repeated player).
var repeatPlayerOps = map[string]bool{
	"LoseLife": true, "GainLife": true, "Draw": true, "Discard": true, "Mill": true, "Sacrifice": true,
	"CreateToken": true, "UnlessPaid": true,
	"ChooseMove": true, // a hand move's owner (DefinedPlayer$ Remembered; startSearch's playerRef)
	"Dig":        true, // the library dug (Doomsday Excruciator)
}

// repeatChooseGreatest lowers Gix's Command's fourth mode body: RepeatEach
// Player.Opponent whose body is one ChooseCard by the repeated player of
// "a creature with the greatest power among creatures they control"
// (Choices$ Creature.greatestPowerControlledByRemembered, Mandatory$,
// RememberChosen$; gorge choose_control.go), the SacrificeAll IsRemembered
// after the loop reading it. Two players: the repeated player is the
// opponent, so the greatest power is Count$Valid Creature.OppCtrl's from
// the resolving controller's view; the chooser's filter is a creature it
// controls with power >= that (kop.PredPowerGEV). ok false: not this shape.
func (lc *lowerCtx) repeatChooseGreatest(who lRef, body *cards.SA) (bool, error) {
	if body.API != "ChooseCard" || body.Sub != nil || who.Kind != rEachOpp {
		return false, nil
	}
	ps := newParams(body.Params)
	if ps.str("Choices") != "Creature.greatestPowerControlledByRemembered" || ps.str("Defined") != "Player.IsRemembered" {
		return false, nil
	}
	if !ps.is("Mandatory", "True") || !ps.is("RememberChosen", "True") {
		return true, refuse("RepeatEach greatest-power ChooseCard shape")
	}
	lc.twoSeat("the repeated opponent's greatest power read as OppCtrl's")
	ps.consume("Choices", "Defined", "Mandatory", "RememberChosen", "Reveal", "ChoiceTitle", "SpellDescription", "StackDescription")
	if left := ps.unread(); len(left) > 0 {
		return true, refuse("RepeatEach greatest-power ChooseCard: unread params %v", left)
	}
	maxV, err := lc.svarValue("Count$Valid Creature.OppCtrl$GreatestCardPower", 0)
	if err != nil {
		return true, err
	}
	f := &lFilter{Clauses: []lClause{{Types: tCreature, Ctrl: whoYou, CmpStat: cmpPower, CmpOp: opGE, CmpV: &maxV}}}
	at := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "RepeatEach", Obj: who, C: 1})
	lc.emit(lInstr{Op: "ChooseCard", Obj: lRef{Kind: rEach}, F: f, C: zBattlefield, CS: "Remember"})
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	return true, nil
}

// a RepeatEach player body reading the repeated
// player's life, hand size or permanents. gorge binds the player as
// Remembered per iteration (effRepeatEach); PlayerCountRemembered$ reads
// that player (effects/count_cov3.go cov3PlayerGroupPlayerCountRemembered:
// LifeTotal, CardsInHand, Valid <spec> routed to the Count$Valid fold) and
// RememberedPlayerCtrl means controlled by a remembered player
// (effects/filter.go). The kernel reads the player as RefEach (s.g.eachPl)
// and the controller word as kop.CtrlEachPl; both only inside the body.

// playerCountRemembered lowers PlayerCountRemembered$<rest> (any /Op tail
// already peeled). Only inside a RepeatEach player body, where Remembered
// is exactly the repeated player (the body check refuses every other
// remembered set read).
func (lc *lowerCtx) playerCountRemembered(rest string, depth int) (lValue, error) {
	if lc.repeatPl == 0 {
		return lValue{}, fmt.Errorf("PlayerCountRemembered$%s outside a RepeatEach player body", rest)
	}
	each := lRef{Kind: rEach}
	switch rest {
	case "LifeTotal":
		return lValue{Kind: vLifeOf, Ref: each}, nil
	case "CardsInHand":
		return lValue{Kind: vHandSizeOf, Ref: each}, nil
	case "LifeLostThisTurn":
		// gorge LifeLostThisTurn of the repeated player; over the opponents
		// that player is the one opponent (Gul Dukat, Colonial Prefect)
		if !lc.repeatOpp {
			return lValue{}, fmt.Errorf("PlayerCountRemembered$%s over every player", rest)
		}
		return lValue{Kind: vOppLifeLostThisTurn}, nil
	}
	spec, ok := strings.CutPrefix(rest, "Valid ")
	if !ok || strings.Contains(spec, "$") {
		return lValue{}, fmt.Errorf("PlayerCountRemembered$%s", rest)
	}
	v, err := lc.countValue("Valid "+spec, depth)
	if err != nil {
		return lValue{}, err
	}
	if v.Kind != vCount || v.F == nil {
		return lValue{}, fmt.Errorf("PlayerCountRemembered$%s", rest)
	}
	for _, c := range v.F.Clauses {
		if c.Ctrl != whoEachPl {
			// a card count that is not the repeated player's permanents
			return lValue{}, fmt.Errorf("PlayerCountRemembered$Valid %q without RememberedPlayerCtrl", spec)
		}
	}
	return v, nil
}

// checkEachPlCtrl refuses RememberedPlayerCtrl outside a RepeatEach player
// body, where no player is bound.
func (lc *lowerCtx) checkEachPlCtrl(expr string, f *lFilter) error {
	if lc.repeatPl > 0 {
		return nil
	}
	for _, c := range f.Clauses {
		if c.Ctrl == whoEachPl {
			return refuse("filter %q: RememberedPlayerCtrl outside a RepeatEach player body", expr)
		}
	}
	return nil
}

// eachPlSVarOK reports whether the SVar body named by a RepeatEach body
// param reads the Remembered player only through the two reads above:
// <PlayerCountRemembered$... | Count$Valid[Hand|Library|Graveyard]
// ...RememberedPlayerCtrl|RememberedPlayerOwn...> with at most one
// /HalfDown, /HalfUp or /Minus.N tail, and that it lowers inside the body.
// Anything else (Remembered$Amount, an operand SVar, ...) is not admitted
// and keeps the caller's refusal.
func (lc *lowerCtx) eachPlSVarOK(body string) bool {
	b := strings.TrimSpace(body)
	for _, t := range []string{"/HalfDown", "/HalfUp"} {
		b = strings.TrimSuffix(b, t)
	}
	if head, n, ok := strings.Cut(b, "/Minus."); ok && n != "" && strings.Trim(n, "0123456789") == "" {
		b = head // Doomsday Excruciator's all-but-six
	}
	switch {
	case strings.HasPrefix(b, "PlayerCountRemembered$"):
		b = strings.TrimPrefix(b, "PlayerCountRemembered$")
	case strings.HasPrefix(b, "Count$Valid "), strings.HasPrefix(b, "Count$ValidHand "), strings.HasPrefix(b, "Count$ValidLibrary "), strings.HasPrefix(b, "Count$ValidGraveyard "):
		// the repeated player's battlefield, hand, library or graveyard
		// cards (Worlds Within Worlds, Doomsday Excruciator, Riverchurn
		// Monument, Singularity Rupture): a card off the battlefield is
		// controlled by its owner, the kernel's walk owner
	default:
		return false
	}
	if _, ok, _ := eachOtherRevealMV(strings.TrimPrefix(b, "Count$")); ok {
		b = "" // Parker Luck's exact read; the trial below proves it lowers
	}
	rest := strings.NewReplacer("RememberedPlayerCtrl", "", "RememberedPlayerOwn", "").Replace(b)
	if strings.Contains(rest, "Remember") || strings.Contains(b, "/") {
		return false
	}
	lc.repeatPl++
	defer func() { lc.repeatPl-- }()
	_, err := lc.svarValue(body, 0)
	return err == nil
}

// eachOtherRevealMV matches exactly the two spellings of "the mana value of
// the card revealed by the OTHER player" (Parker Luck's Own, Keen Duelist's
// Ctrl; off the battlefield a card's controller is its owner) and returns the
// parameterless vNotOwnLibMV read. Nothing else is admitted.
func eachOtherRevealMV(rest string) (lValue, bool, error) {
	switch rest {
	case "ValidLibrary Card.IsRemembered+!RememberedPlayerOwn$CardManaCost",
		"ValidLibrary Card.IsRemembered+!RememberedPlayerCtrl$CardManaCost":
		return lValue{Kind: vNotOwnLibMV}, true, nil
	}
	return lValue{}, false, nil
}
