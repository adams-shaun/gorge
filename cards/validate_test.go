package cards

import "testing"

func TestCoverageCountsAndRanksMissingPrimitives(t *testing.T) {
	r := NewRegistry()
	add := func(src string) {
		c, _ := ParseBytes("f.txt", []byte(src))
		c.Link()
		for _, f := range c.Faces {
			f.ApplyIntrinsics()
		}
		r.Add(c)
	}
	add("Name:A\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n")
	add("Name:B\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 2\nOracle:x\n")
	add("Name:C\nTypes:Sorcery\nA:SP$ Mill | NumCards$ 3\nOracle:x\n")
	add("Name:D\nTypes:Sorcery\nA:SP$ Mill | SubAbility$ X\nSVar:X:DB$ Vote\nOracle:x\n")

	cv := r.Coverage(map[string]bool{"api:Draw": true})
	if cv.Cards != 4 {
		t.Fatalf("Cards = %d", cv.Cards)
	}
	if cv.Supported != 2 {
		t.Fatalf("Supported = %d, want 2", cv.Supported)
	}
	if cv.Missing["api:Mill"] != 2 {
		t.Errorf("api:Mill blocks %d cards, want 2", cv.Missing["api:Mill"])
	}
	if cv.Missing["api:Vote"] != 1 {
		t.Errorf("api:Vote blocks %d cards, want 1", cv.Missing["api:Vote"])
	}
	top := cv.TopMissing(1)
	if len(top) != 1 || top[0].Name != "api:Mill" || top[0].Cards != 2 {
		t.Fatalf("TopMissing = %+v", top)
	}
}

func TestUnsupportedNamesTheMissingPrimitive(t *testing.T) {
	c, _ := ParseBytes("f.txt", []byte("Name:E\nTypes:Sorcery\nA:SP$ Mill | NumCards$ 3\nOracle:x\n"))
	c.Link()
	r := NewRegistry()
	r.Add(c)
	miss := r.Unsupported(c, map[string]bool{"api:Draw": true})
	if len(miss) != 1 || miss[0] != "api:Mill" {
		t.Fatalf("Unsupported = %v", miss)
	}
	if got := r.Unsupported(c, map[string]bool{"api:Mill": true}); len(got) != 0 {
		t.Fatalf("Unsupported = %v, want empty", got)
	}
}

// TestCoverageIgnoresNamelessCard guards the fix for a known Task 4 defect: a
// zero-byte .txt parses without error into a Card holding one Face with every
// field at its zero value. That Face has no primitives, so Unsupported
// reports it as fully supported — a nameless, contentless artifact would
// otherwise inflate the coverage report's "playable cards" headline number.
// A card with no name on any face must not count as either supported or
// blocked.
func TestCoverageIgnoresNamelessCard(t *testing.T) {
	real, _ := ParseBytes("real.txt", []byte("Name:F\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"))
	real.Link()
	for _, f := range real.Faces {
		f.ApplyIntrinsics()
	}
	empty, _ := ParseBytes("empty.txt", []byte(""))
	empty.Link()

	r := NewRegistry()
	r.Add(real)
	r.Add(empty)

	cv := r.Coverage(map[string]bool{"api:Draw": true})
	if cv.Cards != 1 {
		t.Fatalf("Cards = %d, want 1 (nameless card must not count)", cv.Cards)
	}
	if cv.Supported != 1 {
		t.Fatalf("Supported = %d, want 1 (nameless card must not count as supported)", cv.Supported)
	}
}
