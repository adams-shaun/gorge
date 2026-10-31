package draftlab

import (
	"context"
	"fmt"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
)

// Tape is one game's accepted intents, recorded by Options.RecordTape so the
// game re-drives with no player policy: ReplayTape answers both seats from
// the tape instead of any model. The engine log keeps accepted intents only
// (Submit validates before anything is logged), so a replay that re-asks the
// same decisions consumes the tape in order and errors the moment its asks
// stop matching.
type Tape struct {
	Names     [2]string         // the engine config's player names
	Mulligans int               // the config's London mulligan round
	Intents   []decision.Intent // the game's engine log intents, in order
}

// ReplayTape re-drives one recorded game with no player policy: both seats
// answer from the tape. a and b are the decks the game was recorded with,
// seed the engine seed it ran under. The replayed chain head is the record's
// (a tape of a stalled game verifies by head too: the same watchdog caps
// stop the re-drive at the same intent). The re-drive runs under THIS lab's
// own watchdog caps — playGame reads MaxTurns/MaxIntents off the verifying
// lab, never off the tape — so construct the lab with the recorded run's
// caps (read them off the run's Manifest: MaxTurns/MaxIntents, the defaults
// filled in). With LARGER caps the re-drive plays past the recorded end and
// fails "tape exhausted at decision seq %d"; with SMALLER caps it stalls at
// its own earlier watchdog and fails "tape has %d unused intents" (or a
// head mismatch); only matching caps reproduce the recorded head. The two
// seats of the game share one
// cursor; a game is serial — one seat answers at a time — so it needs no
// lock. A nil tape and a tape recorded under a different mulligan round are
// refused; intents left over after the re-drive are an error. The re-drive
// enforces Ruling P2, as replay.Replay does: every recorded intent is
// submitted verbatim and ANY engine rejection is a tape divergence error,
// never a fallback re-answer.
func (l *Lab) ReplayTape(a, b Deck, seed uint64, t *Tape) (string, error) {
	if t == nil {
		return "", fmt.Errorf("draftlab: ReplayTape: nil tape (run with Options.RecordTape)")
	}
	if t.Mulligans != l.mulliganRound() {
		return "", fmt.Errorf("draftlab: ReplayTape: tape mulligan round %d, this lab runs %d", t.Mulligans, l.mulliganRound())
	}
	cur := &tapeCursor{intents: t.Intents}
	p := NamedPlayer{
		Ref: AgentRef{Name: "tape"},
		New: func(uint64) (seat.Seat, error) { return &tapeSeat{cur: cur}, nil },
	}
	res, err := l.playGame(Entrant{Deck: a, Player: p}, Entrant{Deck: b, Player: p}, t.Names, seed, true)
	if err != nil {
		return "", err
	}
	if cur.i < len(cur.intents) {
		return "", fmt.Errorf("draftlab: ReplayTape: tape has %d unused intents", len(cur.intents)-cur.i)
	}
	return res.Head, nil
}

