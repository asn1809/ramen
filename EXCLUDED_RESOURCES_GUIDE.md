# Excluded Resources Configuration Guide

Quick reference for managing Velero backup excluded resources via ConfigMap.

## Quick Start

### View Current Exclusions

```bash
kubectl get configmap default-excluded-resources -n ramen-system -o jsonpath='{.data.resources}' | jq .
```

### Add an Exclusion

```bash
# Get current exclusions
CURRENT=$(kubectl get cm default-excluded-resources -n ramen-system -o jsonpath='{.data.resources}')

# Add 'secrets' to the list (modify the array as needed)
kubectl patch configmap default-excluded-resources -n ramen-system --type merge -p '{
  "data": {
    "resources": "[\"volumereplications.replication.storage.openshift.io\",\"volumegroupreplications.replication.storage.openshift.io\",\"replicationsources.volsync.backube\",\"replicationdestinations.volsync.backube\",\"persistentvolumeclaims\",\"persistentvolumes\",\"endpointslices.discovery.k8s.io\",\"endpoints\",\"volumesnapshots.snapshot.storage.k8s.io\",\"volumegroupsnapshots.groupsnapshot.storage.k8s.io\",\"secrets\"]"
  }
}'
```

### Remove an Exclusion

⚠️ **WARNING:** Do not remove critical resources (VR, VGR, VolSync resources)

```bash
# Edit manually
kubectl edit configmap default-excluded-resources -n ramen-system
```

### Reset to Defaults

```bash
kubectl delete configmap default-excluded-resources -n ramen-system
# Will be auto-recreated with defaults
```

## Common Scenarios

### Scenario 1: Exclude All Secrets

```bash
kubectl patch configmap default-excluded-resources -n ramen-system --type json -p '[
  {
    "op": "replace",
    "path": "/data/resources",
    "value": "[\"volumereplications.replication.storage.openshift.io\",\"volumegroupreplications.replication.storage.openshift.io\",\"replicationsources.volsync.backube\",\"replicationdestinations.volsync.backube\",\"persistentvolumeclaims\",\"persistentvolumes\",\"endpointslices.discovery.k8s.io\",\"endpoints\",\"volumesnapshots.snapshot.storage.k8s.io\",\"volumegroupsnapshots.groupsnapshot.storage.k8s.io\",\"secrets\"]"
  }
]'
```

### Scenario 2: Exclude Service Accounts

```bash
# Add serviceaccounts to the list
kubectl patch configmap default-excluded-resources -n ramen-system --type json -p '[
  {
    "op": "replace",
    "path": "/data/resources",
    "value": "[\"volumereplications.replication.storage.openshift.io\",\"volumegroupreplications.replication.storage.openshift.io\",\"replicationsources.volsync.backube\",\"replicationdestinations.volsync.backube\",\"persistentvolumeclaims\",\"persistentvolumes\",\"endpointslices.discovery.k8s.io\",\"endpoints\",\"volumesnapshots.snapshot.storage.k8s.io\",\"volumegroupsnapshots.groupsnapshot.storage.k8s.io\",\"serviceaccounts\"]"
  }
]'
```

### Scenario 3: Minimal Exclusions (Advanced)

⚠️ **Only exclude critical resources:**

```bash
kubectl patch configmap default-excluded-resources -n ramen-system --type json -p '[
  {
    "op": "replace",
    "path": "/data/resources",
    "value": "[\"volumereplications.replication.storage.openshift.io\",\"volumegroupreplications.replication.storage.openshift.io\",\"replicationsources.volsync.backube\",\"replicationdestinations.volsync.backube\"]"
  }
]'
```

### Scenario 4: Using Comma-Separated Format

```bash
kubectl patch configmap default-excluded-resources -n ramen-system --type merge -p '{
  "data": {
    "resources": "volumereplications.replication.storage.openshift.io,volumegroupreplications.replication.storage.openshift.io,replicationsources.volsync.backube,replicationdestinations.volsync.backube,persistentvolumeclaims,persistentvolumes,endpointslices.discovery.k8s.io,endpoints,volumesnapshots.snapshot.storage.k8s.io,volumegroupsnapshots.groupsnapshot.storage.k8s.io,secrets"
  }
}'
```

## Recipe-Level Exclusions

You can still add exclusions at the Recipe group level:

```yaml
apiVersion: ramendr.openshift.io/v1alpha1
kind: Recipe
metadata:
  name: wordpress-recipe
  namespace: wordpress-namespace
spec:
  groups:
  - name: wordpress-config
    includedNamespaces:
    - wordpress-namespace
    includedResourceTypes:
    - deployments
    - services
    - configmaps
    excludedResourceTypes:
    - secrets              # Exclude secrets from this group
    - ingresses            # Exclude ingresses from this group
```

**Result:** Final exclusions = ConfigMap defaults + Recipe group exclusions

