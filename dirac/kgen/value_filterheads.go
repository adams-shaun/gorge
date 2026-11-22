package kgen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// the value heads of the MageZero sample decks' value-heads
// family (CARD-CENSUS.md), each read exactly as gorge's count evaluator at
// the pin reads it (any other spelling refuses).

// kvfHead lowers the PlayerCountPlayers$, TargetedController$ and
// TriggeredTarget$ and TriggeredDefendingPlayer$ heads.
func (lc *lowerCtx) kvfHead(head, rest string) (lValue, error) {
	switch head {
	case "PlayerCountPlayers":
		if strings.HasPrefix(rest, "HighestValid") {
			// the zone-scoped extreme (value_plrextreme.go): the highest,
			// over the living players, of the member's own-side count in
			// the named zones
			return lc.plExtremeValue(head, rest)
		}
		switch rest {
		case "AttackersDeclared":
			// gorge cov3PlayerScalarAttackersDeclared: the every-player
			// group reads Engine.AttackersThisTurn (every DeclareAttackers
			// event's attackers this turn, summed). Witchstalker Frenzy.
			return lValue{Kind: vAttackersAll}, nil
		case "Amount":
			// gorge playerGroupCount over AliveFrom: the living players,
			// the controller's living opponents and the controller
			// (seat-exact: a player who lost has left the game with
			// everything they controlled, CR 800.4a, so the reader lives)
			return lValue{Kind: vOppExtreme, N: kop.OppExtAmount, Add: 1}, nil
		case "HasPropertyisMonarch":
			return monarchHeldValue(), nil // lower_monarch.go
		case "LowestLifeTotal":
			// gorge lifeExtreme over AliveFrom (engine/effects/
			// count_player.go): the lowest CURRENT life total among the
			// living players, min of both while a two-player game goes on
			// (seat-exact: ExLowestLife reads every living seat),
			// read as the replacement applies. The DSK "unless a player has
			// 13 or less life" lands. HighestLifeTotal and every other
			// property stay refused.
			return lValue{Kind: vLowestLife}, nil
		}
	case "TargetedController":
		if rest != "Counters.Poison" {
			break
		}
		// gorge evalPlayerRefProperty: the controllers of the targets
		// (controllersOf(PickedTargets-else-Targets)), each one's POISON
		// counters summed. Lowered only for one object-only target slot,
		// whose controller is kop.RefTargetController. Anoint with
		// Affliction.
		t := lc.lastTarget()
		if t < 0 || len(lc.chainT) != 1 {
			return lValue{}, refuse("TargetedController$%s without exactly one target slot", rest)
		}
		if f := lc.ab.Targets[t].F; f == nil || f.Players != 0 || len(f.Clauses) == 0 {
			return lValue{}, refuse("TargetedController$%s of a slot that may name a player", rest)
		}
		return lValue{Kind: vPoisonOf, Ref: lRef{Kind: rTargetController, N: t}}, nil
	case "TriggeredDefendingPlayer":
		if spec, ok := strings.CutPrefix(rest, "ValidExile "); ok {
			return lc.defendingExileValue(spec) // below
		}
		if rest != "CardsInHand" {
			break
		}
		// gorge evalPlayerRefProperty's default arm: definedSpec(
		// TriggeredDefendingPlayer), the player the controller's own
		// attack trigger names (the Defined$ reading, lc.defined); its
		// hand size is the cards in that player's hand. Mr. Foxglove.
		if !lc.ownAttackTrigger() {
			return lValue{}, refuse("TriggeredDefendingPlayer$%s outside an attack trigger of the controller's own attack", rest)
		}
		return lValue{Kind: vHandSizeOf, Ref: lRef{Kind: rDefending}}, nil
	case "TriggeredTarget":
		if rest == "CardsInHand" || strings.HasPrefix(rest, "Valid ") {
			return lc.trigTargetPlayerValue(rest) // value_trigreferents.go
		}
		if rest != "LifeTotal" {
			break
		}
		// gorge evalPlayerRefProperty's default arm: definedSpec(
		// TriggeredTarget), the damaged player of a damage trigger, read
		// only where the kernel's trigger player is that player (the
		// Defined$ TriggeredTarget reading). Unstoppable Slasher.
		if !lc.damagedPlayerIsTriggerPlayer() {
			return lValue{}, refuse("TriggeredTarget$%s outside a damage-to-player trigger the kernel's trigger player names", rest)
		}
		return lValue{Kind: vLifeOf, Ref: lRef{Kind: rTriggeredPlayer}}, nil
	}
	return lValue{}, fmt.Errorf("%s$%s", head, rest)
}

