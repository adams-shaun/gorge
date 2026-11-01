package adapter

import (
	"slices"

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

// adapterVersion moves when the log or decisions a table derives change.
const adapterVersion = "2"

// turnTally is what a seat did this turn that the view counts.
type turnTally struct{ spells, tokens int32 }

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

	// exiled[o] is what kernel object o's abilities exiled and is still there.
	exiled [][]state.ObjID
	// vers[o] is object o's zone-change count at the last settle.
	vers [dirac.MaxObj]uint8
	// tally is this turn's casts and tokens, per seat.
	tally     [2]turnTally
	tallyTurn int32
	// building is the object the last decision's cast or activation was building.
	building dirac.Obj

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
	// passed: the last answer passed priority.
	passed bool

	// prev is the position before the last answer, which the log is derived from.
	prev dirac.Projection
	// poison is each seat's count as last logged.
	poison [2]int32

	conceded bool
	loser    uint8
	// stalled is the limit the kernel stopped at, 0 if none.
	stalled   uint8
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
	if g.stalled == 0 || g.pend != nil || g.board.g.Over {
		return ""
	}
	class := "capacity"
	if dirac.StallIsUnmodelled(g.stalled) {
		class = "unmodelled"
	}
	return class + " " + dirac.StallName(g.stalled)
}

// Fork is an independent copy at this point.
func (g *Game) Fork() game.Game {
	c := &Game{eng: g.eng, set: g.set, tbl: g.tbl, cfg: g.cfg, tr: g.tr,
		d: g.d, log: g.log.Clone(), multi: g.multi, conceded: g.conceded, loser: g.loser,
		ids: g.ids, targets: g.targets, stalled: g.stalled, poison: g.poison, manifests: g.manifests, tally: g.tally, tallyTurn: g.tallyTurn, building: g.building, vers: g.vers}
	c.bind()
	c.board.prio, c.board.step, c.board.seen, c.board.resolving, c.board.landOpen = g.board.prio, g.board.step, g.board.seen, g.board.resolving, g.board.landOpen
	for i, l := range g.board.blockers {
		c.board.blockers[i] = slices.Clone(l)
	}
	for _, l := range g.exiled {
		c.exiled = append(c.exiled, slices.Clone(l))
	}
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

// settle reads the position the kernel stopped at: the next decision, the board, the
// log of what changed, and the game's end.
func (g *Game) settle() {
	g.prev = g.kp
	prevIDs := g.ids
	asked := g.d.Kind
	var before [dirac.MaxObj]kop.Zone
	for o := 0; o < int(g.kp.NObj); o++ {
		before[o] = g.kp.Objs[o].Zone
	}
	var top *dirac.ProjStack
	if g.kp.NStack > 0 {
		e := g.kp.Stack[g.kp.NStack-1]
		top = &e
	}
	g.s.StepInto(&g.d)
	g.stalled = g.s.StallReason()
	g.s.Project(&g.kp)
	g.ids.sync(&g.kp)
	g.derive(&g.prev, &prevIDs, asked)
	g.noteExiled(&before, top)
	g.noteTally(&before, int32(g.kp.Turn))
	g.pend = nil
	if g.d.Kind != dirac.DkNone {
		g.pose()
	}
	g.materialise()
	if g.pend != nil && g.pend.Kind == decision.KPriority {
		g.pend.Prompt = decision.PriorityPrompt(g.board.g.Turn, g.board.g.Step, g.board.g.SeatFacingName(g.pend.Player))
	}
	g.logSettled()
}

// logSettled records the end of the game or the ask just posed.
func (g *Game) logSettled() {
	if g.pend != nil {
		g.log.Append(events.Event{Kind: events.DecisionAsk, Player: g.pend.Player, Text: string(g.pend.Kind)})
		return
	}
	if over, win := g.s.Terminal(); over && g.stalled == 0 {
		if win >= 0 {
			g.log.Append(events.Event{Kind: events.PlayerLost, Player: state.PlayerID(1 - win), Text: g.lossText(uint8(1 - win))})
		}
		g.logOver(win)
	}
}

func (g *Game) logOver(win int8) {
	e := events.Event{Kind: events.GameOver}
	if win >= 0 {
		e.Player = state.PlayerID(win)
	} else {
		e.Amount = 1
	}
	g.log.Append(e)
}

func newLog(seed uint64) *events.Log { return events.NewLog(seed) }

// EngineKey pins what a recording replays on: the set, the surface, the tables built
// from the corpus and this adapter's reading of them.
func (g *Game) EngineKey() string {
	key, err := cardset.Key()
	if err != nil {
		key = "no-tables"
	}
	return g.set.Name() + "/" + surfaceName(g.eng.surface) + "/" + key + "/" + adapterVersion
}

func surfaceName(s dirac.Surface) string {
	if s == dirac.SurfaceAutopay {
		return "autopay"
	}
	return "full"
}

// Table is the game's card table.
func (g *Game) Table() *kop.Table { return g.tbl }

// Set is the card set the game's table came from.
func (g *Game) Set() cardset.Set { return g.set }

// CopyState copies the kernel's position into dst, for a conformance check that
// compares it.
func (g *Game) CopyState(dst *dirac.State) { dst.CopyFrom(&g.s) }
