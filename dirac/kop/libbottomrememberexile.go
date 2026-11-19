package kop

// LibBottomRememberExile is OpToLibrary's C for an Origin$ Exile move of a
// target to the bottom of its owner's library with RememberChanged$ True
// (Time Reaper's "put target face-up card they own in exile on the bottom of
// their library. If you do, you gain 3 life"): a card no longer in exile does
// not move (gorge's Origin$ precondition), and a card that reached the
// library joins the remembered set the "if you do" gate reads.
const LibBottomRememberExile = 14
