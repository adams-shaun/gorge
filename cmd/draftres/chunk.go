package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// defaultDplay plays a chunk from the module root, where `go run ./cmd/draftres`
// is run. A built dplay binary is faster over many chunks.
const defaultDplay = "go run -trimpath ./dirac/cmd/dplay"

// doneMarker is written into a chunk directory after every step of the chunk
// finished; a chunk without it is redone from scratch.
const doneMarker = "done"

// chunkPlan is one chunk: pods [First, Last) of the run.
type chunkPlan struct{ Index, First, Last int }

// planChunks splits pods into chunks of size pods each (0 or less: one chunk).
func planChunks(pods, size int) []chunkPlan {
	if size <= 0 || size > pods {
		size = pods
	}
	var ps []chunkPlan
	for first := 0; first < pods; first += size {
		ps = append(ps, chunkPlan{Index: len(ps), First: first, Last: min(first+size, pods)})
	}
	return ps
}

func chunkDir(out string, c int) string { return filepath.Join(out, fmt.Sprintf("chunk-%04d", c)) }

func chunkDone(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, doneMarker))
	return err == nil
}

// runChunks runs every chunk of plans whose directory under out has no done
// marker, in order, and returns the indices it ran. A partial chunk directory
// (an interrupted run) is removed first.
func runChunks(out string, plans []chunkPlan, run func(c chunkPlan, dir string) error) ([]int, error) {
	var ran []int
	for _, c := range plans {
		dir := chunkDir(out, c.Index)
		if chunkDone(dir) {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			return ran, err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return ran, err
		}
		if err := run(c, dir); err != nil {
			return ran, fmt.Errorf("chunk %d (pods %d-%d): %w", c.Index, c.First, c.Last-1, err)
		}
		if err := os.WriteFile(filepath.Join(dir, doneMarker), nil, 0o644); err != nil {
			return ran, err
		}
		ran = append(ran, c.Index)
	}
	return ran, nil
}

// dplaySet is dplay's name for a set's card pool.
func dplaySet(set string) string {
	if s := strings.ToLower(set); s != "trk" {
		return s
	}
	return "std-trk"
}

// playChunk plays dir's pairs.tsv in dplay, hbot against hbot on the
// lockstep-autopay surface, skipping pairs whose decks hold a refused card,
// and writes games.jsonl and summary.json beside it.
func playChunk(cmd []string, dir, set, corpus string, workers int) error {
	if len(cmd) == 0 {
		return fmt.Errorf("-dplay: empty command")
	}
	if b, err := os.ReadFile(filepath.Join(dir, "pairs.tsv")); err != nil {
		return err
	} else if len(b) == 0 {
		return os.WriteFile(filepath.Join(dir, "games.jsonl"), nil, 0o644)
	}
	args := append(cmd[1:len(cmd):len(cmd)],
		"-cards", corpus, "-set", dplaySet(set),
		"-decks", filepath.Join(dir, "decks"), "-pool", filepath.Join(dir, "pool.tsv"),
		"-pairs-file", filepath.Join(dir, "pairs.tsv"),
		"-a", "hbot", "-b", "hbot", "-config", "lockstep-autopay", "-skip-refused",
		"-workers", fmt.Sprint(workers),
		"-out", filepath.Join(dir, "games.jsonl"), "-summary", filepath.Join(dir, "summary.json"))
	c := exec.Command(cmd[0], args...)
	c.Stdout, c.Stderr = os.Stderr, os.Stderr
	return c.Run()
}
