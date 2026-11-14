package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkAt(t *testing.T, root, ref string, o CheckOptions) []error {
	t.Helper()
	mb, err := MergeBase(root, ref)
	if err != nil {
		t.Fatal(err)
	}
	base, err := ReadGit(root, mb)
	if err != nil {
		t.Fatal(err)
	}
	tr, err := Read(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	return Check(base, tr, o)
}

func joinErrs(errs []error) string {
	var s []string
	for _, e := range errs {
		s = append(s, e.Error())
	}
	return strings.Join(s, "\n")
}

// TestCheck: each case changes a branch off land and names the violation it
// must raise ("" for none).
func TestCheck(t *testing.T) {
	ticket := Dir + "/CounterKind/CtrTicket"
	ticketBody := "family: CounterKind\nname: CtrTicket\nvalue: 4\nforge: TICKET\ncensus: ctr TICKET all=23 std=0 mbc=0 sole=0\nstate: code 0\nstatus: %s\n"
	ordinal := func(from, to string) func(t *testing.T, root string) {
		return func(t *testing.T, root string) {
			p := filepath.Join(root, filepath.FromSlash(OrdinalsPath))
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), from) {
				t.Fatalf("ordinals has no %q", from)
			}
			if err := os.WriteFile(p, []byte(strings.Replace(string(b), from, to, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	entry := func(p, body string) func(t *testing.T, root string) {
		return func(t *testing.T, root string) { writeEntry(t, root, p, body) }
	}
	cases := []struct {
		name string
		edit func(t *testing.T, root string)
		o    CheckOptions
		want string
	}{
		{"activation", entry(ticket, fmt.Sprintf(ticketBody, "active\nticket: example-3")), CheckOptions{}, ""},
		{"blocked", entry(ticket, fmt.Sprintf(ticketBody, "blocked objspace:tcode")), CheckOptions{}, ""},
		{"new next entry", entry(Dir+"/ExtOp/ExtNew", "family: ExtOp\nname: ExtNew\nvalue: next\nstatus: reserved\n"), CheckOptions{}, ""},
		{"new next entry, final", entry(Dir+"/ExtOp/ExtNew", "family: ExtOp\nname: ExtNew\nvalue: next\nstatus: reserved\n"), CheckOptions{Final: true}, "ExtNew: still value: next"},
		{"value changed", entry(ticket, strings.Replace(fmt.Sprintf(ticketBody, "reserved"), "value: 4", "value: 9", 1)), CheckOptions{}, "value 4 at the base is now 9"},
		{"value to next", entry(ticket, strings.Replace(fmt.Sprintf(ticketBody, "reserved"), "value: 4", "value: next", 1)), CheckOptions{}, "value 4 at the base is now next"},
		{"forge changed", entry(ticket, strings.Replace(fmt.Sprintf(ticketBody, "reserved"), "forge: TICKET", "forge: TICKETS", 1)), CheckOptions{}, `forge changed from "TICKET" to "TICKETS"`},
		{"doc added", entry(ticket, fmt.Sprintf(ticketBody, "reserved")+"doc: late words\n"), CheckOptions{}, "doc changed"},
		{"deleted", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(ticket))); err != nil {
				t.Fatal(err)
			}
		}, CheckOptions{}, "CtrTicket: deleted"},
		{"hand constant adopted by hand", entry(Dir+"/ExtOp/ExtA", "family: ExtOp\nname: ExtA\nvalue: 1\nstatus: active\n"), CheckOptions{}, "ExtA is a hand ExtOp constant at the base"},
		{"ordinal changed", ordinal("ExtB\tExtOp\t3", "ExtB\tExtOp\t7"), CheckOptions{}, "row ExtB ExtOp 3 at the base is now ExtOp 7"},
		{"ordinal changed, allowed", ordinal("ExtB\tExtOp\t3", "ExtB\tExtOp\t7"), CheckOptions{AllowOrdinal: []string{"ExtB"}}, ""},
		{"ordinal vanished", ordinal("ExtB\tExtOp\t3\n", ""), CheckOptions{}, "row ExtB vanished"},
		{"invalid tree", entry(Dir+"/ExtOp/ExtTwin", "family: ExtOp\nname: ExtTwin\nvalue: 5\nstatus: reserved\n"), CheckOptions{}, "value 5 is already"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := landRepo(t)
			gitT(t, root, "checkout", "-q", "-b", "unit")
			c.edit(t, root)
			got := joinErrs(checkAt(t, root, "land", c.o))
			if c.want == "" && got != "" || !strings.Contains(got, c.want) {
				t.Fatalf("check:\n%s\nwant %q", got, c.want)
			}
		})
	}
}

// TestCheckHoldsToTheMergeBase: an entry land added after the branch forked
// is not one the branch deleted.
func TestCheckHoldsToTheMergeBase(t *testing.T) {
	root := landRepo(t)
	gitT(t, root, "checkout", "-q", "-b", "unit")
	writeEntry(t, root, Dir+"/ExtOp/ExtUnit", "family: ExtOp\nname: ExtUnit\nvalue: next\nstatus: reserved\n")
	commitAll(t, root, "unit")
	gitT(t, root, "checkout", "-q", "land")
	writeEntry(t, root, Dir+"/ExtOp/ExtLand", "family: ExtOp\nname: ExtLand\nvalue: 4\nstatus: active\n")
	commitAll(t, root, "land moves")
	gitT(t, root, "checkout", "-q", "unit")
	if errs := checkAt(t, root, "land", CheckOptions{}); len(errs) > 0 {
		t.Fatalf("check against the merge base: %s", joinErrs(errs))
	}
	tip, err := ReadGit(root, "land")
	if err != nil {
		t.Fatal(err)
	}
	tr := load(t, os.DirFS(root))
	if got := joinErrs(Check(tip, tr, CheckOptions{})); !strings.Contains(got, "ExtLand: deleted") {
		t.Errorf("check against land's tip: %q, want ExtLand reported (why the merge base is used)", got)
	}
}
