// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).
//
// The byte-level helpers below (tributeCut, tributeDigits) keep this file
// off the strings/strconv machinery the no-strings ratchet counts; they run
// once per face at corpus compile, and the shapes they recognise are
// load-time boundary text. Everything else goes through parseParams, the
// allowlisted boundary parser.

package cards

// kwTribute is CR 702.104's tribute: "Tribute N" means "As this creature
// enters, an opponent of its controller chooses for this creature to enter
// with N +1/+1 counters on it, or not to." The expansion mints the Forge
// CardFactoryUtil pair (CardFactoryUtil.java:1888, CountersPutEffect.java:493):
//
//   - one Moved->Battlefield replacement whose body is
//     DB$ PutCounter | Defined$ ReplacedCard | ETB$ True | CounterType$ P1P1
//     | CounterNum$ N | Tribute$ True -- the Tribute$ marker is what
//     putcounter_params turns into the two CR 702.104 asks (the entering
//     creature's controller picks the opponent, the chosen opponent elects
//     pay/decline), and its presence also blocks the entry-counter fold
//     (EntryFoldBlocked) so the asks actually run;
//   - the "if tribute wasn't paid" trigger, Execute$ the face's own
//     TrigNotTribute SVar (every corpus carrier ships one; its
//     SpellDescription$ is the minted trigger's description). The gate is the
//     notTributed predicate on the entering permanent: an accepted tribute
//     folds events.AlterAttribute "Tributed" (engine-transient designation,
//     cleared with the other designations on battlefield entry), a decline
//     folds nothing.
//
// The param is a bare literal count on every corpus line (measured: 11
// files, values 2/3/6), so it is spliced in verbatim after any trailing
// display field is cut.
func kwTribute(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	n := tributeCount(param)
	if !has("R", k) {
		put := "__kwTributePut" + tributeDigits(i)
		f.setSVar(put, "DB$ PutCounter | Defined$ ReplacedCard | ETB$ True | CounterType$ P1P1 | CounterNum$ "+n+" | Tribute$ True")
		p := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + put + " | Keyword$ Tribute")
		p["KeywordLine"] = k
		f.Repls = append(f.Repls, Repl{Event: "Moved", Params: p})
	}
	if !has("T", k) && f.SVars["TrigNotTribute"] != "" {
		// The trigger executes the face's OWN TrigNotTribute SVar -- a body
		// the corpus card ships (every carrier does, measured) and Link
		// validates by name -- so the expansion mints the trigger only when
		// the SVar exists; a face without one keeps the replacement alone
		// rather than minting an Execute$ Link rejects.
		p := parseParams("Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self+notTributed | Execute$ TrigNotTribute | Keyword$ Tribute | TriggerDescription$ " + tributeDesc(f))
		p["KeywordLine"] = k
		f.Triggers = append(f.Triggers, Trigger{Mode: p["Mode"], Params: p})
	}
}

// tributeCount is the leading count field of the K:Tribute line: the value
// before the first space, colon or comma (a display suffix, if any, is
// dropped). An unreadable value falls back to 1 rather than to a body the
// evaluator would read as an SVar name.
func tributeCount(param string) string {
	param = parseParams("N$ " + param)["N"]
	if j := tributeCut(param); j >= 0 {
		param = param[:j]
	}
	if param == "" {
		return "1"
	}
	return param
}

// tributeCut reports the index of the first space, colon or comma in s, or
// -1 when s holds none.
func tributeCut(s string) int {
	for j := 0; j < len(s); j++ {
		switch s[j] {
		case ' ', ':', ',':
			return j
		}
	}
	return -1
}

// tributeDigits renders n as its decimal digits (the __kw SVar suffix),
// hand-rolled so the suffix minting costs the ratchet no strconv site.
func tributeDigits(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return tributeDigits(n/10) + string(rune('0'+n%10))
}

// tributeDesc is the minted trigger's TriggerDescription: the face's own
// TrigNotTribute body's SpellDescription$ (the card's printed oracle line),
// or the bare keyword when the face ships none. parseParams reads the field
// by key, so a later `| Key$ …` in the body cannot leak into the text.
func tributeDesc(f *Face) string {
	if d := parseParams(f.SVars["TrigNotTribute"])["SpellDescription"]; d != "" {
		return d
	}
	return "Tribute"
}

func init() { registerKeyword(kwTribute, "Tribute") }
