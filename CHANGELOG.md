# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Migrate to AWS SDK for Go v2.
- Use OpenTelemetry for metrics.
- Disable `controller-runtime` cache for `ConfigMaps` to decrease memory usage.

## [0.22.0] - 2025-09-10

### Changed

- Change controllers sync period to 2 minutes.

## [0.21.0] - 2025-09-02

### Changed

- Create karpenter custom resources in workload clusters.

## [0.20.0] - 2025-06-23

### Changed

- Change `renovate` configuration to only get upgrades for CAPA compatible versions of k8s dependencies.
- Don't reconcile the `ShareReconciler` if only the status field has changed.

### Fixed

- Only remove `finalizer` after there are no more karpenter instances to terminate.

## [0.19.0] - 2025-06-17

### Added

- Add crossplane controller to create crossplane provider cluster config.

## [0.18.0] - 2025-06-16

### Added

- Add controller to create node pool bootstrap data on S3.

### Changed

- Dynamically calculate CAPI and CAPA versions from go cache, so that we use the right path when installing the CRDs during tests.
- Add tags to VPC created for acceptance tests.

### Fixed

- Resolve several linting errors.

## [0.17.0] - 2024-09-30

### Changed

- Enable ShareReconciler.

## [0.16.0] - 2024-08-21

### Changed

- Update CAPI/CAPA/controller-runtime Go modules

### Fixed

- Disable logger development mode to avoid panicking

## [0.15.0] - 2024-04-17

### Changed

- Update capa to v2.3.0.
- Use ResourceID instead of ID for CAPA Subnets.

## [0.14.6] - 2024-04-16

### Changed

- Add toleration for `node.cluster.x-k8s.io/uninitialized` taint.
- Add node affinity to prefer scheduling CAPI pods to control-plane nodes.

## [0.14.5] - 2024-02-07

### Fixed

- Fix finding NS record of the DNS zone. This only happens in rare cases.

## [0.14.4] - 2024-01-31

### Changed

- Avoid deletion reconciliation if finalizer is already gone

## [0.14.3] - 2024-01-30

### Changed

- Further avoidance for Route53 rate limiting

  - Cache `ListResourceRecordSets` responses for NS record of a zone
  - Only upsert DNS records if not recently done

## [0.14.2] - 2024-01-29

### Changed

- Cache hosted zone ID on creation
- List many hosted zones at once in one Route53 request and cache all returned zones. This further reduces the number of Route53 requests and therefore avoids rate limit (throttling) errors.

## [0.14.1] - 2024-01-26

### Fixed

- Cache `GetHostedZoneIdByName` responses to avoid Route53 rate limit. The hosted zone IDs are unlikely to change so quickly. If reconciliation of an object (e.g. `AWSCluster`) permanently gets retriggered outside of this operator's control, it could previously lead to triggering the [account-wide AWS Route53 rate limit of five requests per second](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/DNSLimitations.html#limits-api-requests).

## [0.14.0] - 2024-01-25

### Changed

- Do not reconcile bastion machines anymore. They were removed in [cluster-aws v0.53.0](https://github.com/giantswarm/cluster-aws/blob/master/CHANGELOG.md#0530---2023-12-13) in favor of Teleport. On such newer clusters without a bastion host, the controller would retry unnecessary Route53 calls every minute, leading to a rate limit – this is fixed by removing the whole logic.

## [0.13.0] - 2023-12-21

### Added

- Added `RouteReconciler`.
- Add `TransitGatewayAttachmentReconciler`.
- Add `PrefixListEntryReconciler`.
- Add `ShareReconciler`.
- Add acceptance tests.

### Changed

- Configure `gsoci.azurecr.io` as the default container image registry.

## [0.12.0] - 2023-11-02

### Changed

- Set user-agent to `giantswarm-capa-operator` when making requests to the k8s API.
- Most controllers will skip reconciliation events that only changed the `resourceVersion` field of the CR.

### Added

- Add tags from `AWSCluster.Spec.AdditionalTags` and `AWSManagedControlPlane.Spec.AdditionalTags` to  all created resources.
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

[Unreleased]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.22.0...HEAD
[0.22.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.21.0...v0.22.0
[0.21.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.20.0...v0.21.0
[0.20.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.19.0...v0.20.0
[0.19.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.18.0...v0.19.0
[0.18.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.17.0...v0.18.0
[0.17.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.16.0...v0.17.0
[0.16.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.15.0...v0.16.0
[0.15.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.14.6...v0.15.0
[0.14.6]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.14.5...v0.14.6
[0.14.5]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.14.4...v0.14.5
[0.14.4]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.14.3...v0.14.4
[0.14.3]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.14.2...v0.14.3
[0.14.2]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.14.1...v0.14.2
[0.14.1]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.14.0...v0.14.1
[0.14.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.11.0...v0.12.0
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
