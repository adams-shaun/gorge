package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// crookedScalesScript is an inline fixture with the gate shape of the card's
// ability chain (Forge card scripts, GPL, live only in gitignored .cards/):
// two targets, then a Repeat gated on the imprint pile, whose coin-flip subs
// clear the imprint when the process ends.
const crookedScalesScript = `Name:Crooked Scales
ManaCost:4
Types:Artifact
A:AB$ TargetOnly | Cost$ 4 T | ValidTgts$ Creature.OppCtrl | RememberObjects$ ThisTargetedCard | SubAbility$ PickYours
SVar:PickYours:DB$ TargetOnly | ImprintCards$ ThisTargetedCard | ValidTgts$ Creature.YouCtrl | SubAbility$ Loop
SVar:Loop:DB$ Repeat | RepeatPresent$ Card | RepeatDefined$ Imprinted | RepeatSVarCompare$ EQ0 | RepeatSubAbility$ Flip
SVar:Flip:DB$ FlipCoin | LoseSubAbility$ KillYours | WinSubAbility$ KillOpp
SVar:KillOpp:DB$ Destroy | SubAbility$ Wipe | Defined$ Remembered
SVar:KillYours:DB$ Destroy | UnlessPayer$ You | Defined$ Imprinted | UnlessCost$ 3 | UnlessResolveSubs$ WhenNotPaid | SubAbility$ Wipe
SVar:Wipe:DB$ Cleanup | ClearImprinted$ True | ClearRemembered$ True
`

// unpayableUnlessHost is the double standing in for a rules host whose payer
// cannot pay the offered unless cost: it reports every cost unpayable, the
// same answer UnlessCostPayableFromCtx gives with no floating or tappable
// mana on the board.
type unpayableUnlessHost struct {
	askHost
}

func (h *unpayableUnlessHost) UnlessCostPayableFromCtx(state.PlayerID, string, *Ctx) bool {
	return false
}

