package eventhost

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// TestBo3CorruptLockedDeckBlocksEvent drives a bot-only Bo3 event whose
// seat-0 deck file is corrupted the moment round-1 series (0,3) game 1
// ends. The between-games path must read both locked decks through
// Store.readDeck BEFORE it logs the continuing GameStarted (brief §2), so
// the corrupt file blocks the event loudly in the state's Error instead of
// starting game 2 on a table whose deck cannot load.
func TestBo3CorruptLockedDeckBlocksEvent(t *testing.T) {
	dir := t.TempDir()
	var once sync.Once
	corrupted := make(chan struct{}, 1)
	p := startGamePair(t, dir, func(_ *Options, ho *host.Options) {
		inner := ho.OnMatchEnd
		ho.OnMatchEnd = func(tid host.TableID, k int, m protocol.MatchInfo) error {
			// Round-1 series (0,3) game 1's match just ended: corrupt
			// seat 0's locked (play) deck before the host turns the
			// result into the event's next advance.
			if strings.Contains(string(tid), "-r1-s03-g1-a1") {
				once.Do(func() {
					path := filepath.Join(dir, "e1", "decks", "0-1.json")
					if err := os.WriteFile(path, []byte("{corrupt"), 0o644); err != nil {
						t.Errorf("corrupt deck file: %v", err)
						return
					}
					corrupted <- struct{}{}
				})
			}
			return inner(tid, k, m)
		}
	})
	req := botEventReq("bot", "lethal-pressure", "forge-heuristic", "bot")
	req.BestOf = 3
	resp := createOK(t, p.h, req)
	id := resp.Event

	// Precondition: the intercept fired — game 1 of (0,3) really ended.
	select {
	case <-corrupted:
	case <-time.After(50 * time.Second):
		t.Fatal("round-1 (0,3) game 1 never ended; the corrupting intercept never fired")
	}

	// The event blocks loudly: the state carries the unreadable-deck
	// error.
	var stErr string
	deadline := time.Now().Add(50 * time.Second)
	for {
		st, err := p.h.EventState(id, "")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(st.Error, "unreadable") {
			stErr = st.Error
			break
		}
		if strings.Contains(st.Error, "halted") {
			t.Fatalf("event halted, not deck-blocked: %s", st.Error)
		}
		if time.Now().After(deadline) {
			t.Fatalf("event never reported the unreadable deck in 50s (phase %s, error %q)", st.Phase, st.Error)
		}
		time.Sleep(pollEvery)
	}
	if !strings.Contains(stErr, "seat 0 deck rev 1") {
		t.Fatalf("blocked error %q does not name seat 0's rev-1 deck", stErr)
	}

	// The series stopped between games: both Sideboarded entries logged,
	// game 1 ended, and no game-2 GameStarted or GameAborted.
	r := runOf(t, p.h, id)
	r.mu.Lock()
	log := r.ev.Log()
	r.mu.Unlock()
	sideb, ends1, starts2, aborts2 := 0, 0, 0, 0
	for _, en := range log {
		serie := en.Round == 1 && en.A == 0 && en.B == 3
		switch {
		case serie && en.Kind == event.KindSideboarded && en.Game == 2:
			sideb++
		case serie && en.Kind == event.KindGameEnded && en.Game == 1:
			ends1++
		case serie && en.Kind == event.KindGameStarted && en.Game == 2:
			starts2++
		case serie && en.Kind == event.KindGameAborted && en.Game == 2:
			aborts2++
		}
	}
	if sideb != 2 {
		t.Fatalf("(0,3) game-2 Sideboarded entries = %d, want 2", sideb)
	}
	if ends1 != 1 {
		t.Fatalf("(0,3) game-1 GameEnded entries = %d, want 1", ends1)
	}
	if starts2 != 0 || aborts2 != 0 {
		t.Fatalf("game 2 moved despite the corrupt deck: starts=%d aborts=%d", starts2, aborts2)
	}
}
