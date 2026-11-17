package kgen

import "testing"

// counterRemovedForms: Mode$ CounterRemoved / CounterRemovedOnce on Card.Self
// (gorge trigmatch counterRemovedMatches), the battlefield and exile forms.
var counterRemovedForms = []formCase{
	{card: "Chandra, Fire Artisan", form: "CounterRemovedOnce LOYALTY on the battlefield", lir: "trigger CounterRemovedOnce"},
	{card: "Watcher of Hours", form: "CounterRemoved TIME from exile, ValidPlayer$ You", lir: "trigger CounterRemoved"},
	{card: "Dinosaurs on a Spaceship", form: "CounterRemoved TIME from exile, one trigger per counter", lir: "trigger CounterRemoved"},
	{card: "Veiling Oddity", form: "CounterRemoved TIME from exile, NewCounterAmount$ 0", lir: "trigger CounterRemoved"},
}

func TestCounterRemovedForms(t *testing.T) { checkForms(t, counterRemovedForms) }
