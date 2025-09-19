package versionskew

import (
	"fmt"

	"github.com/blang/semver/v4"
)

// IsSkewAllowed checks if the worker version can be updated based on the control plane version.
// The workers can't use a newer k8s version than the one used by the control plane.
//
// This implements Kubernetes version skew policy https://kubernetes.io/releases/version-skew-policy/
//
// Returns: (allowed bool, error)
func IsSkewAllowed(controlPlaneVersion, workerVersion string) (bool, error) {
	// Parse versions using semantic versioning for proper comparison
	controlPlaneCurrentK8sVersion, err := semver.ParseTolerant(controlPlaneVersion)
	if err != nil {
		return false, fmt.Errorf("failed to parse control plane k8s version %q: %w", controlPlaneVersion, err)
	}

	workerDesiredK8sVersion, err := semver.ParseTolerant(workerVersion)
	if err != nil {
		return false, fmt.Errorf("failed to parse worker desired k8s version %q: %w", workerVersion, err)
	}

	// Allow if control plane version >= desired worker version
	return controlPlaneCurrentK8sVersion.GE(workerDesiredK8sVersion), nil
}
