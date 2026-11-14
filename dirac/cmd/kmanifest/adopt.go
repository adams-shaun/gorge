package main

import (
	"fmt"

	"github.com/adams-shaun/gorge/cli"
	"github.com/adams-shaun/gorge/dirac/manifest"
)

func adoptCmd(root *string) *cli.Command {
	var base string
	return &cli.Command{
		Name:  "adopt",
		Short: "landing: move a rebased unit's hand-written constants onto the manifest",
		Long: "adopt runs in a unit rebased onto --base (the landing branch's tip). Each\n" +
			"kop constant of a manifest family that the unit declares by hand loses its\n" +
			"declaration and name registrations, and then: an entry of the same name\n" +
			"takes over; an entry for the same vocabulary item under another name takes\n" +
			"over and the uses are rewritten to it (gofmt -r); with no entry the\n" +
			"declaration becomes a value: next entry. An entry the kernel references\n" +
			"turns active. It prints each constant and regenerates the generated files;\n" +
			"regenerate ordinals.tsv, then run assign and check.",
		ValidArgs: cli.NoArgs,
		Flags: func(fs *cli.FlagSet) {
			fs.StringVar(&base, "base", "", "the commit the unit was rebased onto (required)")
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
			got, err := manifest.Adopt(dir, b)
			for _, a := range got {
				status := "reserved"
				if a.Active {
					status = "active"
				}
				fmt.Fprintf(c.Stdout, "adopted %s -> %s (%s, %s)\n", a.Name, a.Entry, a.Case, status)
			}
			return err
		},
	}
}
