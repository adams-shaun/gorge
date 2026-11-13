package kop

// StatProtYourColors is a Continuous-style grant of "protection from the
// colors of permanents you control" (Forge's Protection from colorsYouCtrl,
// Empty-Shrine Kannushi; CR 702.16): each battlefield permanent matching
// Filter can't be damaged (prevented), blocked, targeted, enchanted or
// equipped by a source that shares a colour with some permanent its
// controller controls. The colours are read live when the check is made, the
// protected permanent itself included. A layer-6 grant: it does not survive
// a later loss of the recipient's abilities. Kernel half:
// dirac/kw_protyours.go. Past StatBlockTax: every ordinal below 257 is taken.
const StatProtYourColors StaticKind = 257
