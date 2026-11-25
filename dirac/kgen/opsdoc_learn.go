package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ext:Learn", "OpExt ExtLearn (F the Lesson filter)",
			"DB$ Learn / AB$ Learn (CR 701.48a; gorge effLearn): the resolving controller is asked once (ChLearn, gorge's \"learn\" KChoose): mode 0 do nothing, first; mode 1 over each sideboard card they own matching F (Card.Lesson+YouOwn), in object order, which is revealed and put into their hand; mode 2 rummage, offered only with a card in hand. Rummage asks the discard (ChRecruitDiscard, gorge's \"discard\" KChoose in hand order, asked even with one card), then draws a card (the dredge ask first when a candidate exists). A seat without a sideboard is offered nothing or the rummage. Arcane Subtraction, Eyetwitch, Poet's Quill, Study Break. dirac/act_learn.go."})
}
