package oracle

import "testing"

// CR 702.193b with CR 106.6: Molly Hayes, Runaway entered this turn, so its
// Power-up ability costs {3}. Gorge's printed-cost planner cannot witness
// {5}{R} from the four untapped sources, but its prefix script search floats
// Mountain -> Mossfire Valley's {R}{G} and Island's {U}, and that pool pays
// the reduced charge, so gorge offers the activation. dirac refused it:
// canActivate dropped a Power-up ability whose printed witness failed before
// the filter script read the reduced cost.
func TestMBCMollyHayesRunawayReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "power_up_filter_script_offer")
}
