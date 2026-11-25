package scripts

// TestOneRunCheckAcceptsExactNamesAndOneSubtest guards test-one's RUN check for the
// packages too big to run whole (./engine/rules, ./dirac/oracle): exact top-level names
// pass, optionally with one /subtest pattern (re-record or replay a single pin of
// TestPinManifest); a broad pattern is refused.

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestOneRunCheckAcceptsExactNamesAndOneSubtest(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash on PATH")
	}
	b, err := os.ReadFile("test-targets.sh")
	if err != nil {
		t.Fatal(err)
	}
	var check string
	for _, line := range strings.Split(string(b), "\n") {
		if l := strings.TrimSpace(line); strings.HasPrefix(l, "[[ $RUN =~ ") {
			check = strings.TrimSuffix(l, " ||")
			break
		}
	}
	if check == "" {
		t.Fatal("test-targets.sh: no [[ $RUN =~ ... ]] check found")
	}
	for _, c := range []struct {
		run string
		ok  bool
	}{
		{`^TestX$`, true},
		{`^TestX$|^TestY$`, true},
		{`^(TestX|TestY)$`, false},
		{`^TestPinManifest$/^TestX\.0$`, true},
		{`^TestPinManifest$/^(TestX\.0|TestY\.1)$`, true},
		{`^TestPinManifest$/^TestX\.0$/^Y$`, false},
		{`TestX`, false},
		{`^Test.*$`, false},
		{`^TestPinManifest$/.*`, false},
	} {
		cmd := exec.Command("bash", "-c", check)
		cmd.Env = append(os.Environ(), "RUN="+c.run)
		err := cmd.Run()
		if got := err == nil; got != c.ok {
			t.Errorf("RUN=%q: accepted %v, want %v", c.run, got, c.ok)
		}
	}
}
