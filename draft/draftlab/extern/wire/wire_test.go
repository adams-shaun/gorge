package wire

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/protocol"
)

// TestMessageGoldens walks every committed message shape: it decodes,
// validates for the direction that sends it, and re-encodes to the
// identical bytes.
func TestMessageGoldens(t *testing.T) {
	cases := []struct {
		file string
		dir  Direction
	}{
		{"hello_gorge", FromGorge},
		{"hello_agent", FromAgent},
		{"batch_pick", FromGorge},
		{"batch_build", FromGorge},
		{"batch_decide", FromGorge},
		{"result", FromAgent},
		{"result_refused", FromAgent},
		{"stream_end", FromGorge},
		{"bye", FromGorge},
		{"error", FromAgent},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", tc.file+".json"))
			if err != nil {
				t.Fatal(err)
			}
			line := strings.TrimRight(string(raw), "\n")
			if strings.Contains(line, "\n") {
				t.Fatalf("%s is not a single line", tc.file)
			}
			var m Message
			if err := json.Unmarshal([]byte(line), &m); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if err := m.Validate(tc.dir); err != nil {
				t.Fatalf("validate: %v", err)
			}
			re, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if string(re) != line {
				t.Errorf("re-encode differs:\n got %s\nwant %s", re, line)
			}
		})
	}
}

// baseReturns returns a valid message to break one field at a time.
func baseHello() Message {
	return Message{V: 1, Type: THello, Protocol: Protocol, Agent: "random", ModelVersion: "1",
		Kinds: []string{KindPick, KindBuild, KindDecide}, MaxBatch: 0, ViewOmit: []string{"library"}}
}

func baseBatch() Message {
	return Message{V: 1, Type: TBatch, Batch: 1, Items: []Item{
		{ID: 1, Kind: KindPick, Seed: 7, Set: "FDN", Round: 1, PickNo: 1, Pack: []protocol.DraftCard{{ID: "c1", Name: "Bear", Set: "FDN", Number: "1", Rarity: "C"}}},
	}}
}

func baseResult() Message {
	return Message{V: 1, Type: TResult, Batch: 1, Answers: []Answer{{ID: 1, Pick: "c1"}}}
}

