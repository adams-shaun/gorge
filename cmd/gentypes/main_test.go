package main

import (
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/protocol"
)

func TestCommittedProtocolTSIsFresh(t *testing.T) {
	want, err := Render()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../../web/src/protocol.ts")
	if err != nil {
		t.Fatalf("%v — run make gentypes", err)
	}
	if string(got) != want {
		t.Fatal("web/src/protocol.ts is stale — run make gentypes")
	}
}

func TestFrameTypeUnionListsEveryConstant(t *testing.T) {
	src, _ := Render()
	for _, ft := range []protocol.FrameType{protocol.THello, protocol.TWidget, protocol.TMatchStart, protocol.TSnapshot,
		protocol.TEvent, protocol.TDecision, protocol.TMatchEnd, protocol.TTableHalted, protocol.TOverflow, protocol.TError} {
		if !strings.Contains(src, `"`+string(ft)+`"`) {
			t.Errorf("FrameType union lacks %q", ft)
		}
	}
	for _, name := range []string{"View", "PlayerView", "CardView", "StackView", "TargetView", "PendingView", "Printing", "Decision", "Option"} {
		if !strings.Contains(src, "export interface "+name+" {") {
			t.Errorf("view/decision type %s missing from the generated output", name)
		}
	}
}
