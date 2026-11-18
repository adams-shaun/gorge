//go:build manabrew

package manabrew

import "testing"

func TestDirectiveCodecRefusedTypes(t *testing.T) {
	for _, typ := range []string{"concede", "startRecording", "stopRecording", "requestRepeat", "repeatVote"} {
		in := ClientMessage{Value: ClientDirective{Kind: "directive", Directive: DirectiveInput{Type: typ}}}
		raw, err := Encode(in)
		if err != nil {
			t.Fatalf("%s: encode: %v", typ, err)
		}
		var out ClientMessage
		if _, err := Decode(raw, &out); err != nil {
			t.Fatalf("%s: decode %s: %v", typ, raw, err)
		}
		cd, ok := out.Value.(ClientDirective)
		if !ok || cd.Directive.Type != typ {
			t.Fatalf("%s: decoded %#v", typ, out.Value)
		}
	}
	var out ClientMessage
	raw := []byte(`{"kind":"directive","directive":{"type":"requestRepeat","iterations":3}}`)
	if _, err := Decode(raw, &out); err != nil {
		t.Fatalf("requestRepeat with iterations: %v", err)
	}
}

func TestDirectiveCodecUnknownTypesRejected(t *testing.T) {
	for _, typ := range []string{"requestRestore", "fooBar", ""} {
		if _, err := Encode(ClientMessage{Value: ClientDirective{Kind: "directive", Directive: DirectiveInput{Type: typ}}}); err == nil {
			t.Fatalf("%q: encode accepted", typ)
		}
		var out ClientMessage
		raw := []byte(`{"kind":"directive","directive":{"type":"` + typ + `"}}`)
		if _, err := Decode(raw, &out); err == nil {
			t.Fatalf("%q: decode accepted", typ)
		}
	}
}
