# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Ramen is a Kubernetes-native Disaster Recovery system for Open Cluster Management (OCM). It orchestrates workload protection and placement across managed clusters through:
- **Relocate**: Planned migration to a peer cluster
- **Failover**: Unplanned recovery after cluster failure

Ramen operates in two controller modes:
- **Hub controller**: Runs on the OCM hub cluster, manages DR orchestration
- **DR-cluster controller**: Runs on managed clusters, handles local replication

All APIs are currently **alpha** and subject to breaking changes.

## Key Architecture

### Core Custom Resource Definitions

- **DRPolicy**: Defines DR policies between cluster pairs
- **DRPlacementControl (DRPC)**: Orchestrates DR actions (deploy/relocate/failover)
- **VolumeReplicationGroup (VRG)**: Manages PVC replication and recovery on managed clusters
- **DRCluster**: Represents a managed cluster in the DR topology
- **DRClusterConfig**: DR configuration for a managed cluster
- **ProtectedVolumeReplicationGroupList**: Hub view of VRGs across clusters
- **ReplicationGroupSource/Destination**: Manages cross-cluster replication

### Data Protection Approaches

1. **Storage Vendor Assisted**: Uses CSI storage replication via csi-addons APIs (e.g., ceph-csi)
2. **VolSync Based**: Uses volsync rsync for storage supporting VolumeSnapshots
3. **HA Storage**: No replication needed, uses NetworkFence for access control

### Workload Protection Approaches

1. **GitOps** (Recommended): ArgoCD ApplicationSets with OCM Placements
2. **Discovered Applications**: Uses velero to backup/restore application resources
3. **Recipe-based**: Vendor-supplied workflows for complex stateful apps

### Code Organization

- `api/v1alpha1/`: CRD type definitions
- `internal/controller/`: Controller reconciliation logic
  - `drplacementcontrol_controller.go`: Hub DRPC controller
  - `volumereplicationgroup_controller.go`: VRG controller on managed clusters
  - `drpolicy_controller.go`, `drcluster_controller.go`, etc.
  - `volsync/`: VolSync-specific replication logic
  - `hooks/`: Application hooks (exec, check, scale) for recipes
  - `kubeobjects/`: Velero integration for discovered apps
  - `util/`: Shared utilities (PVC handling, etc.)
- `cmd/main.go`: Operator entry point
- `config/`: Kustomize configurations for hub and dr-cluster deployments
- `test/`: Development environment setup (drenv tool)
- `e2e/`: End-to-end tests
- `ramendev/`: Python tool for deploying/configuring Ramen
- `ramenctl/`: CLI tool for Ramen operations

## Build and Development Commands

### Building

```bash
make build                 # Build manager binary to bin/manager
make docker-build          # Build container image (quay.io/ramendr/ramen-operator:latest)
make generate              # Generate DeepCopy methods for API types
make manifests             # Generate CRDs, RBAC, webhooks
```

### Code Quality

```bash
make lint                  # Run golangci-lint and pre-commit checks
make fmt                   # Format code with golangci-lint
```

Pre-commit hook automatically signs off commits. Install with:
```bash
cp hack/commit-msg .git/hooks/
```

### Testing

#### Unit and Integration Tests

Tests use ginkgo/gomega with envtest (single API server). Mock interfaces simulate OCM, S3, and CSI components.

```bash
make test                  # Run all tests
make coverage              # Open HTML coverage report

# Focused test targets:
make test-vrg              # VolumeReplicationGroup tests
make test-drpc             # DRPlacementControl tests
make test-vs               # VolSync tests
make test-drcluster        # DRCluster tests
make test-drpolicy         # DRPolicy tests
make test-kubeobjects      # Velero integration tests
make test-util             # Utility tests
```

See `Makefile` for complete list of `test-*` targets.

#### Development Environment Tests

Requires Python virtual environment and 3 minikube clusters (8 CPUs, 16 GiB RAM recommended).

```bash
# Setup
make venv                  # Create Python virtual environment
source venv                # Activate virtual environment

# Create/destroy 3-cluster test environment
make create-rdr-env        # Uses drenv to create regional-dr environment
make destroy-rdr-env       # Tear down environment

# Deploy and configure Ramen
ramendev deploy test/envs/regional-dr.yaml
ramendev config test/envs/regional-dr.yaml

# Run tests
make test-drenv            # Run drenv tests
test/basic-test/run test/envs/regional-dr.yaml  # Run basic DR flow test

# Cleanup
ramendev unconfig test/envs/regional-dr.yaml
ramendev undeploy test/envs/regional-dr.yaml
```

See `test/README.md` for detailed drenv setup and `docs/devel-quick-start.md` for development workflow.

#### End-to-End Tests

```bash
make e2e-rdr               # Run e2e tests (requires deployed environment)
```

### Deployment Commands

