// Package manifest reads and checks dirac's namespace manifest: one small file
// per vocabulary code, under dirac/manifest/<Family>/<GoName>, that gives the
// code its value once and for all. The generator (dirac/cmd/kmanifest) turns
// the entries into Go constants and name registrations.
//
// # Entry format
//
// An entry is a few "key: value" lines, stdlib-parsed, with no extension:
//
//	family: CounterKind
//	name: CtrTicket
//	value: 82
//	forge: TICKET
//	census: ctr TICKET all=23 std=0 mbc=0 sole=0
//	state: code 0
//	status: reserved
//
// The keys, each at most once and in any order:
//
//   - family (required): the Go type, one of Families. It names the entry's
//     directory.
//   - name (required): the Go identifier, exported for a kop family and
//     unexported for a dirac one, with the family's prefix. It names the file.
//   - value (required): a decimal integer, or "next" for a late-bound entry
//     that landing gives its final value (Assign). A flag family's value is a
//     single bit.
//   - approval: who approved a value in a reserved band (Op's hot band
//     [158,249]). Without it neither an explicit value nor "next" may land
//     in the band.
//   - forge: the Forge vocabulary token(s), separated by " | ". Vocabulary
//     facts only, never script text.
//   - census: the census row the entry was seeded from (informational).
//   - state: the State category and bytes ("code 0", "slotbit 16", ...).
//   - contexts: a FilterWord's matcher kinds, comma-separated, from perm
//     (the permanent matcher), card (a card off the battlefield), spell (a
//     spell on the stack) and cur (the layer-walk matcher). kgen refuses a
//     word in a matcher not listed. Only a FilterWord has one, and it has one.
//   - status (required): reserved, active, "blocked <why>" or
//     "deferred <why>". The only line an activating change edits.
//   - ticket: the activating ticket.
//   - doc: free text.
//
// Blank lines are ignored; anything else is an error naming the file.
//
// # Invariants
//
//  1. A value never changes and a name is never deleted: of an entry with a
//     value, only status and ticket change.
//  2. An entry is active exactly when the kernel implements its code.
//  3. No inactive code reaches a table.
//
// Validate holds one tree to invariant 1 (one value per code, no collision
// with any constant ordinals.tsv records, the family's legal range and
// reserved bands) and supplies invariant 3's lookup: Tree.Lookup finds the
// entry a table's code names, and Entry.RefusalClass is the kgen refusal class
// of a code that is not active. Check holds a change to invariant 1 against
// its base, and holds every ordinals.tsv row of the base in place.
//
// ordinals.tsv records the hand constants only: the row of a constant that is a
// kop entry's own is derived from the entry (Tree.EntryOrdinals), so a new
// entry edits no shared file. Resolution ask kinds (the ask family, package
// dirac's ra* constants) are the same: asks.tsv freezes the hand ones, Check
// refuses any row it would add, and a new kind is an entry with "value: next".
//
// # Seeding
//
// PlanSeed pre-maps the vocabulary once: one reserved entry per census item
// kgen does not lower (counter types, keywords, APIs, Count$ heads, trigger,
// static and replacement modes, zones) and per bounded State key, at the
// lowest free values, so a change activates a code that already exists.
// aliases.tsv lists the items that are not new codes: existing constants,
// partially lowered items, and rows that name no code of their family. A
// re-run adds only new vocabulary; it never edits an entry.
//
// # Extension handlers
//
// A new extension sub-op adds no case to the hand switch (execExt,
// dirac/extop.go). Its handler, a State method with execExt's signature in the
// feature's own file, carries "//kmanifest:ext ExtFoo" in its doc comment, and
// gen writes execExtGen into dirac/manifest_gen.go: one case per handler, run
// from the hand switch's default arm (ScanExt).
//
// The other kernel switches take the same shape (ScanKernel). "//kmanifest:op
// OpFoo" marks a hot-band op's handler (func (s *State) m(k *prep, in
// *kop.Instr) (ok, done bool)), run by execOpGen from the cold op chain's end.
// A resolution ask has two: "//kmanifest:askopts raFoo" on func (s *State)
// m(k *prep, d *Decision) and "//kmanifest:askapply raFoo" on func (s *State)
// m(k *prep, a Action), run by askOptionsGen and applyAskGen from the ends of
// the hand chains (askOptionsCold1, applyAskCold1). A code whose hand switch
// already has a case is refused, since the hand arm would run first.
//
// A filter word (family FilterWord, a literal of a kop.PredProg filter
// program) has one handler, "//kmanifest:fword FwFoo" on func (s *State)
// m(c *fctx, l *kop.FLit) bool in its own file dirac/filter_<name>.go; gen writes
// fwRun, the switch filterProg runs each literal through, and fwHandled.
//
// # Late binding
//
// A "value: next" entry is rendered at a provisional value, the lowest free
// one of its family, so a change can build and test before its code has a
// number. Landing runs Assign serially against the landing branch's tip,
// which writes the final value into the entry's value line. Nothing has seen
// a provisional value, so moving one is safe.
package manifest