// defendingExileValue lowers TriggeredDefendingPlayer$ValidExile <spec>: the
// exiled cards matching spec with the defending player as the spec's You
// (Ravenloft Adventurer's "each card they own in exile with a hit counter").
// Dirac is two-seat, so the defending player of the controller's own attack
// is the controller's one opponent: YouOwn rebinds to OppOwn and the count is
// ExCountExile (every exile, the Owner property narrows). Any other player
// word in a spec alternative would bind to the controller's side, so it
// refuses. Gorge's defaultRefPropertyModelled does not read this head for
// TriggeredDefendingPlayer yet (it counts 0): see the ticket
// engine-dungeons-completed-and-defending-validexile-heads.
func (lc *lowerCtx) defendingExileValue(spec string) (lValue, error) {
	if !lc.ownAttackTrigger() {
		return lValue{}, refuse("TriggeredDefendingPlayer$ValidExile outside an attack trigger of the controller's own attack")
	}
	lc.twoSeat("TriggeredDefendingPlayer read as the one opponent")
	alts := strings.Split(strings.TrimSpace(spec), ",")
	for i, alt := range alts {
		typ, props, _ := strings.Cut(strings.TrimSpace(alt), ".")
		kept := []string{}
		for _, p := range strings.Split(props, "+") {
			switch {
			case p == "YouOwn":
				kept = append(kept, "OppOwn")
			case strings.Contains(p, "You") || strings.Contains(p, "Opp") || strings.Contains(p, "Own") || strings.Contains(p, "Ctrl") || strings.Contains(p, "Player"):
				return lValue{}, refuse("TriggeredDefendingPlayer$ValidExile %q: player word %q", spec, p)
			default:
				kept = append(kept, p)
			}
		}
		alts[i] = typ + "." + strings.Join(kept, "+")
	}
	f, err := lc.filter(strings.Join(alts, ","))
	if err != nil {
		return lValue{}, err
	}
	return lValue{Kind: vCountExile, F: f}, nil
}

// metalcraftValue lowers Count$Metalcraft.<yes>.<no> (gorge
// cov3BranchHoldsMetalcraft: Count$Valid Artifact.YouCtrl >= 3; each arm a
// literal or an SVar through countBranchOperand). Galvanic Blast.
func (lc *lowerCtx) metalcraftValue(arms string, depth int) (lValue, error) {
	parts := strings.Split(arms, ".")
	if len(parts) != 2 {
		return lValue{}, refuse("Count$Metalcraft arms %q", arms)
	}
	f, err := lc.filter("Artifact.YouCtrl")
	if err != nil {
		return lValue{}, err
	}
	v := lValue{Kind: vCountAtLeast, F: f, N: 3}
	for _, p := range parts {
		if n, err := strconv.Atoi(p); err == nil {
			v.Sub = append(v.Sub, cst(n))
			continue
		}
		body, ok := lc.face.SVars[p]
		if !ok {
			return lValue{}, refuse("Count$Metalcraft arm %q names no SVar", p)
		}
		sv, err := lc.svarValue(body, depth+1)
		if err != nil {
			return lValue{}, err
		}
		v.Sub = append(v.Sub, sv)
	}
	return v, nil
}

