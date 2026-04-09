# ConfigMap-Based Excluded Resources Management - Implementation Summary

## Overview

This implementation provides a flexible, user-configurable approach for managing Velero backup excluded resources through a Kubernetes ConfigMap instead of hardcoded values.

## Problem Solved

Previously, the list of resources to exclude from Velero backups was hardcoded in `internal/controller/kubeobjects/velero/requests.go`. This required code changes and redeployment to modify the exclusion list, making it difficult for users to customize based on their needs.

## Solution

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  VolumeReplicationGroupReconciler (controller startup)      │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  ExcludedResourcesManager.EnsureConfigMap()            │ │
│  │  ├─ Check if ConfigMap exists                          │ │
│  │  ├─ If not: Create with hardcoded defaults             │ │
│  │  └─ Parse and cache excluded resources                 │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│  During Backup (VRGInstance.kubeObjectsGroupCapture)        │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  mergeExcludedResources()                              │ │
│  │  ├─ Get ConfigMap defaults (cached)                    │ │
│  │  ├─ Get Recipe group exclusions                        │ │
│  │  ├─ Merge: defaults ∪ recipe exclusions                │ │
│  │  └─ Deduplicate                                        │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│  Velero Backup Creation                                     │
│  (ExcludedResources = merged list)                          │
└─────────────────────────────────────────────────────────────┘
```

### Key Components

#### 1. ExcludedResourcesManager (`internal/controller/excludedresources.go`)

New file that manages the ConfigMap lifecycle:

- **EnsureConfigMap()**: Ensures ConfigMap exists, creates with defaults if missing
- **ReloadExcludedResources()**: Reloads when ConfigMap is updated
- **parseExcludedResources()**: Supports JSON and comma-separated formats
- **validateCriticalExclusions()**: Warns if critical resources are missing

**Constants:**
- `DefaultExcludedResourcesConfigMapName = "default-excluded-resources"`
- `ExcludedResourcesKey = "resources"`

#### 2. VolumeReplicationGroupReconciler Updates

**New Fields:**
```go
excludedResourcesMgr   *ExcludedResourcesManager
excludedResourcesMutex sync.RWMutex
cachedExcludedResources []string
```

**Initialization (SetupWithManager):**
```go
r.excludedResourcesMgr = NewExcludedResourcesManager(
    r.Client,
    RamenOperatorNamespace(),
    r.Log.WithName("excluded-resources"),
)
```

**ConfigMap Watching:**
- Extended `configMapFun()` to watch for `default-excluded-resources` ConfigMap
- Automatically reloads when ConfigMap is updated

#### 3. VRGInstance Merging Logic (`vrg_kubeobjects.go`)

**New Method:**
```go
func (v *VRGInstance) mergeExcludedResources(spec kubeobjects.Spec) kubeobjects.Spec
```

**Merge Strategy:**
1. Get ConfigMap defaults (thread-safe read from cache)
2. Append Recipe group-level exclusions
3. Deduplicate the combined list
4. Return new Spec with merged exclusions

**Called from:** `kubeObjectsGroupCapture()` before `ProtectRequestCreate()`

#### 4. Velero Package Cleanup (`kubeobjects/velero/requests.go`)

**Before:**
```go
ExcludedResources: append(objectsSpec.ExcludedResources, 
    "volumereplications.replication.storage.openshift.io",
    "volumegroupreplications.replication.storage.openshift.io", 
    // ... hardcoded list
),
```

**After:**
```go
// ExcludedResources are now managed via ConfigMap
ExcludedResources: objectsSpec.ExcludedResources,
```

## ConfigMap Structure

**Name:** `default-excluded-resources`  
**Namespace:** Ramen operator namespace (from `POD_NAMESPACE` env var)  
**Format:** JSON (primary) or comma-separated (fallback)

### Example ConfigMap (JSON format)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: default-excluded-resources
  namespace: ramen-system
  labels:
    app.kubernetes.io/name: ramen-dr-cluster-operator
    app.kubernetes.io/component: kubeobjects-protection
    app.kubernetes.io/managed-by: ramen-operator
  annotations:
    ramen.openshift.io/description: |
      Default list of resources to exclude from Velero backups.
      You can modify this ConfigMap to customize the exclusions.
      Changes will be picked up on the next reconciliation.
    ramen.openshift.io/version: v1
data:
  resources: |
    [
      "volumereplications.replication.storage.openshift.io",
      "volumegroupreplications.replication.storage.openshift.io",
      "replicationsources.volsync.backube",
      "replicationdestinations.volsync.backube",
      "persistentvolumeclaims",
      "persistentvolumes",
      "endpointslices.discovery.k8s.io",
      "endpoints",
      "volumesnapshots.snapshot.storage.k8s.io",
      "volumegroupsnapshots.groupsnapshot.storage.k8s.io"
    ]
```

