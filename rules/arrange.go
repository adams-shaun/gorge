package rules

import (
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// handleArrange applies an answered KArrange decision -- the ordered-subset
// ask a mid-resolution library-arranging effect (RearrangeTopOfLibrary,
// Ponder; later Scry/Surveil/Dig) poses through effects.Host.Ask (Ruling
// J0). It is the engine's one KArrange handler, the mid-resolution sibling
// of handleModes.
//
// The contract it applies, from the KArrange doc comment: the chosen
// options, IN THE ORDER THE ANSWER GIVES THEM, become pile A; the options
// not chosen, in the order they were OFFERED, become pile B. Min/Max bound
// pile A's size (a RearrangeTopOfLibrary is Min == Max == N, so pile B is
// empty; Scry will pose Min == Max == 1 with the unchosen card heading to
// pile B / "bottom"). The new library is pile A, then pile B, then the
// untouched remainder beneath the top N. pile A index 0 is the card closest
// to the top -- the next card drawn.
//
// The reorder is recorded as a single events.LibraryOrder carrying the
// complete new library (Ruling J1), which is also what makes the answer
// replay-legitimate (Ruling J2): the logged intent plus this one event
// re-derive the same order on a reconstruction, and a fresh engine fed the
// same logged intents re-poses the same question. Pile B is not discarded by
// this handler but simply placed beneath pile A; the effect that later needs
// to send it to a destination (Scry's "bottom", a Dig's "exile") reads its
// own option Kind and moves it separately.
//
// `chosen` is handed on to resumeResolution, which re-enters the suspended
// resolution the way handleModes does -- the engine's one resume mechanism,
// not a second one. The re-entered effect (effRearrangeTopOfLibrary) sees
// Ctx.Arrange set and returns without re-asking, so only the chained
// SubAbility$ runs. A KArrange answer with no suspended resolution is only
// reachable from a hand-built decision, never from a real ask; it degrades
// with a Note rather than panicking, the same totality stance every handler
// takes.
func (e *Engine) handleArrange(d *decision.Decision, in decision.Intent) {
	if e.resume == nil {
		e.emit(events.Event{Kind: events.Note, Player: in.Player,
			Text: "arrange answered with no resolution suspended"})
		return
	}
	rp := e.resume
	e.resume = nil
	chosen := d.Chosen(in)
	// Pile A: the chosen options in the order the answer gave them.
	pileA := make([]state.ObjID, 0, len(chosen))
	chosenSet := make(map[int]bool, len(chosen))
	for _, o := range chosen {
		pileA = append(pileA, o.Obj)
		chosenSet[o.Index] = true
	}
	// Pile B: the options not chosen, in the order they were offered.
	var pileB []state.ObjID
	for _, o := range d.Options {
		if !chosenSet[o.Index] {
			pileB = append(pileB, o.Obj)
		}
	}
	// The complete new library: pile A, then pile B, then the untouched
	// remainder beneath the top N. The top N never exceeds the library
	// (effRearrangeTopOfLibrary clamps to len(lib)), so the remainder slice
	// is always valid. A suspended resolution cannot have the library change
	// underneath it, so d.Options' N and the live library still agree.
	lib := e.G.Zone(state.ZLibrary, d.Player)
	k := len(d.Options)
	newLib := make([]state.ObjID, 0, len(pileA)+len(pileB)+len(lib)-k)
	newLib = append(newLib, pileA...)
	newLib = append(newLib, pileB...)
	newLib = append(newLib, lib[k:]...)
	e.emit(events.Event{Kind: events.LibraryOrder, Player: d.Player,
		IDs: newLib, Secret: true})
	e.resumeResolution(rp, chosen)
}
