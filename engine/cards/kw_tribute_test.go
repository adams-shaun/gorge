package cards

import "testing"

// TestTributeExpandsToETBReplacementAndUnpaidTrigger pins the K:Tribute
// expansion (CR 702.104, task kw-tribute-expansion): one Moved→Battlefield
// replacement whose body is the tribute offer (DB$ PutCounter on the
// ReplacedCard with the Tribute$ marker), plus the "if tribute wasn't paid"
// ETB trigger executing the face's own TrigNotTribute SVar with the
// SpellDescription$ as its description.
func TestTributeExpandsToETBReplacementAndUnpaidTrigger(t *testing.T) {
	f := expanded(t, "Name:C\nManaCost:1\nTypes:Creature\nPT:1/1\nK:Tribute:2\n"+
		"SVar:TrigNotTribute:DB$ GainLife | LifeAmount$ 4 | SpellDescription$ When CARDNAME enters, if tribute wasn't paid, you gain 4 life.\n"+
		"Oracle:x\n")
	if len(f.Repls) != 1 {
		t.Fatalf("%d repls, want 1", len(f.Repls))
	}
	r := f.Repls[0]
	if r.Event != "Moved" || r.Params["Destination"] != "Battlefield" ||
		r.Params["Keyword"] != "Tribute" || r.Params["KeywordLine"] != "Tribute:2" ||
		r.Params["ReplacementResult"] != "Updated" || r.With == nil {
		t.Fatalf("entry replacement: %+v", r)
	}
	w := r.With
	if w.API != "PutCounter" || w.Params["Defined"] != "ReplacedCard" ||
		w.Params["ETB"] != "True" || w.Params["CounterType"] != "P1P1" ||
		w.Params["CounterNum"] != "2" || w.Params["Tribute"] != "True" {
		t.Fatalf("tribute body: %+v", w)
	}
	if len(f.Triggers) != 1 {
		t.Fatalf("%d triggers, want 1", len(f.Triggers))
	}
	tr := f.Triggers[0]
	if tr.Mode != "ChangesZone" || tr.Params["Destination"] != "Battlefield" ||
		tr.Params["ValidCard"] != "Card.Self+notTributed" ||
		tr.Params["Execute"] != "TrigNotTribute" ||
		tr.Params["Keyword"] != "Tribute" || tr.Params["KeywordLine"] != "Tribute:2" {
		t.Fatalf("unpaid trigger: %+v", tr)
	}
	if got := tr.Params["TriggerDescription"]; got != "When CARDNAME enters, if tribute wasn't paid, you gain 4 life." {
		t.Fatalf("TriggerDescription = %q", got)
	}
}

// TestTributeSuffixAndIdempotence pins the param grammar: a display suffix
// ("3, where...") keeps only the leading count, a second expandKeywords
// never re-expands the line, and a face without the TrigNotTribute SVar
// keeps the replacement alone (the trigger executes that SVar by name, so
// Link would reject minting one without it).
func TestTributeSuffixAndIdempotence(t *testing.T) {
	f := expanded(t, "Name:C\nManaCost:1\nTypes:Creature\nPT:1/1\nK:Tribute:3:you pay life\nOracle:x\n")
	if got := f.Repls[0].With.Params["CounterNum"]; got != "3" {
		t.Fatalf("suffixed CounterNum = %q", got)
	}
	if len(f.Triggers) != 0 {
		t.Fatalf("%d triggers on a face without TrigNotTribute, want 0", len(f.Triggers))
	}
	n, m := len(f.Repls), len(f.Triggers)
	f.expandKeywords()
	if len(f.Repls) != n || len(f.Triggers) != m {
		t.Fatal("a second expansion re-expanded the Tribute line")
	}
}
