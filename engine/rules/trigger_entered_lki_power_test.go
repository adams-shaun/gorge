package rules

import "testing"

// Ensign Harry Kim, Operations' "if it has greater power than Ensign Kim" is
// checked again as the trigger resolves (CR 603.4). A creature that has left
// the battlefield by then is read as it last existed there (CR 603.10, 608.2h),
// its power with every continuous effect applied. Under a Glorious Anthem Kim
// is 2/3 and Grizzly Bears 3/3; with Kim's trigger on the stack the Bears die,
// and their last known power, 3, is still greater than Kim's 2: Kim gets the
// counter. Read off the printed card plus counters, the Bears would have had
// power 2 and Kim no counter, as gorge did before the snapshot's power was
// bound to the trigger's Valid read. A Bears still on the battlefield is read live:
// the control.
func TestEnteredCreatureDepartedReadsLastKnownPower(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, steps, kimPT string
	}{
		{"departed", `{"op":"cast","seat":0,"card":"p0:Murder","mana":"CBB","targets":["p0:Grizzly Bears"]},{"op":"resolve"}`, "3/4"},
		{"sacrificed", `{"op":"activate","seat":0,"card":"p0:Viscera Seer","answers":[{"kind":"choose","pick":["Grizzly Bears"]}]},{"op":"resolve"}`, "3/4"},
		{"live", `{"op":"resolve"}`, "3/4"},
	} {
		t.Run(c.name, func(t *testing.T) {
			snaps := runPTFilterScenario(t, `{"name":"kim-lki-`+c.name+`","setup":{
			  "p0":{"battlefield":["Ensign Harry Kim, Operations","Glorious Anthem","Viscera Seer"],"hand":["Grizzly Bears","Murder"]}},
			 "steps":[{"op":"cast","seat":0,"card":"p0:Grizzly Bears","mana":"CG"},{"op":"pass","seat":0},{"op":"pass","seat":1},`+c.steps+`]}`)
			// both players passed once: the Bears resolved, Kim's trigger waits
			wantPT(t, snaps[3], "p0:Ensign Harry Kim, Operations", "2/3")
			wantPT(t, snaps[3], "p0:Grizzly Bears", "3/3")
			wantPT(t, snaps[len(snaps)-1], "p0:Ensign Harry Kim, Operations", c.kimPT)
			wantAlive(t, snaps[len(snaps)-1], "p0:Grizzly Bears", c.name == "live")
		})
	}
}
