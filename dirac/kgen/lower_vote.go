package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// vote lowers will of the council over a card ballot (gorge effVote's
// VoteCard$ shape, effCardVote; CR 701.38a) as OpExt ExtVote. Every player
// votes (Defined$ Player, gorge definedPlayers from the controller), each for
// one battlefield permanent matching VoteCard$, and the kernel remembers the
// most-voted, every member of a tie. The one VoteSubAbility$ that lowers is
// the exile of that remembered set from the battlefield (Council's
// Judgment), folded into the op as C bit 0: gorge moves the set as one
// simultaneous departure batch, which the kernel's single-object Exile over
// RefRemembered is not. VoteMessage$ is the prompt and AILogic$ a Forge AI
// hint; neither changes the rules. The fixed-list and player ballots
// (Choices$, VotePlayer$), the tally publishers (StoreVoteNum$,
// RememberVotedObjects$), secret and up-to votes, a tie ability and any
// other sub-ability refuse.
func (lc *lowerCtx) vote(sa *cards.SA, ps *params) error {
	for _, k := range sortedParamKeys(ps.m) {
		switch {
		case strings.HasPrefix(k, "Condition"):
		case k == "Defined", k == "VoteCard", k == "VoteSubAbility", k == "VoteMessage", k == "AILogic",
			k == "StackDescription", k == "SpellDescription", k == "SubAbility":
		default:
			return refuse("Vote param %q", k)
		}
	}
	if d := ps.str("Defined"); d != "Player" {
		return refuse("Vote Defined$ %q", d)
	}
	ballot := ps.str("VoteCard")
	if ballot == "" {
		return refuse("Vote without VoteCard$")
	}
	f, err := lc.filter(ballot)
	if err != nil {
		return err
	}
	if f.Players != 0 || len(f.Clauses) == 0 || f.Each {
		return refuse("Vote VoteCard$ %q: not a permanent ballot", ballot)
	}
	name := ps.str("VoteSubAbility")
	sub := cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(name))
	if sub == nil {
		return refuse("Vote VoteSubAbility$ %q", name)
	}
	if !voteExilesRemembered(sub) {
		return refuse("Vote VoteSubAbility$ %q: not the exile of the most-voted", name)
	}
	ps.consume("VoteMessage", "AILogic")
	lc.emit(lInstr{Op: "Vote", F: f, C: 1, Note: "each player votes; the most-voted are exiled"})
	return nil
}

// voteExilesRemembered reports whether sub is exactly the move of the
// remembered set from the battlefield to exile, with nothing chained.
func voteExilesRemembered(sub *cards.SA) bool {
	if sub.API != "ChangeZone" || sub.Sub != nil {
		return false
	}
	want := map[string]string{"Defined": "Remembered", "Origin": "Battlefield", "Destination": "Exile"}
	for k, v := range sub.Params {
		if k == "StackDescription" || k == "SpellDescription" {
			continue
		}
		if w, ok := want[k]; !ok || strings.TrimSpace(v) != w {
			return false
		}
	}
	for k := range want {
		if _, ok := sub.Params[k]; !ok {
			return false
		}
	}
	return true
}
