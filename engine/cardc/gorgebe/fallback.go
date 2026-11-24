package gorgebe

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Fallback is one register entry: a card served from Forge on purpose.
type Fallback struct {
	Name, Class string
	Issue       int
}

var fallbackClasses = map[string]bool{"mtgish-bug": true, "cardc-gap": true, "forge-bug": true, "equivalent": true}

var nonStem = regexp.MustCompile(`[^a-z0-9]+`)

// Stem is a card name's register file stem: lower-cased, each run of
// characters outside [a-z0-9] replaced by "_".
func Stem(name string) string { return nonStem.ReplaceAllString(strings.ToLower(name), "_") }

// ReadFallbacks reads the register under dir. A missing directory is an empty
// map. A malformed file, a bad class or a stem that does not match its name is
// an error naming the file.
func ReadFallbacks(dir string) (map[string]Fallback, error) {
	ents, err := os.ReadDir(dir)
	out := map[string]Fallback{}
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var fb Fallback
		seen := map[string]bool{}
		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			key, val, ok := strings.Cut(line, " ")
			if !ok || seen[key] {
				return nil, fmt.Errorf("gorgebe: %s: malformed line %q", p, line)
			}
			seen[key] = true
			switch key {
			case "name":
				fb.Name = val
			case "class":
				fb.Class = val
			case "issue":
				n, err := strconv.Atoi(val)
				if err != nil || n < 0 {
					return nil, fmt.Errorf("gorgebe: %s: bad issue %q", p, val)
				}
				fb.Issue = n
			default:
				return nil, fmt.Errorf("gorgebe: %s: unknown key %q", p, key)
			}
		}
		if len(seen) != 3 || fb.Name == "" {
			return nil, fmt.Errorf("gorgebe: %s: want name, class and issue lines", p)
		}
		if !fallbackClasses[fb.Class] {
			return nil, fmt.Errorf("gorgebe: %s: bad class %q", p, fb.Class)
		}
		if Stem(fb.Name)+".txt" != e.Name() {
			return nil, fmt.Errorf("gorgebe: %s: stem does not match name %q", p, fb.Name)
		}
		out[fb.Name] = fb
	}
	return out, nil
}
