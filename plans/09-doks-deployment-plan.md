# DOKS Deployment Plan

## Status On `main`

This document started as a no-action deployment plan. Parts of it are now
implemented on `main`.

Current canonical state:

- GitHub Actions builds on push to `main`
- the release workflow pushes a SHA-tagged image and deploys by digest
- deployment config lives in GitHub environment vars and secrets, not in the repo
- the live kernel topology is `as-apid`, `as-registryd`, `as-materializerd`,
  and `as-webd`
- `execd` runs alongside `webd` in the `as-webd` pod
- public ingress is Cloudflare-fronted and origin-locked

This document therefore serves two purposes:

- record the intended deployment boundary and safety rules
- record the still-transitional parts of the current implementation

For the operational release path, also see:

- `.github/workflows/as-prod-release.yml`
- `kernel/runbook.md`

## Purpose

This document defines the first production-style deployment plan for
Autosoftware on DigitalOcean.

The goals are:

- use a dedicated DigitalOcean account workspace
- keep execution isolated from any existing systems in that account
- allow public access where appropriate
- keep runtime data durable even if the cluster is replaced
- avoid accidental interference with existing clusters, databases, load
  balancers, or VPC resources

This is still primarily a planning document, but some sections now describe the
implemented baseline on `main` rather than purely hypothetical future state.

---

## Constraints

### 1. Separate execution plane

The AS deployment should run in its own network and cluster.

- new VPC
- new DOKS cluster
- new managed PostgreSQL instance
- new load balancers
- no reuse of any existing cluster
- no reuse of any existing managed database

The only shared infrastructure that is acceptable for the first deployment is
the existing DigitalOcean container registry.

### 2. Durable data

The cluster is disposable.

The runtime data is not.

Durability should come from:

- managed PostgreSQL in the new VPC
- source-controlled repo contents for seed and realization definitions

Do not treat pod-local or node-local storage as durable state.

### 3. Current codebase reality

The current codebase is not yet a full cluster-native runtime manager.

- `webd`, `apid`, `registryd`, and `materializerd` are normal Go HTTP
  processes.
- the services expect access to the repo tree via `AS_REPO_ROOT`
- realization execution in shared deployment is currently source-backed and
  transitional
- `apid` currently mixes public and internal route families

This means the current deployment should focus on:

- the kernel services
- public registry and web surfaces
- durable runtime storage

It should not be described as a final packaged realization runtime model yet.

---

## Target Topology

### DigitalOcean resources

Create these new resources only:

- project: `<deployment-project>`
- VPC: `<deployment-vpc>`
- DOKS cluster: `<deployment-cluster>`
- managed PostgreSQL 17 cluster: `<deployment-postgres>`

Do not attach these resources to any pre-existing VPC.

### Kubernetes namespace

Use one dedicated namespace for the first cut:

- `as-system`

### Services

Deploy four separate services:

- `as-webd`
- `as-registryd`
- `as-apid`
- `as-materializerd`

Each should be its own Deployment and ClusterIP Service.

Even if all four run from the same image, keep them as separate Kubernetes
workloads so failure domains stay clean.

The current implementation also runs:

- `execd` as a second container in the `as-webd` pod

This is intentional for the present source-backed execution model, because
realization upstreams are localhost-backed.

---

## Public vs Internal Exposure

### Public

These should be internet-accessible:

- `webd`
- `registryd`
- `apid`

### Internal only

This should stay private:

- `materializerd`

### Important note about `apid`

`apid` is conceptually public, but the current binary still includes internal
runtime and growth routes.

For the first deployment:

- give `apid` a public hostname
- expose only the currently safe route subset
- restrict internal route families at ingress until the public/internal API
  split plan is implemented

See:

- [08-public-and-internal-api-split-plan.md](/Users/splash/AS/plans/08-public-and-internal-api-split-plan.md)

---

## Recommended Hostnames

Assuming AS lives under a dedicated public domain or subdomain:

- `<as-web-domain>` -> `webd`
- `<as-registry-domain>` -> `registryd`
- `<as-api-domain>` -> `apid`

If a different domain or subdomain is preferred, keep the same structure:

- one hostname for the human web surface
- one hostname for registry authority
- one hostname for machine-facing API

Set:

- `AS_BASE_DOMAIN=<as-base-domain>`

This keeps future subdomain routing coherent under `webd`, even if realization
routing stays mostly dormant for the first deployment.

---

## Kubernetes Sizing

### Cluster

Recommended first cluster:

- region: `tor1`
- Kubernetes version: current supported stable version at execution time
- HA control plane: enabled
- worker pool: 2 nodes
- node size: `s-2vcpu-4gb`

Why:

- enough headroom for ingress plus four small Go services
- tolerates one node disruption better than a single-node cluster
- stays within the current account droplet limit margin

