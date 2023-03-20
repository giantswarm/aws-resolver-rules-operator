# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.4.1...HEAD
[0.4.1]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.0.1...v0.1.0
[0.0.1]: https://github.com/giantswarm/aws-resolver-rules-operator/releases/tag/v0.0.1
