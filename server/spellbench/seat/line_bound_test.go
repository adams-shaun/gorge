package seat

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func TestRoundTripBoundsTheAgentLine(t *testing.T) {
	huge := strings.Repeat("x", wire.MaxLineBytes+1) + "\n"
	var sent bytes.Buffer
	a := &Agent{stdin: nopWriteCloser{&sent}, reader: wire.NewReader(strings.NewReader(huge))}
	line, err := a.roundTrip(context.Background(), helloReq{RequestType: "hello", RequestID: "r-1"})
	if err == nil {
		t.Fatalf("roundTrip accepted a %d-byte line", len(line))
	}
	if !strings.Contains(err.Error(), wire.ErrLineTooLong.Error()) {
		t.Fatalf("err = %v, want it to carry %v", err, wire.ErrLineTooLong)
	}
}
