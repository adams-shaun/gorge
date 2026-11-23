// Command checkratchet refuses a pull request that lowers a shrink-only ratchet. Run it in the
// lint job with the pull request's base and head commits: ratchets are lowered on main by a
// generator, after the merge, never by a branch (docs/agents/invariants.md, invariant 8).
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/adams-shaun/gorge/internal/codeshape"
)

func main() {
	if len(os.Args) < 2 || len(os.Args) > 4 {
		fmt.Fprintln(os.Stderr, "usage: checkratchet BASE [HEAD_REF [HEAD_SHA]]")
		os.Exit(2)
	}
	base := os.Args[1]
	headRef, head := "", "HEAD"
	if len(os.Args) > 2 {
		headRef = os.Args[2]
	}
	if len(os.Args) > 3 {
		head = os.Args[3]
	}
	if codeshape.GuardExempt(headRef) {
		fmt.Printf("checkratchet: %s lowers ratchets by design; not checked\n", headRef)
		return
	}
	lowered, mb, err := check(".", base, head)
	if err != nil {
		fail("%v", err)
	}
	if len(lowered) > 0 {
		fmt.Fprint(os.Stderr, codeshape.LoweringReport(lowered))
		fmt.Fprintln(os.Stderr, "A ratchet is lowered on main after merge, so a lowering branch cannot race a branch that grows the same function. Drop the lowering from this pull request. (A function deleted with its ceiling row: leave the row; the daily job, CODESHAPE_LOWER=1 go test -run '^TestFuncCeilingsLowerOnly$' ./internal/codeshape, drops a stale one. It lowers only func_ceilings.tsv: a ratchet_test.go constant is lowered by a follow-up change after the merge.)")
		os.Exit(1)
	}
	fmt.Printf("checkratchet: no ratchet lowered from %s\n", mb[:12])
}

const ceilingsPath = "internal/codeshape/testdata/func_ceilings.tsv"

// check compares head's tree with its merge base with base and returns what it lowers, with the
// merge base's sha. It reads both trees from git, never the working tree: the checkout in a
// pull_request job is the merge ref (current main merged onto the branch), whose rows differ
// from the branch's whenever main lowered one after the pull request was opened.
func check(dir, base, head string) (lowered []string, mb string, err error) {
	head, err = git(dir, "rev-parse", "--verify", head+"^{commit}")
	if err != nil {
		return nil, "", fmt.Errorf("resolve head: %v", err)
	}
	head = strings.TrimSpace(head)
	mb, err = git(dir, "merge-base", base, head)
	if err != nil {
		return nil, "", fmt.Errorf("merge-base %s %s: %v", base, head, err)
	}
	mb = strings.TrimSpace(mb)
	// The merge base's files, not head's: a branch that deletes or renames a ratchet file is seen.
	// ls-tree takes literal paths and no glob, so the names are filtered here.
	names, err := treePaths(dir, mb)
	if err != nil {
		return nil, "", err
	}
	headNames, err := treePaths(dir, head)
	if err != nil {
		return nil, "", err
	}
	headHas := make(map[string]bool, len(headNames))
	for _, name := range headNames {
		headHas[name] = true
	}
	type file struct {
		path string
		run  func(base, head []byte) ([]string, error)
	}
	files := []file{{path: ceilingsPath, run: codeshape.LoweredCeilings}}
	for _, name := range names {
		if strings.HasSuffix(name, "ratchet_test.go") {
			files = append(files, file{path: name, run: codeshape.LoweredConstants})
		}
	}
	for _, f := range files {
		old, err := git(dir, "show", mb+":"+f.path)
		if err != nil {
			continue // the file did not exist at the merge base: nothing to lower
		}
		if !headHas[f.path] {
			lowered = append(lowered, f.path+": file removed")
			continue
		}
		now, err := git(dir, "show", head+":"+f.path)
		if err != nil {
			return nil, "", fmt.Errorf("read %s at %s: %v", f.path, head[:12], err)
		}
		got, err := f.run([]byte(old), []byte(now))
		if err != nil {
			return nil, "", fmt.Errorf("%s: %v", f.path, err)
		}
		for _, l := range got {
			lowered = append(lowered, f.path+": "+l)
		}
	}
	return lowered, mb, nil
}

func treePaths(dir, rev string) ([]string, error) {
	out, err := git(dir, "ls-tree", "-r", "--name-only", rev)
	if err != nil {
		return nil, fmt.Errorf("ls-tree %s: %v", rev[:12], err)
	}
	return strings.Split(out, "\n"), nil
}

func git(dir string, args ...string) (string, error) {
	var out, stderr bytes.Buffer
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out.String(), nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "checkratchet: "+format+"\n", args...)
	os.Exit(2)
}
