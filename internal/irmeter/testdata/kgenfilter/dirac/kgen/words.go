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

// refusal keeps refuse itself free of a refusal site.
type refusal string

func (r refusal) Error() string { return string(r) }

func refuse(format string, _ ...any) error { return refusal(format) }
