package search

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Options.RaidHold (:raidhold[=NAMES]) holds a raid creature's cast until
// after combat. A measured network-guided search on dirac cast Gorehorn Raider ({4}{R} 4/4, "Raid -- when this creature
// enters, if you attacked this turn, it deals 2 damage to any target") in
// its precombat main phase at 259 of 299 casts (86.6%); the raid fired on
// none of those and on 27 of the 40 postcombat casts, and in about 47% of
// all casts the same turn's attack would have enabled it. The prior is
// 0.60-0.65 on the cast against pass's 0.13-0.16, the bot's first move is
// the cast at 206 of 229 precombat decisions, and the line that earns the
// raid (pass, attack, blocks, the postcombat cast, the trigger's target) is
// at least five plies deep at 300 simulations, so where Q is near flat the
// search plays the prior's cast. The rule takes the precombat cast away
// while an attack is possible, so the cast comes after combat, where the
// search values it as ever; it is an arm for 040's decision metrics (raid
// per cast, casts per game) and for training data that holds raid
// positions, not a claim that the postcombat cast is always right (a
// power-4 combat trigger can reward the precombat cast; 23 of the 259).
//
// The rule:
//
//   - The window (raidOn): a priority decision of the active player in its
//     own precombat main phase (StepMain1) with an empty stack while it
//     controls a creature that could attack now (dirac State.HasAttacker,
//     the attackers decision's own canAttack), the searching seat's own or,
//     under OpponentNodes with OpponentMirror, the opponent's (as
//     HoldUpkeep holds it). The postcombat main phase, a decision with
//     something on the stack, a turn with no possible attacker and the
//     other player's turn are unaffected.
//   - The held casts (raidCast): every AkCast (kicked and alternative modes
//     too) of a raid card. With RaidHoldOnly empty that is a raid-trigger
//     card (RaidTrigger: an ETB-of-self triggered ability under an
//     intervening raid condition: an "enters-the-battlefield raid
//     trigger"); RaidHoldOnly instead names the cards held, each a
//     RaidCard -- a raid trigger or an as-this-enters replacement under a
//     raid condition (FDN's Goblin Boarders, "enters with a +1/+1
//     counter", is held only when named: its raid is no trigger, and the
//     default arm moves only the trigger cards). In the window they leave
//     the priority candidates before the cap (priorityCands), as
//     HoldUpkeep's held casts do; CardSimsForce never forces one back.
//   - The bot's answer: a bot answer that is a held cast is played as pass
//     (raidBot) -- substituted before the decision's point is built, so
//     pass is candidate 0 and the rest of the window's vocabulary (the
//     other casts, the activations) is still searched. Here the rule
//     departs from HoldUpkeep's, which leaves such a decision unsearched:
//     in the upkeep the casts it holds are the vocabulary's bulk, but in
//     the first main phase the other casts are the turn's development (the
//     bot's first move is the Raider because it is the costliest castable
//     spell), and an unsearched pass there would also skip every other
//     precombat play whenever the bot liked the raid card, a second change
//     the arm would measure with the first. A decision left with one
//     candidate (pass) is played unsearched, as any is. Applied at the
//     root (every path that plays a bot answer, Sims 0 too) and at every
//     in-walk decision the window opens at, point or not.
//   - The prior at such a point: the network is asked with the bot's mark
//     on its own answer, the held cast (Searcher.raidMark, buildPoint), as
//     it is without the rule -- not on the pass standing in, which the
//     bot never chose: the mark is a network input (policynet's
//     Option.BotPick residual, and the list position of the planner-paid
//     casts), and on pass it raised pass's prior over the other plays (the
//     review of 54a3d40, which marked pass).
//     The held point's prior is the unheld one with the held casts removed
//     and renormalised (TestRaidHoldPrior). Result.RaidBot names the held
//     answer at the root, and a host's corpus record encodes its options
//     with it as the bot's answer and marks no candidate the bot's (none
//     is), where an unheld record marks candidate 0; candidate 0, pass,
//     is still the record's first candidate, so a bot-label reading of the
//     record (policynet VisitLabelBot) learns pass there.
//   - Counters: Stats.RaidHeld (1 when the root's window held a cast),
//     RaidHeldBot (1 when the root's bot answer was one, pass played or
//     searched as candidate 0 in its place), RaidHeldWalk and RaidHeldOpp
//     (in-walk decisions of the seat and of a mirrored opponent at which a
//     held cast was offered and the rule changed the answer or the point;
//     walk work, as HeldUpkeepWalk).
//
// Off (the default) none of this runs and the search is the old one.

// RaidTrigger reports whether card c of table t has an enters raid
// trigger: a triggered ability on its own entering (TrigETB with TfSelf)
// whose intervening condition is a raid test (raidCond) -- RaidHold's
// default set. The FDN pool's (TestRaidCardsFDN): Gorehorn Raider,
// Gutless Plunderer, Skyship Buccaneer and Strongbox Raider.
func RaidTrigger(t *kop.Table, c kop.CardID) bool { return raidEnters(t, c, false) }

// RaidCard reports whether card c of table t enters with a raid check: a
// raid trigger (RaidTrigger) or an as-this-enters replacement
// (ReplEntersProgram, ReplEntersWithCounters, ReplEntersTapped) whose
// condition is a raid test -- the cards RaidHoldOnly may name. The FDN
// pool's: RaidTrigger's four and Goblin Boarders (enters with a +1/+1
// counter); its other raid cards -- Alesha, Who Laughs at Fate, Midnight
// Snack, Perforating Artist, Searslicer Goblin -- test the raid at the end
// step, which a postcombat cast does not change.
func RaidCard(t *kop.Table, c kop.CardID) bool { return raidEnters(t, c, true) }

