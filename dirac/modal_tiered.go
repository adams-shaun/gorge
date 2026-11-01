package dirac

// DebugTieredNoModeCost is the mutation switch of TestFintieredMutation:
// spreeCost adds nothing, so a Tiered cast's chosen mode is free. The kernel
// records HasModeCost, not which keyword printed it (Spree or Tiered), so
// this switch disables every mode cost; the Tiered lockstep is the only test
// that sets it.
var DebugTieredNoModeCost bool
