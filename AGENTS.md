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
