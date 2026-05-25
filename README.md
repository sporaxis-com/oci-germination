# OCI Germination

`oci-germination` is the seed repository for building very small OCI layouts around PostgreSQL extensions.

The working goal is simple: render several related containers into the smallest practical, operationally clear shape, starting with an extremely minimal PostgreSQL runtime that carries `pgRDF`, and then layering `pgCK` on top where composition makes sense.

## Current intention

The first germination target is:

- a minimal PostgreSQL image
- as close to distroless as PostgreSQL and extension loading will realistically allow
- capable of running `pgrdf`
- designed so `pgck` can be composed in later without rethinking the entire base

This repository starts as documentation-first. We are defining the constraints, the upstream components, and the direction before adding build logic.

## Why this exists

Upstream projects already own the extension logic:

- `pgRDF`: RDF, SPARQL, SHACL, and OWL reasoning inside PostgreSQL
- `pgCK`: a PostgreSQL extension that composes `pgRDF` and adds concept-kernel runtime responsibilities

This repository is not trying to replace either upstream project. It exists to explore the OCI side:

- image minimization
- extension packaging
- runtime dependency trimming
- container composition
- repeatable, auditable build surfaces

## Upstream references

- `pgRDF`: <https://github.com/styk-tv/pgRDF>
- `pgRDF` install notes: <https://pgrdf.styk.tv/v0.5/operations/install>
- `pgCK`: <https://github.com/styk-tv/pgCK>

## Near-term scope

The initial repo scope is intentionally narrow:

1. document the target runtime shape
2. define a minimal base for PostgreSQL + `pgrdf`
3. identify what prevents a truly distroless runtime
4. prepare for a follow-on layer or sibling image that includes `pgck`

## Initial assumptions

- PostgreSQL remains the runtime anchor
- extension artifacts and their shared-library dependencies will likely determine how close to distroless we can get
- the first useful outcome is probably a minimal runtime image, not a fully pure distroless image
- `pgck` should be treated as composition over `pgrdf`, not as a separate stack

## Planned repository shape

As the repo grows, expect small focused additions such as:

- `Dockerfile` or `Containerfile` variants
- OCI build notes
- extension packaging experiments
- size and dependency reports
- example runtime manifests

## Status

Bootstrap only. No container build, packaging logic, or PostgreSQL runtime has been committed yet.
