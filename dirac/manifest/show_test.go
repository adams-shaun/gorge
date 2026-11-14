package manifest

import (
	"strings"
	"testing"
)

func TestFind(t *testing.T) {
	tr := load(t, fixture(t))
	for q, want := range map[string]string{
		"CtrTicket":                               "dirac/manifest/CounterKind/CtrTicket",
		"ticket":                                  "dirac/manifest/CounterKind/CtrTicket",
		"CounterKind/CtrTicket":                   "dirac/manifest/CounterKind/CtrTicket",
		"dirac/manifest/CounterKind/CtrTicket":    "dirac/manifest/CounterKind/CtrTicket",
		"manifest:reserved:CounterKind/CtrTicket": "dirac/manifest/CounterKind/CtrTicket",
		"voyage":                                  "dirac/manifest/CounterKind/CtrVoyage",
		"Foo":                                     "dirac/manifest/TrigEvent/TrigFoo",
		"nothing":                                 "",
	} {
		var got []string
		for _, e := range tr.Find(q) {
			got = append(got, e.Path)
		}
		if strings.Join(got, " ") != want {
			t.Errorf("Find(%q) = %v, want %q", q, got, want)
		}
	}
}

func TestShow(t *testing.T) {
	tr := load(t, fixture(t))
	for p, want := range map[string]string{
		Dir + "/CounterKind/CtrTicket": "dirac/manifest/CounterKind/CtrTicket value 4 reserved state code 0\n" +
			"\tforge: TICKET\n\tcensus: ctr TICKET all=23 std=0 mbc=0 sole=0\n",
		Dir + "/ExtOp/ExtFoo": "dirac/manifest/ExtOp/ExtFoo value next (provisional 2) reserved\n" +
			"\tcensus: api Foo all=3 std=1 mbc=0 sole=1\n",
		Dir + "/Op/OpHot": "dirac/manifest/Op/OpHot value next (provisional 158) active\n" +
			"\tapproval: hot-path review\n",
	} {
		if got := tr.Entry(p).Show(); got != want {
			t.Errorf("Show %s:\n%q\nwant\n%q", p, got, want)
		}
	}
}

// TestFormatRoundTrips: Format writes what ParseEntry reads back, and a
// fixture written in canonical order formats to its own bytes.
func TestFormatRoundTrips(t *testing.T) {
	for _, e := range load(t, fixture(t)).Entries {
		back, err := ParseEntry(e.Path, e.Format())
		if err != nil {
			t.Fatalf("%s: %v", e.Path, err)
		}
		if string(back.Format()) != string(e.Format()) {
			t.Errorf("%s: round trip\n%s\nbecame\n%s", e.Path, e.Format(), back.Format())
		}
		if !e.Next && string(e.Format()) != string(e.raw) {
			t.Errorf("%s: Format\n%s\nwant the fixture's bytes\n%s", e.Path, e.Format(), e.raw)
		}
	}
}
