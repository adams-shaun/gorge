package kgen

import "errors"

var errNoTypedReading = errors.New("no typed reading") // the fallback sentinel: no site

var errTwoColours = errors.New("two colours") // typed-only

func lFilterFromIR(x int) error {
	switch x {
	case 0:
		return errNoTypedReading
	case 2:
		return errTwoColours
	}
	var c lClause
	return c.setOne()
}

// parseFilterText falls back to parseFilter: the typed reach stops there.
func parseFilterText(s string) error {
	if err := lFilterFromIR(len(s)); errors.Is(err, errNoTypedReading) {
		return parseFilter(s)
	}
	return nil
}
