package main

import (
	"flag"
	"fmt"
	"os"
	"slices"

	"github.com/adams-shaun/gorge/dirac/oracle"
)

// reproRemap is -remap.
var reproRemap bool

func init() {
	flag.BoolVar(&reproRemap, "remap", false, "-repro: rewrite the repro's vocabulary picks (creature type, land type, name, colour) from their labels after a vocabulary change, and write it back in place")
}

// remapRepro writes rep back to path with the ints the replay served for its
// labelled picks (pol was run with Remap). It refuses a replay that skipped an
// action, ended before the recorded prefix, or failed inside it: the rewritten
// prefix would not be the game that was recorded.
func remapRepro(path string, rep *oracle.Repro, pol *oracle.ScriptPolicy, res *oracle.Result) int {
	n := len(rep.Actions)
	switch {
	case res.Div != nil && res.Div.At < n:
		fmt.Fprintf(os.Stderr, "kfuzz: -remap: the replay failed at decision %d (%s %s): %s\n", res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail)
		return 1
	case pol.Skipped > 0:
		fmt.Fprintf(os.Stderr, "kfuzz: -remap: the replay skipped %d actions; the path has drifted past what a label can repair\n", pol.Skipped)
		return 1
	case len(res.Actions) < n:
		fmt.Fprintf(os.Stderr, "kfuzz: -remap: the replay served %d of %d actions\n", len(res.Actions), n)
		return 1
	}
	next := *rep
	next.Actions = slices.Clone(res.Actions[:n])
	next.Labels = nil
	for _, l := range res.Labels {
		if l.At < n {
			next.Labels = append(next.Labels, l)
		}
	}
	changed := 0
	for i := range next.Actions {
		if next.Actions[i] != rep.Actions[i] {
			changed++
		}
	}
	if changed == 0 && slices.Equal(next.Labels, rep.Labels) {
		fmt.Printf("remap: %s already agrees with its labels (%d labelled picks)\n", path, len(rep.Labels))
		return 0
	}
	if err := next.Write(path); err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 1
	}
	fmt.Printf("remap: %s: %d actions rewritten, %d labelled picks\n", path, changed, len(next.Labels))
	return 0
}

// rerunRepro is rep.Rerun, or with -remap the same replay under a Remap policy
// whose result is written back to path (remapRepro).
func rerunRepro(rep *oracle.Repro, path string, m *oracle.Matchup, mk oracle.SideFactory) oracle.Result {
	if !reproRemap {
		return rep.Rerun(m, mk, 200)
	}
	pol := rep.Script()
	pol.Remap = true
	res := rep.RerunWith(m, mk, 200, pol)
	remapRepro(path, rep, pol, &res)
	return res
}
