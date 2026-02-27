#!/usr/bin/env bash
# Apply PodRemediator CRD + RBAC on the cluster (e.g. had-18 / controller-0).
# Run from repo root on a host that has 'oc' and cluster access:
#   ./hack/apply-podremediator-setup-had18.sh
# Or from had-18, after copying this repo or config/podremediator_cluster_setup.yaml:
#   oc apply -f config/podremediator_cluster_setup.yaml
set -e

REPO_ROOT="${REPO_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
SETUP_FILE="${REPO_ROOT}/config/podremediator_cluster_setup.yaml"
OC="${OC:-$(command -v oc 2>/dev/null)}"
[[ -z "$OC" && -x "$REPO_ROOT/.tmp/oc/oc" ]] && OC="$REPO_ROOT/.tmp/oc/oc"

if [[ -z "$OC" || ! -x "$OC" ]]; then
  echo "oc not found. Install OpenShift CLI or run: curl -sL https://mirror.openshift.com/pub/openshift-v4/clients/ocp/stable/openshift-client-linux.tar.gz | tar xz -C .tmp/oc"
  echo "Then set KUBECONFIG to your cluster and run: oc apply -f ${SETUP_FILE}"
  exit 1
fi

if [[ ! -f "$SETUP_FILE" ]]; then
  echo "Setup file not found: $SETUP_FILE"
  exit 1
fi

echo "Applying PodRemediator CRD + RBAC from $SETUP_FILE ..."
"$OC" apply -f "$SETUP_FILE"
echo "Done. Verify: $OC get crd podremediators.remediation.openstack.org && $OC -n openstack-operators get pods -l app.kubernetes.io/name=infra-operator"
