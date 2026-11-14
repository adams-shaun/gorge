package kgen

import (
	"errors"
	"fmt"
	"go/format"
	"io"
	"os"
	"reflect"
	"slices"

	"github.com/adams-shaun/gorge"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// runSupported is kgen -supported: every entry of a pool's table, tokens included.
func runSupported(pool string, w io.Writer) error { return runSupportedView(pool, false, false, w) }

// runSupportedView is runSupported; allowUnverified counts the cards with no lockstep
// evidence yet as playable, as kfuzz -allow-unverified plays them. A refused card stays false.
// fourSeat also counts a card a pod cannot hold (kop.CardPodLimited) as false: a
// pod holding one stalls StallTwoSeat at its start, so a four-seat deck pool
// (decks.py --four-seat) replaces it as it replaces a refused card.
func runSupportedView(pool string, allowUnverified, fourSeat bool, w io.Writer) error {
	s, err := cardset.ByName(pool)
	if err != nil && cardset.IsSynthName(pool) {
		s, err = cardset.SynthByName(pool)
	}
	if err != nil {
		return err
	}
	if allowUnverified {
		s = cardset.AllowUnverified(s)
	}
	sup := s.Supported()
	for i, n := range s.Names() {
		fmt.Fprintf(w, "%s\t%v\n", n, sup[i] && !(fourSeat && kop.CardPodLimited(s.Def(i))))
	}
	return nil
}

// runShow is kgen -show: the kop.CardDef a pool's table holds for a card or token.
// Its Name is blank, since names live in the table's name list.
func runShow(pool, card string, w io.Writer) error {
	if card == "" {
		return errors.New("-show wants a card name argument")
	}
	s, err := cardset.RawByName(pool)
	if err != nil && cardset.IsSynthName(pool) {
		s, err = cardset.SynthByName(pool)
	}
	if err != nil {
		return err
	}
	i := slices.Index(s.Names(), card)
	if i < 0 {
		return fmt.Errorf("no %q in pool %s", card, pool)
	}
	src, err := format.Source([]byte(lit(reflect.ValueOf(*s.Def(i)))))
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", src)
	return err
}

// runOps is kgen -ops: a pool's OPS.md, built from this binary's sources. cards is
// the -cards flag when given, else the corpus is located the way the loader does.
func runOps(pool, cards, out string) error {
	if out == "" {
		return errors.New("-ops wants -out <file>")
	}
	if cards == "" {
		var err error
		if cards, err = cardset.CorpusDir(); err != nil {
			return err
		}
	}
	p, _, prev, err := readPool(gorge.TableSources(), pool, cards)
	if err != nil {
		return err
	}
	pb, err := buildForOutput(p, prev)
	if err != nil {
		return err
	}
	return os.WriteFile(out, writeOpsDoc(p, pb.pt, pb.all, pb.bs), 0o644)
}
