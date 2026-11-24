package kgen

import (
	"strings"
)

func (lc *lowerCtx) token(ps *params) error {
	script := ps.str("TokenScript")
	xpt, err := lc.tokenXPTGate(ps, script) // lower_tokenxpt.go
	if err != nil {
		return err
	}
	if strings.Contains(script, ",") {
		return lc.tokenList(ps, script)
	}
	if script == "" {
		return refuse("TokenScript %q", script)
	}
	if _, ok := lc.p.reg.Token(script); !ok {
		return refuse("token script %q not in the corpus", script)
	}
	var n lValue
	if ps.str("TokenAmount") == "Excess" {
		n = lValue{Kind: vExcessDamage}
	} else if n, err = lc.amount(ps, "TokenAmount", 1); err != nil {
		return err
	}
	owner := lRef{Kind: rYou}
	andYou := false
	if who, ok, err := lc.tokenAndYou(ps, xpt); err != nil {
		return err
	} else if ok {
		// gorge's tokenOwnerPlayers resolves the alias through the wider
		// Defined$ grammar: the targeted player first, then the controller,
		// one mint each. The second mint is emitted below, only for the
		// plain shape.
		owner, andYou = who, true
	} else if o, ok := ps.get("TokenOwner"); ok && o == "RememberedOwner" {
		// the owner of the card the enters trigger exiled (CR 108.3), only
		// in the remembered-persist shape (lower_rempersist.go)
		if !lc.remPersistShape() {
			return refuse("Token TokenOwner$ RememberedOwner outside the remembered-persist shape")
		}
		owner = lRef{Kind: rRememberedOwner}
	} else if o, ok := ps.get("TokenOwner"); ok && o == "RememberedController" {
		// gorge rememberedControllersLKI: the remembered object's
		// controller, its RememberLKI$ capture once it left the battlefield
		owner = lRef{Kind: rRememberedController}
	} else if ok {
		if (o == "Targeted" || o == "TargetedPlayer") && strings.TrimSpace(ps.str("ValidTgts")) != "" {
			// gorge's tokenOwnerPlayers (effects/token.go:144) reads
			// definedSpec for Targeted/TargetedPlayer: the Token SA's own
			// targets, so its own ValidTgts$ answers, not the chain's last
			// slot. Only a player-only, one-target declaration is exact; a
			// count rider would make the created token's owner ambiguous
			// here.
			for _, k := range []string{"TargetMin", "TargetMax", "TargetUnique", "TgtZone"} {
				if _, ok := ps.get(k); ok {
					return refuse("Token TokenOwner$ %q with %s$", o, k)
				}
			}
			vt := ps.str("ValidTgts")
			pf, err := lc.filter(vt)
			if err != nil {
				return err
			}
			if pf.Players == 0 || len(pf.Clauses) != 0 {
				return refuse("Token TokenOwner$ %q ValidTgts$ %q", o, vt)
			}
			owner = lRef{Kind: rTarget, N: lc.addTarget(lTarget{F: pf, Zone: zNone})}
			ps.consume("ValidTgts", "TgtPrompt")
		} else if owner, err = lc.defined(o); err != nil {
			return err
		}
	}
	// LockTokenScript$ (a gift's Fish): Forge keeps the script from being
	// replaced; gorge reads no such parameter and creates the script as is
	ps.consume("LockTokenScript")
	in := lInstr{Op: "CreateToken", Obj: owner, A: n, CS: script}
	// AttachedTo$: a Role token enters attached (CR 111.10j-r, 303.4g). The
	// destination rides B.N's high byte (kop.TokAttach); Obj2 carries the
	// Ref. gorge resolves it once before the mint loop (effects/token.go
	// tokenAttachTargets): a Token SA's own ValidTgts$ answers with its own
	// picked target, everything else through DefinedSpec (the chain's
	// single target), and Self names the source.
	if v, ok := ps.get("AttachedTo"); ok {
		var dst lRef
		switch v {
		case "Targeted", "ThisTargetedCard":
			if vt, own := ps.get("ValidTgts"); own && strings.TrimSpace(vt) != "" {
				var err error
				dst, err = lc.objOrTarget(ps, "", zBattlefield)
				if err != nil {
					return err
				}
				if dst.Kind != rTarget {
					return refuse("Token AttachedTo %q", v)
				}
				if dst.N > 2 {
					return refuse("Token AttachedTo target slot %d", dst.N)
				}
			} else {
				if v == "ThisTargetedCard" {
					return refuse("Token AttachedTo ThisTargetedCard without its own ValidTgts")
				}
				// gorge's DefinedSpec(Targeted) is the chain's target; the
				// kernel's union ref is ordered differently, so only the
				// single-slot chain is read exactly.
				if len(lc.chainT) != 1 {
					return refuse("Token AttachedTo Targeted over %d chain target slots", len(lc.chainT))
				}
				if lc.chainT[0] > 2 {
					return refuse("Token AttachedTo target slot %d", lc.chainT[0])
				}
				dst = lRef{Kind: rTarget, N: lc.chainT[0]}
			}
		case "Self":
			dst = lRef{Kind: rSelf}
		case "Remembered":
			// the repeated card of a RepeatEach over cards (lower_repeatcards.go)
			if lc.repeatCard == 0 {
				return refuse("Token AttachedTo %q", v)
			}
			dst = lRef{Kind: rEach}
		default:
			return refuse("Token AttachedTo %q", v)
		}
		if a, ok := ps.get("TokenAmount"); ok && strings.TrimSpace(a) != "1" {
			return refuse("Token AttachedTo with TokenAmount %q", a)
		}
		if !tokenIsAura(lc.p, script) {
			return refuse("Token AttachedTo on a non-Aura token %q", script)
		}
		ps.consume("TgtPrompt")
		in.C |= 32
		in.Obj2 = dst
	} else if tokenIsAura(lc.p, script) {
		// an Aura token minted without a destination has gorge's CR
		// 303.4f gate / non-cast entry ask, which the kernel does not model.
		return refuse("Aura token %q without AttachedTo$", script)
	}
	if ps.is("TokenTapped", "True") {
		in.C |= 1
	}
	if v, ok := ps.get("TokenAttacking"); ok {
		c, err := lc.tokenAttacking(v)
		if err != nil {
			return err
		}
		in.C |= c
	}
	if ps.is("RememberOriginalTokens", "True") {
		// gorge effects/token_params.go: p.Remember = RememberTokens ||
		// RememberOriginalTokens, so the created tokens join Ctx.Remembered
		in.C |= 2 | 4 // kop.TokRemember | kop.TokRememberObj
	}
	if v, ok := ps.get("AtEOT"); ok {
		if err := lc.tokenAtEOTBits(&in, v); err != nil {
			return err
		}
		if err := tokenAtEOTGroupRefusal(v, n, owner, andYou); err != nil {
			return err
		}
	}
	if ps.is("RememberTokens", "True") {
		// gorge token.go: each mint is appended to Ctx.Remembered. The
		// kernel's remembered set lives for one resolution while gorge's
		// persists on the source, so the tokens must be read later in
		// this chain, not by another ability.
		if !lc.restReadsRemembered() {
			return refuse("RememberTokens read outside its own resolution")
		}
		in.C |= 4
	}
	if err := lc.tokenMemory(ps, script, &in); err != nil { // lower_imprintzone.go, lower_tokenrem.go
		return err
	}
	var pump []string
	if kw, ok := ps.get("PumpKeywords"); ok {
		// gorge token.go: each minted token gets one LAbilities effect
		// sourced to itself, until end of turn when PumpDuration$ is
		// absent or EOT (Sokenzan's "They gain haste until end of turn").
		// The kernel remembers the mints and grants the keywords over the
		// remembered set, so the set must hold only these mints when the
		// PumpAll reads it: an earlier instruction may have written it (a
		// preceding Destroy writes nothing, a RememberChanged Exile/Dig or
		// a previous token does), and ClearRemembered just before the mint
		// makes that irrelevant. Exact because nothing later in the chain
		// reads the set (restReadsRemembered) and an earlier delayed
		// trigger captured its object by value when it registered
		// (dirac/interp.go OpDelayedTrigger, interp_gen.go
		// OpDelayedProgram), so clearing now cannot change what it fires
		// with. RememberTokens$ on this Token is still refused: clearing
		// then would drop the tokens its own later reader expects.
		if d, ok := ps.get("PumpDuration"); ok && d != "EOT" {
			return refuse("Token PumpDuration$ %q", d)
		}
		if in.C&4 != 0 || lc.restReadsRemembered() {
			return refuse("Token PumpKeywords$ beside another remembered set")
		}
		ks, err := pumpKeywords(kw)
		if err != nil {
			return err
		}
		in.C |= 4
		pump = ks
	} else if _, ok := ps.get("PumpDuration"); ok {
		return refuse("Token PumpDuration$ without PumpKeywords$")
	}
	xptIdx := 0
	if xpt {
		// the Ex form counts the mint (tokMade) for ExtTokenPT
		if in.C&(64|128) != 0 {
			return refuse("Token TokenPower$ with TokenAttacking$")
		}
		in.C |= 256
		if xptIdx, err = lc.tokenXPTOverride(ps); err != nil {
			return err
		}
	}
	lc.out.Tokens = append(lc.out.Tokens, script)
	if pump != nil && len(lc.ab.Prog) != 0 {
		// Forget whatever an earlier instruction remembered (see the guard
		// above): the PumpAll that follows the mint must see only it.
		lc.emit(lInstr{Op: "ClearRemembered"})
	}
	if xpt {
		// the Ext reads the X/X, then runs the CreateToken that follows
		lc.emit(lInstr{Op: "TokenPT", C: xptIdx})
	}
	lc.emitMints(in, andYou)
	if pump != nil {
		f, err := lc.filter("Card.IsRemembered")
		if err != nil {
			return err
		}
		lc.emit(lInstr{Op: "PumpAll", F: f, A: cst(0), B: cst(0), CS: strings.Join(pump, "&")})
	}
	return nil
}

