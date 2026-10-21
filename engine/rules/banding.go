package rules

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Banding (CR 702.22). As attackers are declared, the attacking player may
// put one or more attacking creatures with banding and up to one attacking
// creature without banding, all attacking the same player, planeswalker or
// battle, into a band (702.22c-d, 802.3b). A band lasts for the rest of
// combat (702.22e) and is recorded on each member's state.Object.Band by a
// Choose "band" event. When a blocker blocks one member, it blocks every
// member (702.22h, bandBlockPairs); an effect that blocks one member blocks
// the band (702.22i, engine/effects/becomesblocked.go). "Bands with other"
// (702.22b) is not implemented: kw:Bands with other stays unregistered.

// chooseBand is the chooseFor of one band-declaration ask.
const chooseBand chooseFor = 54

// bandAsk is the band declaration's resumable state: the banding attackers
// (leaders) in declaration order, the cursor of the leader being asked, and
// the answered declaration startExertAsks resumes with after the last ask.
type bandAsk struct {
	leaders  []state.ObjID
	next     int
	declared []decision.Option
}

// startBandAsks opens the band declaration for an answered attack
// declaration and reports whether an ask is now pending. Leaders are the
// declared attackers with banding, deduplicated, in option order.
func (e *Engine) startBandAsks(chosen []decision.Option) bool {
	var leaders []state.ObjID
	for _, opt := range chosen {
		if opt.Obj != 0 && !slices.Contains(leaders, opt.Obj) && e.hasKeywordH(opt.Obj, kwhBanding) {
			leaders = append(leaders, opt.Obj)
		}
	}
	if len(leaders) == 0 {
		return false
	}
	e.bandAskState = bandAsk{leaders: leaders, declared: chosen}
	return e.askNextBand()
}

// bandCandidates lists the attackers that may join leader's band now, in
// the declaration's option order: attacking the same defender as leader,
// under the same controller, in no band yet, and -- when leader's band
// already holds a creature without banding -- with banding themselves. A
// leader already taken into another leader's band has none.
func (e *Engine) bandCandidates(leader state.ObjID) []state.ObjID {
	lo := e.G.Obj(leader)
	if lo == nil || !lo.IsAttacking || lo.Zone != state.ZBattlefield || (lo.Band != 0 && lo.Band != leader) {
		return nil
	}
	plainTaken := false
	for _, m := range e.G.BandMates(leader) {
		if !e.hasKeywordH(m, kwhBanding) {
			plainTaken = true
		}
	}
	var out []state.ObjID
	for _, opt := range e.bandAskState.declared {
		id := opt.Obj
		o := e.G.Obj(id)
		if id == leader || o == nil || slices.Contains(out, id) || !o.IsAttacking || o.Zone != state.ZBattlefield ||
			o.Band != 0 || o.Controller != lo.Controller || o.Attacking != lo.Attacking || o.AttackingBattle != lo.AttackingBattle {
			continue
		}
		if plainTaken && !e.hasKeywordH(id, kwhBanding) {
			continue
		}
		out = append(out, id)
	}
	return out
}

// askNextBand poses the next band ask and returns true, or, when every
// leader is done, clears the state, resumes the exert election and returns
// false. One ask is one Min 1 / Max 1 KChoose for the current leader:
// option 0 declines ("no more creatures in this band"), each other option
// adds one candidate. An accepted candidate re-asks the same leader, so a
// band grows one creature per answer. Option 0 first is the exert shape: a
// bot that takes the first offer never bands.
func (e *Engine) askNextBand() bool {
	for e.bandAskState.next < len(e.bandAskState.leaders) {
		leader := e.bandAskState.leaders[e.bandAskState.next]
		cands := e.bandCandidates(leader)
		if len(cands) == 0 {
			e.bandAskState.next++
			continue
		}
		name := e.G.Obj(leader).Face().Name
		d := &decision.Decision{Player: e.G.Active, Kind: decision.KChoose, Min: 1, Max: 1,
			Prompt: "Band another attacking creature with " + name + "? (CR 702.22)",
			Source: leader}
		d.Options = append(d.Options, decision.Option{Index: 0, Kind: "band",
			Label: "No more creatures in " + name + "'s band", Player: e.G.Active})
		for _, id := range cands {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "band",
				Label: "Band " + e.G.Obj(id).Face().Name + " with " + name, Obj: id, Amount: 1, Player: e.G.Active})
		}
		e.choosing = chooseBand
		e.ask(d)
		return true
	}
	declared := e.bandAskState.declared
	e.bandAskState = bandAsk{}
	e.startExertAsks(declared)
	return false
}

// bandAnswer applies one band answer: an accepted candidate joins the
// leader's band through a Choose "band" event (Obj the leader, IDs the new
// member); a decline moves to the next leader.
func (e *Engine) bandAnswer(d *decision.Decision, in decision.Intent) {
	e.choosing = chooseNone
	chosen := d.Chosen(in)
	if len(chosen) == 1 && chosen[0].Amount == 1 && chosen[0].Obj != 0 {
		e.emit(events.Event{Kind: events.Choose, Obj: d.Source, Player: d.Player, Counter: "band",
			IDs: []state.ObjID{chosen[0].Obj}})
	} else {
		e.bandAskState.next++
	}
	e.askNextBand()
}

// bandBlockPairs is CR 702.22h: a blocker declared blocking a member of a
// band also blocks every other member. pairs are (attacker, blocker); each
// missing (mate, blocker) pair is appended after the declared ones, in
// declaration then battlefield order, with no duplicates.
func (e *Engine) bandBlockPairs(pairs [][2]state.ObjID) [][2]state.ObjID {
	n := len(pairs)
	for i := 0; i < n; i++ {
		for _, m := range e.G.BandMates(pairs[i][0]) {
			p := [2]state.ObjID{m, pairs[i][1]}
			if !slices.Contains(pairs, p) {
				pairs = append(pairs, p)
			}
		}
	}
	return pairs
}