// TestValidateRejects breaks one field per case and expects the named
// rejection.
func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name string
		msg  Message
		dir  Direction
		want string
	}{
		{"wrong v", func() Message { m := baseHello(); m.V = 2; return m }(), FromAgent, `"v"`},
		{"unknown type", func() Message { m := baseHello(); m.Type = "nudge"; return m }(), FromAgent, `"type"`},
		{"batch from agent", baseBatch(), FromAgent, `"type"`},
		{"result from gorge", baseResult(), FromGorge, `"type"`},
		{"bye from agent", Message{V: 1, Type: TBye}, FromAgent, `"type"`},
		{"bad protocol", func() Message { m := baseHello(); m.Protocol = "gorge-agent/2"; return m }(), FromAgent, `"protocol"`},
		{"bad agent name", func() Message { m := baseHello(); m.Agent = "Random"; return m }(), FromAgent, `"agent"`},
		{"empty agent name", func() Message { m := baseHello(); m.Agent = ""; return m }(), FromAgent, `"agent"`},
		{"@ in model_version", func() Message { m := baseHello(); m.ModelVersion = "gpt@4"; return m }(), FromAgent, `"model_version"`},
		{"empty kinds", func() Message { m := baseHello(); m.Kinds = nil; return m }(), FromAgent, `"kinds"`},
		{"unknown kind", func() Message { m := baseHello(); m.Kinds = []string{"pick", "drafter"}; return m }(), FromAgent, `"kinds"`},
		{"duplicate kind", func() Message { m := baseHello(); m.Kinds = []string{"pick", "pick"}; return m }(), FromAgent, `"kinds"`},
		{"negative max_batch", func() Message { m := baseHello(); m.MaxBatch = -1; return m }(), FromAgent, `"max_batch"`},
		{"unknown view_omit", func() Message { m := baseHello(); m.ViewOmit = []string{"library", "hands"}; return m }(), FromAgent, `"view_omit"`},
		{`decision as view_omit name`, func() Message { m := baseHello(); m.ViewOmit = []string{"decision"}; return m }(), FromAgent, `"view_omit"`},
		{"batch zero", func() Message { m := baseBatch(); m.Batch = 0; return m }(), FromGorge, `"batch"`},
		{"batch no items", func() Message { m := baseBatch(); m.Items = nil; return m }(), FromGorge, `"items"`},
		{"duplicate item id", func() Message { m := baseBatch(); m.Items = append(m.Items, m.Items[0]); return m }(), FromGorge, `"id"`},
		{"unknown item kind", func() Message { m := baseBatch(); m.Items[0].Kind = "swap"; return m }(), FromGorge, "unknown kind"},
		{"pick without set", func() Message { m := baseBatch(); m.Items[0].Set = ""; return m }(), FromGorge, `"set"`},
		{"pick round zero", func() Message { m := baseBatch(); m.Items[0].Round = 0; return m }(), FromGorge, `"round"`},
		{"pick_no zero", func() Message { m := baseBatch(); m.Items[0].PickNo = 0; return m }(), FromGorge, `"pick_no"`},
		{"pick empty pack", func() Message { m := baseBatch(); m.Items[0].Pack = nil; return m }(), FromGorge, `"pack"`},
		{"build empty pool", func() Message {
			m := baseBatch()
			m.Items[0] = Item{ID: 1, Kind: KindBuild, Seed: 7, Set: "FDN", DeckSize: 40}
			return m
		}(), FromGorge, `"pool"`},
		{"build small deck", func() Message {
			m := baseBatch()
			m.Items[0] = Item{ID: 1, Kind: KindBuild, Seed: 7, Set: "FDN", DeckSize: 39, Pool: []protocol.DraftCard{{ID: "c1", Set: "FDN"}}}
			return m
		}(), FromGorge, `"deck_size"`},
		{"decide empty stream", func() Message {
			m := baseBatch()
			m.Items[0] = Item{ID: 1, Kind: KindDecide, Seed: 7, View: json.RawMessage(`{"decision":{}}`)}
			return m
		}(), FromGorge, `"stream"`},
		{"decide empty view", func() Message {
			m := baseBatch()
			m.Items[0] = Item{ID: 1, Kind: KindDecide, Seed: 7, Stream: "g1.0"}
			return m
		}(), FromGorge, `"view"`},
		{"result batch zero", func() Message { m := baseResult(); m.Batch = 0; return m }(), FromAgent, `"batch"`},
		{"result no answers", func() Message { m := baseResult(); m.Answers = nil; return m }(), FromAgent, `"answers"`},
		{"duplicate answer id", func() Message { m := baseResult(); m.Answers = append(m.Answers, m.Answers[0]); return m }(), FromAgent, `"id"`},
		{"answer with two fields", func() Message { m := baseResult(); m.Answers[0].Deck = &protocol.DeckRequest{}; return m }(), FromAgent, "exactly one"},
		{"answer with no fields", func() Message { m := baseResult(); m.Answers[0].Pick = ""; return m }(), FromAgent, "exactly one"},
		{"error without code", func() Message { m := Message{V: 1, Type: TError, Message: "boom"}; return m }(), FromAgent, `"code"`},
		{"stream_end without stream", Message{V: 1, Type: TStreamEnd}, FromGorge, `"stream"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.Validate(tc.dir)
			if err == nil {
				t.Fatalf("Validate accepted a broken message (want %s)", tc.want)
			}
			if !errors.Is(err, ErrProtocol) {
				t.Fatalf("error does not wrap ErrProtocol: %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %s", err, tc.want)
			}
		})
	}
}

// TestReaderLineCap refuses a line one byte over the cap and still reads
// the next line afterwards.
func TestReaderLineCap(t *testing.T) {
	in := strings.Repeat("x", 25) + "\n" + `{"v":1,"type":"bye"}` + "\n"
	r := NewReader(strings.NewReader(in), 24)
	if _, err := r.Read(); !errors.Is(err, ErrLineTooLong) {
		t.Fatalf("want ErrLineTooLong, got %v", err)
	}
	m, err := r.Read()
	if err != nil {
		t.Fatalf("next line: %v", err)
	}
	if m.Type != TBye {
		t.Errorf("next line decoded as %q, want bye", m.Type)
	}
	if _, err := r.Read(); !errors.Is(err, io.EOF) {
		t.Errorf("want io.EOF at clean end, got %v", err)
	}
}

// TestReaderEOF covers clean ends, blank lines and malformed JSON.
func TestReaderEOF(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if _, err := NewReader(strings.NewReader(""), 0).Read(); !errors.Is(err, io.EOF) {
			t.Errorf("want io.EOF, got %v", err)
		}
	})
	t.Run("after bye with blank lines", func(t *testing.T) {
		in := "\n" + `{"v":1,"type":"bye"}` + "\n\n"
		r := NewReader(strings.NewReader(in), 0)
		m, err := r.Read()
		if err != nil || m.Type != TBye {
			t.Fatalf("got %v (%v), want bye", m.Type, err)
		}
		if _, err := r.Read(); !errors.Is(err, io.EOF) {
			t.Errorf("want io.EOF, got %v", err)
		}
	})
	t.Run("malformed json", func(t *testing.T) {
		_, err := NewReader(strings.NewReader("not json\n"), 0).Read()
		if !errors.Is(err, ErrProtocol) {
			t.Errorf("want ErrProtocol, got %v", err)
		}
	})
	t.Run("unterminated final line", func(t *testing.T) {
		m, err := NewReader(strings.NewReader(`{"v":1,"type":"bye"}`), 0).Read()
		if err != nil || m.Type != TBye {
			t.Errorf("got %v (%v), want bye", m, err)
		}
	})
}

// countingWriter records every Write call separately.
type countingWriter struct {
	calls [][]byte
}

func (w *countingWriter) Write(p []byte) (int, error) {
	cp := append([]byte(nil), p...)
	w.calls = append(w.calls, cp)
	return len(p), nil
}

// TestWriterOneWritePerMessage asserts one Write call per message, each
// ending in a newline, and that the frames still read back.
func TestWriterOneWritePerMessage(t *testing.T) {
	var cw countingWriter
	w := NewWriter(&cw)
	msgs := []Message{
		{V: 1, Type: TBatch, Batch: 1, Items: []Item{{ID: 1, Kind: KindPick, Seed: 1, Set: "FDN", Round: 1, PickNo: 1, Pack: []protocol.DraftCard{{ID: "c1", Set: "FDN"}}}}},
		{V: 1, Type: TBye},
	}
	for _, m := range msgs {
		if err := w.Write(m); err != nil {
			t.Fatal(err)
		}
	}
	if len(cw.calls) != len(msgs) {
		t.Fatalf("%d Write calls for %d messages", len(cw.calls), len(msgs))
	}
	for i, line := range cw.calls {
		if line[len(line)-1] != '\n' {
			t.Errorf("call %d does not end in a newline", i)
		}
	}
	r := NewReader(strings.NewReader(string(cw.calls[0])+string(cw.calls[1])), 0)
	for i, want := range msgs {
		got, err := r.Read()
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		re, _ := json.Marshal(got)
		we, _ := json.Marshal(want)
		if string(re) != string(we) {
			t.Errorf("read %d: got %s want %s", i, re, we)
		}
	}
}

// TestParseViewOmit ORs the named bits and rejects unknown names.
func TestParseViewOmit(t *testing.T) {
	if len(ViewOmitNames) != 9 {
		t.Fatalf("ViewOmitNames has %d entries, want 9 (OmitDecision is not a name)", len(ViewOmitNames))
	}
	got, err := ParseViewOmit([]string{"library", "card_lists"})
	if err != nil {
		t.Fatal(err)
	}
	if got != ViewOmitNames[0].Bit|ViewOmitNames[7].Bit {
		t.Errorf("got %d, want library|card_lists", got)
	}
	if zero, err := ParseViewOmit(nil); err != nil || zero != 0 {
		t.Errorf("nil names: got %d, %v; want 0, nil", zero, err)
	}
	if _, err := ParseViewOmit([]string{"decision"}); err == nil {
		t.Error(`"decision" was accepted as a view_omit name`)
	}
	if _, err := ParseViewOmit([]string{"hands"}); err == nil {
		t.Error("unknown name accepted")
	}
}

// TestSeed53 keeps the folded seed below 2^53.
func TestSeed53(t *testing.T) {
	if got := Seed53(0); got != 0 {
		t.Errorf("Seed53(0) = %d", got)
	}
	big := uint64(0xFFFFFFFFFFFFFFFF)
	if got := Seed53(big); got != big>>11 || got >= 1<<53 {
		t.Errorf("Seed53(max) = %d, want %d and < 2^53", got, big>>11)
	}
}

// TestDecisionOf decodes the decision out of the committed decide view and
// refuses a view without one.
func TestDecisionOf(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "batch_decide.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m Message
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	d, err := DecisionOf(m.Items[0].View)
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != "priority" || len(d.Options) != 3 {
		t.Errorf("decoded %q with %d options", d.Kind, len(d.Options))
	}
	if _, err := DecisionOf(json.RawMessage(`{"viewer":1}`)); err == nil {
		t.Error("a view with no decision was accepted")
	}
	if _, err := DecisionOf(json.RawMessage(`not json`)); err == nil {
		t.Error("malformed view accepted")
	}
}