// VerifyEventTapes verifies a finished event result the way VerifyEvent does
// — the log replays event-for-event, re-marshals to the identical bytes and
// carries the manifest seed — and then re-drives every logged game from its
// recorded tape instead of player policies: the re-driven chain head must
// equal the logged one. Every GameResult in r must carry a tape (run with
// Options.RecordTape); a stalled game's tape verifies by head too. The
// re-drives run under THIS lab's own watchdog caps (ReplayTape reads
// MaxTurns/MaxIntents off the verifying lab, never off the tape), so
// construct the lab with the result's Manifest caps: with larger caps a
// stalled game's re-drive overruns its tape ("tape exhausted"), with
// smaller caps it stalls early ("tape has N unused intents" or a head
// mismatch), and only matching caps reproduce the logged heads.
func (l *Lab) VerifyEventTapes(r EventResult) error {
	ev, err := verifyEventLog(r)
	if err != nil {
		return fmt.Errorf("draftlab: VerifyEventTapes: %w", err)
	}
	mode, set := ev.Mode(), ev.SetCode()
	for _, en := range ev.Log() {
		if en.Kind != event.KindGameEnded {
			continue
		}
		t, ok := tapeFor(r.Games, en.Round, en.A, en.B, en.Game)
		if !ok || t == nil {
			return fmt.Errorf("draftlab: VerifyEventTapes: game round %d seats %d–%d has no tape: run with Options.RecordTape", en.Round, en.A, en.B)
		}
		da, err := eventDeck(ev, set, mode, en.A)
		if err != nil {
			return fmt.Errorf("draftlab: VerifyEventTapes: game round %d seats %d–%d: %w", en.Round, en.A, en.B, err)
		}
		db, err := eventDeck(ev, set, mode, en.B)
		if err != nil {
			return fmt.Errorf("draftlab: VerifyEventTapes: game round %d seats %d–%d: %w", en.Round, en.A, en.B, err)
		}
		head, err := l.ReplayTape(da, db, gameSeed(ev.Seed(), en.Round, en.A, en.B, en.Game), t)
		if err != nil {
			return fmt.Errorf("draftlab: VerifyEventTapes: game round %d seats %d–%d: %w", en.Round, en.A, en.B, err)
		}
		if head != en.Head {
			return fmt.Errorf("draftlab: VerifyEventTapes: game round %d seats %d–%d: replayed chain head %s, log has %s", en.Round, en.A, en.B, head, en.Head)
		}
	}
	return nil
}

// tapeFor finds r.Games' element with the equal (Round, A, B, Game)
// identity: a loop over the slice, never a map, so no iteration order can
// reach the answer.
func tapeFor(games []GameResult, round, a, b, game int) (*Tape, bool) {
	for i := range games {
		g := &games[i]
		if g.Round == round && g.A == a && g.B == b && g.Game == game {
			return g.Tape, true
		}
	}
	return nil, false
}

// tapeCursor is the two seats' shared tape read position. A game is serial —
// one seat answers at a time, on one goroutine — so it needs no lock.
type tapeCursor struct {
	intents []decision.Intent
	i       int
}

// tapeSeat is one game seat re-driven from the tape: every Decide consumes
// the next recorded intent and errors the moment the engine's ask stops
// matching it. It reads nothing from the game, so it projects the leanest
// View (every view.Omit bit).
type tapeSeat struct {
	cur *tapeCursor
}

// tapeOmitAll is every view.Omit bit (engine/view/lean.go's list, in order).
const tapeOmitAll = view.OmitLibrary | view.OmitAvailable | view.OmitArchetype |
	view.OmitAbilityCosts | view.OmitEffectiveCost | view.OmitDecision |
	view.OmitOwnDeck | view.OmitPotential | view.OmitCardLists | view.OmitDerivedChars

// Decide consumes the tape's next intent. Exhausted, or an ask whose
// (seq, player) does not match the recorded intent, is an error — a diverged
// replay must never answer a different question with a recorded answer.
func (s *tapeSeat) Decide(_ context.Context, _ view.View, d decision.Decision) (decision.Intent, error) {
	if s.cur.i >= len(s.cur.intents) {
		return decision.Intent{}, fmt.Errorf("tape exhausted at decision seq %d", d.Seq)
	}
	in := s.cur.intents[s.cur.i]
	s.cur.i++
	if in.Seq != d.Seq || in.Player != d.Player {
		return decision.Intent{}, fmt.Errorf("tape diverges at intent %d: recorded seq %d player %d, engine asks seq %d player %d", s.cur.i-1, in.Seq, in.Player, d.Seq, d.Player)
	}
	return in, nil
}

// ViewOmit omits every part: the seat reads nothing (E5-d plan §7).
func (s *tapeSeat) ViewOmit(*decision.Decision) view.Omit { return tapeOmitAll }
