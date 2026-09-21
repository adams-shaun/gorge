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
// Encoding (no events.Event schema change). The carrier must carry the RAW
// BALLOTS, not a pre-split: the two List$ referent sets are relative to the
// TRIGGER SOURCE'S CONTROLLER, which is unknowable when the vote resolves
// (the vote's caster and the carrier permanent's controller are frequently
// different seats in multiplayer). So Player is the vote's caster (for the
// transcript), Obj the resolving vote spell, and Pairs one entry per voter:
// [PlayerRef(voter), ObjID(pick+1)], pick -1 ("voted for nothing") encoded as
// ObjID(0) so a self-contained pair never collides with a valid pick. A
// non-zero Amount marks that a real ballot existed (Choices$/VoteCard$
// admitted at least one option); an empty-ballot vote carries Amount 0 and
// binds neither set. events.Apply folds Note as an inert marker, so the pairs
// are never re-interpreted; the log keeps them, and a replay re-derives the
// trigger's referents from the same event bytes.
const VoteNotePrefix = "vote finished"

// VoteBallot is one voter's answered ballot: the voting player and the option
// index they chose (-1 when they voted for nothing -- an out-of-range answer
// or an empty ballot).
type VoteBallot struct {
	Player state.PlayerID
	Pick   int
}

// VoteFinishedNote builds the canonical vote-finished event. caster is the
// vote's caster (transcript/log only); ballots is every voter's raw answer in
// voter order; ballotExisted reports whether the vote offered any option at
// all (false for an empty ballot, which binds neither referent set).
func VoteFinishedNote(caster state.PlayerID, source state.ObjID, ballots []VoteBallot, ballotExisted bool) events.Event {
	ev := events.Event{Kind: events.Note, Player: caster, Obj: source, Text: VoteNotePrefix}
	if ballotExisted {
		ev.Amount = 1
	}
	for _, b := range ballots {
		ev.Pairs = append(ev.Pairs, [2]state.ObjID{state.PlayerRef(b.Player), state.ObjID(b.Pick + 1)})
	}
	return ev
}

// VoteFinishedResult decodes the canonical vote-finished Note: the caster
// (ev.Player), the raw ballots in voter order, whether a real ballot existed,
// and whether ev is one. Any other Note -- in particular the per-voter
// "votes for ..." notes, which carry no Pairs -- is not one.
func VoteFinishedResult(ev events.Event) (caster state.PlayerID, ballots []VoteBallot, ballotExisted bool, ok bool) {
	if ev.Kind != events.Note || ev.Text != VoteNotePrefix {
		return 0, nil, false, false
	}
	for _, pr := range ev.Pairs {
		p, isPlayer := pr[0].PlayerRef()
		if !isPlayer {
			continue
		}
		ballots = append(ballots, VoteBallot{Player: p, Pick: int(pr[1]) - 1})
	}
	return ev.Player, ballots, ev.Amount != 0, true
}

// VoteSplit splits the voting players who are not the ANCHOR into the two
// List$ sets, in voter order: the same set holds those who voted for a choice
// the anchor also voted for, the diff set everyone else. The anchor is the
// controller of the permanent whose Mode$ Vote trigger is being bound, NOT
// the vote's caster -- "a choice you voted for" means the carrier
// controller's own ballot, so the rules-side capture re-splits the raw
// ballots against e.controllerOf(source).
//
// ballotExisted is the empty-ballot case: the vote offered no option at all
// (a fixed-list Vote with no Choices$, or a card ballot whose VoteCard$
// matched no permanent). Then NO voter cast a real vote, so neither set is
// bound -- no one voted for a choice the anchor voted for, and no one voted
// for a different one, because there was no choice. Without this, every
// pick is -1 and the anchorPick != -1 guard below would route every voting
// opponent into the DIFF set, making Grudge Keeper drain life from every
// opponent and Erestor scry the whole table after a vote in which nobody
// voted for anything.
//
// Otherwise the anchor's own pick anchors "a choice you voted for"; an anchor
// who is not among the voters (or voted for nothing while others voted)
// anchors nothing, so the same set is empty and every voting opponent lands
// in the diff set -- "a choice you didn't vote for" covers every choice when
// you made none.
func VoteSplit(anchor state.PlayerID, ballots []VoteBallot, ballotExisted bool) (same, diff []state.PlayerID) {
	if !ballotExisted || len(ballots) == 0 {
		return nil, nil
	}
	anchorPick := -1
	for _, b := range ballots {
		if b.Player == anchor {
			anchorPick = b.Pick
			break
		}
	}
	for _, b := range ballots {
		if b.Player == anchor {
			continue
		}
		if anchorPick != -1 && b.Pick == anchorPick {
			same = append(same, b.Player)
		} else {
			diff = append(diff, b.Player)
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
// resolution's raw ballots, behind the battlefieldHasVoteTrigger gate.
func emitVoteFinished(h Host, c *Ctx, ballots []VoteBallot, ballotExisted bool) {
	if !battlefieldHasVoteTrigger(h.Game()) {
		return
	}
	h.Emit(VoteFinishedNote(c.Controller, c.Source, ballots, ballotExisted))
}