```bash
# Hub controller
make install-hub           # Install hub CRDs
make deploy-hub            # Deploy hub controller
make undeploy-hub          # Remove hub controller
make uninstall-hub         # Remove hub CRDs

# DR-cluster controller
make install-dr-cluster    # Install dr-cluster CRDs
make deploy-dr-cluster     # Deploy dr-cluster controller
make undeploy-dr-cluster   # Remove dr-cluster controller
make uninstall-dr-cluster  # Remove dr-cluster CRDs

# Both
make install               # Install both hub and dr-cluster CRDs
make deploy                # Deploy both controllers
make undeploy              # Remove both controllers
make uninstall             # Remove both CRDs
```

Environment variables:
- `IMAGE_REGISTRY`: Container registry (default: quay.io)
- `IMAGE_REPOSITORY`: Repository (default: ramendr)
- `IMAGE_TAG`: Image tag (default: latest)
- `PLATFORM`: Deployment platform - k8s or ocp (default: k8s)

### Running Locally

```bash
# Run hub controller locally against ~/.kube/config cluster
make run-hub

# Run dr-cluster controller locally
make run-dr-cluster
```

## Development Notes

### Controller Configuration

Controllers read configuration from YAML files:
- `examples/dr_hub_config.yaml`: Hub controller config
- `examples/dr_cluster_config.yaml`: DR-cluster controller config

Config includes RamenControllerType (dr-hub or dr-cluster), metrics bind address, leader election settings, etc.

### Testing with Mock Interfaces

Ramen uses interfaces to enable testing without external dependencies:
- **ObjectStorer**: S3 operations (see `internal/controller/s3utils.go`)
- **MultiClusterView**: OCM managed cluster views
- **CSI components**: Storage replication APIs

Mock implementations in `*_test.go` files allow simulating success/failure scenarios.

### Code Generation

Ramen uses kubebuilder v4 for scaffolding. After modifying API types:

```bash
make generate              # Update generated DeepCopy methods
make manifests             # Update CRDs, RBAC, webhooks
```

### Commit Message Format

All commits must be signed off (DCO). The commit-msg hook adds this automatically:
```
Signed-off-by: Your Name <email@example.com>
```

Follow conventional commit style. Check recent commits with:
```bash
git log --oneline -10
```

### Linting Requirements

golangci-lint runs multiple linters (see `.golangci.yaml`). The `hack/pre-commit.sh` script also checks:
- Shell scripts with shellcheck
- YAML files with yamllint
- Markdown files with mdl
- REUSE compliance for license headers

Each Go module (root, e2e, api) is linted separately.

### Tools Installation

Development tools are installed to local directories:
- `bin/`: controller-gen, kustomize, opm, operator-sdk
- `testbin/`: golangci-lint, setup-envtest

Installation scripts in `hack/install-*.sh` download specific versions.

### Kubernetes Client Authentication

The operator imports all k8s client auth plugins (`k8s.io/client-go/plugin/pkg/client/auth`) to support various authentication methods (GCP, Azure, OIDC, etc.).

## Common Workflows

### Adding a New Controller

1. Use kubebuilder to scaffold (or manually create in `internal/controller/`)
2. Update `cmd/main.go` to register the controller
3. Add RBAC markers in controller code
4. Run `make generate manifests` to update generated files
5. Add tests in `*_test.go` with ginkgo/gomega
6. Update `suite_test.go` if needed for test setup

### Modifying API Types

1. Edit types in `api/v1alpha1/*_types.go`
2. Add kubebuilder markers for validation, printing, etc.
3. Run `make generate manifests`
4. Update controller logic to handle new fields
5. Add tests for new behavior

### Working on Hooks (Recipes)

Recipe hooks are in `internal/controller/hooks/`:
- `exec_hook.go`: Execute commands in pods (pre/post actions)
- `check_hook.go`: Check application status
- `scale_hook.go`: Scale deployments/statefulsets

Each hook type has corresponding pod listers (`exec_pod_lister_*.go`) for finding pods from various workload types (Deployment, StatefulSet, DaemonSet, Job, CronJob).

### Debugging Controllers

1. Enable verbose logging by setting controller config `zap` development mode
2. Use `kubectl logs` to view controller logs in deployed pods
3. For local development, run controllers with `make run-hub` or `make run-dr-cluster`
4. Check CR status conditions for detailed error messages
5. Use metrics endpoint for observability (configured in controller config)

### Working with drenv

drenv is a Python-based tool for creating test environments. Key files:
- `test/envs/*.yaml`: Environment definitions (regional-dr, metro-dr, etc.)
- `test/drenv/`: drenv Python package
- `test/addons/`: OCM, Submariner, MinIO, Velero, etc. deployment configs

Common drenv commands:
```bash
drenv start envs/regional-dr.yaml          # Start clusters
drenv stop envs/regional-dr.yaml           # Stop clusters
drenv delete envs/regional-dr.yaml         # Delete clusters
drenv kubectl get pods --all --context hub # Run kubectl across clusters
```
