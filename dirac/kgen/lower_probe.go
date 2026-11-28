package kgen

import (
	"errors"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// probeRefKeys are the reference params a refused node's stopped reading can
// hide. probeParam reads one through the Defined grammar, as its node would;
// ValidPlayers is a player filter where the node reads it as one (DamageAll)
// and a reference elsewhere (ProtectionAll's player set).
var probeRefKeys = map[string]bool{
	"Defined": true, "DefinedPlayer": true, "Chooser": true,
	"TargetingPlayer": true, "ValidPlayers": true, "UnlessPayer": true,
}

// probeCondKeys are the node's condition params whose SVar or reference can
// hide a blocker: ConditionCheckSVar$ and CheckSVar$ read through value,
// ConditionDefined$ through the Defined grammar.
var probeCondKeys = map[string]bool{
	"ConditionCheckSVar": true, "CheckSVar": true, "ConditionDefined": true,
}

// errProbeSkip is a key this probe has no reader for in its context: it stays
// in the unread-after-refusal row rather than becoming a false blocker.
var errProbeSkip = errors.New("probe skipped")

// probeUnread probes every param the refused node left unread, recording the
// reader's refusal as a blocker and returning the keys no probe read. Each
// probe runs under its own scratch ability: a reader that publishes an
// instruction (value's remembered-sacrifice note) must not touch the real
// ability, and one probe's scratch program must not change the next reader's
// context (the triggered-cost reader tests whether the program is empty).
func (lc *lowerCtx) probeUnread(p *params, sa *cards.SA) []string {
	keys := p.unread()
	if len(keys) == 0 {
		return nil
	}
	var left []string
	for _, k := range keys {
		var err error
		_ = lc.probeScratch(func() error {
			err = lc.probeNodeParam(sa, k, p.m[k])
			return nil
		})
		if errors.Is(err, errProbeSkip) {
			left = append(left, k)
			continue
		}
		if err != nil {
			lc.refuseNode(lc.blockerNode(), "%s$ %s: %s", sa.Kind, sa.API, err.Error())
			p.used[k] = true
			continue
		}
		left = append(left, k)
	}
	return left
}

// probeScratch runs fn under a scratch "Probe" ability shaped like the one
// being lowered. The chain's targets and mode stay visible to the reader
// (Defined$ Targeted reads them), but a writer's instructions land in the
// scratch program.
func (lc *lowerCtx) probeScratch(fn func() error) error {
	saved := lc.ab
	at := len(lc.out.Abilities)
	probe := lc.newLatent("Probe")
	if saved != nil {
		*probe = *saved
		probe.Index, probe.Name = at, "Probe"
		probe.Prog = append([]lInstr(nil), saved.Prog...)
		probe.Targets = append([]lTarget(nil), saved.Targets...)
	}
	savedT, savedMode, savedRem, savedX := lc.chainT, lc.mode, lc.remTgt, lc.xPaidAb
	savedRoll, savedFlip, savedRepeat := lc.rollAb, lc.flipTallyAb, lc.repeatCheckAb
	savedOpp, savedPl, savedMid := lc.chosenOppAb, lc.chosenPlAb, lc.chosenMidAb
	savedRes, savedEach, savedRemOpp := lc.chosenResAb, lc.chosenEachAb, lc.remOppAb
	// A reader that compares the current ability by identity (a bare X on
	// its own {X} cost, a RollDice result, a chosen player) must see the
	// probe as that ability.
	remap := func(p **lAbility) {
		if saved != nil && *p == saved {
			*p = probe
		}
	}
	err := lc.withAbility(probe, func() error {
		lc.chainT, lc.mode, lc.remTgt, lc.xPaidAb = savedT, savedMode, savedRem, savedX
		remap(&lc.xPaidAb)
		remap(&lc.rollAb)
		remap(&lc.flipTallyAb)
		remap(&lc.repeatCheckAb)
		remap(&lc.chosenOppAb)
		remap(&lc.chosenPlAb)
		remap(&lc.chosenMidAb)
		remap(&lc.chosenResAb)
		remap(&lc.chosenEachAb)
		remap(&lc.remOppAb)
		return fn()
	})
	lc.rollAb, lc.flipTallyAb, lc.repeatCheckAb = savedRoll, savedFlip, savedRepeat
	lc.chosenOppAb, lc.chosenPlAb, lc.chosenMidAb = savedOpp, savedPl, savedMid
	lc.chosenResAb, lc.chosenEachAb, lc.remOppAb = savedRes, savedEach, savedRemOpp
	// The scratch program is not part of the card's lowering: drop it so the
	// ability indices the census prints for later refusals do not shift.
	if len(lc.out.Abilities) == at+1 {
		lc.out.Abilities = lc.out.Abilities[:at]
	}
	return err
}

// probeReference reads a reference param through the reader the node would
// use. ValidPlayers$ is a player filter on DamageAll (damageAll's
// lc.filter), the only node family that reads it as one; everywhere else it
// names a player through the Defined grammar (ProtectionAll$ You).
func (lc *lowerCtx) probeReference(k, v string) error {
	if k == "Defined" && lc.curNode != nil {
		// These APIs read Defined$ Valid <filter> through lc.filter in their
		// own body, not through the Defined grammar.
		if spec, ok := strings.CutPrefix(strings.TrimSpace(v), "Valid "); ok {
			switch lc.curNode.API {
			case "ChangeCombatants", "Goad", "Airbend", "Phases":
				_, err := lc.filter(strings.TrimSpace(spec))
				return err
			}
		}
	}
	if k == "ValidPlayers" && (lc.curNode == nil || lc.curNode.API != "DamageAll") {
		_, err := lc.defined(v)
		return err
	}
	if k == "ValidPlayers" {
		switch strings.TrimSpace(v) {
		case "OppNonTriggeredTarget", "Targeted":
			// damageAll reads these two before the filter (lower/effect_damage.go):
			// no filter head to read, so leave the key unprobed rather than
			// report a false one.
			return nil
		}
		_, err := lc.filter(v)
		return err
	}
	if k == "UnlessPayer" && strings.TrimSpace(v) == "ThisTargetedController" {
		// unless maps this spelling onto TargetedController before calling
		// defined (lower_effect_node.go).
		v = "TargetedController"
	}
	_, err := lc.defined(v)
	return err
}

// probeCondition reads a condition param through the reader its node would
// use. ConditionDefined$ Collected and CastSA>Collected are collected-
// evidence spellings, not Defined specs, so no probe reads them.
func (lc *lowerCtx) probeCondition(k, v string) error {
	if k == "ConditionDefined" {
		switch strings.TrimSpace(v) {
		case "Collected", "CastSA>Collected":
			return errProbeSkip
		}
		_, err := lc.defined(v)
		return err
	}
	_, err := lc.value(v)
	return err
}

// probeUnlessCost reads an UnlessCost$ through the readers lowerCtx.unless
// uses, in its order, without emitting the payment wrapper.
func (lc *lowerCtx) probeUnlessCost(v string) error {
	uc := strings.TrimSpace(v)
	// unless restricts an amount read to the node's own single target while
	// it prices an unless (lower_effect_node.go); mirror it so a target-
	// reading amount is not a false blocker.
	savedMV := lc.mvNode
	defer func() { lc.mvNode = savedMV }()
	if lc.ab != nil && lc.ab.Kind == akTriggered && lc.curNode != nil &&
		!strings.Contains(uc, "Discard<") && !strings.Contains(uc, "Sac<") {
		lc.mvNode = lc.curNode
	}
	if _, isRes, err := lc.resUnlessToken(uc); err != nil { // lower_costtokens.go
		return err
	} else if isRes {
		return nil
	}
	if _, isRes, err := lc.resUnlessDiscard(uc); err != nil {
		return err
	} else if isRes {
		return nil
	}
	if uc == "DefinedCost_ChosenCard" {
		return nil
	}
	if _, isVar := lc.face.SVars[uc]; isVar {
		_, err := lc.value(uc)
		return err
	}
	lc.wbOK = lc.ab != nil && lc.ab.Kind != akLatent
	lc.unlessParse = lc.wbOK
	_, err := lc.costUnless(uc)
	lc.unlessParse, lc.wbOK = false, false
	return err
}

// probeNodeCost reads a chain link's Cost$ through the reader lowerCtx.effect
// uses: the triggered body's own cost may use the unless selection methods
// only while the body's program is empty, everything else is an ordinary
// resolution cost.
func (lc *lowerCtx) probeNodeCost(v string) error {
	parse := lc.cost
	if lc.ab != nil && lc.ab.Kind == akTriggered && len(lc.ab.Prog) == 0 {
		lc.trigBodyCost = true
		parse = lc.costUnless
	}
	_, err := parse(v)
	lc.trigBodyCost = false
	return err
}
