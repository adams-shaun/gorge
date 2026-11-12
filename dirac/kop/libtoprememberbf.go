package kop

// LibTopRememberBF is OpToLibrary's C for an Origin$ Battlefield move of the
// source itself to the top of its owner's library with RememberChanged$ True
// (Elixir of Immortality's "shuffle Elixir into its owner's library", whose
// RememberedOwner shuffle follows): an object no longer on the battlefield
// does not move (gorge's Origin$ precondition), and a card that reached the
// library joins the remembered set.
const LibTopRememberBF = 13
