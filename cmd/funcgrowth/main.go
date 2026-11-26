// Command funcgrowth holds every function in codeshape.ShapeDirs to a
// base-relative growth rule (internal/codeshape/growth.go): there are no
// per-function rows shared between pull requests. A function over a threshold
// at the head (codeshape.LongFuncLines lines, codeshape.ComplexFuncThreshold
// cyclomatic complexity) may not have grown by more than a percentage of its
// size at the merge base (codeshape.GrowthPercent, floors
// codeshape.GrowthFloorLines and codeshape.ComplexGrowthFloor), and a
// function newly over a threshold, or one that crosses it, is refused. A
// function at or below both thresholds is never checked, and shrinking always
// passes.
//
// It measures the changed files of the diff from the merge base of -base and
// HEAD (origin/main by default) to the head, so a stacked merge-queue batch
// is measured against the batch's real base. It runs in `make lint` and in
// ci.yml's lint job (against the pull request's base there, origin/main
// otherwise); a run on an unchanged tree measures nothing and passes.
//
//	go run ./cmd/funcgrowth [-base origin/main] [-long-lines N] [-long-percent N] ...
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/adams-shaun/gorge/internal/codeshape"
)

func main() {
	base := flag.String("base", "origin/main", "base ref; the rule measures growth from the merge base of it and HEAD")
	longLines := flag.Int("long-lines", codeshape.LongFuncLines, "the line threshold (0 disables the lines metric)")
	longPct := flag.Int("long-percent", codeshape.GrowthPercent, "the allowed line growth, in percent of the base size")
	longFloor := flag.Int("long-floor", codeshape.GrowthFloorLines, "the smallest line allowance")
	cxThreshold := flag.Int("complex-threshold", codeshape.ComplexFuncThreshold, "the complexity threshold (0 disables the metric)")
	cxPct := flag.Int("complex-percent", codeshape.ComplexGrowthPercent, "the allowed complexity growth, in percent of the base")
	cxFloor := flag.Int("complex-floor", codeshape.ComplexGrowthFloor, "the smallest complexity allowance")
	flag.Parse()
	lim := codeshape.GrowthLimits{
		Lines: *longLines, LinesPercent: *longPct, LinesFloor: *longFloor,
		Complexity: *cxThreshold, ComplexityPercent: *cxPct, ComplexityFloor: *cxFloor,
	}
	fails, oks, n, mb, err := run(".", *base, lim)
	if err != nil {
		fmt.Fprintf(os.Stderr, "funcgrowth: %v\n", err)
		os.Exit(2)
	}
	noun := "changed files"
	if n == 1 {
		noun = "changed file"
	}
	fmt.Printf("funcgrowth: %d %s against base %s; %d functions over a threshold at head\n",
		n, noun, short(mb), len(oks)+len(fails))
	// The functions over a threshold at head are reported either way, so a
	// reviewer sees what was checked and what each was allowed.
	for _, o := range oks {
		fmt.Printf("  ok %s (%s:%d) %s %d (base %d, allowed %d)\n",
			o.Shape.Name, o.Shape.File, o.Shape.Line, o.Metric, o.Head, o.Base, o.Limit)
	}
	for _, f := range fails {
		fmt.Println(f.String())
	}
	if len(fails) > 0 {
		fmt.Fprintf(os.Stderr, "funcgrowth: %d functions grew past their allowance against base %s\n",
			len(fails), short(mb))
		os.Exit(1)
	}
	fmt.Println("funcgrowth: ok")
}

// run measures the changed files of base..head in dir and holds them to lim.
// The head side is the working tree (so an uncommitted change is checked
// locally; in CI the checkout is the head); the base side is extracted from
// git into a temp directory, never a worktree switch.
func run(dir, baseRef string, lim codeshape.GrowthLimits) (fails, oks []codeshape.GrowthFailure, changed int, mergeBase string, err error) {
	root, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, nil, 0, "", fmt.Errorf("find the repo root: %v", err)
	}
	root = strings.TrimSpace(root)
	// The head side is the working tree; the base is the merge base of the
	// given ref and HEAD, so a branch is measured against its own fork point
	// and a stacked batch against the batch's base.
	mb, err := git(dir, "merge-base", "HEAD", baseRef)
	if err != nil {
		return nil, nil, 0, "", fmt.Errorf("merge-base HEAD %s: %v", baseRef, err)
	}
	mb = strings.TrimSpace(mb)
	// The changed files of the merge base against the working tree (a
	// one-commit diff is commit-vs-worktree): uncommitted local growth is
	// checked too, and in CI the checkout is the head. --no-renames: a moved
	// function's two files both enter the diff, so both sides of the move are
	// measured and the key match sees the move.
	out, err := git(dir, append([]string{"diff", "--name-only", "-z", "--no-renames", mb, "--"},
		codeshape.ShapeDirs...)...)
	if err != nil {
		return nil, nil, 0, "", fmt.Errorf("diff %s..worktree: %v", mb[:12], err)
	}
	var rels []string
	for _, rel := range strings.Split(strings.TrimRight(out, "\x00"), "\x00") {
		if rel == "" || !isShapeFile(rel) {
			continue
		}
		rels = append(rels, rel)
	}
	if len(rels) == 0 {
		return nil, nil, 0, mb, nil
	}
	tmp, err := os.MkdirTemp("", "funcgrowth-base")
	if err != nil {
		return nil, nil, 0, "", err
	}
	defer os.RemoveAll(tmp)
	headFiles := map[string]string{}
	baseFiles := map[string]string{}
	for _, rel := range rels {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if _, err := os.Stat(abs); err == nil {
			headFiles[rel] = abs // deleted in the head: the base side alone says nothing about growth
		}
		b, err := gitBytes(dir, "show", mb+":"+rel)
		if err != nil {
			continue // not at the base: a new file, whose functions are all new
		}
		p := filepath.Join(tmp, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, nil, 0, "", err
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return nil, nil, 0, "", err
		}
		baseFiles[rel] = p
	}
	baseShapes, err := codeshape.MeasureFuncShapesIn(baseFiles)
	if err != nil {
		return nil, nil, 0, "", err
	}
	headShapes, err := codeshape.MeasureFuncShapesIn(headFiles)
	if err != nil {
		return nil, nil, 0, "", err
	}
	fails, oks = codeshape.CheckFuncGrowth(baseShapes, headShapes, lim)
	return fails, oks, len(rels), mb, nil
}

// isShapeFile reports whether rel is a non-test .go file under one of
// codeshape.ShapeDirs.
func isShapeFile(rel string) bool {
	if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
		return false
	}
	for _, dir := range codeshape.ShapeDirs {
		if rel == dir || strings.HasPrefix(rel, dir+"/") {
			return true
		}
	}
	return false
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func git(dir string, args ...string) (string, error) {
	out, err := gitBytes(dir, args...)
	return string(out), err
}

func gitBytes(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out.Bytes(), nil
}
