// Command kmanifest keeps dirac's namespace manifest (dirac/manifest, one file
// per vocabulary code) and the Go it generates.
//
//	kmanifest gen [--check]       write the generated files; --check lists stale ones and exits 1
//	kmanifest assign --base REF   landing only: give each value: next entry its final value, then gen
//	kmanifest check --base REF    hold the change to the manifest's invariants against REF's merge base
//	kmanifest show QUERY          print the entries a name, forge token or refusal class names
//	kmanifest seed --census F --classes F [--aliases F] [--dry-run]
//	                              cutover and FORGE_REF bumps: add an entry per new vocabulary item
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
		Sub: []*cli.Command{genCmd(&root), assignCmd(&root), checkCmd(&root), showCmd(&root), seedCmd(&root)},
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

func assignCmd(root *string) *cli.Command {
	var base string
	return &cli.Command{
		Name:  "assign",
		Short: "landing only: write final values into value: next entries, then gen",
		Long: "assign gives every value: next entry the lowest free value of its family\n" +
			"that neither the working tree nor the commit --base names holds (the\n" +
			"landing branch's tip), writes it into the entry's value line, prints each, and then runs gen.\n" +
			"Run it serially, one unit at a time, after rebasing the unit onto --base.",
		ValidArgs: cli.NoArgs,
		Flags: func(fs *cli.FlagSet) {
			fs.StringVar(&base, "base", "", "the commit the unit lands on (required)")
		},
		Run: func(c *cli.Context, _ []string) error {
			if base == "" {
				return cli.Usagef("--base is required")
			}
			dir, err := moduleRoot(*root)
			if err != nil {
				return err
			}
			b, err := manifest.ReadGit(dir, base)
			if err != nil {
				return err
			}
			got, err := manifest.Assign(dir, b)
			if err != nil {
				return err
			}
			for _, a := range got {
				fmt.Fprintf(c.Stdout, "assigned %s = %d\n", a.Path, a.Value)
			}
			changed, err := manifest.WriteGen(dir, false)
			for _, p := range changed {
				fmt.Fprintln(c.Stdout, "wrote", p)
			}
			return err
		},
	}
}

func checkCmd(root *string) *cli.Command {
	var (
		base  string
		final bool
		allow []string
	)
	return &cli.Command{
		Name:  "check",
		Short: "hold the working tree's manifest to its invariants against a base",
		Long: "check validates the working tree's manifest and compares it with the merge\n" +
			"base of --base and HEAD: no entry deleted or renamed, no assigned value\n" +
			"changed, only status and ticket edited on an assigned entry, no new entry\n" +
			"named for an existing constant, and no ordinals.tsv row of the base changed\n" +
			"or gone (--allow-ordinal NAME excepts one, for a deliberate legacy change).\n" +
			"--final also refuses value: next (landing, after assign). Prints each\n" +
			"violation and exits 1 if there is one.",
		ValidArgs: cli.NoArgs,
		Flags: func(fs *cli.FlagSet) {
			fs.StringVar(&base, "base", "", "the branch the change is held to, the landing branch (required)")
			fs.BoolVar(&final, "final", false, "refuse any value: next entry")
			fs.StringSliceVar(&allow, "allow-ordinal", nil, "an ordinals.tsv row the change may alter (repeatable)")
		},
		Run: func(c *cli.Context, _ []string) error {
			if base == "" {
				return cli.Usagef("--base is required")
			}
			dir, err := moduleRoot(*root)
			if err != nil {
				return err
			}
			mb, err := manifest.MergeBase(dir, base)
			if err != nil {
				return err
			}
			b, err := manifest.ReadGit(dir, mb)
			if err != nil {
				return err
			}
			t, err := manifest.Read(os.DirFS(dir))
			if err != nil {
				return err
			}
			errs := manifest.Check(b, t, manifest.CheckOptions{AllowOrdinal: allow, Final: final})
			for _, e := range errs {
				fmt.Fprintln(c.Stdout, e)
			}
			if len(errs) > 0 {
				return cli.Exit(1)
			}
			return nil
		},
	}
}

func showCmd(root *string) *cli.Command {
	return &cli.Command{
		Name:  "show",
		Short: "print the entries a Go name, forge token or refusal class names",
		Long: "show prints each entry QUERY names (a Go name, <Family>/<Name>, an entry\n" +
			"path, a refusal class manifest:<status>:<Family>/<Name>, or a forge token,\n" +
			"without case): its path, value, status and State category, then its other\n" +
			"lines. It exits 1 when nothing matches.",
		Args:      "<query>",
		ValidArgs: cli.ExactArgs(1),
		Run: func(c *cli.Context, args []string) error {
			dir, err := moduleRoot(*root)
			if err != nil {
				return err
			}
			t, err := manifest.Load(os.DirFS(dir))
			if err != nil {
				return err
			}
			es := t.Find(args[0])
			if len(es) == 0 {
				return fmt.Errorf("no manifest entry names %q", args[0])
			}
			for _, e := range es {
				fmt.Fprint(c.Stdout, e.Show())
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
