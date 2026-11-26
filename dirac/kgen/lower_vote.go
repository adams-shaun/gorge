package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
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
// A fixed-list Vote on a triggered ability lowers like a spell's: gorge
// resolves a triggered Vote at the trigger's own resolution (effVote asks each
// voter; askTriggerModes returns false for API Vote — a placement mode would
// pose the ballot to the trigger controller and record a ChosenModes effVote
// never reads), so the op sits in the trigger's program with no mode election
// around it.
//
// VoteMessage$ is the prompt and AILogic$ a Forge AI hint; neither changes
// the rules. Secretly$ on the fixed list changes nothing the kernel tracks. StoreVoteNum$ True makes every option body resolve with VoteNum
// bound to that option's own tally (gorge resolveVoteOutcomes); the option
// branches lower with the tally value kind and the kernel runs them all in
// Choices$ order. A read of VoteNum outside an option branch refuses by name.
// Zone$ Graveyard (Custodi Squire) makes the card ballot graveyard cards
// (C bit 2) whose VoteSubAbility$ returns the most-voted to hand. Player
// ballots (VotePlayer$), RememberVotedObjects$, secret and up-to votes, and
// any other Zone$ refuse.
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
		case k == "Zone":
		default:
			return refuse("Vote param %q", k)
		}
	}
	grave := false
	switch z := ps.str("Zone"); z {
	case "", "Battlefield":
	case "Graveyard":
		grave = true
	default:
		return refuse("Vote Zone$ %q", z)
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
	if grave {
		// Custodi Squire: the ballot is graveyard cards (C bit 2) and the
		// one VoteSubAbility$ that lowers returns the remembered set from
		// the graveyard to hand, folded into the op.
		if !voteMovesRemembered(sub, "Graveyard", "Hand") {
			return refuse("Vote VoteSubAbility$ %q: not the return of the most-voted to hand", name)
		}
		ps.consume("VoteMessage", "AILogic", "Zone")
		lc.emit(lInstr{Op: "Vote", F: f, C: 1 << 2, Note: "each player votes for a graveyard card; the most-voted return to hand"})
		return nil
	}
	ps.consume("Zone")
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
			k == "StoreVoteNum", k == "Secretly",
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
	store := false
	if v := ps.str("StoreVoteNum"); v != "" {
		switch {
		case strings.EqualFold(v, "True"):
			store = true
		case strings.EqualFold(v, "False"):
		default:
			return refuse("Vote StoreVoteNum$ %q", v)
		}
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
	// Secretly$ True hides the ballots until every vote is in: the votes are
	// asked in the same order and tallied the same (gorge effVote keeps the
	// ask and the tally, withholding only the picks from the public log).
	ps.consume("VoteMessage", "AILogic", "EachVote", "TargetType", "StoreVoteNum", "Secretly")
	if store && tie != "" {
		// gorge's StoreVoteNum branch returns before the tie branch, so a
		// VoteTiedAbility$ beside StoreVoteNum$ never resolves; do not hide
		// a shape the corpus does not carry.
		return refuse("Vote StoreVoteNum$ beside VoteTiedAbility$")
	}
	return lc.emitFixedVote(names, tie, store)
}

// emitFixedVote emits the ExtVote op and its branch blocks.
func (lc *lowerCtx) emitFixedVote(names []string, tie string, store bool) error {
	c := 1<<1 | len(names)<<2
	if store {
		c |= kop.VoteStore
	} else if tie != "" {
		c |= 1 << 5
	}
	lc.emit(lInstr{Op: "Vote", C: c, Note: "each player votes; the winning choice runs"})
	for i, name := range names {
		if err := lc.voteBranch(i, cards.ResolveSVar(lc.face.SVars, name), store); err != nil {
			return err
		}
	}
	if tie != "" && !store {
		if err := lc.voteBranch(len(names), cards.ResolveSVar(lc.face.SVars, tie), false); err != nil {
			return err
		}
	}
	return nil
}

// voteTallyName is the SVar Forge rebinds at each StoreVoteNum$ option:
// resolveVoteOutcomes writes "Number$<count>" into a private SVar copy.
const voteTallyName = "VoteNum"

// voteTallyValue lowers a VoteNum read inside a StoreVoteNum$ option branch to
// the current branch's tally. Anywhere else it refuses by name: the value is
// defined only while an option branch resolves (gorge binds it per option).
func (lc *lowerCtx) voteTallyValue() (lValue, error) {
	if !lc.voteTally {
		return lValue{}, refuse("%s outside a vote option branch", voteTallyName)
	}
	return lValue{Kind: vVoteTally}, nil
}

// voteBranch lowers one fixed-list branch as a Branch block: the kernel runs
// C branch when the vote selected it (dirac's OpBranch). In StoreVoteNum$
// mode the branch is flagged so the kernel runs every option in order, and
// VoteNum reads resolve to the branch's tally while the body lowers.
func (lc *lowerCtx) voteBranch(branch int, sa *cards.SA, store bool) error {
	bat := len(lc.ab.Prog)
	c := branch
	if store {
		c |= kop.VoteBranchStore
	}
	lc.emit(lInstr{Op: "Branch", C: c})
	prev := lc.voteTally
	lc.voteTally = store
	err := lc.chain(sa, nil)
	lc.voteTally = prev
	if err != nil {
		return err
	}
	lc.ab.Prog[bat].Skip = len(lc.ab.Prog) - bat - 1
	return nil
}

// voteExilesRemembered reports whether sub is exactly the move of the
// remembered set from the battlefield to exile, with nothing chained.
func voteExilesRemembered(sub *cards.SA) bool {
	return voteMovesRemembered(sub, "Battlefield", "Exile")
}

// voteMovesRemembered reports whether sub is exactly the move of the
// remembered set from origin to dest, with nothing chained.
func voteMovesRemembered(sub *cards.SA, origin, dest string) bool {
	if sub.API != "ChangeZone" || sub.Sub != nil {
		return false
	}
	want := map[string]string{"Defined": "Remembered", "Origin": origin, "Destination": dest}
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