// tokenAndYou is targetedAndYou for a Token's TokenOwner$: gorge mints once for
// the targeted player, then once for the controller, so only the plain shape
// (the tapped rider alone) is lowered; any other rider would have to ride
// both mints.
func (lc *lowerCtx) tokenAndYou(ps *params, xpt bool) (lRef, bool, error) {
	who, ok, err := lc.targetedAndYou(ps, "TokenOwner")
	if err != nil || !ok {
		return who, ok, err
	}
	if xpt {
		return lRef{}, false, refuse("Token TokenOwner$ TargetedAndYou with a TokenPower$ override")
	}
	for _, k := range []string{"TokenAttacking", "AttachedTo", "RememberTokens", "RememberOriginalTokens", "AtEOT", "ImprintTokens", "PumpKeywords", "PumpDuration"} {
		if _, has := ps.get(k); has {
			return lRef{}, false, refuse("Token TokenOwner$ TargetedAndYou with %s$", k)
		}
	}
	return who, true, nil
}

// emitMints emits a Token's CreateToken; under TargetedAndYou it adds the
// controller's mint after the targeted player's.
func (lc *lowerCtx) emitMints(in lInstr, andYou bool) {
	lc.emit(in)
	if andYou {
		in.Obj = lRef{Kind: rYou}
		lc.emit(in)
	}
}

