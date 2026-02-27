# Cluster setup for PodRemediator (CRD + RBAC)

When deploying a custom infra-operator image that includes the PodRemediator controller, the following must be applied on the cluster:

1. **CRD** for `PodRemediator` (remediation.openstack.org/v1beta1)
2. **RBAC** that allows the infra-operator ServiceAccount to list (and use) PVCs at cluster scope and to manage `podremediators` resources.

On the cluster the ServiceAccount is: `system:serviceaccount:openstack-operators:infra-operator-controller-manager`.

## Quick apply (CRD + RBAC in one step)

From the repo root, on a host that has `oc` and cluster access (e.g. had-18 or controller-0):

```bash
oc apply -f config/podremediator_cluster_setup.yaml
```

Or run the helper script:

```bash
./hack/apply-podremediator-setup-had18.sh
```

The combined manifest is `config/podremediator_cluster_setup.yaml` (CRD + supplemental ClusterRole and ClusterRoleBinding).

## 1. Apply the CRD only

From a machine with cluster access (e.g. from had-18 as `zuul` on controller-0):

```bash
oc apply -f config/crd/bases/remediation.openstack.org_podremediators.yaml
```

## 2. Add missing permissions

If the operator was installed via OLM/OpenStack operator, the existing ClusterRole may not yet include `persistentvolumeclaims` (cluster scope) and `podremediators`. You can add them in two ways.

### Option A: Apply the full role from the repo

If the ClusterRole used by infra-operator is named `manager-role` and you want to replace it with the one from the repo (which already includes PVC and podremediators):

```bash
oc apply -f config/rbac/role.yaml
```

Ensure the ClusterRoleBinding exists and binds `manager-role` to the ServiceAccount `openstack-operators:infra-operator-controller-manager`. If not, create or update the binding.

### Option B: Supplemental role (recommended if you prefer not to change the existing role)

Apply the manifest that adds only the permissions required for PodRemediator and PVCs:

```bash
oc apply -f config/rbac/podremediator_extra_rbac.yaml
```

(File path: `config/rbac/podremediator_extra_rbac.yaml`.)

## 3. Verification

- Confirm the CRD is present:
  ```bash
  oc get crd podremediators.remediation.openstack.org
  ```
- Confirm the infra-operator pod is back to `Running`:
  ```bash
  oc -n openstack-operators get pods -l app.kubernetes.io/name=infra-operator
  ```
- Optionally create a test PodRemediator (see `config/samples/remediation_v1beta1_podremediator.yaml`).

## References

- CRD: `config/crd/bases/remediation.openstack.org_podremediators.yaml`
- Full RBAC: `config/rbac/role.yaml`
- Sample: `config/samples/remediation_v1beta1_podremediator.yaml`
