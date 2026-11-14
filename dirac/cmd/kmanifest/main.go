// Command kmanifest keeps dirac's namespace manifest (dirac/manifest, one file
// per vocabulary code) and the Go it generates.
//
//	kmanifest gen [--check]   write the generated files; --check lists stale ones and exits 1
//
// Every subcommand works on the module at --root, by default the one holding
// the working directory. See package dirac/manifest for the entry format.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adams-shaun/gorge/cli"
	"github.com/adams-shaun/gorge/dirac/manifest"
)

func main() {
	os.Exit(cli.Execute(newRoot(), os.Args[1:], os.Environ(), os.Stdout, os.Stderr))
}

func newRoot() *cli.Command {
	var root string
	return &cli.Command{
		Name:  "kmanifest",
		Short: "keep dirac's namespace manifest and its generated Go",
		Long: "kmanifest reads dirac/manifest (one file per vocabulary code) and\n" +
			"dirac/kop/testdata/ordinals.tsv, and writes the constants and name\n" +
			"registrations generated from them.",
		Flags: func(fs *cli.FlagSet) {
			fs.StringVar(&root, "root", "", "module root (default: the module holding the working directory)")
		},
		Sub: []*cli.Command{genCmd(&root)},
	}
}

func genCmd(root *string) *cli.Command {
	var check bool
	return &cli.Command{
		Name:  "gen",
		Short: "write the generated files from the manifest",
		Long: "gen writes " + manifest.KopGen + ", " + manifest.DiracGen + " and\n" +
			manifest.KgenGen + " from the manifest, giving each value: next\n" +
			"entry a provisional value, and prints each file it wrote. A second run\n" +
			"writes nothing. --check writes nothing, prints each stale file and exits 1\n" +
			"if there is one.",
		ValidArgs: cli.NoArgs,
		Flags: func(fs *cli.FlagSet) {
			fs.BoolVar(&check, "check", false, "write nothing; list stale files and exit 1 if any")
		},
		Run: func(c *cli.Context, _ []string) error {
			dir, err := moduleRoot(*root)
			if err != nil {
				return err
			}
			changed, err := manifest.WriteGen(dir, check)
			if err != nil {
				return err
			}
			verb := "wrote"
			if check {
				verb = "stale"
			}
			for _, p := range changed {
				fmt.Fprintln(c.Stdout, verb, p)
			}
			if check && len(changed) > 0 {
				return cli.Exit(1)
			}
			return nil
		},
	}
}

// moduleRoot is dir, or else the nearest directory above the working
// directory that holds go.mod.
func moduleRoot(dir string) (string, error) {
	if dir != "" {
		return dir, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for d := wd; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		if filepath.Dir(d) == d {
			return "", errors.New("no go.mod above " + wd + "; pass --root")
		}
	}
}
