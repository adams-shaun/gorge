package cards

import "testing"

func modularFace(t *testing.T, kline string) *Face {
	t.Helper()
	f, _ := modularCard(t, kline)
	return f
}

func modularCard(t *testing.T, kline string) (*Face, *Card) {
	t.Helper()
	c, diags := ParseBytes("modular.txt", []byte("Name:Modular fixture\nManaCost:1\nTypes:Artifact Creature Construct\nPT:0/0\n"+kline+"\nOracle:x\n"))
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	if d := c.Link(); len(d) != 0 {
		t.Fatal(d)
	}
	return c.Faces[0], c
}

func TestModularExpandsEntryReplacementAndDiesTrigger(t *testing.T) {
	f, c := modularCard(t, "K:Modular:3")
	if len(f.Keywords) != 1 || f.Keywords[0] != "Modular:3" {
		t.Fatalf("precondition: keywords = %v, want [Modular:3]", f.Keywords)
	}
	check := func() {
		t.Helper()
		if len(f.Repls) != 1 || len(f.Triggers) != 1 {
			t.Fatalf("repls = %+v, triggers = %+v, want one of each", f.Repls, f.Triggers)
		}
		r := f.Repls[0]
		if r.Event != "Moved" || r.Params["Keyword"] != "Modular" || r.Params["ValidCard"] != "Card.Self" || r.Params["Destination"] != "Battlefield" {
			t.Fatalf("replacement = %+v, want a Moved entry replacement tagged Modular", r)
		}
		put := ResolveSVar(f.SVars, r.Params["ReplaceWith"])
		if put == nil || put.API != "PutCounter" || put.Params["CounterType"] != "P1P1" || put.Params["CounterNum"] != "3" || put.Params["Defined"] != "Self" {
			t.Fatalf("entry body = %+v, want PutCounter 3 P1P1 on Self", put)
		}
		tr := f.Triggers[0]
		if tr.Mode != "ChangesZone" || tr.Params["Origin"] != "Battlefield" || tr.Params["Destination"] != "Graveyard" ||
			tr.Params["ValidCard"] != "Card.Self" || tr.Params["OptionalDecider"] != "You" || tr.Params["Keyword"] != "Modular" {
			t.Fatalf("trigger = %+v, want an optional dies trigger tagged Modular", tr)
		}
		body := ResolveSVar(f.SVars, tr.Params["Execute"])
		if body == nil || body.API != "PutCounter" || body.Params["ValidTgts"] != "Artifact.Creature" || body.Params["CounterType"] != "P1P1" {
			t.Fatalf("dies body = %+v, want PutCounter on target Artifact.Creature", body)
		}
		if got := f.SVars[body.Params["CounterNum"]]; got != "TriggeredCard$CardCounters.P1P1" {
			t.Fatalf("dies count = %q (SVar %q), want the dying permanent's P1P1 count", got, body.Params["CounterNum"])
		}
	}
	check()
	if d := c.Link(); len(d) != 0 {
		t.Fatal(d)
	}
	check()
}

func TestModularSunburstCountsConvergedColours(t *testing.T) {
	f := modularFace(t, "K:Modular:Sunburst")
	if len(f.Repls) != 1 {
		t.Fatalf("repls = %+v, want one", f.Repls)
	}
	put := ResolveSVar(f.SVars, f.Repls[0].Params["ReplaceWith"])
	// The count rides its own SVar, whose body is Count$Converge: the shape the
	// cast's converge gate (rules faceWantsConverge) finds, so the colours spent
	// are recorded and counted.
	if put == nil {
		t.Fatalf("entry body missing: repl %+v", f.Repls[0])
	}
	if got := f.SVars[put.Params["CounterNum"]]; got != "Count$Converge" {
		t.Fatalf("entry count = %q (CounterNum %q), want an SVar whose body is Count$Converge", got, put.Params["CounterNum"])
	}
}
