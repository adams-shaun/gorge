package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
	"github.com/adams-shaun/gorge/engine/cardc/ogram"
)

const ogramUsage = "usage: cardc ogram census [-dir DIR] | cardc ogram filters -slice fdn|blb [-dir DIR] [-top N]"

// ogramCmd is `cardc ogram`, the oracle-grammar release (engine/cardc/ogram):
//
//	census   one TSV row per outcome: outcome, faces, Commander-legal paper faces
//	filters  for a slice's uniquely read faces: one TSV row per refused
//	         variant (refused, key, count; the -top most frequent), then a
//	         totals line (operands found, lowered)
func ogramCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "census" && args[0] != "filters") {
		fmt.Fprintln(stderr, ogramUsage)
		return 2
	}
	fs := flag.NewFlagSet("ogram "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", ogram.DataDir(), "release directory")
	slice := fs.String("slice", "fdn", "fdn or blb (filters)")
	top := fs.Int("top", 10, "refused variants to list (filters; 0 lists all)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "cardc: %v\n", err)
		return 1
	}
	if !ogram.Fetched(*dir) {
		fmt.Fprintln(stderr, ogram.SkipMessage)
		return 1
	}
	if args[0] == "census" {
		res, err := ogram.Census(*dir)
		if err != nil {
			return fail(err)
		}
		fmt.Fprintln(stdout, "outcome\tfaces\tcommander_paper")
		for _, r := range res.Rows {
			fmt.Fprintf(stdout, "%s\t%d\t%d\n", r.Outcome, r.All, r.Commander)
		}
		return 0
	}
	if *slice != "fdn" && *slice != "blb" {
		fmt.Fprintln(stderr, ogramUsage)
		return 2
	}
	root, err := mtgish.ModuleRoot()
	if err != nil {
		return fail(err)
	}
	names, err := mtgish.SliceNames(root, *slice)
	if err != nil {
		return fail(err)
	}
	st, err := ogram.SliceFilters(*dir, names, nil)
	if err != nil {
		return fail(err)
	}
	for i, r := range st.TopRefused() {
		if *top > 0 && i == *top {
			break
		}
		fmt.Fprintf(stdout, "refused\t%s\t%d\n", r.Key, r.N)
	}
	fmt.Fprintf(stdout, "total\tslice=%s\tnames=%d\tunique_faces=%d\toperands=%d\tlowered=%d\trefused=%d\trefused_variants=%d\n",
		*slice, st.Names, st.Faces, st.Operands, st.Lowered, st.Operands-st.Lowered, len(st.Refused))
	return 0
}
