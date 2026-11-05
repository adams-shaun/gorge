// Command kgen compiles card pools from gorge's compiled card IR into the
// dirac kernel's binary tables: with -tables or -synth-pools it writes each
// pool's .ktab and .coverage.md, and -tables also writes the pool's ids.tsv,
// under the corpus's gitignored kernel-tables directory. Package kgen holds
// the compiler and documents the flags.
package main

import (
	"os"

	"github.com/adams-shaun/gorge/dirac/kgen"
)

func main() { os.Exit(kgen.Main(os.Args[1:])) }
