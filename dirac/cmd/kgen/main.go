// Command kgen compiles card pools from gorge's compiled card IR into the
// dirac kernel's binary tables. -tables writes each pool's .ktab,
// .coverage.md and manifest.tsv under `<corpus>/kernel-tables/<key>/`; when a
// CardID changes it writes that pool's ids.tsv to
// dirac/cardset/pools/<pool>/ids.tsv in the source tree and exits 3 without
// publishing tables, so rebuild kgen and rerun. -synth-pools writes its
// tables under `<corpus>/kernel-synth-tables/<key>/`. Package kgen holds the compiler and
// documents the flags.
package main

import (
	"os"

	"github.com/adams-shaun/gorge/dirac/kgen"
)

func main() { os.Exit(kgen.Main(os.Args[1:])) }
