package kgen

import "fmt"

func (c *lClause) setOne() error {
	if c.n != 0 {
		return fmt.Errorf("two in one clause") // shared
	}
	c.n = 1
	return nil
}

func stringOnlyWord(p string) error { return refuse("unknown property %q", p) } // string-only
