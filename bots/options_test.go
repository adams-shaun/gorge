package bots_test

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/seat"
)

func TestRegisterRejectsBadOptionSpec(t *testing.T) {
	factory := func(bots.Options) (seat.Seat, error) { return nil, nil }
	cases := []struct {
		name string
		spec bots.OptionSpec
	}{
		{"uppercase-key", bots.OptionSpec{Key: "Bad", Type: "string"}},
		{"leading-hyphen", bots.OptionSpec{Key: "-bad", Type: "string"}},
		{"unknown-type", bots.OptionSpec{Key: "key", Type: "duration"}},
		{"bad-int-default", bots.OptionSpec{Key: "key", Type: "int", Default: "nope"}},
		{"bad-float-default", bots.OptionSpec{Key: "key", Type: "float", Default: "nope"}},
		{"bad-bool-default", bots.OptionSpec{Key: "key", Type: "bool", Default: "maybe"}},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("Register accepted invalid option spec %+v", tc.spec)
				}
			}()
			bots.Register(bots.Entry{
				Info: bots.Info{
					Name:    fmt.Sprintf("bad-option-%d", i),
					Tier:    bots.Experimental,
					Options: []bots.OptionSpec{tc.spec},
				},
				New: factory,
			})
		})
	}
}

func TestRegisterRejectsDuplicateOptionSpec(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Register accepted duplicate option keys")
		}
	}()
	bots.Register(bots.Entry{
		Info: bots.Info{
			Name: "duplicate-option",
			Tier: bots.Experimental,
			Options: []bots.OptionSpec{
				{Key: "key", Type: "string"},
				{Key: "key", Type: "int"},
			},
		},
		New: func(bots.Options) (seat.Seat, error) { return nil, nil },
	})
}

func TestOptionsConfigAccessors(t *testing.T) {
	o := bots.Options{Config: map[string]string{
		"name":    "hello",
		"count":   "7",
		"ratio":   "1.25",
		"enabled": "true",
		"dir":     "/tmp/bot-option-path",
	}}
	if got := o.String("name"); got != "hello" {
		t.Errorf("String = %q, want hello", got)
	}
	if got := o.Int("count"); got != 7 {
		t.Errorf("Int = %d, want 7", got)
	}
	if got := o.Float("ratio"); got != 1.25 {
		t.Errorf("Float = %v, want 1.25", got)
	}
	if got := o.Bool("enabled"); !got {
		t.Errorf("Bool = %v, want true", got)
	}
	if got := o.String("dir"); got != "/tmp/bot-option-path" {
		t.Errorf("String(path) = %q, want /tmp/bot-option-path", got)
	}

	for _, tc := range []struct {
		name string
		call func()
	}{
		{"undeclared-string", func() { o.String("missing") }},
		{"undeclared-int", func() { o.Int("missing") }},
		{"undeclared-float", func() { o.Float("missing") }},
		{"undeclared-bool", func() { o.Bool("missing") }},
		{"bad-int", func() { (bots.Options{Config: map[string]string{"key": "x"}}).Int("key") }},
		{"bad-float", func() { (bots.Options{Config: map[string]string{"key": "x"}}).Float("key") }},
		{"bad-bool", func() { (bots.Options{Config: map[string]string{"key": "x"}}).Bool("key") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("%s did not panic", tc.name)
				}
			}()
			tc.call()
		})
	}
}
