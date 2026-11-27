package encodertest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/encoder"
	"github.com/adams-shaun/gorge/bots/encoder/encodertest"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// recorder is a T that keeps what Check reports.
type recorder struct{ msgs []string }

func (r *recorder) Helper() {}
func (r *recorder) Errorf(f string, a ...any) {
	r.msgs = append(r.msgs, fmt.Sprintf(f, a...))
}

// mapRanger is the planted bug: a real encoder whose state's first dense
// columns are the seat's counter values in map iteration order.
type mapRanger struct{ encoder.ViewEncoder }

func (m mapRanger) EncodeState(buf *policynet.StateBuf, v *view.View, seat state.PlayerID, diag *encoder.Diag) policynet.State {
	st := m.ViewEncoder.EncodeState(buf, v, seat, diag)
	i := 0
	for _, n := range v.Players[seat].Counters {
		st.Dense[i] = float32(n)
		i++
	}
	return st
}

// TestEveryEncoderDeterministic: every registered encoder encodes each
// fixture identically across repeated runs, pooled and fresh buffers and
// shuffled map insertion orders; and the kit itself fails on a planted map
// range, so a pass above is not vacuous.
func TestEveryEncoderDeterministic(t *testing.T) {
	if len(encoder.List()) == 0 {
		t.Fatal("no encoder registered")
	}
	t.Run("registered", func(t *testing.T) { encodertest.CheckAll(t) })

	t.Run("planted map range is caught", func(t *testing.T) {
		ps := encodertest.Positions()
		if n := len(ps[0].View.Players[0].Counters); n < 6 {
			t.Fatalf("fixture counters have %d keys; the planted range needs several", n)
		}
		base, ok := encoder.Lookup("entity")
		if !ok {
			t.Fatal("entity not registered")
		}
		var clean recorder
		encodertest.Check(&clean, base, ps)
		if len(clean.msgs) != 0 {
			t.Fatalf("the unplanted encoder reported: %v", clean.msgs)
		}
		var planted recorder
		encodertest.Check(&planted, mapRanger{base}, ps)
		if len(planted.msgs) == 0 {
			t.Fatal("planted map range not detected")
		}
		if !strings.Contains(planted.msgs[0], "state Dense") {
			t.Errorf("unexpected report: %s", planted.msgs[0])
		}
	})
}
