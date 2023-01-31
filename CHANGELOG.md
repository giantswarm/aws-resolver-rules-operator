# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Associate existing Resolver Rules with workload cluster VPC.
- Add the use of runtime/default seccomp profile.

### Fixed

- Don't create RAM resource share if it's already created.

## [0.1.1] - 2023-01-24

### Fixed

- Don't try to find resolver rules associations filtering by association name, but only use resolver rule id and vpc id.

## [0.1.0] - 2023-01-24

### Added

- Initial implementation.

## [0.0.1] - 2022-12-20

- changed: `app.giantswarm.io` label group was changed to `application.giantswarm.io`

[Unreleased]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/aws-resolver-rules-operator/compare/v0.0.1...v0.1.0
[0.0.1]: https://github.com/giantswarm/aws-resolver-rules-operator/releases/tag/v0.0.1
