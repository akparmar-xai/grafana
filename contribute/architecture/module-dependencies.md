# Module dependency map

Use this note to understand how the main Grafana backend and frontend modules depend on each other, and which five modules have the widest blast radius.

The interactive view of the same module set is a Cursor canvas. This page records the module list, the measurement method, and the criticality call so the result is reviewable in git.

## Before you begin

Before you begin, ensure you have the following:

- A checkout of this repository.
- Enough context to read Go import paths and TypeScript package imports.

## Modules in the map

The map is a representative cut of the main modules, not a graph of every package under `pkg/`.

**Backend directories:**

- `pkg/api`, `pkg/services`, `pkg/server`, `pkg/tsdb`, `pkg/plugins`, `pkg/infra`, `pkg/middleware`, and `pkg/setting`.
- `apps/dashboard`, `apps/folder`, and `apps/alerting` (the four alerting Go modules aggregated: `rules`, `notifications`, `alertenrichment`, `alertrulequality`).

**Extra backend hub:**

- `pkg/apimachinery` is included because `apps/dashboard`, `apps/folder`, and `apps/alerting` declare it in their `go.mod` files, and 148 Go packages under `pkg/services` import it. Without it, the apps look disconnected from the monolith.

**Frontend:**

- `public/app/core`, `public/app/features`, `public/app/plugins`, and `public/app/store`.
- Workspace packages `@grafana/data`, `@grafana/ui`, `@grafana/runtime`, and `@grafana/schema`.
- `@grafana/scenes` at version 8.19.0 from the root `package.json`. It is an external npm dependency, not a workspace package.

## How edges were measured

**Go imports.** Production `*.go` files under `pkg/`, `apps/dashboard`, `apps/folder`, and `apps/alerting` were scanned. Files named `*_test.go` are excluded. An edge `A → B` means a file in A imports a package in B. Fan-in is the number of distinct importer packages outside the module.

**Wire.** Startup composition is the OSS graph in `pkg/server/bootstrap/wire/wire_gen.go`. `pkg/cmd/grafana/main.go` calls `server.Initialize`.

**TypeScript imports.** Production `*.ts` and `*.tsx` files under `public/app/{core,features,plugins,store}` and `packages/grafana-{data,ui,runtime,schema}/src` were scanned. `*.test.*`, `*.spec.*`, and story files are excluded. Imports of `app/...` and `@grafana/...` are counted, including relative imports that resolve into another module. Fan-in is the number of distinct files outside the module.

**HTTP.** The frontend does not import the Go API. `getBackendSrv()` in `packages/grafana-runtime/src/services/backendSrv.ts` calls `/api`. `pkg/api/api.go` documents that base path and contains 264 `Get`, `Post`, `Put`, `Patch`, `Delete`, or `Any` route registrations. `HTTPServer.addMiddlewaresAndStaticRoutes` in `pkg/api/http_server.go` installs tracing, metrics, logging, CSRF, and the context handler before those routes run.

## Criticality criteria

A module is critical when the measured fan-in is high and a break stops boot or the main request path.

- **Fan-in.** Go: importer packages, as defined above. Frontend: importer files.
- **Hot path.** Startup if `pkg/cmd/grafana/main.go` or `pkg/server/bootstrap/wire/wire_gen.go` references it. Request path if `pkg/api` routes or middleware call it, or if the browser reaches it through `getBackendSrv()`.
- **Blast radius.** Whether a failure stops process boot, every authenticated HTTP request, or the frontend compile and render.

Fan-in alone does not decide the five. `@grafana/ui` has more file fan-in than `@grafana/runtime`, and `pkg/apimachinery` has more package fan-in than `pkg/services`. Those two are runners-up because they are not the boot input or the browser-to-API client.

## The five critical modules

**`pkg/services`.** 149 other Go packages import it, and 1,408 production Go files live in the tree. The OSS Wire file has 140 `pkg/services` import lines, which is the largest provider set in that graph. `pkg/api` (68 files, 11 packages) and `pkg/server` call into it. The most imported service subtrees from outside `pkg/services` are `apiserver` (70 packages), `featuremgmt` (55), `accesscontrol` (50), `sqlstore` (28), and `ngalert` (26). A break takes out auth, dashboards, alerting, and query together.

**`pkg/infra`.** 333 other Go packages import it, the highest backend fan-in in this scan. 448 production files in `pkg/services` import it. Wire starts it with the server. It holds logging, SQL access, cache, tracing, and metrics. A break removes persistence and observability from under every service.

