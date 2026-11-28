package kgen

import (
	"errors"
	"fmt"
)

type lClause struct{ n int }

func parseFilter(s string) error {
	if s == "" {
		return fmt.Errorf("empty filter") // string-only
	}
	return parseClause(s)
}

func parseClause(s string) error {
	var c lClause
	return c.prop(s)
}

func historicClauses() error { return errors.New("historic") } // string-only

func (c *lClause) prop(p string) error {
	if p == "a" {
		return c.setOne()
	}
	return stringOnlyWord(p)
}

func unreached() error { return fmt.Errorf("no reader reaches this") }
