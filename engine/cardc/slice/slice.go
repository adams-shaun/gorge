// Package slice is the one recipe for "the cardc registry over named slices".
package slice

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/gorgebe"
	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
	"github.com/adams-shaun/gorge/engine/cards"
)

// ErrNoData is returned when the mtgish data file is missing.
var ErrNoData = errors.New(mtgish.SkipMessage)

// Build compiles the named slices' cards (each slice's names in order, a repeated name kept first), reads the
// fallback register under root/engine/cardc/testdata/fallback, and builds the gorge backend's registry.
// A missing mtgish data file is ErrNoData; an unresolved name is an error naming it.
func Build(root string, forge *cards.Registry, slices ...string) (*gorgebe.Registry, error) {
	p := mtgish.DataPath()
	if p == "" {
		return nil, ErrNoData
	}
	if _, err := os.Stat(p); err != nil {
		return nil, ErrNoData
	}
	var names []string
	seen := map[string]bool{}
	for _, s := range slices {
		ns, err := mtgish.SliceNames(root, s)
		if err != nil {
			return nil, err
		}
		if ns == nil {
			return nil, fmt.Errorf("slice: %q is not a slice of names", s)
		}
		for _, n := range ns {
			if !seen[n] {
				seen[n] = true
				names = append(names, n)
			}
		}
	}
	all, err := mtgish.LoadAll(p)
	if err != nil {
		return nil, err
	}
	ix := mtgish.NewIndex(all)
	var compiled []*ir.Card
	var missing []string
	for _, n := range names {
		c, ok := ix.Lookup(n)
		if !ok {
			missing = append(missing, n)
			continue
		}
		compiled = append(compiled, mtgish.Compile(c))
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("slice: %d names do not resolve: %s", len(missing), strings.Join(missing, "; "))
	}
	fb, err := gorgebe.ReadFallbacks(filepath.Join(root, "engine", "cardc", "testdata", "fallback"))
	if err != nil {
		return nil, err
	}
	return gorgebe.Build(forge, compiled, fb)
}
