// The operator is a SEPARATE Go module so its heavy Kubernetes dependency tree
// (controller-runtime, client-go) never burdens the core `shoebill` build. The core
// module builds and tests with two direct dependencies; this module is built only when
// you are developing the on-cluster controller. Run `go mod tidy` here in a
// cluster-dev environment to resolve the versions below.
module github.com/dibakshya01/chaotic-shoebill-stork/operator

go 1.24.0

require (
	k8s.io/apimachinery v0.31.3
	sigs.k8s.io/controller-runtime v0.19.3
)
