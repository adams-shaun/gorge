package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// vote lowers will of the council (CR 701.38a). Two shapes:
//
//   - the card ballot (gorge effVote's VoteCard$ shape, effCardVote) as
//     OpExt ExtVote. Every player votes (Defined$ Player, gorge
//     definedPlayers from the controller), each for one battlefield
//     permanent matching VoteCard$, and the kernel remembers the
//     most-voted, every member of a tie. The one VoteSubAbility$ that lowers
//     is the exile of that remembered set from the battlefield (Council's
//     Judgment), folded into the op as C bit 0: gorge moves the set as one
//     simultaneous departure batch, which the kernel's single-object Exile
//     over RefRemembered is not.
//
//   - the fixed list (gorge effVote's Choices$ shape, askFixedVote and
//     resolveVoteOutcomes): Choices$ names 2..4 SVar branches, each voter
//     picks one index, and the winning branch runs. A tie runs
//     VoteTiedAbility$ when present, else the first tied option. The op's C
//     carries the kernel's fixed-list mode bit, the choice count and the
//     tie-branch bit; the branches follow as OpBranch blocks (the
//     GenericChoice shape).
//
// VoteMessage$ is the prompt and AILogic$ a Forge AI hint; neither changes
// the rules. Player ballots (VotePlayer$), the tally publishers
// (StoreVoteNum$, RememberVotedObjects$), secret and up-to votes, and Zone$
// refuse.
func (lc *lowerCtx) vote(sa *cards.SA, ps *params) error {
	if ps.str("VoteCard") != "" {
		return lc.cardVote(ps)
	}
	if _, ok := ps.get("Choices"); ok {
		return lc.fixedVote(ps)
	}
	return lc.cardVote(ps) // no ballot: the card shape's named refusal
}

// cardVote is vote's card-ballot shape (the pre-existing lowering).
func (lc *lowerCtx) cardVote(ps *params) error {
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

// fixedVote is vote's Choices$ shape: the choice branches and the optional
// tie branch lower as OpBranch blocks after the ExtVote op, whose C carries
// the fixed-list mode (bit 1), the option count (bits 2..4) and the tie
// branch bit (bit 5), mirroring dirac/act_vote.go.
func (lc *lowerCtx) fixedVote(ps *params) error {
	for _, k := range sortedParamKeys(ps.m) {
		switch {
		case strings.HasPrefix(k, "Condition"):
		case k == "Defined", k == "Choices", k == "VoteTiedAbility", k == "VoteMessage", k == "AILogic",
			k == "EachVote", k == "StackDescription", k == "SpellDescription", k == "SubAbility",
			k == "ValidTgts", k == "TgtZone", k == "TargetType",
			k == "TargetMin", k == "TargetMax", k == "TargetUnique",
			k == "TargetsWithSameController", k == "TargetsWithDifferentControllers", k == "TargetsWithDefinedController":
		default:
			return refuse("Vote param %q", k)
		}
	}
	if d := ps.str("Defined"); d != "Player" {
		return refuse("Vote Defined$ %q", d)
	}
	names := strings.Split(ps.str("Choices"), ",")
	if len(names) < 2 || len(names) > 4 {
		return refuse("Vote Choices$ %d options", len(names))
	}
	for i, name := range names {
		names[i] = strings.TrimSpace(name)
		if cards.ResolveSVar(lc.face.SVars, names[i]) == nil {
			return refuse("Vote Choices$ %q", name)
		}
	}
	tie := ps.str("VoteTiedAbility")
	if tie != "" && cards.ResolveSVar(lc.face.SVars, tie) == nil {
		return refuse("Vote VoteTiedAbility$ %q", tie)
	}
	if _, ok := ps.get("ValidTgts"); ok {
		if _, err := lc.objOrTarget(ps, "", zBattlefield); err != nil {
			return err
		}
	}
	ps.consume("VoteMessage", "AILogic", "EachVote", "TargetType")
	if lc.ab.Kind == akTriggered {
		return lc.triggeredFixedVote(names, tie)
	}
	return lc.emitFixedVote(names, tie)
}

// triggeredFixedVote lowers a fixed-list Vote on a triggered ability. Gorge
// treats a triggered Vote's Choices$ as modal at trigger placement
// (charm_params.go isModalSA) and asks the controller one mode before the
// trigger goes on the stack; the chosen mode is not read by effVote, which
// still asks every voter. The kernel asks the same placement mode only when
// the ability carries modes, so the vote (and its branches) is repeated once
// per mode behind an OpCharm block: the chosen mode runs its copy, the
// others are skipped, and every copy resolves the same vote.
func (lc *lowerCtx) triggeredFixedVote(names []string, tie string) error {
	if lc.ab.Modes != 0 {
		return refuse("Vote on an ability that is already modal")
	}
	lc.ab.Modes, lc.ab.ModesPick, lc.ab.ModesMin = len(names), 1, 1
	for i := range names {
		at := len(lc.ab.Prog)
		lc.emit(lInstr{Op: "Charm", C: i})
		if err := lc.emitFixedVote(names, tie); err != nil {
			return err
		}
		lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	}
	return nil
}

// emitFixedVote emits the ExtVote op and its winning/tie Branch blocks.
func (lc *lowerCtx) emitFixedVote(names []string, tie string) error {
	c := 1<<1 | len(names)<<2
	if tie != "" {
		c |= 1 << 5
	}
	lc.emit(lInstr{Op: "Vote", C: c, Note: "each player votes; the winning choice runs"})
	for i, name := range names {
		if err := lc.voteBranch(i, cards.ResolveSVar(lc.face.SVars, name)); err != nil {
			return err
		}
	}
	if tie != "" {
		if err := lc.voteBranch(len(names), cards.ResolveSVar(lc.face.SVars, tie)); err != nil {
			return err
		}
	}
	return nil
}

// voteBranch lowers one fixed-list branch as a Branch block: the kernel runs
// C branch when the vote selected it (dirac's OpBranch).
func (lc *lowerCtx) voteBranch(branch int, sa *cards.SA) error {
	bat := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "Branch", C: branch})
	if err := lc.chain(sa, nil); err != nil {
		return err
	}
	lc.ab.Prog[bat].Skip = len(lc.ab.Prog) - bat - 1
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
