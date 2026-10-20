package minihost_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/minihost"
)

type errorAgent struct{ atStart bool }

func (a errorAgent) Round(q []byte) ([]byte, error) {
	if !a.atStart && bytes.Contains(q, []byte(`"request_type":"game_start"`)) {
		return []byte(`{"response_type":"ack"}`), nil
	}
	return []byte(`{"response_type":"error","error":{"code":"internal_error","message":"failed"}}`), nil
}

func TestAgentErrorsCannotBecomeCandidateZero(t *testing.T) {
	d, _ := catalog.ByID("Burn")
	for _, atStart := range []bool{true, false} {
		_, err := host(t).Play(0, d, "london", nil, [2]minihost.Link{errorAgent{atStart}, &minihost.Uniform{}})
		if err == nil || !strings.Contains(err.Error(), `"response_type":"error"`) {
			t.Fatalf("agent failure was played: %v", err)
		}
	}
}
