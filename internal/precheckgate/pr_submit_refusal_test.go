package precheckgate

// pr-precheck's refuse-on-red step (REFUSE_ON_RED=apply) commits the refusals on the branch, moving HEAD under
// pr-submit.sh. pr-submit must check that commit (one more pr-precheck run) and push it, not end "the tree moved".

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrSubmitRechecksARefusalCommit(t *testing.T) {
	r := newSubmitRepo(t)
	runs := filepath.Join(r.root, "precheck.runs")
	// The stub commits a refusal the first time, as refuse-on-red.sh's apply mode does; the rerun sees it and passes.
	r.write(t, filepath.Join(r.wt, "scripts", "pr-precheck.sh"), `#!/usr/bin/env bash
echo run >>"`+runs+`"
if ! git log -1 --format=%s | grep -q '^dirac: refuse '; then
  mkdir -p dirac/support/refused && echo '{}' >dirac/support/refused/card-a.json
  git add -- dirac && git commit -q -m 'dirac: refuse 1 cards the rules fix diverges on' -m 'Rule: CR 601.2' -- dirac
  echo 'refuse-on-red: refused 1 cards'
fi
echo 'pr-precheck: ok'
`)
	gitIn(t, r.wt, "add", "scripts/pr-precheck.sh")
	r.commit(t, r.wt, "feat: add b", map[string]string{"b.txt": "b\n"})
	out, rc := r.run(t, r.wt)
	if strings.Contains(out, "the tree moved during pr-precheck") || rc != 0 {
		t.Fatalf("rc=%d, want a push of the refusal commit:\n%s", rc, out)
	}
	if got := gitOut(t, r.origin, "log", "-1", "--format=%s", "refs/heads/feat"); got != "dirac: refuse 1 cards the rules fix diverges on" {
		t.Fatalf("pushed head = %q, want the refusal commit", got)
	}
	b, err := os.ReadFile(runs)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(b), "run"); n != 2 {
		t.Fatalf("pr-precheck ran %d times, want 2 (the refusal commit is checked too)", n)
	}
}

func TestPrSubmitRefusesAnOtherMoveDuringPrecheck(t *testing.T) {
	r := newSubmitRepo(t)
	r.write(t, filepath.Join(r.wt, "scripts", "pr-precheck.sh"), `#!/usr/bin/env bash
git commit -q --allow-empty -m 'something else'
echo 'pr-precheck: ok'
`)
	gitIn(t, r.wt, "add", "scripts/pr-precheck.sh")
	r.commit(t, r.wt, "feat: add b", map[string]string{"b.txt": "b\n"})
	out, rc := r.run(t, r.wt)
	if rc != 1 || !strings.Contains(out, "the tree moved during pr-precheck; nothing pushed") {
		t.Fatalf("rc=%d, want the move refused:\n%s", rc, out)
	}
	if r.remoteHasBranch(t) {
		t.Error("a branch that moved during precheck was pushed")
	}
}
