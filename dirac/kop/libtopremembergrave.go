package kop

// LibTopRememberGrave is OpToLibrary's C for an Origin$ Graveyard move of a
// target to the top of its owner's library with RememberChanged$ True
// (Lagonna-Band Storyteller's "put target enchantment card from your
// graveyard on top of your library. If you do, you gain life equal to its
// mana value"): a card no longer in the graveyard does not move (gorge's
// Origin$ precondition), and a card that reached the library joins the
// remembered set the following "if you do" read sees.
const LibTopRememberGrave = 15
