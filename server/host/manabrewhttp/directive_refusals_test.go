//go:build manabrew

package manabrewhttp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// postRaw POSTs body verbatim to seat 1's send endpoint and returns the
// status and raw reply body.
func postRaw(t *testing.T, ts *testServer, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.srv.URL+"/t1/matches/1/send", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer s1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func wantRefusal(t *testing.T, typ string, code int, body []byte) {
	t.Helper()
	if code != http.StatusBadRequest {
		t.Fatalf("%s: status %d body %s, want 400", typ, code, body)
	}
	var pe mb.ProtocolError
	if err := json.Unmarshal(body, &pe); err != nil {
		t.Fatalf("%s: body %s: %v", typ, body, err)
	}
	if pe.Code != mb.CodeInvalidShape {
		t.Fatalf("%s: code %q (body %s), want invalidShape ProtocolError", typ, pe.Code, body)
	}
	if pe.PromptID != nil {
		t.Fatalf("%s: promptId %v, want none", typ, *pe.PromptID)
	}
	if !strings.Contains(pe.Message, typ) || !strings.Contains(pe.Message, "not supported") {
		t.Fatalf("%s: message %q", typ, pe.Message)
	}
}

func TestDirectiveRefusals(t *testing.T) {
	ts := newTestServer(t, []int{1}, 0, 1)
	waitForPending(t, ts.reg, "t1", 1, 1)

	for _, typ := range []string{"startRecording", "stopRecording", "repeatVote", "requestRepeat"} {
		msg := mb.ClientMessage{Value: mb.ClientDirective{Kind: "directive", Directive: mb.DirectiveInput{Type: typ}}}
		code, pe := send(t, ts, "s1", msg)
		if code != http.StatusBadRequest || pe.Code != mb.CodeInvalidShape {
			t.Fatalf("%s: status %d code %q, want 400 invalidShape", typ, code, pe.Code)
		}
		raw, _ := mb.Encode(pe)
		wantRefusal(t, typ, code, raw)
	}

	code, body := postRaw(t, ts, `{"kind":"directive","directive":{"type":"requestRepeat","iterations":3}}`)
	wantRefusal(t, "requestRepeat", code, body)

	for _, typ := range []string{"requestRestore", "fooBar"} {
		code, body := postRaw(t, ts, `{"kind":"directive","directive":{"type":"`+typ+`"}}`)
		if code != http.StatusBadRequest {
			t.Fatalf("%s: status %d, want 400", typ, code)
		}
		var eb struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(body, &eb); err != nil || eb.Code != "bad_request" {
			t.Fatalf("%s: body %s, want bad_request", typ, body)
		}
	}

	// concede is still accepted (queued behind the mulligan ask).
	concede := mb.ClientMessage{Value: mb.ClientDirective{Directive: mb.DirectiveInput{Type: "concede"}}}
	if code, pe := send(t, ts, "s1", concede); code != http.StatusNoContent {
		t.Fatalf("concede: status %d body %+v, want 204", code, pe)
	}
}