// tokenIsAura reports whether the token script's face is an Aura with an
// Enchant keyword line (gorge's CR 303.4f/g entry path keys on the same).
func tokenIsAura(p *pool, script string) bool {
	c, ok := p.reg.Token(script)
	if !ok || len(c.Faces) == 0 {
		return false
	}
	f := c.Faces[0]
	aura, enchant := false, false
	for _, t := range f.Types {
		if t == "Aura" {
			aura = true
		}
	}
	for _, k := range f.Keywords {
		if strings.HasPrefix(k, "Enchant:") {
			enchant = true
		}
	}
	return aura && enchant
}

// tokenAtEOTGroupRefusal refuses an AtEOT$ Destroy or ExileCombat rider over
// a mint that can make several tokens (an amount other than the constant 1,
// an AndYou second mint, a player-set owner): gorge registers one delayed
// trigger naming them all (scheduleAtEOTGroup, CR 603.7a/b), while the kernel
// joins only the sacrifice (sacAtEndJoin) and end-step exile
// (exileAtEndJoin, Feral Lightning) riders.
func tokenAtEOTGroupRefusal(v string, n lValue, owner lRef, andYou bool) error {
	switch v {
	case "Destroy", "ExileCombat":
	default:
		return nil
	}
	switch {
	case andYou, !n.isConst() || n.N != 1,
		owner.Kind == rEachOpp, owner.Kind == rEachPlayer, owner.Kind == rTriggerOpps, owner.Kind == rTargetUnion:
		return refuse("Token AtEOT %q over several tokens is one grouped trigger", v)
	}
	return nil
}

