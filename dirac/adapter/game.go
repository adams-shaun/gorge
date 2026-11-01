package adapter

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

var (
	_ game.Game    = (*Game)(nil)
	_ game.Stalled = (*Game)(nil)
)

// Game is one running game. The kernel is read only inside New, Submit and Fork;
// everything a host reads between them is data those calls left behind.
type Game struct {
	eng *Engine
	set cardset.Set
	tbl *kop.Table
	cfg game.Config

	s  dirac.State
	d  dirac.Decision
	kp dirac.Projection
	tr canon.Translator

	// ws is the scratch position an answer is dry-run on, wtr its translator.
	ws  *dirac.State
	wd  dirac.Decision
	wkp dirac.Projection
	wtr canon.Translator

	log  *events.Log
	pend *decision.Decision
	// acts[i] is native option i's kernel action, canons[i] its canonical form;
	// paid[i] is payment action i's.
	acts   []dirac.Action
	canons []canon.Canon
	paid   []dirac.Action
	// multi: the answer is several kernel picks.
	multi bool

	conceded  bool
	loser     uint8
	manifests []deck.Manifest

	ids     idTable
	targets targetSlot
	board   board
}

// bind points the translators at this game's own storage.
func (g *Game) bind() {
	g.tr.S, g.tr.D, g.tr.KP = &g.s, &g.d, &g.kp
	g.wtr = g.tr
	g.wtr.D, g.wtr.KP = &g.wd, &g.wkp
}

func (g *Game) Board() *state.Game { return g.board.g }

func (g *Game) Chars() view.Chars { return &g.board }

func (g *Game) Log() *events.Log { return g.log }

func (g *Game) Pending() *decision.Decision { return g.pend }

func (g *Game) Caps() game.Caps { return g.eng.Caps() }

// StallReason is "" unless the kernel stopped a live game at a declared limit or
// a rule gap.
func (g *Game) StallReason() string {
	if g.pend != nil || g.board.g.Over {
		return ""
	}
	r := g.s.StallReason()
	class := "capacity"
	if dirac.StallIsUnmodelled(r) {
		class = "unmodelled"
	}
	return class + " " + dirac.StallName(r)
}

// Fork is an independent copy at this point.
func (g *Game) Fork() game.Game {
	c := &Game{eng: g.eng, set: g.set, tbl: g.tbl, cfg: g.cfg, tr: g.tr,
		d: g.d, log: g.log.Clone(), multi: g.multi, conceded: g.conceded, loser: g.loser,
		ids: g.ids, targets: g.targets, manifests: g.manifests}
	c.bind()
	dirac.Register(g.tbl)
	c.s.CopyFrom(&g.s)
	c.s.Project(&c.kp)
	if g.pend != nil {
		c.pend = g.pend.Clone()
	}
	c.acts = append([]dirac.Action(nil), g.acts...)
	c.canons = append([]canon.Canon(nil), g.canons...)
	c.paid = append([]dirac.Action(nil), g.paid...)
	c.materialise()
	return c
}

// settle reads the position the kernel stopped at: the next decision, the board,
// and the game's end.
func (g *Game) settle() {
	g.s.StepInto(&g.d)
	g.s.Project(&g.kp)
	g.ids.sync(&g.kp)
	g.pend = nil
	if g.d.Kind != dirac.DkNone {
		g.pose()
	}
	g.materialise()
	g.logSettled()
}

// logSettled records the end of the game or the ask just posed.
func (g *Game) logSettled() {
	if g.pend != nil {
		g.log.Append(events.Event{Kind: events.DecisionAsk, Player: g.pend.Player, Text: string(g.pend.Kind)})
		return
	}
	if over, win := g.s.Terminal(); over {
		g.logOver(win)
	}
}

func (g *Game) logOver(win int8) {
	e := events.Event{Kind: events.GameOver}
	if win >= 0 {
		e.Player = state.PlayerID(win)
	}
	g.log.Append(e)
}

func newLog(seed uint64) *events.Log { return events.NewLog(seed) }

// Table is the game's card table.
func (g *Game) Table() *kop.Table { return g.tbl }

// CopyState copies the kernel's position into dst, for a conformance check that
// compares it.
func (g *Game) CopyState(dst *dirac.State) { dst.CopyFrom(&g.s) }