// crookedScalesBoard builds the card's own battlefield — the Scales for p0,
// the Memnite (its "creature you control" target) also for p0, the Ornithopter
// (its "creature an opponent controls" target) for p1 — and the ability
// resolution the way the real cast-time chain binds it: the opponent-side
// target as the root's answer, the you-side one as DBTarget's recorded
// cast-time pre-ask (Forge asks the whole chain's targets before payment).
// The creature IDs come back, not pointers: state.Game.Objs is a value slice,
// so pointers into it go stale across the board's later AddObject calls.
func crookedScalesBoard(t *testing.T, g *state.Game) (h *fakeHost, head *cards.SA, c *Ctx, memnite, ornith state.ObjID) {
	t.Helper()
	card := mkCard(t, crookedScalesScript)
	scales := g.AddObject(card, 0)
	repeatTestZone(g, scales, state.ZBattlefield, 0)
	memniteObj := g.AddObject(mkCard(t, "Name:Memnite\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	repeatTestZone(g, memniteObj, state.ZBattlefield, 0)
	ornithObj := g.AddObject(mkCard(t, "Name:Ornithopter\nTypes:Creature\nPT:0/2\nOracle:x\n"), 1)
	repeatTestZone(g, ornithObj, state.ZBattlefield, 1)
	if g.Obj(memniteObj.ID).Zone != state.ZBattlefield || g.Obj(ornithObj.ID).Zone != state.ZBattlefield ||
		g.Obj(scales.ID).Zone != state.ZBattlefield {
		t.Fatal("board fixture is not on the battlefield")
	}
	head = card.Faces[0].Abilities[0]
	dbTarget := head.Sub
	if dbTarget == nil || dbTarget.Sub == nil || dbTarget.Sub.API != "Repeat" {
		t.Fatalf("Crooked Scales DBTarget link not compiled: %+v", head.Sub)
	}
	h = &fakeHost{g: g}
	c = &Ctx{
		Source:         scales.ID,
		Controller:     0,
		TargetsOffered: true,
		OfferedSA:      head,
		Targets:        []state.Target{{Obj: ornithObj.ID}},
		SVars:          card.Faces[0].SVars,
		SubPreAsk:      map[string][]state.Target{dbTarget.Line: {{Obj: memniteObj.ID}}},
	}
	return h, head, c, memniteObj.ID, ornithObj.ID
}

// flipsIn reports how many coin flips the host's log recorded.
func flipsIn(h *fakeHost) int {
	n := 0
	for _, ev := range h.log {
		if _, _, ok := FlipNoteResult(ev); ok {
			n++
		}
	}
	return n
}

// TestCrookedScalesWinningFlipDestroysOpponentCreatureOnce pins the
// between-iteration gate Forge's RepeatEffect runs over the defined pile: the
// win branch's Cleanup empties it, and the gate — which compares the pile's
// size against Forge's RepeatCompare GE1 default — stops the process after
// the one flip. A win ends the process, destroying exactly the opponent's
// creature.
func TestCrookedScalesWinningFlipDestroysOpponentCreatureOnce(t *testing.T) {
	h, head, c, memnite, ornith := crookedScalesBoard(t, state.NewGame(names(2)))
	Resolve(h, c, head)
	if h.Suspended() {
		t.Fatal("resolution suspended on the effects double")
	}
	if flips := flipsIn(h); flips != 1 {
		t.Fatalf("winning first flip ran %d flips, want 1 (a win ends the process)", flips)
	}
	if z := h.g.Obj(ornith).Zone; z != state.ZGraveyard {
		t.Fatalf("Ornithopter zone = %v, want graveyard (destroyed by the win branch)", z)
	}
	if z := h.g.Obj(memnite).Zone; z != state.ZBattlefield {
		t.Fatalf("Memnite zone = %v, want battlefield (a win never destroys your creature)", z)
	}
}

// TestCrookedScalesUnpaidLossDestroysYourCreatureOnce pins the loss side with
// {3} unaffordable: the election is not payable, so the deterministic decline
// applies, DestroyYours destroys your creature, its Cleanup empties the
// imprint pile, and the gate stops — no second flip after an unpaid loss
// (Forge and XMage destroy exactly one creature, the one you control).
func TestCrookedScalesUnpaidLossDestroysYourCreatureOnce(t *testing.T) {
	fh, head, c, memnite, ornith := crookedScalesBoard(t, state.NewGame(names(2)))
	th := tailsHost{fh}
	Resolve(th, c, head)
	if th.Suspended() {
		t.Fatal("resolution suspended on the effects double")
	}
	if flips := flipsIn(fh); flips != 1 {
		t.Fatalf("losing first flip ran %d flips, want 1 (an unpaid loss ends the process)", flips)
	}
	if z := fh.g.Obj(memnite).Zone; z != state.ZGraveyard {
		t.Fatalf("Memnite zone = %v, want graveyard (destroyed when {3} was not paid)", z)
	}
	if z := fh.g.Obj(ornith).Zone; z != state.ZBattlefield {
		t.Fatalf("Ornithopter zone = %v, want battlefield (an unpaid loss never reaches the opponent's creature)", z)
	}
}

// TestUnlessCostUnaffordableOffersOnlyDecline is a pin, not a fix test (no
// hunk of this change alters it): it guards the CR 118.3 half of the
// symptom on the shared gate: a payer the host reports as unable to pay
// ({3}, no mana of any kind) is offered only the decline option — never a
// pay option they cannot honestly choose.
func TestUnlessCostUnaffordableOffersOnlyDecline(t *testing.T) {
	h := &unpayableUnlessHost{}
	h.g = state.NewGame(names(2))
	Resolve(h, &Ctx{Controller: 0}, sa(t, "DB$ GainLife | Defined$ You | LifeAmount$ 2 | UnlessCost$ 3 | UnlessPayer$ You"))
	if h.asked == nil {
		t.Fatal("the unaffordable unless election was never posed")
	}
	if len(h.asked.Options) != 1 || h.asked.Options[0].Mode != decision.ModeUnlessDecline {
		t.Fatalf("unaffordable {3} options = %+v, want the lone decline", h.asked.Options)
	}
	if h.g.Players[0].Life != 22 {
		t.Fatalf("life = %d, want 22 (the unswitched body still runs on the decline)", h.g.Players[0].Life)
	}
}

// payingFlipHost loses the first flip and wins every later one, and settles
// each unless election as paid in place the way rules' answer record does: it
// remembers the resolution Ctx the payability probe was handed and writes the
// "pay" answer into it when the ask is served.
type payingFlipHost struct {
	*fakeHost
	ctx   *Ctx
	asks  uint64
	draws int
}

func (h *payingFlipHost) Rand(int) int {
	h.draws++
	if h.draws == 1 {
		return 1
	}
	return 0
}

func (h *payingFlipHost) UnlessCostPayableFromCtx(_ state.PlayerID, _ string, c *Ctx) bool {
	h.ctx = c
	return true
}

func (h *payingFlipHost) AskCount() uint64 { return h.asks }

func (h *payingFlipHost) TapeAnswer(d *decision.Decision) (decision.Intent, bool) {
	h.asks++
	for _, o := range d.Options {
		if o.Mode == decision.ModeUnlessPay && h.ctx != nil {
			h.ctx.UnlessPay, h.ctx.UnlessNext = "pay", 0
			return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}, true
		}
	}
	return decision.Intent{}, false
}

// TestCrookedScalesPaidLossRepeatsTheProcess pins the continue side of the
// gate: a lost flip whose {3} is paid skips DestroyYours and its Cleanup, the
// imprint stays, and the process repeats — here into a win, which destroys the
// opponent's creature and ends it. Two flips, Memnite alive, Ornithopter dead.
func TestCrookedScalesPaidLossRepeatsTheProcess(t *testing.T) {
	fh, head, c, memnite, ornith := crookedScalesBoard(t, state.NewGame(names(2)))
	ph := &payingFlipHost{fakeHost: fh}
	Resolve(ph, c, head)
	if ph.asks != 1 {
		t.Fatalf("unless elections served = %d, want 1 (the lost flip's {3} offer)", ph.asks)
	}
	if flips := flipsIn(fh); flips != 2 {
		t.Fatalf("paid loss then win ran %d flips, want 2 (a paid loss repeats, a win ends)", flips)
	}
	if z := fh.g.Obj(ornith).Zone; z != state.ZGraveyard {
		t.Fatalf("Ornithopter zone = %v, want graveyard (the second flip won)", z)
	}
	if z := fh.g.Obj(memnite).Zone; z != state.ZBattlefield {
		t.Fatalf("Memnite zone = %v, want battlefield (the {3} was paid)", z)
	}
}
