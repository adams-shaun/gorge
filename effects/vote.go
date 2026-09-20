package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The canonical vote-finished carrier (trig:Vote). api:Vote's two shapes
// (the fixed-list effVote and the card-ballot effCardVote) already recorded
// one Note per voting player; this file adds the ONE canonical "players
// finished voting" Note a Mode$ Vote trigger fires on, the same shape
// api:FlipCoin's FlipCoinNote/FlipNoteResult pair serves for
// trig:FlippedCoin (rules/trigger_match.go's flippedCoinMatches).
//
// Encoding (no events.Event schema change): Player is the vote's caster, Obj
// the resolving vote spell, and the two List$ referent sets ride the event's
// spare slots as player refs -- IDs carries the SAME-set opponents
// (state.PlayerRef-encoded, "voted for a choice the caster voted for") and
// Pairs the DIFF-set ones ("voted for a different choice", both pair slots
// carrying the same player ref so a pair is a self-contained player id).
// events.Apply folds Note as an inert marker, so neither slot is ever
// re-interpreted; the log keeps both, and a replay re-derives the trigger's
// referents from the same event bytes -- the same replay discipline
// FlipCoinNote's Amount encoding uses.
const VoteNotePrefix = "vote finished"

// VoteFinishedNote builds the canonical vote-finished event. same/diff are
// the caster's opponents split by how they voted, in voter order.
func VoteFinishedNote(caster state.PlayerID, source state.ObjID, same, diff []state.PlayerID) events.Event {
	ev := events.Event{Kind: events.Note, Player: caster, Obj: source, Text: VoteNotePrefix}
	for _, p := range same {
		ev.IDs = append(ev.IDs, state.PlayerRef(p))
	}
	for _, p := range diff {
		ev.Pairs = append(ev.Pairs, [2]state.ObjID{state.PlayerRef(p), state.PlayerRef(p)})
	}
	return ev
}

// VoteFinishedResult decodes the canonical vote-finished Note: the caster
// (ev.Player), the two List$ referent sets and whether ev is one. Any other
// Note -- in particular the per-voter "votes for ..." notes, which carry no
// IDs/Pairs -- is not one.
func VoteFinishedResult(ev events.Event) (caster state.PlayerID, same, diff []state.PlayerID, ok bool) {
	if ev.Kind != events.Note || ev.Text != VoteNotePrefix {
		return 0, nil, nil, false
	}
	for _, id := range ev.IDs {
		if p, ok := id.PlayerRef(); ok {
			same = append(same, p)
		}
	}
	for _, pr := range ev.Pairs {
		if p, ok := pr[0].PlayerRef(); ok {
			diff = append(diff, p)
		}
	}
	return ev.Player, same, diff, true
}

// voteSameDiff splits the voting opponents of the vote's caster into the two
// List$ sets, in voter order. picks[i] is the option index voter i chose
// (-1: an out-of-range answer, i.e. a vote for nothing).
//
// noChoices is the empty-ballot case: the vote offered no option at all (a
// fixed-list Vote with no Choices$, or a card ballot whose VoteCard$ matched
// no permanent). Then NO voter cast a real vote, so neither set is bound --
// no one voted for a choice the caster voted for, and no one voted for a
// different one, because there was no choice. Without this, every voter's
// pick is -1 and the casterPick != -1 guard below would route every voting
// opponent into the DIFF set, making Grudge Keeper drain life from every
// opponent and Erestor scry the whole table after a vote in which nobody
// voted for anything.
//
// Otherwise the caster's own pick anchors "a choice you voted for"; a caster
// who is not among the voters (or voted for nothing while others voted)
// anchors nothing, so the same set is empty and every voting opponent lands
// in the diff set -- "a choice you didn't vote for" covers every choice when
// you made none.
func voteSameDiff(g *state.Game, caster state.PlayerID, voters []state.Target, picks []int, noChoices bool) (same, diff []state.PlayerID) {
	if noChoices {
		return nil, nil
	}
	casterPick := -1
	for i, t := range voters {
		if t.IsPlayer && t.Player == caster && i < len(picks) {
			casterPick = picks[i]
			break
		}
	}
	for i, t := range voters {
		if !t.IsPlayer || t.Player == caster || i >= len(picks) {
			continue
		}
		if casterPick != -1 && picks[i] == casterPick {
			same = append(same, t.Player)
		} else {
			diff = append(diff, t.Player)
		}
	}
	return same, diff
}

// battlefieldHasVoteTrigger is the carrier's emit gate: the canonical
// vote-finished Note is proposed only when some battlefield permanent's face
// carries a Mode$ Vote trigger. A vote nobody can observe finished is
// indistinguishable from a canonical Note nobody matches -- and emitting it
// unconditionally would append a new event to every existing recorded game
// that resolves a vote (the golden acceptance games resolve Council's
// Judgment votes), moving every chain head. The gate is deterministic board
// state, so replay takes the same branch and no recorded game changes.
//
// The scan is BATTLEFIELD-only, matching where the engine's ordinary trigger
// scan looks for a permanent's face: a Mode$ Vote trigger carried by a card
// in another zone would not be found, so its carrier would stay silent. No
// corpus Vote carrier declares a non-battlefield TriggerZones$ today; if one
// ever does, this scan has to widen with it.
func battlefieldHasVoteTrigger(g *state.Game) bool {
	if g == nil {
		return false
	}
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			for _, tg := range o.Face().Triggers {
				if strings.EqualFold(tg.Mode, "Vote") {
					return true
				}
			}
		}
	}
	return false
}

// emitVoteFinished proposes the canonical vote-finished Note for one
// resolution's same/diff sets, behind the battlefieldHasVoteTrigger gate.
func emitVoteFinished(h Host, c *Ctx, same, diff []state.PlayerID) {
	if !battlefieldHasVoteTrigger(h.Game()) {
		return
	}
	h.Emit(VoteFinishedNote(c.Controller, c.Source, same, diff))
}
