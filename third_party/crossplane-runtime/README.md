# Vendored crossplane-runtime (patched)

This is `github.com/crossplane/crossplane-runtime/v2@v2.3.3` with the fix from
[crossplane/crossplane-runtime#928](https://github.com/crossplane/crossplane-runtime/pull/928)
("fix ssa patch by applying all managed fields at once") applied to
`pkg/reconciler/managed/api.go`.

## Why

`APISimpleReferenceResolver` (the built-in reference resolver every managed
resource with a `ResolveReferences` method goes through) applies its
server-side-apply patch using only the fields that changed relative to the
last-seen snapshot. When a resource has more than one reference field, the
API server interprets a field absent from a given apply as that field
manager intentionally relinquishing/deleting it - so each reconcile can end
up deleting whatever reference field it didn't just touch, and the next
reconcile "restores" it while deleting the other one, forever. This exactly
matches the incident that led to scoping `Group`'s `roleRefs` rollout down
in this repo's `v1.1.1`/`v1.1.2`: `Group` has two reference fields
(`managingOrganizationRef` and `roleRefs`), which is enough to trigger it.

PR #928 fixes this by extracting the reference resolver's own previously-
managed fields from the object's `metadata.managedFields` and merging them
into every new patch, so a patch always includes everything the resolver
owns, not just what changed this time.

## Removing this

Once crossplane-runtime releases a version containing this fix (or an
equivalent), delete this directory, remove the `replace` directive for
`github.com/crossplane/crossplane-runtime/v2` in `go.mod`, and bump the
dependency in `require` to the fixed release.

This is trimmed to just what's needed to build (`apis/`, `pkg/`, `go.mod`,
`go.sum`, `generate.go`, `LICENSE`) - CI/nix/tooling files from the upstream
repo were removed.