### Example ConfigMap (Comma-separated format)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: default-excluded-resources
  namespace: ramen-system
data:
  resources: |
    volumereplications.replication.storage.openshift.io,
    volumegroupreplications.replication.storage.openshift.io,
    replicationsources.volsync.backube,
    replicationdestinations.volsync.backube,
    persistentvolumeclaims,
    persistentvolumes,
    endpointslices.discovery.k8s.io,
    endpoints,
    volumesnapshots.snapshot.storage.k8s.io,
    volumegroupsnapshots.groupsnapshot.storage.k8s.io
```

## Default Excluded Resources

The ConfigMap is auto-created with these defaults if not present:

1. **volumereplications.replication.storage.openshift.io** ⚠️ CRITICAL
   - Reason: VRG creates these; see https://github.com/RamenDR/ramen/issues/884

2. **volumegroupreplications.replication.storage.openshift.io** ⚠️ CRITICAL
   - Reason: VRG manages these for VolumeGroup replication

3. **replicationsources.volsync.backube** ⚠️ CRITICAL
   - Reason: VolSync resources managed by VRG

4. **replicationdestinations.volsync.backube** ⚠️ CRITICAL
   - Reason: VolSync resources managed by VRG

5. **persistentvolumeclaims**
   - Reason: Handled separately by VRG volume protection

6. **persistentvolumes**
   - Reason: Handled separately by VRG volume protection

7. **endpointslices.discovery.k8s.io**
   - Reason: Prevents Submariner conflicts; see https://github.com/RamenDR/ramen/issues/1889

8. **endpoints**
   - Reason: Prevents Submariner conflicts

9. **volumesnapshots.snapshot.storage.k8s.io**
   - Reason: Snapshots handled separately

10. **volumegroupsnapshots.groupsnapshot.storage.k8s.io**
    - Reason: GroupSnapshots handled separately

⚠️ **CRITICAL** resources should NOT be removed from exclusions as they are required for proper VRG operation.

## Usage

### For Operators/Administrators

#### View Current Exclusions

```bash
kubectl get configmap default-excluded-resources -n ramen-system -o yaml
```

#### Modify Exclusions

```bash
kubectl edit configmap default-excluded-resources -n ramen-system
```

**Add a new exclusion:**
```bash
kubectl patch configmap default-excluded-resources -n ramen-system --type merge -p '
{
  "data": {
    "resources": "[\"volumereplications.replication.storage.openshift.io\",\"volumegroupreplications.replication.storage.openshift.io\",\"replicationsources.volsync.backube\",\"replicationdestinations.volsync.backube\",\"persistentvolumeclaims\",\"persistentvolumes\",\"endpointslices.discovery.k8s.io\",\"endpoints\",\"volumesnapshots.snapshot.storage.k8s.io\",\"volumegroupsnapshots.groupsnapshot.storage.k8s.io\",\"secrets\"]"
  }
}'
```

#### Restore Defaults

Delete the ConfigMap; it will be recreated with defaults:

```bash
kubectl delete configmap default-excluded-resources -n ramen-system
```

### For Recipe Authors

Recipe groups can still specify additional exclusions via `excludedResourceTypes`:

```yaml
apiVersion: ramendr.openshift.io/v1alpha1
kind: Recipe
metadata:
  name: my-app-recipe
spec:
  groups:
  - name: app-config
    includedNamespaces:
    - my-app-namespace
    excludedResourceTypes:
    - secrets          # Added to ConfigMap defaults
    - configmaps       # Added to ConfigMap defaults
```

**Final exclusions = ConfigMap defaults ∪ Recipe group exclusions**

## Behavior

### On Controller Startup

1. Check if `default-excluded-resources` ConfigMap exists
2. If **exists**: Parse and cache exclusions
3. If **missing**: Create with hardcoded defaults, then cache
4. Validate critical exclusions, log warnings if missing

### During Backup Creation

1. Read cached defaults (thread-safe)
2. Merge with Recipe group exclusions
3. Deduplicate
4. Pass to Velero API

### On ConfigMap Update

1. Detect via Watch
2. Reload exclusions
3. Validate and cache
4. **Next backup** uses new exclusions (not retroactive)

### On ConfigMap Deletion

1. Detect via Watch
2. Auto-recreate with defaults
3. Log the recreation

## RBAC

Updated permissions for VolumeReplicationGroup controller:

```yaml
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch", "create"]
```

## Validation & Safety

### Critical Resource Validation

If critical resources are missing from the ConfigMap, warnings are logged:

```
WARNING: Critical resource not in exclusions list, this may cause issues
  resource=volumereplications.replication.storage.openshift.io
  reason=Required by Ramen for proper VRG operation