// raidEnters is RaidTrigger, or with repl RaidCard.
func raidEnters(t *kop.Table, c kop.CardID, repl bool) bool {
	if int(c) >= len(t.Cards) {
		return false
	}
	cd := &t.Cards[c]
	for i := range cd.Abilities {
		ab := &cd.Abilities[i]
		switch ab.Kind {
		case kop.AbTriggered:
			if ab.Trig.Event == kop.TrigETB && ab.Trig.Flags&kop.TfSelf != 0 && raidCond(t, ab.Trig.Cond) {
				return true
			}
		case kop.AbReplacement:
			if !repl {
				continue
			}
			switch ab.Repl.Kind {
			case kop.ReplEntersProgram, kop.ReplEntersWithCounters, kop.ReplEntersTapped:
				if raidCond(t, ab.Repl.Cond) {
					return true
				}
			}
		}
	}
	return false
}

// raidCond reports whether condition id of t is a raid test: "you attacked
// this turn", gorge's Count$AttackersDeclared >= 1 as cmd/kgen writes it --
// a CondValue, not negated, comparing an ExAttackedThisTurn amount (the
// creatures its controller attacked with this turn) at least 1 (GE a
// constant >= 1, or GT one >= 0).
func raidCond(t *kop.Table, id kop.CondID) bool {
	if id == 0 || int(id) >= len(t.Conds) {
		return false
	}
	c := &t.Conds[id]
	if c.Kind != kop.CondValue || c.Neg || c.A.K != kop.VExpr || c.B.K != kop.VConst ||
		int(c.A.N) <= 0 || int(c.A.N) >= len(t.Exprs) || t.Exprs[c.A.N].Kind != kop.ExAttackedThisTurn {
		return false
	}
	switch c.Op {
	case kop.CmpGE:
		return c.B.N >= 1
	case kop.CmpGT:
		return c.B.N >= 0
	}
	return false
}

// raidOn reports whether Options.RaidHold's window is open at decision d
// of s for this searcher (the caller checks RaidHold): a priority decision
// of the active player in its precombat main phase with an empty stack
// (raidStep) while that player controls a creature that could attack.
func (sr *Searcher) raidOn(s *dirac.State, d *dirac.Decision) bool {
	return sr.raidStep(s, d) && s.HasAttacker()
}

// raidStep is raidOn's window without the attacker test (field reads
// only): a priority decision of the active player in its precombat main
// phase with an empty stack, the searching seat's own or, under
// OpponentNodes with OpponentMirror, the opponent's.
func (sr *Searcher) raidStep(s *dirac.State, d *dirac.Decision) bool {
	if d.Kind != dirac.DkPriority || d.Player != s.ActivePlayer() || s.StackLen() != 0 || s.StepNow() != kop.StepMain1 {
		return false
	}
	return d.Player == sr.seat || sr.Opts.OpponentNodes && sr.Opts.OpponentMirror
}

// raidCast reports whether option a is a held raid cast: an AkCast of a
// raid card (RaidCard) that RaidHoldOnly names (by table name), or of a
// raid-trigger card (RaidTrigger) when the list is empty.
func (sr *Searcher) raidCast(s *dirac.State, a dirac.Action) bool {
	if a.Kind() != dirac.AkCast {
		return false
	}
	t := s.Table()
	c := s.ObjCard(dirac.Obj(a.Source()))
	only := &sr.Opts.RaidHoldOnly
	if only[0] == "" {
		return RaidTrigger(t, c)
	}
	if !RaidCard(t, c) {
		return false
	}
	name := t.Cards[c].Name
	for _, n := range only {
		if n == "" {
			break
		}
		if n == name {
			return true
		}
	}
	return false
}

// Results of raidBot.
const (
	raidNone = iota // the window is shut or offers no held cast
	raidHeld        // a held cast is offered; the bot's answer stands
	raidPass        // the bot's answer was a held cast; pass stands in
)

// raidBot is Options.RaidHold at decision d of s with the bot's answer bot
// (the caller checks RaidHold), before the decision's point is built: it
// returns the answer to use -- d's pass in place of a held cast -- and
// what the rule did (raidNone, raidHeld, raidPass). A decision whose
// window offers no held cast is untouched (raidNone).
func (sr *Searcher) raidBot(s *dirac.State, d *dirac.Decision, bot int) (int, int) {
	if !sr.raidStep(s, d) {
		return bot, raidNone
	}
	held := false
	for i := 0; i < int(d.N) && !held; i++ {
		held = sr.raidCast(s, d.Options[i])
	}
	if !held || !s.HasAttacker() {
		return bot, raidNone
	}
	if !sr.raidCast(s, d.Options[bot]) {
		return bot, raidHeld
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkPass {
			return i, raidPass
		}
	}
	return bot, raidHeld // no pass offered (never at priority): priorityCands leaves it unsearched
}

// raidWalk counts an in-walk decision d the rule touched (raidBot's what,
// not raidNone) in RaidHeldWalk (the searching seat's) or RaidHeldOpp: one
// whose bot answer it replaced, or a point it dropped a held cast from.
func (sr *Searcher) raidWalk(d *dirac.Decision, what int, point bool) {
	if what == raidPass || point {
		if d.Player == sr.seat {
			sr.St.RaidHeldWalk++
		} else {
			sr.St.RaidHeldOpp++
		}
	}
}
