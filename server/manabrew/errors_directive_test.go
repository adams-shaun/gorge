//go:build manabrew

package manabrew

import (
	"strings"
	"testing"

	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

func TestDirectiveRefusedTranslate(t *testing.T) {
	tr := New("table", 2, nil)
	for _, typ := range []string{"startRecording", "stopRecording", "requestRepeat", "repeatVote"} {
		msg := mb.ClientMessage{Value: mb.ClientDirective{Kind: "directive", Directive: mb.DirectiveInput{Type: typ}}}
		o := tr.TranslateResponse(msg, nil, 1)
		if o.Err == nil {
			t.Fatalf("%s: no error, outcome %#v", typ, o)
		}
		if o.Err.Code != mb.CodeInvalidShape || o.Err.PromptID != nil {
			t.Fatalf("%s: code %q promptId %v", typ, o.Err.Code, o.Err.PromptID)
		}
		if !strings.Contains(o.Err.Message, typ) || !strings.Contains(o.Err.Message, "not supported") {
			t.Fatalf("%s: message %q", typ, o.Err.Message)
		}
	}
	if o := tr.TranslateResponse(dirFor(), nil, 1); o.Err != nil || !o.Queued {
		t.Fatalf("concede: %#v", o)
	}
}
