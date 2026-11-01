package rules

// Dual-run tests (layering design §7.7): each converted ask site,
// driven on legacy and on the tape kernel by the same deterministic driver
// (tapeDual), must stay event-, intent-, head- and RNG-identical, and its
// asks must be served from the tape with no legacy switch.
