package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/dirac/oracle"
)

// customKernel is the label of a recording whose table is no registered pool's:
// the one its own decks need, built from their card names.
const customKernel = "custom"

// customReproSide is runRepro's newSide for a custom recording: it lowers the
// cards of the recording's decks into a table of their own (kgen's printed-delve
// pool, never committed) and plays every lowered card, refused ones included,
// as the oracle's replay tests do: a recording replays the game it recorded.
func customReproSide(c *oracle.Corpus, cardsDir string) func(*oracle.Repro, oracle.RunConfig) (oracle.SideFactory, error) {
	return func(rep *oracle.Repro, rrc oracle.RunConfig) (oracle.SideFactory, error) {
		decks := append([][]string{rep.Cards[0], rep.Cards[1]}, rep.MoreCards...)
		set, err := customSet(cardsDir, decks)
		if err != nil {
			return nil, err
		}
		es := make([][]cardset.Entry, len(decks))
		for i, d := range decks {
			es[i] = oracle.ReproEntries(c.Reg, set, d)
		}
		tbl, _, err := set.MatchupN(es)
		if err != nil {
			return nil, err
		}
		return oracle.SetSide(set, tbl, rrc.Config), nil
	}
}

// customSet lowers every card named in decks into a fresh table and loads it.
func customSet(cardsDir string, decks [][]string) (cardset.Set, error) {
	var names []string
	for _, d := range decks {
		names = append(names, d...)
	}
	slices.Sort(names)
	names = slices.Compact(names)
	dir, err := os.MkdirTemp("", "kfuzz-custom-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		return nil, err
	}
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		return nil, err
	}
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", list, "-pool", "printed-delve", "-out", out}); code != 0 {
		return nil, fmt.Errorf("kgen: exit %d lowering the recording's cards", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "printed-delve.ktab"))
	if err != nil {
		return nil, err
	}
	return cardset.Raw(cardset.AllowUnverified(cardset.NewDataSet(pd))), nil
}

// customKernelFor is the -kernel a run plays: the recording's own when it is a
// custom one and no -kernel was given (a custom recording names no registered
// pool, so no flag value could ask for it), else the flag's.
func customKernelFor(kernel, reproPath string, synth, given bool) string {
	if reproPath == "" || synth || given {
		return kernel
	}
	if rep, err := oracle.ReadRepro(reproPath); err == nil && rep.Kernel == customKernel {
		return customKernel
	}
	return kernel
}

// runCustomRepro replays a custom recording; -kernel custom without one has no
// table to deal from.
func runCustomRepro(c *oracle.Corpus, reproPath, lead string, rc oracle.RunConfig, cardsDir string) int {
	if reproPath == "" {
		fmt.Fprintf(os.Stderr, "kfuzz: -kernel %s replays a custom-pool recording: it needs -repro\n", customKernel)
		return 2
	}
	return runRepro(c, reproPath, lead, rc, customReproSide(c, cardsDir))
}

// checkRepro reads the recording at path and checks it against this run (see
// reproRun): the surface to play and 0, or the exit code of a refusal.
func checkRepro(path, surface string, surfaceGiven bool) (string, int) {
	rep, err := oracle.ReadRepro(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return surface, 1
	}
	if surface, err = reproRun(rep, path, surface, surfaceGiven); err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return surface, 2
	}
	return surface, 0
}
