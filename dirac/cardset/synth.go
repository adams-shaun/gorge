package cardset

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Synthetic evidence pools: per-card-list tables that exist to be played in
// lockstep, so that a card no human deck holds can still earn evidence. Nothing
// about them is tracked. kgen derives their lists from the Standard set lists and
// the corpus, and writes their tables, gitignored, beside the deck pools':
//
//	<cards>/kernel-synth-tables/<key>/manifest.tsv            as kernel-tables
//	<cards>/kernel-synth-tables/<key>/<pool>.ktab             one table
//	<cards>/kernel-synth-tables/<key>/<pool>.coverage.md      kgen's refusal report
//	<cards>/kernel-synth-tables/<key>/tail-unlowered.txt      names no tail pool holds
//
// They are a separate tree because ByName and Names serve the pools a game may
// use, and because they are a build input to no game outside lockstep.
//
// The pools are std-<set> for each line of standard.txt and tail-NN for every lowered
// card outside those lists, a list that does not fit one table cut in two (tail-07-a).

const (
	// SynthTablesSubdir is the directory under a corpus that holds the key directories.
	SynthTablesSubdir = "kernel-synth-tables"
	// TailUnloweredFile lists the non-Standard corpus names no tail bucket holds,
	// because kgen does not lower them.
	TailUnloweredFile = "tail-unlowered.txt"
	// TailBucket is how many cards a tail pool holds on average (TailBucketCount sizes
	// the cut; a bucket varies around it). It keeps tables under the
	// limit on a table's subtypes, which refuses a big enough list outright.
	TailBucket = 250
	// TailBucketCount is how many tail pools there are: ceil(lowered tail cards /
	// TailBucket), measured on land by `make dirac-tables` (19,056 lowered cards
	// -> tail-00 .. tail-76 = 77 pools, the last holding 56). It is fixed rather than derived so that a
	// card's pool is a function of its name alone: derived from the card count,
	// every newly lowered card would eventually change it and move every card.
	// Raise it deliberately, in its own change, when buckets near the table limit.
	TailBucketCount = 77
	// DraftSplitCards is the most cards a draft pool's table holds: a longer
	// draft list splits by hash (dr-mbc-aaaa ...). A split passes its evidence
	// run only when every one of its games is clean, so one divergent card holds
	// back its whole split; at 200, MBC's 2195 names are 16 splits of about 137.
	DraftSplitCards = 200
)

// SynthBasics are the lands every synthetic pool holds, whatever its list says:
// a deal fills up with them. They are in WUBRG order.
var SynthBasics = [...]string{"Plains", "Island", "Swamp", "Mountain", "Forest"}

// IsSynthBasic reports whether name is one of SynthBasics.
func IsSynthBasic(name string) bool {
	return slices.Contains(SynthBasics[:], name)
}

// SynthNameHash is the FNV-1a 32 of a card name, the only thing that decides which
// bucket or half of a split list a card lands in.
func SynthNameHash(name string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(name))
	return h.Sum32()
}

// SynthDraftPrefix names a draft pool's synthetic pool (dirac/cardset/draftpools.txt): dr-<code>.
const SynthDraftPrefix = "dr-"

// IsSynthName reports whether a pool name is a synthetic evidence pool's: std-<set>,
// dr-<code> or tail-NN, each possibly with a split suffix.
func IsSynthName(name string) bool {
	return strings.HasPrefix(name, "std-") || strings.HasPrefix(name, SynthDraftPrefix) || strings.HasPrefix(name, "tail-")
}

// TailBuckets deals names into TailBucketCount buckets by each name's own hash
// (SynthNameHash mod TailBucketCount), so a card's bucket never depends on which
// other cards are lowered: lowering or unlowering one card moves only that card.
// It always returns TailBucketCount buckets, in bucket order; one that no name
// hashes to is empty, and the caller skips it. Each bucket comes back sorted by name.
func TailBuckets(names []string) [][]string {
	out := make([][]string, TailBucketCount)
	for _, n := range names {
		i := TailBucketOf(n)
		out[i] = append(out[i], n)
	}
	for _, b := range out {
		slices.Sort(b)
	}
	return out
}

// TailBucketOf is the tail bucket a lowered card name belongs to, a function of the
// name alone.
func TailBucketOf(name string) int {
	return int(SynthNameHash(name) % TailBucketCount)
}

// SplitByHash divides names in two on one bit of their hash, keeping their order. It
// is how a list too big for one table becomes two (dirac/kgen picks the bit).
func SplitByHash(names []string, bit int) (a, b []string) {
	for _, n := range names {
		if SynthNameHash(n)>>(bit%32)&1 == 0 {
			a = append(a, n)
		} else {
			b = append(b, n)
		}
	}
	return a, b
}

// SynthDirFor is the synthetic table directory for the corpus at dir and the key
// this binary uses with it, whether or not it exists.
func SynthDirFor(dir string) (string, error) {
	key, err := KeyFor(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, SynthTablesSubdir, key), nil
}

// SynthDir returns this binary's synthetic table directory, symlinks resolved.
func SynthDir() (string, error) {
	corpus, err := corpusFound()
	if err != nil {
		return "", err
	}
	dir, err := SynthDirFor(corpus)
	if err != nil {
		return "", err
	}
	if !published(dir) {
		return "", fmt.Errorf("%w: no synthetic pools at %s: run make dirac-tables", ErrNoTables, dir)
	}
	return filepath.EvalSymlinks(dir)
}

// SynthByName returns the synthetic pool named name, every lowered card playable (see
// RawByName). It never reads the deck pools: a name in the wrong namespace is an
// error, not a game on the other pool's cards.
func SynthByName(name string) (Set, error) {
	dir, err := SynthDir()
	if err != nil {
		return nil, err
	}
	s, err := loadFrom(dir, name)
	if err != nil {
		return nil, err
	}
	return viewOf(s.(*dataSet).d, lowered), nil
}

// SynthNames lists the synthetic pools, sorted.
func SynthNames() ([]string, error) {
	dir, err := SynthDir()
	if err != nil {
		return nil, err
	}
	loadMu.Lock()
	defer loadMu.Unlock()
	rows, err := manifestOf(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(rows))
	for i := range rows {
		out[i] = rows[i].Pool
	}
	return out, nil
}

// SynthTailUnlowered is the non-Standard corpus names no tail pool holds.
func SynthTailUnlowered() ([]string, error) {
	dir, err := SynthDir()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(dir, TailUnloweredFile))
	if err != nil {
		return nil, fmt.Errorf("cardset: %w", err)
	}
	if len(b) == 0 {
		return nil, nil
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n"), nil
}