**`@grafana/data`.** 2,623 production TypeScript files outside the package import it, the highest frontend fan-in. `@grafana/ui` (372 files) and `@grafana/runtime` (49 files) both import it, and both declare it in `package.json`. `public/app/features` contributes 1,622 of those files. A break stops data frames, time ranges, and query types across the UI.

**`pkg/setting`.** 254 other Go packages import it. `server.Initialize` takes `*setting.Cfg` as an argument from `pkg/cmd/grafana/main.go`. 156 packages inside `pkg/services` import it. A break stops the process before it serves a request.

**`@grafana/runtime`.** 1,165 production files import it. It owns `getBackendSrv()`, and 12 files in the package mention that client. Features (86 files), core (25), and app plugins (13) mention it too. A break stops the UI from loading datasources, dashboards, and config even when components still render.

## Runners-up

**`@grafana/ui`** has 2,121 importer files. It is the component library. `@grafana/runtime` is ranked ahead of it because runtime sits on the HTTP path into `pkg/api`.

**`pkg/apimachinery`** has 284 importer packages. `apps/dashboard` reaches it from 10 packages and 19 files. `apps/folder` and `apps/alerting` depend on it from their `go.mod` files. The OSS Wire file does not import it directly. It is the app contract, not the process entry.

**`pkg/api`** is the HTTP edge: the middleware chain and about 264 route registrations in `pkg/api/api.go`. 72 packages import it. Fewer libraries depend on it because callers sit above it. `pkg/services` also imports `pkg/api` (114 files, 53 packages), so the two form a cycle.

## Strongest compile-time edges

These are the edges that carry the map. Counts are production files, then distinct importer packages, for Go. Frontend counts are files.

**Backend, file depends on the target:**

- `pkg/services` → `pkg/infra`: 448 files, 206 packages.
- `pkg/services` → `pkg/apimachinery`: 296 files, 148 packages.
- `pkg/services` → `pkg/setting`: 257 files, 156 packages.
- `pkg/services` → `pkg/api`: 114 files, 53 packages.
- `pkg/api` → `pkg/services`: 68 files, 11 packages.
- `pkg/services` → `pkg/plugins`: 55 files, 39 packages.
- `pkg/server` → `pkg/services`: 20 files, 474 import paths, 4 packages. This is the Wire composition root, not 474 call sites spread through the server.
- `pkg/services` → `apps/dashboard`: 20 files, 15 packages.
- `apps/dashboard` → `pkg/apimachinery`: 19 files, 10 packages.
- `pkg/services` → `apps/folder`: 17 files, 12 packages.
- `pkg/services` → `apps/alerting`: 7 files, 4 packages.

**Frontend, file depends on the target:**

- `public/app/features` → `@grafana/data`: 1,622 files.
- `public/app/features` → `@grafana/ui`: 1,611 files.
- `public/app/features` → `@grafana/runtime`: 898 files.
- `public/app/features` → `public/app/core`: 634 files.
- `public/app/plugins` → `@grafana/data`: 377 files.
- `@grafana/ui` → `@grafana/data`: 372 files. Also declared in `packages/grafana-ui/package.json`.
- `public/app/features` → `@grafana/scenes`: 371 files, mostly `dashboard-scene`.
- `@grafana/data` → `@grafana/schema`: 24 files, and `@grafana/schema` is a `package.json` dependency of `@grafana/data`.

## What this map does not cover

- It is not an exhaustive graph of every file. `pkg/registry`, `pkg/storage/unified`, `pkg/util`, and the other directories under `apps/` show up only as fan-in into the modules above.
- Go fan-in includes non-`*_test.go` helpers under `pkg/tests`. Dropping `pkg/tests` and `pkg/operators` changes the leaders only slightly (`pkg/infra` 333 → 321, `pkg/services` 149 → 140, `pkg/setting` 254 → 245). The top five do not change.
- `pkg/plugins` and `pkg/apimachinery` are separate Go modules in `go.work`. `pkg/services` is not.
- Cycles are real: `pkg/services` ↔ `pkg/api`, `pkg/services` ↔ `pkg/infra`, `public/app/features` ↔ `public/app/core`, and `public/app/features` ↔ `public/app/plugins`.
- There is no compile-time edge from TypeScript to Go. The only frontend-to-backend relationship in this map is HTTP.
- Counts were taken from `main` when this note was written. Re-count imports before treating a number as current.

## Next steps

- For how a single backend service is packaged, refer to [Package hierarchy](/contribute/backend/package-hierarchy.md).
- For how the browser calls the API, refer to [Frontend data requests](frontend-data-requests.md).
- For the Kubernetes-style app layout behind `pkg/apimachinery`, refer to [Kubernetes-inspired backend architecture](k8s-inspired-backend-arch.md).
