package host

import (
	"context"
	"errors"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
)

// HumanTap is the optional companion of a HumanSeat: a second front for the
// same parked decision, for a codec that answers it through another protocol
// (the SpellBench v2 candidates, server/spellbench/humanseat). The HumanSeat
// stays the seat the match plays: its slot, think timeout, caretaker, undo and
// registry paths are untouched, so a tapped seat times out to its caretaker
// and rewinds exactly as a plain one does. The tap only watches decisions park
// and answers them through the seat's own submit path.
//
// A HumanSeat without a tap (every seat today) pays one nil check in park.
type HumanTap interface {
	// AttachMatch is bots.MatchSeat: the tap receives the live game and the
	// Projection lock exactly as a bot seat does. An error crashes the match.
	bots.MatchSeat
	// Parked is called on the match goroutine, from park, each time the seat
	// parks a decision (again after an engine refusal). It must not block: it
	// starts whatever answers d. d is an owned copy.
	//
	// answer submits an intent for d the way a registry submit does and
	// returns once the engine took it (nil) or refused it (its reason); it
	// errors at once when the decision is no longer pending. gone is closed
	// when the seat stops accepting an answer without one having been
	// delivered: think timeout or cancellation (the caretaker answered), an
	// undo. The engine is then about to move, so the tap must not read it.
	Parked(ctx context.Context, v view.View, d decision.Decision, answer func(decision.Intent) error, gone <-chan struct{})
}

// SetTap installs the tap before the match starts (Options.Seats builds the
// seat; the embedder taps it). Setting nil removes it.
func (s *HumanSeat) SetTap(t HumanTap) {
	s.mu.Lock()
	s.tap = t
	s.mu.Unlock()
}

// attachTap hands the tap its match context; a no-op for an untapped seat.
// The tap answers from payment plans, which a human table builds only with
// AutoMana, so a tap on a manual-mana table refuses the match instead of
// posing no castable spell.
func (s *HumanSeat) attachTap(mc *bots.MatchContext, autoPayMana bool) error {
	s.mu.Lock()
	tap := s.tap
	s.mu.Unlock()
	if tap == nil {
		return nil
	}
	if !autoPayMana {
		return errors.New("a tapped human seat needs an AutoMana table")
	}
	return tap.AttachMatch(mc)
}

// HumanTapFactory builds the tap of one human seat at match start. It runs on
// the match goroutine before the seat's first decision, so it may read the
// match context (the table's seat count and format, the game id) and must
// return promptly. A nil tap leaves the seat untapped; an error crashes the
// match. server/gorged's -sbv2 uses it to install a SpellBench v2 codec only on
// a two-seat constructed AutoMana table.
type HumanTapFactory func(table TableID, match int, autoPayMana bool, mc *bots.MatchContext) (HumanTap, error)
