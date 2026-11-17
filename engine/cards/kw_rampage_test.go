package cards

import "testing"

func TestRampageExpandsOneBlockedTrigger(t *testing.T) {
	c, diags := ParseBytes("rampage.txt", []byte("Name:Rampage fixture\nManaCost:3 G\nTypes:Creature Giant\nPT:3/3\nK:Rampage:2\nOracle:x\n"))
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	if d := c.Link(); len(d) != 0 {
		t.Fatal(d)
	}
	f := c.Faces[0]
	if len(f.Keywords) != 1 || f.Keywords[0] != "Rampage:2" {
		t.Fatalf("precondition: keywords = %v, want [Rampage:2]", f.Keywords)
	}
	check := func() {
		t.Helper()
		if len(f.Triggers) != 1 {
			t.Fatalf("triggers = %+v, want exactly one", f.Triggers)
		}
		tr := f.Triggers[0]
		if tr.Mode != "AttackerBlocked" || tr.Params["ValidCard"] != "Card.Self" ||
			tr.Params["Keyword"] != "Rampage" || tr.Params["KeywordLine"] != "Rampage:2" {
			t.Fatalf("trigger = %+v, want AttackerBlocked Card.Self from Rampage:2", tr)
		}
		body := ResolveSVar(f.SVars, tr.Params["Execute"])
		if body == nil || body.API != "Pump" || body.Params["Defined"] != "Self" ||
			body.Params["NumAtt"] != "__kwRampageX" || body.Params["NumDef"] != "__kwRampageX" {
			t.Fatalf("body = %+v, want Pump Defined$ Self by the Rampage amount", body)
		}
		if f.SVars["__kwRampageX"] != "SVar$__kwRampageY/Times.2" {
			t.Fatalf("amount SVar = %q, want the blocker count times 2", f.SVars["__kwRampageX"])
		}
	}
	check()
	if d := c.Link(); len(d) != 0 {
		t.Fatal(d)
	}
	check()
}

func TestRampageWithoutAnAmountDoesNotExpand(t *testing.T) {
	c, diags := ParseBytes("rampage.txt", []byte("Name:Rampage fixture\nManaCost:3 G\nTypes:Creature Giant\nPT:3/3\nK:Rampage\nOracle:x\n"))
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	if d := c.Link(); len(d) != 0 {
		t.Fatal(d)
	}
	if f := c.Faces[0]; len(f.Triggers) != 0 {
		t.Fatalf("triggers = %+v, want none for a bare Rampage", f.Triggers)
	}
}
