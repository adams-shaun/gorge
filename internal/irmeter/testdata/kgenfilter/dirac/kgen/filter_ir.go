package kgen

import "errors"

var errNoTypedReading = errors.New("no typed reading") // typed-only

func lFilterFromIR(x int) error {
	if x == 0 {
		return errNoTypedReading
	}
	var c lClause
	return c.setOne()
}

// parseFilterText falls back to parseFilter: the typed reach stops there.
func parseFilterText(s string) error {
	if err := lFilterFromIR(len(s)); err != nil {
		return parseFilter(s)
	}
	return nil
}
