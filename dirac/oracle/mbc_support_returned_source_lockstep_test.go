package oracle

import "testing"

// CR 701.41a with CR 400.7: support's "other target creatures" excludes the
// source only while it is the same object. Mirror Image enters as a copy of
// Saddleback Lagac and its support 2 trigger goes on the stack; in response
// Nephalia Smuggler flickers it, and it returns as a Lagac copy again. When
// the first trigger resolves, the Mirror Image on the battlefield is a new
// object, so it is an eligible counter_pick (gorge's SourceReturned). dirac
// once compared the object id alone and withheld it (decision 723; kfuzz
// -synth tail-32, focus Saddleback Lagac + Salt Marsh, pair 1, autopay,
// mulligans 1, at land 81a6ccdb4b).
func TestMBCMirrorImageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "support_returned_source")
}
