package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/adams-shaun/gorge/cli"
	"github.com/adams-shaun/gorge/dirac/manifest"
)

func seedCmd(root *string) *cli.Command {
	var census, classes, aliases string
	var dry bool
	return &cli.Command{
		Name:  "seed",
		Short: "add an entry for each new vocabulary item of the research census",
		Long: "seed reads the census (--census, kind/item/status rows) and the refusal\n" +
			"classes (--classes, for the bounded State keys), both plain or gzip TSV,\n" +
			"and writes one reserved entry per vocabulary item kgen does not lower that\n" +
			"has no entry yet, except the items --aliases (default " + manifest.AliasesPath + ")\n" +
			"aliases to an existing constant or excludes. It never edits an entry, so a\n" +
			"re-run at a FORGE_REF bump adds only the new vocabulary. It prints the new\n" +
			"entries per family; --dry-run writes nothing.",
		ValidArgs: cli.NoArgs,
		Flags: func(fs *cli.FlagSet) {
			fs.StringVar(&census, "census", "", "the census TSV (required)")
			fs.StringVar(&classes, "classes", "", "the refusal-class TSV (required)")
			fs.StringVar(&aliases, "aliases", "", "the alias list (default: the module's "+manifest.AliasesPath+")")
			fs.BoolVar(&dry, "dry-run", false, "write nothing")
		},
		Run: func(c *cli.Context, _ []string) error {
			if census == "" || classes == "" {
				return cli.Usagef("--census and --classes are required")
			}
			dir, err := moduleRoot(*root)
			if err != nil {
				return err
			}
			if aliases == "" {
				aliases = filepath.Join(dir, filepath.FromSlash(manifest.AliasesPath))
			}
			s, err := planSeed(dir, census, classes, aliases)
			if err != nil {
				return err
			}
			if !dry {
				if err := manifest.WriteSeed(dir, s); err != nil {
					return err
				}
			}
			n := s.Counts()
			fams := make([]string, 0, len(n))
			for f := range n {
				fams = append(fams, f)
			}
			sort.Strings(fams)
			for _, f := range fams {
				fmt.Fprintf(c.Stdout, "%s\t%d\n", f, n[f])
			}
			fmt.Fprintf(c.Stdout, "total\t%d\nskipped\t%d\n", len(s.Entries), len(s.Skipped))
			return nil
		},
	}
}

// planSeed reads the module at dir and the three inputs and plans the seed.
func planSeed(dir, census, classes, aliases string) (*manifest.Seed, error) {
	t, err := manifest.Load(os.DirFS(dir))
	if err != nil {
		return nil, err
	}
	declared, err := manifest.Declared(dir)
	if err != nil {
		return nil, err
	}
	var rows []manifest.CensusRow
	var cls []manifest.ClassRow
	var as []manifest.Alias
	for _, in := range []struct {
		path string
		read func(f *os.File) error
	}{
		{census, func(f *os.File) (err error) { rows, err = manifest.ReadCensus(census, f); return }},
		{classes, func(f *os.File) (err error) { cls, err = manifest.ReadClasses(classes, f); return }},
		{aliases, func(f *os.File) (err error) { as, err = manifest.ReadAliases(aliases, f); return }},
	} {
		f, err := os.Open(in.path)
		if err != nil {
			return nil, err
		}
		err = in.read(f)
		f.Close()
		if err != nil {
			return nil, err
		}
	}
	return manifest.PlanSeed(t, rows, cls, as, declared)
}