## Verification

### Check Effective Exclusions in Velero Backup

```bash
# List recent backups
kubectl get backup -n velero --sort-by=.metadata.creationTimestamp

# Check exclusions in a specific backup
kubectl get backup <backup-name> -n velero -o jsonpath='{.spec.excludedResources}' | jq .
```

### Watch ConfigMap Changes

```bash
kubectl get cm default-excluded-resources -n ramen-system -w
```

### Check Operator Logs

```bash
kubectl logs -n ramen-system deployment/ramen-dr-cluster-operator -f | grep -i "excluded\|configmap"
```

## Critical Resources

**DO NOT REMOVE** these from the ConfigMap:

- ✅ `volumereplications.replication.storage.openshift.io`
- ✅ `volumegroupreplications.replication.storage.openshift.io`
- ✅ `replicationsources.volsync.backube`
- ✅ `replicationdestinations.volsync.backube`

Removing these will cause VRG reconciliation issues!

## ConfigMap Schema

### JSON Format (Recommended)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: default-excluded-resources
  namespace: ramen-system
data:
  resources: |
    [
      "resource1",
      "resource2.api.group",
      "resource3"
    ]
```

### Comma-Separated Format

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: default-excluded-resources
  namespace: ramen-system
data:
  resources: |
    resource1,
    resource2.api.group,
    resource3
```

## Kubectl One-Liners

```bash
# Add a resource
kubectl get cm default-excluded-resources -n ramen-system -o json \
  | jq '.data.resources = (.data.resources | fromjson + ["secrets"] | tojson)' \
  | kubectl apply -f -

# Remove a resource (advanced)
kubectl get cm default-excluded-resources -n ramen-system -o json \
  | jq '.data.resources = (.data.resources | fromjson | del(.[] | select(. == "endpoints")) | tojson)' \
  | kubectl apply -f -

# Count exclusions
kubectl get cm default-excluded-resources -n ramen-system -o jsonpath='{.data.resources}' | jq 'length'
```

## Automation Example

### Script to Update Exclusions

```bash
#!/bin/bash
# add-exclusion.sh - Add a resource to default exclusions

NAMESPACE="ramen-system"
CM_NAME="default-excluded-resources"
RESOURCE_TO_ADD="$1"

if [ -z "$RESOURCE_TO_ADD" ]; then
  echo "Usage: $0 <resource-to-add>"
  exit 1
fi

# Get current list
CURRENT=$(kubectl get cm $CM_NAME -n $NAMESPACE -o jsonpath='{.data.resources}')

# Add new resource
NEW_LIST=$(echo "$CURRENT" | jq --arg res "$RESOURCE_TO_ADD" '. + [$res] | unique')

# Update ConfigMap
kubectl patch cm $CM_NAME -n $NAMESPACE --type merge -p "{\"data\":{\"resources\":\"$NEW_LIST\"}}"

echo "Added $RESOURCE_TO_ADD to exclusions"
```

Usage:
```bash
chmod +x add-exclusion.sh
./add-exclusion.sh secrets
```

## Best Practices

1. **Always backup before modifying:**
   ```bash
   kubectl get cm default-excluded-resources -n ramen-system -o yaml > excluded-resources-backup.yaml
   ```

2. **Test in dev/staging first**

3. **Document your changes:**
   ```bash
   kubectl annotate cm default-excluded-resources -n ramen-system \
     change-reason="Added secrets exclusion for security requirements"
   ```

4. **Monitor after changes:**
   ```bash
   kubectl logs -n ramen-system deployment/ramen-dr-cluster-operator -f
   ```

5. **Use JSON format** for easier programmatic updates

## Troubleshooting

### Changes not taking effect

ConfigMap changes are **not retroactive**. They apply to **new backups** only.

**Solution:** Trigger a new backup or wait for the next scheduled backup.

### ConfigMap keeps getting recreated

The operator auto-recreates the ConfigMap if deleted. This is by design.

**Solution:** Modify the ConfigMap instead of deleting it.

### Invalid JSON format error

**Symptom:** Operator logs show JSON parse errors

**Solution:** Validate JSON syntax:
```bash
kubectl get cm default-excluded-resources -n ramen-system -o jsonpath='{.data.resources}' | jq .
```

### Critical resource warnings in logs

**Symptom:** Logs show warnings about missing critical resources

**Action:** Add the critical resources back to the ConfigMap immediately.

## Additional Resources

- [IMPLEMENTATION_SUMMARY.md](./IMPLEMENTATION_SUMMARY.md) - Full technical details
- [Velero Backup API Reference](https://velero.io/docs/main/api-types/backup/)
- [Ramen Documentation](./docs/)

## Support

For issues or questions:
- GitHub Issues: https://github.com/RamenDR/ramen/issues
- Slack: #ramen-dr channel