```

### Error Handling

- **Invalid ConfigMap format**: Falls back to hardcoded defaults, logs error
- **Empty ConfigMap**: Treats as empty list, logs info
- **Reload failure**: Uses previous cache, logs error
- **ConfigMap deleted**: Auto-recreates with defaults

## Testing

### Unit Testing

Test the ExcludedResourcesManager:

```go
// Test ConfigMap creation
// Test parsing JSON format
// Test parsing comma-separated format
// Test validation
// Test reload
// Test merge logic
```

### Integration Testing

1. Deploy Ramen operator
2. Verify ConfigMap auto-creation
3. Modify ConfigMap
4. Trigger backup
5. Verify exclusions in Velero Backup CR

### Manual Verification

```bash
# 1. Check ConfigMap exists
kubectl get cm default-excluded-resources -n ramen-system

# 2. Create a VRG with KubeObjectProtection
kubectl apply -f my-vrg.yaml

# 3. Check Velero Backup CR
kubectl get backup -n velero -o yaml | grep -A 20 excludedResources

# 4. Modify ConfigMap
kubectl edit cm default-excluded-resources -n ramen-system

# 5. Trigger new backup (update VRG)
kubectl annotate vrg my-vrg force-backup=true

# 6. Verify new exclusions
kubectl get backup -n velero -o yaml | grep -A 20 excludedResources
```

## Migration Notes

### From Existing Deployments

For existing Ramen deployments, the ConfigMap will be auto-created on controller restart with current hardcoded values. **No manual migration needed.**

### Rollback Procedure

If issues arise:

1. Revert to previous Ramen version
2. Delete the ConfigMap (optional)
3. Previous hardcoded behavior will be restored

## Future Enhancements

Potential improvements:

1. **Per-Recipe Exclusions**: Add recipe-level global exclusions (in addition to group-level)
2. **Namespace-specific Exclusions**: Different defaults per namespace
3. **Exclusion Policies**: Support patterns/wildcards (e.g., `*.example.com`)
4. **Negative Exclusions**: Allow "un-excluding" from defaults (e.g., `!volumesnapshots`)
5. **Validation Webhook**: Prevent removal of critical resources
6. **Status Reporting**: Report active exclusions in VRG status

## Troubleshooting

### Issue: Backups excluding wrong resources

**Check:**
```bash
# 1. Verify ConfigMap content
kubectl get cm default-excluded-resources -n ramen-system -o yaml

# 2. Check VRG logs
kubectl logs -n ramen-system deployment/ramen-dr-cluster-operator | grep excluded

# 3. Check Velero Backup CR
kubectl describe backup -n velero <backup-name>
```

### Issue: ConfigMap not being created

**Check:**
```bash
# 1. Verify operator is running
kubectl get pods -n ramen-system

# 2. Check operator logs
kubectl logs -n ramen-system deployment/ramen-dr-cluster-operator

# 3. Check RBAC permissions
kubectl auth can-i create configmaps --as=system:serviceaccount:ramen-system:ramen-dr-cluster-operator -n ramen-system
```

### Issue: Changes not taking effect

**Cause:** Changes are not retroactive; only new backups use updated exclusions.

**Solution:** Trigger a new backup or wait for next scheduled backup.

## Files Modified/Created

| File | Change Type | Description |
|------|-------------|-------------|
| `internal/controller/excludedresources.go` | **Created** | ConfigMap manager implementation |
| `internal/controller/volumereplicationgroup_controller.go` | **Modified** | Added manager, caching, watching |
| `internal/controller/vrg_kubeobjects.go` | **Modified** | Added merge logic |
| `internal/controller/kubeobjects/velero/requests.go` | **Modified** | Removed hardcoded exclusions |
| `config/rbac/role.yaml` | **Modified** | Added ConfigMap permissions |

## References

- [Issue #884](https://github.com/RamenDR/ramen/issues/884): VR exclusion requirement
- [Issue #1889](https://github.com/RamenDR/ramen/issues/1889): Submariner conflicts

## Author

Implementation by: Annaraya Narasagond
Date: 2026-04-09
