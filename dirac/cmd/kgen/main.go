// Command kgen compiles card pools into binary tables. Package kgen holds the
// compiler and documents the flags.
package main

import (
	"os"

	"github.com/adams-shaun/gorge/dirac/kgen"
)

func main() { os.Exit(kgen.Main(os.Args[1:])) }