// tokenAtEOTBits sets the C bits of a Token's AtEOT$ rider v on in (gorge
// token.go -> scheduleAtEOT: one next-end-step promise per minted token, the
// copy-token rider's dly entries).
func (lc *lowerCtx) tokenAtEOTBits(in *lInstr, v string) error {
	switch v {
	case "Sacrifice", "YourSacrifice":
		// gorge effects/ateot.go:26-31 deliberately reads the Your-prefixed
		// end-step values as their plain equivalents (atEOTRiders:
		// YourSacrifice = __kwEncoreSacrifice at StepEnd, as Sacrifice)
		in.C |= 8
	case "Exile", "YourExile":
		in.C |= 16 // YourExile reads as Exile (gorge ateot.go)
	case "YourSacrificeUpkeep":
		// gorge atEOTRiders: the untracked sacrifice at the controller's own
		// next upkeep (VP=You); the kernel's dlyYourUpkeep records
		if in.C&32 != 0 {
			return refuse("Token AtEOT %q beside AttachedTo$", v)
		}
		in.C |= 8 | 16 | tokSacYourUpkeepBit
	case "Destroy", "SacrificeCombat", "ExileCombat":
		// the extended riders: both AtEOT bits (8|16) plus 512 (Destroy at
		// the end step, tracked, through destroy) or 1024 (sacrifice at
		// this turn's end of combat, untracked). The kind rides in
		// B.N's high byte (kop.TokAtEOTDestroy / TokAtEOTSacCombat).
		if in.C&32 != 0 {
			return refuse("Token AtEOT %q beside AttachedTo$", v)
		}
		if v == "Destroy" {
			in.C |= 8 | 16 | 512
			break
		}
		// gorge schedules SacrificeCombat as a thisTurn rider (|TT=turn)
		// at StepEndCombat. Inside an Attacks trigger the resolution is
		// always in this turn's combat, before its end-of-combat step
		// (CR 508.8, 511.1: attackers are declared, then the trigger
		// resolves in the declare-attackers step), so the thisTurn expiry
		// can never bite; elsewhere it could (a main-phase mint).
		if lc.ab.Trigger == nil || lc.ab.Trigger.Event != "Attacks" {
			return refuse("Token AtEOT %s outside an Attacks trigger", v)
		}
		in.C |= 8 | 16 | 1024
		if v == "ExileCombat" {
			in.C |= 512 // kind 3: TokAtEOTExileCombat, the tracked exile at end of combat
		}
	default:
		return refuse("Token AtEOT %q", v)
	}
	return nil
}
