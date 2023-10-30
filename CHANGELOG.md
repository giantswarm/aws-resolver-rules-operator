# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Set user-agent to `giantswarm-capa-operator` when making requests to the k8s API.
- Most controllers will skip reconciliation events that only changed the `resourceVersion` field of the CR.

### Added

- Add `global.podSecurityStandards.enforced` value for PSS migration.
- Add managing prefix lists for management clusters and rename `ManagementClusterTransitGatewayReconciler` to `ManagementClusterNetworkReconciler`.

## [0.11.0] - 2023-10-04

### Changed

- Reconcile `AWSClusters` and `AWSManagedControlPlane` instead of `Cluster` CR when reconciling dns records.
- Only apply finalizer if it was not there.

## [0.10.1] - 2023-10-03

### Changed

- Fix Deletion of delegations DNS records from parent zone in case its in different AWS Account.

## [0.10.0] - 2023-09-21

### Added

- Added ManagementClusterTransitGateway reconciler. This reconciler is still disabled and is part of the merger of the aws-network-topology-operator.

### Changed

- Update `golang.org/x/net` package.
- Use a management cluster IAM Role to create a delegation records for the DNS.

## [0.9.0] - 2023-08-01

### Added

- Add support for EKS to `dnsController`.

### Changed

- Migrate `dnsController` to reconcile `Cluster` CR as preparation for reconciliation of EKS clusters.
- Custom errors no longer inherit from builtin `error` to make linter happy.

### Fixed

- Delete all dns records from hosted zone when deleting a cluster.

## [0.8.0] - 2023-07-06

### Changed

- Make the dns controller requeue if there is a bastion machine but has no IP yet, because we want to create a dns record for the bastion as soon as possible.

## [0.7.1] - 2023-07-05

### Changed

- Fully migrate to `abs`.

## [0.7.0] - 2023-07-05

### Added

- Add necessary values for PSS policy warnings.
- Add new reconciler that creates hosted zones dns records for reconciled clusters.
- Add `readOnlyRootFilesystem` to container `securityContext` in Helm chart.

### Changed

- Don't fail at start up if DNS server settings for resolver rules reconciler are missing.
- Use `abs` to generate chart version metadata.
- Fix `securityContext` indentation in `Deployment` template.

## [0.6.0] - 2023-05-05

### Fixed

- Use resource share status as filter when deleting RAM resource shares.
- Change reconcilation logic to process existing RAM resource shares.

## [0.5.0] - 2023-04-19

### Changed

- Change default registry in Helm chart from quay.io to docker.io.

## [0.4.1] - 2023-03-20

### Added

- Add the use of runtime/default seccomp profile. Allow required volume types in PSP.

## [0.4.0] - 2023-03-15

### Added

- Add condition to `AWSCluster` signaling that resolver rules in AWS account have been associated with workload cluster VPC.
- Add new reconciler that unpauses the `AWSCluster` CR when VPC, Subnets and Resolver Rules conditions are marked as ready.

## [0.3.0] - 2023-02-22

### Changed

- Update `golang.org/x/net` to v0.7.0.

## [0.2.1] - 2023-02-08

### Fix

- Revert seccomp profile.

## [0.2.0] - 2023-02-08

### Added

- Associate existing Resolver Rules with workload cluster VPC.
- Add the use of runtime/default seccomp profile.
- Improve logging.

### Fixed

- Don't create RAM resource share if it's already created.
- When creating the Resolver endpoints, only create it for the subnets with a specific tag on them.

## [0.1.1] - 2023-01-24

### Fixed

- Don't try to find resolver rules associations filtering by association name, but only use resolver rule id and vpc id.

## [0.1.0] - 2023-01-24

### Added

- Initial implementation.

## [0.0.1] - 2022-12-20

- changed: `app.giantswarm.io` label group was changed to `application.giantswarm.io`

[Unreleased]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.11.0...HEAD
[0.11.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.10.1...v0.11.0
[0.10.1]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.10.0...v0.10.1
[0.10.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.7.1...v0.8.0
[0.7.1]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.4.1...v0.5.0
[0.4.1]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.0.1...v0.1.0
[0.0.1]: https://github.com/giantswarm/aws-resolver-rules-operator/releases/tag/v0.0.1