// kvfHalfUp is the /HalfUp modifier (gorge applyCountOp: v = (v+1)/2, Go's
// truncating division): Sub[0] + 1 under the kernel Expr's Div 2, which is
// the same truncating division (State.expr's v /= Div), so it is exact for a
// negative operand too.
func kvfHalfUp(v lValue, m string, hasArg bool) (lValue, error) {
	if hasArg || v.Mul != 0 || v.Div != 0 || v.Add != 0 || v.Neg {
		return lValue{}, fmt.Errorf("value modifier %q", m)
	}
	return lValue{Kind: vSum, Sub: []lValue{v, cst(1)}, Div: 2}, nil
}

// kvfThirdUp is the /ThirdUp modifier (gorge applyCountOp: v = (v+2)/3, Go's
// truncating division), lowered like kvfHalfUp: Sub[0] + 2 under Div 3.
func kvfThirdUp(v lValue, m string, hasArg bool) (lValue, error) {
	if hasArg || v.Mul != 0 || v.Div != 0 || v.Add != 0 || v.Neg {
		return lValue{}, fmt.Errorf("value modifier %q", m)
	}
	return lValue{Kind: vSum, Sub: []lValue{v, cst(2)}, Div: 3}, nil
}

// kvfTimesSVar is /Times.<SVar> (gorge applyCountOpOperand: the operand's
// value, then applyCountOp's Times arm): the product of the two values.
// Lunar Convocation's LifeLostThisTurn/Times.X.
func (lc *lowerCtx) kvfTimesSVar(v lValue, m, arg string, depth int) (lValue, error) {
	body, ok := lc.face.SVars[arg]
	if !ok {
		return lValue{}, fmt.Errorf("value modifier %q: %s is neither a number nor an SVar", m, arg)
	}
	sv, err := lc.svarValue(body, depth+1)
	if err != nil {
		return lValue{}, fmt.Errorf("value modifier %q: SVar %s=%q: %w", m, arg, body, err)
	}
	return lValue{Kind: vProduct, Sub: []lValue{v, sv}}, nil
}

// kvfInlineOperand reports whether a modifier operand is an inline count
// expression the lowering reads exactly (gorge countOperandValue's '$'
// fallback): Remembered$Amount (Refurbished Familiar) and the remembered
// objects matching a Valid spec (Hollow Marauder), each the value the same
// text reads as a head.
func kvfInlineOperand(arg string) bool {
	return arg == "Remembered$Amount" || strings.HasPrefix(arg, "Remembered$Valid ")
}

// kvfTriggerConds adds the player gates to a trigger's
// conditions and reports whether one is a fire-time-only gate:
//   - ValidPlayer$ You.descended: a permanent card you own entered a
//     graveyard this turn (gorge descendedThisTurn over g.Entered: To
//     graveyard, Owner you, PermanentCard -- not a token);
//   - CheckDefinedPlayer$ You.withMostLife: your life is not less than every
//     living opponent's (gorge checkDefinedPlayerHolds -> playerHasMost).
//
// Attacked$ Player.withMostLife is not here: it lowers to the shared
// dethroneCond over the defended seat (lower_dethrone.go), which is exact at
// every seat count.
func (lc *lowerCtx) kvfTriggerConds(tp *params, conds *[]lCond, descended bool) (bool, error) {
	you := lValue{Kind: vLifeOf, Ref: lRef{Kind: rYou}}
	fire := false
	if descended {
		v, err := lc.countValue("ThisTurnEntered_Graveyard_Permanent.YouOwn+nonToken", 0)
		if err != nil {
			return false, err
		}
		*conds = append(*conds, lCond{Kind: cValue, A: v, Op: opGE, B: cst(1)})
		fire = true
	}
	if tp.str("CheckDefinedPlayer") == "You.withMostLife" {
		tp.consume("CheckDefinedPlayer")
		opps := lValue{Kind: vOppExtreme, N: kop.OppExtLife}
		*conds = append(*conds, lCond{Kind: cValue, A: you, Op: opGE, B: opps})
	}
	return fire, nil
}
