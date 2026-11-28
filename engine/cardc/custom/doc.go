// Package custom holds the register of card-specific nodes: the abilities the
// typed card interface (engine/cardc/ir) does not describe, which the engines
// implement in code written for that one card. Each such ability is an
// ir.Ability whose Custom field is set, and each has one row in REGISTER.tsv
// (card, scope, class, a one-sentence reason in our own words).
//
// The register is the counted tail of "every card". Its size is the constant
// customRegisterSize in register_test.go, which always equals the row count.
// Adding a row means raising that constant, and only the operator may sign off
// on a raise. Deleting a row lowers the constant in the same change.
package custom
