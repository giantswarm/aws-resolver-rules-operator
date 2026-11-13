<!-- BEGIN devctl-managed: edits in this block are overwritten -->
<!-- markdownlint-disable-file MD041 -->

## Giant Swarm conventions

devctl maintains this block from [its template][devctl-agents-block] and
overwrites changes to it. Instructions for this repository go in the section
below. Where they differ from this block, they take precedence.

[devctl-agents-block]: https://github.com/giantswarm/devctl/blob/main/pkg/gen/input/llm/internal/file/agents_block.md.template

- Never put customer names or other customer information in code, commit
  messages, issues or pull requests. Most Giant Swarm repositories are public,
  and private ones are kept ready to become public.
- Giant Swarm's style guides are in <https://github.com/giantswarm/fmt>, one
  directory per topic (go, helm, kubernetes, prometheus, …). Read the relevant
  guide before you write or review that kind of code.
- Use conventional commits for commit messages and pull request titles.
- If `.github/workflows/zz_generated.auto_release.yaml` exists, GitHub Releases
  are written from pull request titles: do not edit CHANGELOG.md. Otherwise,
  add your change to CHANGELOG.md as described in
  <https://github.com/giantswarm/fmt/blob/main/releases/changelog.md>.
- If `.pre-commit-config.yaml` exists, run
  `pre-commit run --files <changed files>` before committing; not every clone
  has the hooks installed.
- Do not add linters, formatters or other tooling unless asked; use what the
  repository already has.
- Put documentation in `docs/` rather than growing README.md, which gives an
  overview and links into `docs/`. When a change affects documented behaviour,
  update the documentation in the same pull request.
- If the skills of the `giantswarm/claude-code` plugin marketplace are not
  available to you and the user works at Giant Swarm, tell them once that
  they can install the `gs-base`, `gs-dev`, `gs-godev` and `gs-appdev`
  plugins from it.
- Go: run `goimports -local github.com/giantswarm/<repository>` on the files
  you change. CI with architect-orb fails when imports are grouped
  differently.

<!-- END devctl-managed -->

# aws-resolver-rules-operator

Kubernetes operator for managing different resources that are required by CAPA clusters to operate.
It contains many different controllers that each manage a different resources.

## Directory Structure
```
controllers/    # Reconcilers
pkg/aws/        # AWS clients (EC2, Route53, RAM, etc.)
pkg/resolver/   # Core interfaces & types
helm/           # Helm chart
```

## Critical Rules

### Never Edit These Auto-Generated files

- `config/crd/bases/*.yaml` - from `make manifests`
- `**/zz_generated.*.go` - from `make generate`
- `PROJECT` - from `kubebuilder [OPTIONS]`

### Testing

After making changes, always make sure unit tests and integration tests pass. Tests use **Ginkgo + Gomega** (BDD style).

Always run the tests using the Makefile:
```
make test-unit        # Run unit tests
make test-integration # Run integration tests
```

Fakes in `*fakes/` dirs (Counterfeiter). Integration uses LocalStack.

### Implementation Rules

- **Idempotent reconciliation**: Safe to run multiple times
- **Re-fetch before updates**: `r.Get(ctx, req.NamespacedName, obj)` before `r.Update` to avoid conflicts
- **Structured logging**: `log := log.FromContext(ctx); log.Info("msg", "key", val)`
- **Owner references**: Enable automatic garbage collection (`SetControllerReference`)
- **Finalizers**: Clean up external resources (buckets, VMs, DNS entries)
- **Tests**: When changing logic in go files, make sure to update existing tests accordingly, or add new ones that cover the new logic.
- **Changelog**: All code changes need to be documented in the changelog following the current structure and style.

## Deployment

There is a helm chart in the @helm folder.
If adding new parameters to the `main.go` file, please update the `deployment.yaml` manifest in the helm chart accordingly.

## References

### Essential Reading
- **Kubebuilder Book**: https://book.kubebuilder.io (comprehensive guide)
- **controller-runtime FAQ**: https://github.com/kubernetes-sigs/controller-runtime/blob/main/FAQ.md (common patterns and questions)
- **Good Practices**: https://book.kubebuilder.io/reference/good-practices.html (why reconciliation is idempotent, status conditions, etc.)

### API Design & Implementation
- **API Conventions**: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md
- **Operator Pattern**: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/
- **Markers Reference**: https://book.kubebuilder.io/reference/markers.html

### Tools & Libraries
- **controller-runtime**: https://github.com/kubernetes-sigs/controller-runtime
- **controller-tools**: https://github.com/kubernetes-sigs/controller-tools
- **Kubebuilder Repo**: https://github.com/kubernetes-sigs/kubebuilder
