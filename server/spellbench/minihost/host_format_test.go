//go:build spellbench

package minihost_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/minihost"
	"github.com/adams-shaun/gorge/server/spellbench/validate"
)

// resetSpy is an engine Link that records every round and answers hello_ok,
// then stops the game with an engine error on the reset: the test reads the
// reset out of the recording, it never plays.
type resetSpy struct{ reqs [][]byte }

func (s *resetSpy) Round(req []byte) ([]byte, error) {
	s.reqs = append(s.reqs, req)
	var head struct {
		RequestType string `json:"request_type"`
		RequestID   string `json:"request_id"`
	}
	json.Unmarshal(req, &head)
	if head.RequestType == "reset" {
		return []byte(fmt.Sprintf(`{"response_type":"error","request_id":%q,"error":{"code":"unsupported_format","message":"spy stop"}}`, head.RequestID)), nil
	}
	return []byte(fmt.Sprintf(`{"response_type":"hello_ok","request_id":%q,"protocol":"spellbench/v2","engine_defaults":{}}`, head.RequestID)), nil
}

// TestResetCarriesHostFormat: an empty Host.Format keeps today's reset bytes
// (pauper-bo1); a set Format reaches the reset the host sends.
func TestResetCarriesHostFormat(t *testing.T) {
	deck := catalog.Decks()[0]
	for _, tc := range []struct {
		name, format, want string
	}{
		{"default_is_pauper_bo1", "", "pauper-bo1"},
		{"set_format_reaches_the_reset", "fdn-limited-bo1", "fdn-limited-bo1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var spy resetSpy
			h := &minihost.Host{RunSecret: make([]byte, 32), Engine: &spy,
				Profile: validate.Profile{Extensions: map[string]bool{}}, MaxSteps: 1, MaxDecisions: 1}
			h.Format = tc.format
			_, _ = h.Play(0, deck, "london", nil, [2]minihost.Link{minihost.First{}, minihost.First{}})
			// The game stopped at the reset: exactly hello, then the reset, and
			// no step after it.
			if len(spy.reqs) != 2 {
				t.Fatalf("engine saw %d rounds, want 2 (hello, reset)", len(spy.reqs))
			}
			var reset struct {
				RequestType string `json:"request_type"`
				Format      string `json:"format"`
			}
			if err := json.Unmarshal(spy.reqs[1], &reset); err != nil {
				t.Fatal(err)
			}
			if reset.RequestType != "reset" {
				t.Fatalf("second round is %q, want reset", reset.RequestType)
			}
			if reset.Format != tc.want {
				t.Fatalf("reset format %q, want %q", reset.Format, tc.want)
			}
		})
	}
}
