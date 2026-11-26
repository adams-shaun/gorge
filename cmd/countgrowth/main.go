// Command countgrowth holds the tree-wide count metrics to a base-relative
// growth rule (internal/codeshape/countgrowth.go): there are no stored
// ceilings shared between pull requests. The shape census
// (internal/codeshape's CountMetrics: interface widths, the *Engine surface,
// string-keyed dispatch and parameter leaks) and the no-strings counts
// (internal/codeshape's StringScopes: strings.*/fmt.Sprint*/strconv.* calls
// and map[string] types per scope) may not have grown by more than
// CountGrowthPercent of their value at the merge base, with a floor of
// CountGrowthFloor; a metric absent at the merge base is held to the floor
// alone, and shrinking always passes.
//
// It measures the whole trees at the merge base of -base and HEAD
// (origin/main by default) and at the head (the working tree, so an
// uncommitted change is checked locally and an untracked file is counted),
// so a stacked merge-queue batch is measured against the batch's real base.
// It runs in `make lint` and in ci.yml's lint job (against the pull
// request's base there, origin/main otherwise); a run on an unchanged tree
// measures and passes.
//
//	go run ./cmd/countgrowth [-base origin/main] [-percent N] [-floor N]
package main

import (
	"archive/tar"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/adams-shaun/gorge/internal/codeshape"
)

func main() {
	base := flag.String("base", "origin/main", "base ref; the rule measures growth from the merge base of it and HEAD")
	pct := flag.Int("percent", codeshape.CountGrowthPercent, "the allowed growth, in percent of the base value")
	floor := flag.Int("floor", codeshape.CountGrowthFloor, "the smallest allowance")
	flag.Parse()
	fails, oks, n, mb, err := run(".", *base, *pct, *floor)
	if err != nil {
		fmt.Fprintf(os.Stderr, "countgrowth: %v\n", err)
		os.Exit(2)
	}
	fmt.Printf("countgrowth: %d metrics against base %s\n", n, short(mb))
	// The metrics that grew within their allowance are reported, so a
	// reviewer sees what was checked and what each was allowed.
	for _, o := range oks {
		fmt.Printf("  ok %s %d (base %d, allowed %d)\n", o.Name, o.Head, o.Base, o.Limit)
	}
	for _, f := range fails {
		fmt.Println(f.String())
	}
	if len(fails) > 0 {
		fmt.Fprintf(os.Stderr, "countgrowth: %d metrics grew past their allowance against base %s\n",
			len(fails), short(mb))
		os.Exit(1)
	}
	fmt.Println("countgrowth: ok")
}

// run measures the whole count trees at the head (the working tree) and at
// the merge base (extracted from git into a temp directory, never a
// worktree switch), and holds every metric to the growth rule.
func run(dir, baseRef string, pct, floor int) (fails, oks []codeshape.CountFailure, n int, mb string, err error) {
	root, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, nil, 0, "", fmt.Errorf("find the repo root: %v", err)
	}
	root = strings.TrimSpace(root)
	mb, err = git(dir, "merge-base", "HEAD", baseRef)
	if err != nil {
		return nil, nil, 0, "", fmt.Errorf("merge-base HEAD %s: %v", baseRef, err)
	}
	mb = strings.TrimSpace(mb)

	// The head side is the working tree.
	head, err := codeshape.Measure(root)
	if err != nil {
		return nil, nil, 0, "", fmt.Errorf("measure the head: %v", err)
	}
	if head.TrigmatchBoardMethods == 0 {
		return nil, nil, 0, "", fmt.Errorf("trigmatch.Board measured no methods: " +
			"engine/rules/trigmatch/board.go's `type Board interface` moved or was renamed; " +
			"update codeshape.Measure so the count cannot read zero")
	}
	headStrings, err := measureStringsScopes(root)
	if err != nil {
		return nil, nil, 0, "", err
	}

	// The base side, extracted from git.
	tmp, err := os.MkdirTemp("", "countgrowth-base")
	if err != nil {
		return nil, nil, 0, "", err
	}
	defer os.RemoveAll(tmp)
	if err := extractBase(dir, mb, tmp); err != nil {
		return nil, nil, 0, "", err
	}
	baseM, err := codeshape.Measure(tmp)
	if err != nil {
		return nil, nil, 0, "", fmt.Errorf("measure the base %s: %v", short(mb), err)
	}

	var samples []codeshape.CountSample
	for _, cm := range codeshape.CountMetrics() {
		samples = append(samples, codeshape.CountSample{Name: cm.Name,
			Base: cm.Get(baseM), Head: cm.Get(head), Advice: cm.Advice(head)})
	}
	for _, scope := range codeshape.StringScopes {
		var bs codeshape.StringsCounts
		if dirExists(filepath.Join(tmp, scope)) {
			if bs, err = codeshape.MeasureStrings(tmp, scope, map[string]bool{}); err != nil {
				return nil, nil, 0, "", fmt.Errorf("measure the base %s strings: %v", scope, err)
			}
		} // else the scope is absent at the base: every count is 0 there
		hs := headStrings[scope]
		for _, sm := range codeshape.StringCountMetrics() {
			samples = append(samples, codeshape.CountSample{Name: scope + ": " + sm.Name,
				Base: sm.Get(bs), Head: sm.Get(hs), Advice: sm.Advice})
		}
	}
	fails, oks = codeshape.CheckCountGrowth(samples, pct, floor)
	return fails, oks, len(samples), mb, nil
}

// measureStringsScopes measures every string scope of the working tree.
func measureStringsScopes(root string) (map[string]codeshape.StringsCounts, error) {
	out := map[string]codeshape.StringsCounts{}
	for _, scope := range codeshape.StringScopes {
		// A scope absent from the tree (a new one, or one retired) measures
		// zero, like a metric absent at the base.
		if !dirExists(filepath.Join(root, scope)) {
			out[scope] = codeshape.StringsCounts{}
			continue
		}
		c, err := codeshape.MeasureStrings(root, scope, map[string]bool{})
		if err != nil {
			return nil, fmt.Errorf("measure the head %s strings: %v", scope, err)
		}
		out[scope] = c
	}
	return out, nil
}

// extractBase extracts the count trees at mb into tmp. A tree absent at the
// base (a new scope) is skipped: its metrics measure 0 there.
func extractBase(dir, mb, tmp string) error {
	var dirs []string
	seenDir := map[string]bool{}
	for _, d := range append(append([]string{}, codeshape.ScannedDirs...), codeshape.StringScopes...) {
		if !seenDir[d] {
			seenDir[d] = true
			dirs = append(dirs, d)
		}
	}
	for _, d := range dirs {
		out, err := git(dir, "ls-tree", "--name-only", mb, "--", d)
		if err != nil {
			return fmt.Errorf("ls-tree %s %s: %v", short(mb), d, err)
		}
		if strings.TrimSpace(out) == "" {
			continue
		}
		var buf bytes.Buffer
		cmd := exec.Command("git", "archive", "--format=tar", mb, "--", d)
		cmd.Dir = dir
		cmd.Stdout = &buf
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git archive %s %s: %v", short(mb), d, err)
		}
		tr := tar.NewReader(&buf)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("read the archive of %s: %v", d, err)
			}
			if hdr.Typeflag != tar.TypeReg {
				continue
			}
			p := filepath.Join(tmp, filepath.FromSlash(hdr.Name))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err = io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out.String(), nil
}
