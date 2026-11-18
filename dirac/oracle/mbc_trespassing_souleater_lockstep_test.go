package oracle

import "testing"

// CR 107.4f / 601.2b: Trespassing Souleater's {U/P} activation, announced with
// only Crumbling Necropolis ({T}: U, B or R) untapped, offers both faces (pay
// {U}, pay 2 life). The kernel once offered only the life face
// (kfuzz tail-71-c000 pair 0, decision 274).
func TestMBCTrespassingSouleaterPipFaceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_pip_face_from_trisource_land")
}
