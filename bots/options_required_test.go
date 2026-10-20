package bots

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/seat"
)

// TestNewRequiredOption lives in the internal package so it can remove the
// entry it registers: Register has no exported unregister, and leaving a
// fifth entry behind would break the built-in count TestEntries pins.
func TestNewRequiredOption(t *testing.T) {
	const name = "required-option-test"
	Register(Entry{
		Info: Info{
			Name:    name,
			Tier:    Experimental,
			Options: []OptionSpec{{Key: "port", Type: "int", Required: true}},
		},
		New: func(Options) (seat.Seat, error) { return nil, nil },
	})
	t.Cleanup(func() { delete(entries, name) })

	if _, err := New(name, Options{}); err == nil {
		t.Fatal("bots.New accepted a missing required option")
	}
	if _, err := New(name, Options{Config: map[string]string{"port": "8080"}}); err != nil {
		t.Fatalf("bots.New with required option: %v", err)
	}
}
