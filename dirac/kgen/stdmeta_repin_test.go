package kgen

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestStdMetaPinRowRewrites drives the standard-meta pin through repinFile on a
// copy of stdmeta_test.go: a rise rewrites the const line in place, a fall is
// refused with the file untouched — the same rule the acceptance pins follow.
func TestStdMetaPinRowRewrites(t *testing.T) {
	src, err := os.ReadFile("stdmeta_test.go")
	if err != nil {
		t.Fatal(err)
	}
	m := stdMetaPinRow.FindStringSubmatch(string(src))
	if m == nil {
		t.Fatal("no pin row in stdmeta_test.go")
	}
	nums := strings.Split(strings.TrimSpace(strings.TrimPrefix(m[0], m[1])), ", ")
	if len(nums) != 2 {
		t.Fatalf("pin row holds %q, want two numbers", strings.TrimPrefix(m[0], m[1]))
	}
	var pin [2]int
	for i, n := range nums {
		if pin[i], err = strconv.Atoi(n); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "stdmeta_test.go")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	rise := [2]int{pin[0] + 1, pin[1] - 1}
	if _, err := repinFile(path, stdMetaPinRow, "standard-meta", pin, rise, ""); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "stdMetaSupportedPin, stdMetaRefusedPin = "+strconv.Itoa(rise[0])+", "+strconv.Itoa(rise[1])) {
		t.Errorf("a rise did not rewrite the const line:\n%s", after)
	}
	if _, err := repinFile(path, stdMetaPinRow, "standard-meta", rise, pin, ""); err == nil {
		t.Fatal("a fall with no reason was accepted")
	}
	if _, err := repinFile(path, stdMetaPinRow, "standard-meta", rise, pin, "a refusal that closes a divergence"); err != nil {
		t.Fatal(err)
	}
	after, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := stdMetaPinRow.ReplaceAllString(string(src), "${1}"+strconv.Itoa(pin[0])+", "+strconv.Itoa(pin[1])); string(after) != want {
		t.Errorf("the fall with a reason did not leave the row at the pin:\n%s\nwant\n%s", after, want)
	}
}
