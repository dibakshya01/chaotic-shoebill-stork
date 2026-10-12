// Package crd provides helpers to locate and sanity-check the operator's CRD
// manifests from the core module's tests (the operator itself is a separate module).
package crd

// Group is the API group the CRDs belong to.
const Group = "chaos.shoebill.dev"

// Version is the served/storage version.
const Version = "v1alpha1"

// Kinds are the custom resource kinds the operator defines.
var Kinds = []string{"SecurityChaosExperiment", "SecurityChaosRun"}
