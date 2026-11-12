package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// repl:CopySpell is the Twinning Staff shape only (cards.CopySpellExtraOf);
// any other CopySpell body is labelled repl:CopySpell.OtherShape and stays
// unsupported.
func init() { effects.RegisterNonAPI("repl:CopySpell") }

// copySpellReplHost is the narrow engine surface copySpellExtra reads.
type copySpellReplHost interface {
	Game() *state.Game
	replacementFace(id state.ObjID, ev events.Event) *cards.Face
	controllerOf(id state.ObjID) state.PlayerID
}

// CopySpellExtra is the effects seam (effects.copySpellExtra) for R:Event$
// CopySpell replacements: "If you would copy a spell one or more times,
// instead copy it that many times plus an additional time" (Twinning Staff).
func (e *Engine) CopySpellExtra(copier state.PlayerID, spell state.ObjID) int {
	return copySpellExtra(e, copier, spell)
}

// copySpellExtra sums N over every battlefield R:Event$ CopySpell line of the
// parsed shape (cards.CopySpellExtraOf: ReplaceCount$Amount/Plus.N) whose
// ValidPlayer$ admits copier relative to the line's controller, when spell is
// a SPELL on the stack (a copied ability is not "a spell"). Two Staffs add 2.
func copySpellExtra(h copySpellReplHost, copier state.PlayerID, spell state.ObjID) int {
	g := h.Game()
	sp := g.Obj(spell)
	if sp == nil || sp.Zone != state.ZStack || state.StackKindOf(g, sp) != state.StackKindSpell {
		return 0
	}
	probe := events.Event{Kind: events.StackCopy, Obj: spell, Player: copier}
	total := 0
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			f := h.replacementFace(id, probe)
			if f == nil {
				continue
			}
			for i := range f.Repls {
				r := f.Repls[i]
				n, ok := cards.CopySpellExtraOf(f, r)
				if !ok {
					continue
				}
				if vp, has := r.Param(cards.PKValidPlayer); has &&
					!effects.MatchesPlayerSpec(g, vp, copier, h.controllerOf(id)) {
					continue
				}
				total += n
			}
		}
	}
	return total
}
