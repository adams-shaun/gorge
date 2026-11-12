package oracle

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/canon"
)

// TestReproCompactRoundTrip guards the corpus compaction (cleanup B1). Every
// recorded repro on disk is the compact raw encoding — the derived `readable`
// field dropped, no indentation, one line — re-encoded from the file's raw
// JSON, so a koracle-repro-1 file keeps its format name and its action bytes
// byte-for-byte (only the decoded repro is remapped, by UnmarshalJSON). The
// test pins the corpus to that encoding and proves the compact encoding
// decodes to the same Actions, Cards and Seed re-encoded again, and that the
// on-disk `actions` bytes survive the encoding path untouched.
func TestReproCompactRoundTrip(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob(filepath.Join("testdata", "repro", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no repro files under testdata/repro")
	}
	// the corpus still holds koracle-repro-1 recordings: the sweep that
	// compacted it worked on the raw JSON, so a format-1 file kept its
	// format name and its action bytes. A decode-and-re-encode sweep (the
	// way a naive one goes through ReadRepro, which remaps the actions)
	// would migrate every one of them to koracle-repro-2.
	n1 := 0
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte(`"format":"koracle-repro-1"`)) {
			n1++
		}
	}
	if n1 == 0 {
		t.Fatal("no koracle-repro-1 files remain: the corpus was re-encoded through ReadRepro, which remaps a format-1 recording's actions")
	}
	for _, p := range paths {
		p := p
		t.Run(filepath.Base(p), func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			var file map[string]json.RawMessage
			if err := json.Unmarshal(raw, &file); err != nil {
				t.Fatal(err)
			}
			var format string
			if err := json.Unmarshal(file["format"], &format); err != nil {
				t.Fatal(err)
			}
			// the sweep is raw JSON: a file's format name is what it was
			// recorded as, never rewritten
			if format != reproFormat && format != reproFormat1 {
				t.Fatalf("format %q, want %q or %q", format, reproFormat, reproFormat1)
			}
			if _, ok := file["readable"]; ok {
				t.Fatal("file still carries the derived readable field")
			}
			// the file is the canonical compact encoding: Write and the
			// sweep both go through compactReproJSON, and re-encoding the
			// raw JSON must reproduce it byte for byte (a literal '&' in a
			// card name stays literal)
			canon, err := compactReproJSON(raw)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(canon, raw) {
				t.Fatal("file on disk is not the compact encoding")
			}
			// the on-disk `actions` bytes survive the encoding path: decode
			// them directly (Repro's UnmarshalJSON is not in play) and
			// re-encode — the bytes must come back exactly as stored
			var as []Canon
			if err := json.Unmarshal(file["actions"], &as); err != nil {
				t.Fatal(err)
			}
			ab, err := json.Marshal(as)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(ab, file["actions"]) {
				t.Fatal("actions bytes do not survive the encoding path")
			}
			// a format-1 recording stores the format-1 refs its name
			// promises: if any action names a stack ability or seat (the
			// bytes the decode remaps, 0xE0..0xF1), re-encoding the mapped
			// actions must not reproduce the stored bytes
			if format == reproFormat1 {
				remapped := false
				for _, a := range as {
					if s, g := a.Src(), a.Tgt(); (s >= 0xE0 && s <= 0xF1) || (g >= 0xE0 && g <= 0xF1) {
						remapped = true
						break
					}
				}
				if remapped {
					mb, err := json.Marshal(format1Actions(as))
					if err != nil {
						t.Fatal(err)
					}
					if bytes.Equal(mb, file["actions"]) {
						t.Fatal("format-1 file already stores the mapped refs")
					}
				}
			}
			// the compact encoding decodes and re-encodes to the same game
			old, err := ReadRepro(p)
			if err != nil {
				t.Fatal(err)
			}
			if old.Format != reproFormat {
				t.Fatalf("decoded format %q, want %q", old.Format, reproFormat)
			}
			b, err := json.Marshal(old)
			if err != nil {
				t.Fatal(err)
			}
			var fresh Repro
			if err := json.Unmarshal(b, &fresh); err != nil {
				t.Fatal(err)
			}
			if fresh.Seed != old.Seed {
				t.Fatalf("seed moved: %d -> %d", old.Seed, fresh.Seed)
			}
			if !reproCardsEqual(old.Cards, fresh.Cards) {
				t.Fatal("cards moved")
			}
			if len(fresh.Actions) != len(old.Actions) {
				t.Fatalf("actions moved: %d -> %d", len(old.Actions), len(fresh.Actions))
			}
			for i := range old.Actions {
				if fresh.Actions[i] != old.Actions[i] {
					t.Fatalf("action %d moved: %s -> %s", i, old.Actions[i].String(), fresh.Actions[i].String())
				}
			}
		})
	}
}

// TestReproWriteCompact pins Repro.Write to the compact encoding the corpus
// carries: a newly recorded repro is one line, without indentation or the
// derived readable field, key-sorted and produced by the same compactReproJSON
// path TestReproCompactRoundTrip pins the corpus to, and decodes to the game
// it was built from.
func TestReproWriteCompact(t *testing.T) {
	r := &Repro{Format: reproFormat, Decks: [2]string{"d0", "d1"}, Seed: 42,
		Cards:   [2][]string{{"A&B <C>"}, nil},
		Actions: []Canon{canon.Pack(canon.ActPass, canon.RefNone, canon.RefNone, 0)}}
	path := filepath.Join(t.TempDir(), "w.repro.json")
	if err := r.Write(path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(raw, []byte("\n")); n != 1 || raw[len(raw)-1] != '\n' {
		t.Fatalf("written repro is %d lines, want one line ending in a newline", n)
	}
	if bytes.Contains(raw, []byte("readable")) {
		t.Fatal("written repro carries the derived readable field")
	}
	// keys are sorted, as the corpus's map encoding is — not struct-field
	// order (a struct marshal starts with "format", the old mismatch)
	if !bytes.HasPrefix(raw, []byte(`{"actions":`)) {
		t.Fatalf("written repro is not key-sorted: %.80s", raw)
	}
	// the same canonical check TestReproCompactRoundTrip applies to the
	// corpus: the bytes Write emitted are exactly what compactReproJSON
	// produces, so a new recording cannot drift from the swept files
	canon, err := compactReproJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canon, raw) {
		t.Fatalf("written repro is not the canonical compact encoding:\n got %s\nwant %s", raw, canon)
	}
	back, err := ReadRepro(path)
	if err != nil {
		t.Fatal(err)
	}
	if back.Seed != r.Seed || len(back.Actions) != 1 || back.Actions[0] != r.Actions[0] {
		t.Fatal("written repro does not decode to the game it was built from")
	}
	if back.Cards[0][0] != "A&B <C>" {
		t.Fatalf("written repro changed a card name to %q", back.Cards[0][0])
	}
}

func reproCardsEqual(a, b [2][]string) bool {
	for s := range a {
		if len(a[s]) != len(b[s]) {
			return false
		}
		for i := range a[s] {
			if a[s][i] != b[s][i] {
				return false
			}
		}
	}
	return true
}
