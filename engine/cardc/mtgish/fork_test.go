package mtgish

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestMtgishForkRegister(t *testing.T) {
	root, err := ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	lock, err := ReadLock(filepath.Join(root, "engine", "cardc", "MTGISH.lock"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ReadForkRegister(filepath.Join(root, "engine", "cardc", "testdata", "mtgish-fork.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var carried []string
	for _, r := range rows {
		if seen[r.SHA] {
			t.Errorf("duplicate sha %s", r.SHA)
		}
		seen[r.SHA] = true
		if r.Status == "carried" {
			carried = append(carried, r.SHA+" "+strconv.Itoa(r.Issue))
		}
	}
	var locked []string
	for _, p := range lock.Patches {
		locked = append(locked, p.SHA+" "+strconv.Itoa(p.Issue))
	}
	sort.Strings(carried)
	sort.Strings(locked)
	if strings.Join(carried, "\n") != strings.Join(locked, "\n") {
		t.Errorf("carried rows and lock patch lines differ:\nregister carried: %q\nlock patches:     %q", carried, locked)
	}
	if len(lock.Patches) == 0 && lock.Ref != lock.Base {
		t.Errorf("lock has no patch line but ref %s != base %s", lock.Ref, lock.Base)
	}
}

func TestReadForkRegister(t *testing.T) {
	const hdr = "sha\tupstream_issue\tclass\tstatus\tcards\n"
	sha := strings.Repeat("ab", 20)
	row := func(c ...string) string { return strings.Join(c, "\t") + "\n" }
	write := func(body string) string {
		p := filepath.Join(t.TempDir(), "r.tsv")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := hdr + "# note\n" +
		row(sha, "3", "up-to-exact", "carried", "Card A, Card B") +
		row(strings.Repeat("cd", 20), "1", "morph-face-up", "fixed-upstream", "-") +
		row(strings.Repeat("ef", 20), "2", "haste-dropped", "dropped", "Card C")
	rows, err := ReadForkRegister(write(good))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].Issue != 3 || rows[0].Status != "carried" || len(rows[0].Cards) != 2 ||
		rows[0].Cards[1] != "Card B" || rows[1].Cards != nil || rows[2].Class != "haste-dropped" {
		t.Errorf("parsed %+v", rows)
	}
	bad := map[string]string{
		"no header":     row(sha, "3", "up-to-exact", "carried", "-"),
		"wrong header":  "sha\tissue\n",
		"short sha":     hdr + row("abc", "3", "up-to-exact", "carried", "-"),
		"upper sha":     hdr + row(strings.ToUpper(sha), "3", "up-to-exact", "carried", "-"),
		"zero issue":    hdr + row(sha, "0", "up-to-exact", "carried", "-"),
		"issue text":    hdr + row(sha, "x", "up-to-exact", "carried", "-"),
		"bad class":     hdr + row(sha, "3", "Up To", "carried", "-"),
		"bad status":    hdr + row(sha, "3", "up-to-exact", "pending", "-"),
		"empty cards":   hdr + row(sha, "3", "up-to-exact", "carried", ""),
		"too few cols":  hdr + row(sha, "3", "up-to-exact", "carried"),
		"empty card nm": hdr + row(sha, "3", "up-to-exact", "carried", "A,,B"),
	}
	for name, body := range bad {
		if _, err := ReadForkRegister(write(body)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := ReadForkRegister(write(hdr + row(sha, "0", "x", "carried", "-"))); err == nil || !strings.Contains(err.Error(), ":2:") {
		t.Errorf("error does not name its line: %v", err)
	}
}

func TestForkWorkflow(t *testing.T) {
	root, err := ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "engine", "cardc", "fork", "regenerate.yml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, r := range []string{"just parser_generate_mtgish", "just rust_build_ronl_to_bincode", "just rust_build_bincode_to_jsonl", "just convert_to_json"} {
		if !strings.Contains(s, r) {
			t.Errorf("workflow lacks %q", r)
		}
	}
	for _, no := range []string{"pull_request", "secrets.", "git push"} {
		if strings.Contains(s, no) {
			t.Errorf("workflow contains %q", no)
		}
	}
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if last := strings.TrimSpace(lines[len(lines)-1]); last != "run: git diff --exit-code --stat -- data/mtgish.lines.json" {
		t.Errorf("workflow ends with %q, want the git diff check", last)
	}
}