### Database

Recommended first database:

- engine: PostgreSQL 17
- region: `tor1`
- size: `db-s-1vcpu-1gb`

This is enough for an initial deployment of the current runtime tables.
Increase size later only when actual load or storage warrants it.

---

## Image Strategy

### Container registry

Use the existing DigitalOcean registry for now.

On `main`, the canonical release behavior is:

- build on push to `main`
- tag image as `sha-<merge-commit>`
- deploy by immutable digest
- avoid mutable deployment tags in the release path

To minimize repository sprawl:

- use a single new repository for AS kernel images
- example: `registry.digitalocean.com/<registry-name>/as-kernel`

One repository is preferable to one repository per service because:

- it minimizes new registry surface area
- it reduces repository-count pressure
- all four services can share the same build artifact

### Image contents

Build one image containing:

- the repo checkout
- all four compiled binaries
- the runtime SQL files
- seeds, genesis, and other repo-backed materializer inputs

Suggested in-container repo root:

- `/app`

Set:

- `AS_REPO_ROOT=/app`

### Why one image

The services do not run as pure stateless binaries yet.

They read repo content at runtime, so the image needs the repository tree, not
just the compiled executables.

For shared realization execution today, the image also needs the Go toolchain,
because realizations are still launched from source.

---

## Service Configuration

### Common environment

All services should set:

- `AS_REPO_ROOT=/app`

Only `webd` should set:

- `AS_BASE_DOMAIN=<as-base-domain>`
- `AS_GITHUB_URL=<public-repo-url>`

Leave unset for the first deployment unless intentionally federating:

- `AS_REMOTE_REGISTRY_URL`

### Listener addresses

The binaries default to loopback addresses, which is correct locally but wrong
inside a container.

Override them explicitly:

- `AS_WEBD_ADDR=:8090`
- `AS_MATERIALIZER_ADDR=:8091`
- `AS_APID_ADDR=:8092`
- `AS_REGISTRYD_ADDR=:8093`

### Database connection

These services should use the managed PostgreSQL private connection string:

- `webd`
- `apid`
- `registryd`

`materializerd` does not need the runtime database for the first deployment.

### Migration policy

Use:

- `AS_RUNTIME_AUTO_MIGRATE=true` only on `apid`

Use:

- `AS_RUNTIME_AUTO_MIGRATE=false` on `webd`
- `AS_RUNTIME_AUTO_MIGRATE=false` on `registryd`

This keeps the first deployment simple while avoiding simultaneous schema
migration attempts across multiple pods.

Longer-term, a dedicated migration Job would be cleaner.

---

## Health Probes

Use these probe paths:

- `webd`: `GET /`
- `registryd`: `GET /healthz`
- `materializerd`: `GET /healthz`
- `apid`: `GET /v1/runtime/health`

Because `apid` requires the runtime database to start, its health endpoint is a
useful readiness gate.

---

## Ingress Policy

### `webd`

Public ingress:

- hostname: `<as-web-domain>`
- service: `as-webd`
- path: `/`

### `registryd`

Public ingress:

- hostname: `<as-registry-domain>`
- service: `as-registryd`
- path: `/`

### `apid`

Public ingress:

- hostname: `<as-api-domain>`
- service: `as-apid`

But restrict the exposed route set for the first deployment.

#### Public route subset for `apid`

Allow initially:

- `GET /v1/contracts`
- `GET /v1/contracts/{seed_id}/{realization_id}`

Do not expose publicly for now:

- `/v1/runtime/*`
- `/v1/commands/realizations.grow`
- `/v1/projections/realization-growth/*`

This reflects current implementation risk rather than long-term product
intention.

### `materializerd`

No public ingress.

ClusterIP only.

---

## Network Policy

Use Kubernetes NetworkPolicies from day one.

### Default posture

- deny cross-namespace access by default
- allow ingress only from the ingress controller to public services
- allow internal service-to-service traffic only where needed

### Practical allowances

- ingress controller -> `as-webd`
- ingress controller -> `as-registryd`
- ingress controller -> `as-apid`
- `webd` -> PostgreSQL
- `apid` -> PostgreSQL
- `registryd` -> PostgreSQL
- `materializerd` -> none required beyond normal egress unless federation is
  introduced later

Because the services are simple, keep the policy explicit and small.

---

## Secrets and Config

### Secret

Create one Kubernetes Secret for sensitive runtime values:

- `as-runtime-secret`

Contents:

- `AS_RUNTIME_DATABASE_URL`

### ConfigMap

Create one ConfigMap for non-secret environment:

- `as-runtime-config`

Contents:

- `AS_REPO_ROOT`
- `AS_BASE_DOMAIN`
- `AS_GITHUB_URL`
- `AS_WEBD_ADDR`
- `AS_APID_ADDR`
- `AS_REGISTRYD_ADDR`
- `AS_MATERIALIZER_ADDR`
- `AS_RUNTIME_AUTO_MIGRATE` values per workload, or inject directly per
  Deployment if that is simpler

---

## Rollout Order

Use this order for fresh environment creation or disaster recovery:

1. Create the new DigitalOcean project.
2. Create the new VPC.
3. Create the managed PostgreSQL cluster in that VPC.
4. Create the DOKS cluster in that VPC.
5. Build and push the deployment image.
6. Create namespace, Secret, and ConfigMap.
7. Deploy `as-apid` first so migrations can run.
8. Wait for `as-apid` readiness.
9. Deploy `as-registryd`.
10. Deploy `as-webd`.
11. Deploy `as-materializerd`.
12. Create public ingress for `webd`, `registryd`, and the restricted public
    subset of `apid`.
13. Validate externally and internally.

Do not create ingress before the workloads are healthy.

---

## Validation Checklist

Later, after provisioning, validate at minimum:

### Database

- `apid` reports healthy runtime DB connection
- migrations table exists
- expected runtime tables exist

### Public endpoints

- `https://<as-web-domain>/` loads
- `https://<as-registry-domain>/healthz` returns healthy
- `https://<as-registry-domain>/v1/registry/status` returns healthy
- `https://<as-api-domain>/v1/contracts` returns contract discovery data

### Restricted endpoints

Confirm that public access is blocked to:

- `/v1/runtime/*`
- `/v1/commands/realizations.grow`
- `/v1/projections/realization-growth/*`

### Service health

- all Deployments available
- all pods ready
- no crash loops
- no unexpected migration churn

---

## Safe `doctl` Execution Rules

When actual provisioning begins, follow these rules strictly.

### Always use explicit names and IDs

Never rely on defaults when creating resources.

Always pass explicit:

- project
- region
- VPC
- cluster name
- database name
- node size
- node count

### Never mutate existing production resources

Do not run mutating commands against any pre-existing cluster, database, VPC,
load balancer, or registry configuration in the account.

### Keep kubeconfig isolated

Use a dedicated kubeconfig file for the deployment cluster.

Do not merge it into an already-active default context if that can be avoided.

### Avoid leaking database credentials in terminals

Do not use broad `doctl databases get -o json` output unless piped to a
filter immediately.

Managed database JSON output contains connection credentials inline.

---

## Suggested Command Sequence

These are for later execution, not now.

### 1. Create the VPC

```bash
doctl vpcs create <deployment-vpc> --region tor1 --ip-range <private-cidr>
```

### 2. Create PostgreSQL

```bash
doctl databases create <deployment-postgres> \
  --engine pg \
  --version 17 \
  --region tor1 \
  --size db-s-1vcpu-1gb \
  --num-nodes 1 \
  --private-network-uuid <DEPLOYMENT_VPC_UUID> \
  --tag-name <deployment-tag>
```

### 3. Create DOKS

```bash
doctl kubernetes cluster create <deployment-cluster> \
  --region tor1 \
  --version <CURRENT_STABLE_VERSION> \
  --ha \
  --vpc-uuid <DEPLOYMENT_VPC_UUID> \
  --tag <deployment-tag> \
  --size s-2vcpu-4gb \
  --count 2
```

### 4. Save kubeconfig separately

```bash
KUBECONFIG=/path/to/deployment.kubeconfig \
doctl kubernetes cluster kubeconfig save <deployment-cluster>
```

The exact commands should be reviewed again at execution time before use.

---

## Known Limitations

### 1. `apid` is still mixed-surface

Ingress has to express a boundary the code does not fully encode yet.

### 2. Realization execution is not production-grade yet

The kernel can serve, inspect, and materialize repo-backed state, but it does
not yet provide a clean orchestrated model for running arbitrary realization
processes inside the cluster.

### 3. Repo-backed runtime means larger images

This is acceptable for now, but later the materializer and registry authority
should be able to run from more compact signed artifacts or snapshots.

---

## Future Follow-Up

After the first deployment is stable, the next infrastructure work should be:

1. Split `apid` into public and internal binaries.
2. Introduce a proper migration Job.
3. Add CI image builds and provenance.
4. Add scheduled logical PostgreSQL backups to a separate durable target.
5. Move image distribution to GitHub Container Registry if desired.
6. Revisit realization process orchestration as a first-class runtime concern.

---

## Summary

The first AS production deployment on DigitalOcean should be:

- new dedicated VPC
- new dedicated DOKS cluster
- new dedicated PostgreSQL cluster
- shared existing DOCR only for image storage
- public `webd`
- public `registryd`
- public but route-restricted `apid`
- internal `materializerd`

This keeps execution isolated, keeps runtime data durable, and avoids touching
any existing production infrastructure in the account.
