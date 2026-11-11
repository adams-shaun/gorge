package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestOpenAttractionRecordsTheNoAttractionDeckDegrade pins CR 701.51a: with no
// Attraction deck, opening an Attraction resolves as a recorded no-op.
func TestOpenAttractionRecordsTheNoAttractionDeckDegrade(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Source: 0}, sa(t, "SP$ OpenAttraction"))
	if len(h.log) != 1 || h.log[0].Kind != events.Note {
		t.Fatalf("OpenAttraction log = %+v, want one Note", h.log)
	}
	if h.log[0].Text != "open an Attraction (no Attraction deck)" {
		t.Fatalf("OpenAttraction note = %q", h.log[0].Text)
	}
	if h.log[0].Obj != 0 {
		t.Fatalf("sourceless Note carries Obj %d", h.log[0].Obj)
	}
}

// TestOpenAttractionNeverEmitsUnimplemented guards the registration: the API
// is Supported and neither the textual nor the compiled dispatch path reaches
// the unimplemented-API fallback; a chained SubAbility$ still resolves.
func TestOpenAttractionNeverEmitsUnimplemented(t *testing.T) {
	if !Supported()["api:OpenAttraction"] {
		t.Fatal("api:OpenAttraction not registered in Supported()")
	}
	check := func(name string, h *fakeHost) {
		t.Helper()
		if len(h.log) == 0 {
			t.Fatalf("%s: no events", name)
		}
		for _, ev := range h.log {
			if strings.HasPrefix(ev.Text, "unimplemented API") {
				t.Fatalf("%s: fallback Note %q", name, ev.Text)
			}
		}
	}
	h := newHost(t, 2)
	Resolve(h, &Ctx{}, &cards.SA{Kind: "SP", API: "OpenAttraction"})
	check("textual SA", h)

	var ran bool
	Register("TestOpenAttractionChain", func(_ Host, _ *Ctx, _ *cards.SA) { ran = true })
	t.Cleanup(func() { unregister("TestOpenAttractionChain") })
	c, d := cards.ParseBytes("oa.txt", []byte(
		"Name:OA probe\nTypes:Sorcery\n"+
			"A:SP$ OpenAttraction | SubAbility$ Next\n"+
			"SVar:Next:DB$ TestOpenAttractionChain\nOracle:x\n"))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	h2 := newHost(t, 2)
	Resolve(h2, &Ctx{}, c.Faces[0].Abilities[0])
	check("parsed SA", h2)
	if !ran {
		t.Fatal("SubAbility$ after OpenAttraction did not resolve")
	}
}