// bandBlockedAttackers lists the live attackers blocker b is blocking, in
// the active player's battlefield order. In gorge a blocker blocks more than
// one attacker only through CR 702.22h (the block declaration admits one
// attacker per blocker), so callers ask only when an attacker is banded.
func (e *Engine) bandBlockedAttackers(b state.ObjID) []state.ObjID {
	var out []state.ObjID
	for _, aid := range e.G.Zone(state.ZBattlefield, e.G.Active) {
		a := e.G.Obj(aid)
		if a == nil || !a.IsAttacking || a.Band == 0 {
			continue
		}
		if slices.Contains(e.liveBlockers(a), b) {
			out = append(out, aid)
		}
	}
	return out
}

// isBandBlocker reports whether id is a creature blocking more than one
// attacker (a band) -- a division queue entry that divides among attackers.
func (e *Engine) isBandBlocker(id state.ObjID) bool {
	o := e.G.Obj(id)
	return o != nil && o.IsBlocking && !o.IsAttacking && len(e.bandBlockedAttackers(id)) > 1
}

// divisionRecipients is what queue entry id divides its combat damage
// among: a band blocker's blocked attackers, otherwise an attacker's live
// blockers.
func (e *Engine) divisionRecipients(id state.ObjID) []state.ObjID {
	if e.isBandBlocker(id) {
		return e.bandBlockedAttackers(id)
	}
	return e.liveBlockers(e.G.Obj(id))
}

// divisionChooser is who divides queue entry id's combat damage, and the
// prompt's noun. CR 510.1c: an attacker's controller (the active player),
// except CR 702.22j: an attacker blocked by a creature with banding is
// divided by the defending player. CR 510.1d: a blocker's controller,
// except CR 702.22k: a blocker blocking a creature with banding is divided
// by the active player (every multi-attacker blocker here blocks a band).
func (e *Engine) divisionChooser(id state.ObjID) (state.PlayerID, string) {
	if e.isBandBlocker(id) {
		for _, aid := range e.bandBlockedAttackers(id) {
			if e.hasKeywordH(aid, kwhBanding) {
				return e.G.Active, "the creatures it blocks"
			}
		}
		return e.G.Obj(id).Controller, "the creatures it blocks"
	}
	a := e.G.Obj(id)
	for _, bid := range e.liveBlockers(a) {
		if e.hasKeywordH(bid, kwhBanding) {
			return a.Attacking, "its blockers"
		}
	}
	return e.G.Active, "its blockers"
}

// appendBandBlockerDivisions appends to the pass's division queue every
// blocker acting this pass that blocks more than one live attacker, with
// power above zero and an enumerable division, after the attackers, in
// attacker-then-blocker order.
func (e *Engine) appendBandBlockerDivisions(out []state.ObjID, pass bool) []state.ObjID {
	for _, aid := range e.G.Zone(state.ZBattlefield, e.G.Active) {
		a := e.G.Obj(aid)
		if a == nil || !a.IsAttacking || a.Band == 0 {
			continue
		}
		for _, bid := range e.liveBlockers(a) {
			if slices.Contains(out, bid) || !e.actsThisDamageStep(bid, pass) || e.combatDamageAmount(bid) <= 0 {
				continue
			}
			if rs := e.bandBlockedAttackers(bid); len(rs) > 1 && e.divisionCount(rs, e.combatDamageAmount(bid)) <= maxDivisionOptions {
				out = append(out, bid)
			}
		}
	}
	return out
}

// bandBlockerAssignments adds the combat damage of every blocker that
// blocks more than one attacker (damageStep skipped them): the answered
// division when there is one, otherwise lethal damage to each blocked
// attacker in order with the rest to the last (CR 510.1d's deterministic
// default, the attacker path's shape).
func (e *Engine) bandBlockerAssignments(as []assignment, firstStrike bool) []assignment {
	var done []state.ObjID
	for _, aid := range e.G.Zone(state.ZBattlefield, e.G.Active) {
		a := e.G.Obj(aid)
		if a == nil || !a.IsAttacking || a.Band == 0 {
			continue
		}
		for _, bid := range e.liveBlockers(a) {
			rs := e.bandBlockedAttackers(bid)
			if len(rs) < 2 || slices.Contains(done, bid) || !e.actsThisDamageStep(bid, firstStrike) {
				continue
			}
			done = append(done, bid)
			bp := e.combatDamageAmount(bid)
			if bp <= 0 {
				continue
			}
			b := e.G.Obj(bid)
			dt := e.hasKeywordH(bid, kwhDeathtouch)
			mk := func(to state.ObjID, n int32) assignment {
				return assignment{toObj: to, amount: n, lifelink: b.Controller, hasLink: e.hasKeywordH(bid, kwhLifelink),
					deathtouch: dt, from: bid, infect: e.hasKeywordH(bid, kwhInfect), wither: e.hasKeywordH(bid, kwhWither)}
			}
			if div := e.chosenDivision(bid); div != nil {
				for i, to := range rs {
					if i < len(div) && div[i] > 0 {
						as = append(as, mk(to, div[i]))
					}
				}
				continue
			}
			remaining := bp
			for i, to := range rs {
				give := remaining
				if need := e.lethalDamageNeed(to, dt); i < len(rs)-1 && give > need {
					give = need
				}
				if give > 0 {
					as = append(as, mk(to, give))
					remaining -= give
				}
				if remaining <= 0 {
					break
				}
			}
		}
	}
	return as
}
