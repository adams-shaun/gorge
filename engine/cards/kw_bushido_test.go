package cards

import "testing"

func TestBushidoExpandsBothBlockHalves(t *testing.T) {
	c, diags := ParseBytes("bushido.txt", []byte("Name:Bushido fixture\nManaCost:1 G\nTypes:Creature Samurai\nPT:2/2\nK:Bushido:2\nOracle:x\n"))
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	if d := c.Link(); len(d) != 0 {
		t.Fatal(d)
	}
	f := c.Faces[0]
	if len(f.Keywords) != 1 || f.Keywords[0] != "Bushido:2" {
		t.Fatalf("precondition: keywords = %v, want [Bushido:2]", f.Keywords)
	}
	check := func() {
		t.Helper()
		if len(f.Triggers) != 2 {
			t.Fatalf("triggers = %+v, want exactly two", f.Triggers)
		}
		if f.Triggers[0].Mode != "Blocks" || f.Triggers[1].Mode != "AttackerBlocked" {
			t.Fatalf("modes = %q, %q, want Blocks, AttackerBlocked", f.Triggers[0].Mode, f.Triggers[1].Mode)
		}
		exec := f.Triggers[0].Params["Execute"]
		if exec == "" || f.Triggers[1].Params["Execute"] != exec {
			t.Fatalf("Execute names = %q, %q, want the same non-empty SVar", exec, f.Triggers[1].Params["Execute"])
		}
		for _, tr := range f.Triggers {
			if tr.Params["ValidCard"] != "Card.Self" || tr.Params["Keyword"] != "Bushido" || tr.Params["KeywordLine"] != "Bushido:2" {
				t.Fatalf("trigger = %+v, want ValidCard Card.Self from Bushido:2", tr)
			}
		}
		body := ResolveSVar(f.SVars, exec)
		if body == nil || body.API != "Pump" || body.Params["Defined"] != "Self" ||
			body.Params["NumAtt"] != "+2" || body.Params["NumDef"] != "+2" {
			t.Fatalf("body = %+v, want Pump Defined$ Self +2/+2", body)
		}
	}
	check()
	if d := c.Link(); len(d) != 0 {
		t.Fatal(d)
	}
	check()
}

func TestBushidoWithoutAnAmountDoesNotExpand(t *testing.T) {
	c, diags := ParseBytes("bushido.txt", []byte("Name:Bushido fixture\nManaCost:1 G\nTypes:Creature Samurai\nPT:2/2\nK:Bushido\nOracle:x\n"))
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	if d := c.Link(); len(d) != 0 {
		t.Fatal(d)
	}
	f := c.Faces[0]
	if len(f.Keywords) != 1 || f.Keywords[0] != "Bushido" {
		t.Fatalf("precondition: keywords = %v, want [Bushido]", f.Keywords)
	}
	if len(f.Triggers) != 0 {
		t.Fatalf("triggers = %+v, want none for a bare Bushido", f.Triggers)
	}
}
