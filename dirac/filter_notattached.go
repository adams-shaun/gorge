package dirac

// kop.FNotAttachedBySrc, the '!EnchantedBy' /
// '!EquippedBy' filter word ("other than enchanted creature"; gorge
// matchPredicate's negation of attachedBy, engine/effects/filter.go). The
// three permanent matchers that read FAttachedBySrc with the source slot in
// hand (matchPermOne, chars.go, static_layerwalk.go) reject the source's live host
// beside the positive test; a source that left the battlefield passes NoSlot
// (srcSlot's version check), so nothing is excluded, and the card matchers
// read the flag as true (gorge: attachedBy is false off the battlefield).

// DebugNotAttachedPositive is a test mutation (TestNotenchMutation): with a
// live source, FNotAttachedBySrc keeps only the source's host, the positive
// form's reading.
var DebugNotAttachedPositive bool
