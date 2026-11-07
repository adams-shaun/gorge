package main

import (
	"fmt"
	"io"
	"os"

	"github.com/adams-shaun/gorge/draft/limited/pickbot"
)

// applyRatingsOverride swaps the drafter's and builder's rating source for
// the -ratings-file values (a sim/training-only path; no server or client
// route reaches it). Only the concrete persona types carry the override:
// a RatingColour or a knob-file Knobbed drafter, and a Best23 builder from
// either a registry name or a knob file.
func applyRatingsOverride(o options, r *runner, out io.Writer) error {
	if o.ratingsFile == "" {
		return nil
	}
	data, err := os.ReadFile(o.ratingsFile)
	if err != nil {
		return err
	}
	ov, err := pickbot.ParseRatingsOverride(data, o.ratingsFile, o.ratingsField, o.set)
	if err != nil {
		return err
	}
	var setDrafter func()
	switch d := r.drafter.(type) {
	case *pickbot.RatingColour:
		setDrafter = func() { d.Ratings = ov }
	case *pickbot.Knobbed:
		setDrafter = func() { d.Ratings = ov }
	default:
		return fmt.Errorf("-ratings-file needs a rating-colour or knobs: drafter, got %T", r.drafter)
	}
	b, ok := r.builder.(*pickbot.Best23)
	if !ok {
		return fmt.Errorf("-ratings-file needs a best23 builder, got %T", r.builder)
	}
	setDrafter()
	b.Ratings = ov
	fmt.Fprintf(out, "ratings override: %s field %s; %d 17Lands-rated cards missing (neutral): %v\n", o.ratingsFile, o.ratingsField, len(ov.Missing), ov.Missing)
	return nil
}
