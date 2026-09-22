// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import (
	"strconv"
	"strings"
)

func kwDevour(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("R", k) {
		return
	}
	// CR 702.83: "As <this> enters the battlefield, you may sacrifice
	// any number of <type>s. This enters the battlefield with a +1/+1
	// counter on it for each creature sacrificed this way." The
	// parameter is "<amount>[:<valid>[:display-text]]" -- amount 1/2/3/X,
	// valid defaults Creature (Feasting Hobbit's Food, Caprichrome's
	// Artifact, Famished Worldsire's Land are the corpus's typed
	// carriers); the trailing display fields are dropped. The expansion
	// is Forge's CardFactoryUtil Devour shape verbatim: one ETB
	// replacement whose body is an optional sacrifice ask (the batch is
	// remembered onto the source object), then the counter put reading
	// RememberedSize/Times.<amount>, then a cleanup clearing the memory.
	// Count$RememberedSize reads the event-backed Remembered list the
	// sacrifice primitive fills; /Times.N is the shared applyCountOp. A
	// Devour X (Thromok the Insatiable) carries the SVar-named operand
	// Times.X -- resolved against the face's own SVar:X (Count$RememberedSize)
	// by the count-op operand resolver (count-plus-svar-operand), which is
	// the oracle read: "enters with X +1/+1 counters on it for each of those
	// creatures" = n per devoured creature = n².
	amount, rest, _ := strings.Cut(param, ":")
	valid, _, _ := strings.Cut(rest, ":")
	amount, valid = strings.TrimSpace(amount), strings.TrimSpace(valid)
	if valid == "" {
		valid = "Creature"
	}
	sv := "__kwDevour" + strconv.Itoa(i)
	sacX := "__kwDevourSacX" + strconv.Itoa(i)
	cntX := "__kwDevourX" + strconv.Itoa(i)
	cn := "__kwDevourCounter" + strconv.Itoa(i)
	cl := "__kwDevourCleanup" + strconv.Itoa(i)
	f.setSVar(sacX, "Count$Valid "+valid+".YouCtrl+Other")
	f.setSVar(cntX, "Count$RememberedSize/Times."+amount)
	f.setSVar(sv, "DB$ Sacrifice | Defined$ You | Amount$ "+sacX+
		" | RememberSacrificed$ True | Optional$ True | SacValid$ "+valid+
		".Other | SubAbility$ "+cn)
	f.setSVar(cn, "DB$ PutCounter | ETB$ True | Defined$ Self | CounterType$ P1P1 | CounterNum$ "+cntX+
		" | SubAbility$ "+cl)
	f.setSVar(cl, "DB$ Cleanup | ClearRemembered$ True")
	p := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self" +
		" | ReplacementResult$ Updated | ReplaceWith$ " + sv + " | Keyword$ Devour")
	p["KeywordLine"] = k
	f.Repls = append(f.Repls, Repl{Event: "Moved", Params: p})
}

func init() { registerKeyword(kwDevour, "Devour") }
