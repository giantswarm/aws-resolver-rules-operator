#!/bin/bash

set -euo pipefail

KIND="${KIND:?kind binary path not exported}"
CLUSTER="${CLUSTER:?cluster not exported}"
IMG="${IMG:?image not exported}"
readonly KIND
readonly CLUSTER
readonly IMG

ensure_kind_cluster() {
  local cluster
  cluster="$1"
  if ! "$KIND" get clusters | grep -q "$cluster"; then
    "$KIND" create cluster --name "$cluster" --wait 5m
  fi
  "$KIND" export kubeconfig --name "$cluster" --kubeconfig "$HOME/.kube/$cluster.yml"
}

ensure_kind_cluster "$CLUSTER"
kubectl create namespace giantswarm --kubeconfig "$HOME/.kube/$CLUSTER.yml" || true
kubectl create namespace "$MANAGEMENT_CLUSTER_NAMESPACE" --kubeconfig "$HOME/.kube/$CLUSTER.yml" || true
"$KIND" load docker-image --name "$CLUSTER" "$IMG"
