# Kubernetes RBAC access monitor

A read-only Go service that collects RBAC grants for a supplied inventory of users
and groups, emits one JSON record per user, and publishes aggregate sensitive-access
counts. Filebeat ships the records to Elasticsearch; Prometheus and Grafana provide
an operational and investigation view.

**Verification status (2026-11-04):** Go tests, race tests, vet and build passed.
The local Compose stack was verified against the authorized cluster using read-only
collection. A complete eight-user snapshot matched across stdout, the report file,
and Elasticsearch. The three-dashboard layout was checked on 2026-11-05: all 18
data-panel queries succeeded, including the snapshot-based permission tables.
This is not a production-readiness or outage-recovery claim.
No impersonation probes or cluster mutations were performed.

## Scope and trust boundary

- Runs immediately, then every five minutes by default; one collection at a time,
  skipping missed slots rather than catching up in bursts.
- Reads RBAC lists, namespace metadata, and discovery metadata, not workload
  instances or Secret contents. The administrator setup below documents the required
  read permissions; an authorized cluster administrator must apply and bind them.
  This project does not change cluster RBAC automatically.
- Matches only usernames and groups in the startup inventory. It does **not** invent
  `system:authenticated` membership, impersonate users, or evaluate admission,
  external authorizers, identity-provider membership, or effective authorization.
- Preserves API groups, scopes, subresources, native verbs, `resourceNames`, original
  rules and role/binding provenance. ClusterRole aggregation uses the observed rules
  produced by Kubernetes, not a second implementation of its controller.
- A missing relevant role or unknown resource scope fails the observation. Counts
  retain the last complete values and their original freshness, not misleading zeroes.
  Complete list/discovery reads are not a transactionally consistent Kubernetes snapshot.
- A successful observation updates metrics **before** independent checked file/stdout
  writes. File acceptance does not prove Elasticsearch delivery; a snapshot may be
  only partially indexed. Empty users still have an access record.
- Applicable name-restricted sensitive grants count as exposure, not as unrestricted
  permission or evidence that an object exists. Name-restricted top-level `create`
  and `deletecollection` do not count as usable permission.

## Layout

| Path | Responsibility |
| --- | --- |
| `cmd/access-monitor/` | Calls package initializers and runs the Fx application |
| `internal/config/` | Typed Viper settings; user and policy inputs remain JSON strings |
| `internal/domain/` | User/policy parsing and validation, RBAC resolution, records and unique-user counting |
| `internal/pkg/client/http/kubernetes/` | client-go transport, pagination, discovery and bounded retries |
| `internal/service/monitor/`, `internal/jobs/` | Collection orchestration and scheduling |
| `internal/logger/` | Checked JSON stdout/file output, Zap diagnostics and rotation |
| `internal/observability/`, `internal/transport/` | Prometheus collectors and Chi/net/http health endpoints |
| `configs/`, `deployments/`, `build/` | Runtime defaults, Compose, Grafana dashboards/provisioning and image build |

The design notes and supplied task files remain locally ignored in `docs/` and
`defenition/`, as configured by the repository owner. Never add the supplied kubeconfig
to Git, a Docker build context, test fixtures, or an image.

## Go development

Use Go **1.24.13** (module minimum 1.24.0). A newer toolchain download was blocked in
the development environment; dependencies are pinned in `go.mod`/`go.sum` to versions
compatible with the installed toolchain, including client-go 0.33.7. Kubernetes
version-skew compatibility with the target cluster must be checked before use.

```sh
go test ./...
go test -race ./...
go vet ./...
go build -o bin/access-monitor ./cmd/access-monitor
```

Tests are Go unit and integration tests, using temporary files, failing writers,
and local HTTP servers. They need neither the supplied cluster nor Elasticsearch.
There is no external-service automated suite or cross-service E2E framework.
Config, domain and entrypoint tests were removed at the repository owner's request;
passing tests do not establish exhaustive RBAC semantic coverage.

## Startup configuration

`APP_ENV` selects one required file in `configs/`: `config.local.yaml`,
`config.development.yaml`, or `config.production.yaml`. An unset `APP_ENV` selects
`local`. There is no `APP_CONFIG_DIR`, base-file merge, or code-level setting defaults.
The environment files currently share the task baseline; production naming does not
imply production sizing or hardening.

`APP_*` environment overrides take precedence over file values. Nested dots become
underscores: `poll.interval` maps to `APP_POLL_INTERVAL`, for example. Supported keys
are inferred from the configuration struct by Viper's struct-binding option, so
overrides also work when a key is absent from YAML; no manual key list is maintained.
Missing files, malformed/unknown settings, or missing required values fail startup.
User and policy inputs are inline JSON strings in YAML keys `user` and `policy`,
overridable with `APP_USER` and `APP_POLICY`. `Config.User` and `Config.Policy` retain
the raw strings. During service construction, domain parsers validate them and return
domain users and policy directly; invalid inputs fail startup. Each JSON value is
limited to 8 MiB. Changes require a restart.
Committed YAML files contain synthetic users. Keep actual user inputs in ignored
`deployments/.env`; do not share or commit that file.
Identity keys are opaque strings, not UUIDs. Duplicate keys/usernames, invalid
identities, oversized inputs and trailing JSON are rejected.

| Setting | Environment override | Default |
| --- | --- | --- |
| User JSON | `APP_USER` | Synthetic examples in YAML; Compose supplies the value from ignored `.env` |
| Sensitivity policy JSON | `APP_POLICY` | Inline baseline policy in YAML |
| Poll interval / work budget | `APP_POLL_INTERVAL` / `APP_POLL_TIMEOUT` | `5m` / `2m` |
| Kubeconfig | `APP_KUBERNETES_KUBECONFIG` | `/run/secrets/kubeconfig` |
| Request timeout / total attempts | `APP_KUBERNETES_REQUEST_TIMEOUT` / `APP_KUBERNETES_ATTEMPTS` | `20s` / `3` |
| Backoff bounds | `APP_KUBERNETES_BACKOFF_MIN` / `APP_KUBERNETES_BACKOFF_MAX` | `500ms` / `5s` |
| Rate / burst / page size | `APP_KUBERNETES_QPS` / `APP_KUBERNETES_BURST` / `APP_KUBERNETES_PAGE_SIZE` | `5` / `10` / `500` |
| HTTP listener / shutdown | `APP_HTTP_ADDR` / `APP_HTTP_SHUTDOWN_TIMEOUT` | `:8080` / `10s` |
| Output path | `APP_OUTPUT_PATH` | YAML: `/var/log/access-monitor/events.jsonl` |
| Rotate bytes / backups | `APP_OUTPUT_MAX_BYTES` / `APP_OUTPUT_BACKUPS` | `20971520` / `10` |
| Diagnostic level | `APP_LOG_LEVEL` | `info` |

The work budget covers fetching/resolving/counting; synchronous file/stdout writes
can block beyond it. Retries, rate waits and pagination share that budget. 401/403
are not retried within a cycle. Exec-authentication plugins, impersonation and
insecure TLS kubeconfigs are rejected. Kubeconfig-referenced certificate/token files,
if used, must be readable inside the container at their configured paths; a kubeconfig
with embedded credentials avoids additional mounts. Do not print its contents.

For a local run, explicitly set a writable output path and credential path:

```sh
APP_USER="$(cat defenition/input.json)" \
APP_KUBERNETES_KUBECONFIG=/secure/path/kubeconfig \
APP_OUTPUT_PATH=./var/log/access-monitor/events.jsonl \
./bin/access-monitor
```

That command contacts the configured cluster; run it only when authorized and ready.

## Local Compose setup

Requires Docker Engine, Compose v2, enough free disk, and memory for Elasticsearch
(512 MiB heap, 1536 MiB container limit) plus the remaining services. Set the host's
`vm.max_map_count` to the requirement of the pinned Elasticsearch release if startup
reports it as insufficient; host tuning is an operator action, not performed by this
project. No Docker socket or privileged container is required.

Pinned defaults: Elasticsearch/Filebeat **9.5.6**, Grafana **13.2.4**, Elasticsearch
data-source plugin **12.9.5**, Prometheus **v3.14.0**, and Grafana's Prometheus plugin
**13.2.4**. These versions were exercised together locally. Grafana installs the
pinned signed plugins synchronously; an empty plugin volume requires network access.
Automatic plugin updates are disabled. Previously verified plugin copies persist
in the Grafana data volume; intermittent download failures remain a fresh-install risk.

Version/query references checked during implementation (2026-11-03):
[Grafana release](https://github.com/grafana/grafana/releases/tag/v13.2.4),
[Elasticsearch release](https://github.com/elastic/elasticsearch/releases/tag/v9.5.6),
[Beats release](https://github.com/elastic/beats/releases/tag/v9.5.6),
[Prometheus release](https://github.com/prometheus/prometheus/releases/tag/v3.14.0),
[Elasticsearch plugin metadata](https://grafana.com/api/plugins/elasticsearch), and
[pinned plugin query schema](https://github.com/grafana/grafana-elasticsearch-datasource/blob/v12.9.5/src/dataquery.gen.ts).

### Cluster administrator prerequisite

The kubeconfig identity needs cluster-wide list access to RBAC objects and
namespaces, plus API discovery access. If these permissions are missing, ask an
authorized cluster administrator to apply this example ClusterRole and create a
ClusterRoleBinding for the monitor's actual kubeconfig identity or ServiceAccount:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: access-monitor-auditor
rules:
  - apiGroups: [rbac.authorization.k8s.io]
    resources: [roles, rolebindings, clusterroles, clusterrolebindings]
    verbs: [list]
  - apiGroups: ['']
    resources: [namespaces]
    verbs: [list]
  - nonResourceURLs: [/api, /api/*, /apis, /apis/*]
    verbs: [get]
```

The role alone grants nothing until bound to an identity. No permission to read
Secret contents or mutate cluster objects is needed. We do not have authorization
to provision cluster RBAC; this is an administrator prerequisite, not a Compose
startup step.

From the repository root:

```sh
cp deployments/.env.example deployments/.env
mkdir -p secrets
chmod 700 secrets
cp defenition/amirhossein-motaghian.k8s.yaml secrets/kubeconfig
chmod 600 secrets/kubeconfig
setfacl -m u:10001:r secrets/kubeconfig
openssl rand -hex 32 > secrets/elastic-password
openssl rand -hex 32 > secrets/filebeat-password
openssl rand -hex 32 > secrets/grafana-reader-password
openssl rand -hex 32 > secrets/grafana-admin-password
chmod 444 secrets/*-password
```

The protected host directory limits access while allowing non-root containers to
read individually mounted secret files. Local Compose file-backed secrets are not
an encrypted secret store: host root/Docker access and the receiving container can
read them. File-backed secret `uid`/`mode` remapping is not reliably applied to bind
sources; check actual readability rather than relying on YAML declarations.

Set `APP_USER` to the full user JSON string in ignored `deployments/.env`, using
the single-quoted format shown in `.env.example`. Keep `policy` in
`configs/config.<APP_ENV>.yaml`. Compose forwards `APP_USER` and mounts the
configuration directory read-only; it does not forward `APP_POLICY`.
Environment values are visible to authorized Docker users; avoid sharing rendered
Compose configuration, which includes the user JSON. No shell export is needed.
During service construction, `domain.ParseUsers` and `domain.ParsePolicy` validate
the strings and create the domain values. Review user data before sharing configuration.

Edit `deployments/.env` paths, especially `KUBECONFIG_FILE`. This is a Compose
manifest environment file, not a service dotenv file; it includes `APP_ENV=local`,
which Compose explicitly forwards to the monitor. Relative paths are resolved from
`deployments/` by Compose. The monitor runs as UID/GID 10001; arrange
read access for that identity using a restricted dedicated copy or ACL. Do not make
the original credential world-readable. Use absolute paths if that is less ambiguous.
The user JSON does not require a container file mount. `deployments/.env` contains settings and
paths, **not** credential values. Do not regenerate passwords during an ordinary
restart: Elasticsearch's bootstrap password does not reset an existing data volume.

Validate without building, pulling, or starting anything:

```sh
docker compose --env-file deployments/.env -f deployments/compose.yaml config --quiet
```

When you choose to start the test phase:

```sh
docker compose --env-file deployments/.env -f deployments/compose.yaml up -d --build
docker compose --env-file deployments/.env -f deployments/compose.yaml ps -a
docker compose --env-file deployments/.env -f deployments/compose.yaml logs --tail=100 setup monitor filebeat grafana
```

The monitor is independent of Elasticsearch startup. Setup generates a local CA and
Elasticsearch HTTPS certificate, waits for Elasticsearch, and provisions indices
and scoped credentials before Filebeat/Grafana start. Setup is not a continuously
running service. Preserve certificate and data volumes across restarts; deleting
only one side requires deliberate recovery. Normal ingestion/dashboard services
never receive the elevated `elastic` credential.

Open **http://127.0.0.1:3000** by default, or the port selected by `GRAFANA_PORT`
in `.env` (the validated local deployment uses **3001**). Use user `admin` and the
Grafana admin password file. Dashboards:

| Dashboard | URL path | Source |
| --- | --- | --- |
| Service health | `/d/access-monitor-health` | Prometheus and Elasticsearch diagnostics |
| User access | `/d/access-monitor` | Elasticsearch access history |
| Sensitive access | `/d/access-monitor-sensitive` | Prometheus sensitive-access metrics |

Each dashboard links to the other two, preserving the selected time range.
Anonymous access and sign-up are disabled. All other endpoints are internal by
default. Dashboard separation is not access control. Grafana read credentials can read the two selected
indices; anyone able to use that data source may query beyond a panel's visible filter.

User, resource and verb dropdowns come from Elasticsearch records within the selected
time range (up to 1,000 values), independently of the sensitive policy. Resource and
verb accept custom values for concrete permissions covered by wildcard grants.
Namespace is a searchable dropdown with an All option and custom input; it affects
historical matching, not the permission tables. Matching searches all API groups;
the permission tables retain API group as a column. Historical matching handles
exact resources, `*`, and `*/subresource` patterns within the same grant, including
verb and scope filters.
The historical matching panel is independent of the table's selected user/snapshot
and returns at most 500 observation records, not continuous authorization evidence.

Permission tables use one user or All users and one complete indexed snapshot.
The resource table retains a username column so users' grants remain separate.
All-users status/non-resource tables return at most 1,000 records. The namespace
dropdown includes up to 1,000 recorded values in range, with custom input available.
The snapshot
dropdown checks the latest 100 observations in the time range, newest first, and
defaults to the first option on a fresh dashboard load. Existing URL selections are
preserved; refresh the dashboard without variable parameters to reset to latest.
Completeness is enforced by the dropdown, not by each permission-panel query.
A manually supplied or restored snapshot URL can bypass that validation; reselect
the snapshot from the populated dropdown before treating the tables as complete.
Completeness requires an exact distinct-username count equal to a consistent positive
`expected_users`, and the same document count. A newer partially indexed snapshot
does not displace an older complete one. Narrow the time range to find older snapshots
outside the 100-candidate window. No options means no complete snapshot was found
within that window, not that users have no permissions.

The status table distinguishes **No recorded permissions** from a missing user record
(no rows). Errors are not empty/healthy states. Resource permissions display API group,
resource, verb, scope, namespace and resource-name restriction. Verbs/name restrictions
expand within the same nested grant; `(none)` means no name restriction. Empty API
group means core. Terms tables use up to 1,000 buckets per column; reaching a limit
means coverage is not guaranteed. Non-resource URL/verb rules are shown in a separate
table cell because their existing source field is not indexed. Original provenance
remains available in the historical records.

The user selector displays usernames but uses numeric character keys internally so
quotes, backslashes and wildcard characters remain literal identity values. Do not
replace these keys with raw usernames in saved dashboard URLs. Raw DSL queries and
Elasticsearch scripts run through the existing read-only Grafana data source; no new
application service or plugin is needed. The deliverables are the three dashboard
JSON files, not a generator or Python runtime.
UI dashboard saves are enabled and persist in Grafana's database, not the JSON
file. Changes to the provisioning JSON can overwrite UI edits; export edits you
want to keep. Plugin versions are pinned and automatic startup updates disabled
to avoid replacing a working plugin during queries.

### Repeatable manual verification

Only perform read-only checks; do not mutate interview-cluster RBAC/workloads to
manufacture fixtures.

1. Confirm setup exits 0, services stay running, and inspect monitor startup and
   collection errors, including historical failures. `/livez` means responsiveness; `/readyz` means initialization,
   not the latest collection or delivery succeeded.
2. In Grafana Explore, query Prometheus `up{job="access-monitor"}`,
   `access_monitor_has_observation`, observation age and sink errors. Access counts
   must be absent before the first complete observation, then reflect the policy.
3. Inspect both Elasticsearch-backed panels. Confirm access events go to
   `access-history-v1`, operational events to `access-monitor-logs-v1`; the `grants`
   mapping must be `nested`. Check stable event identity is used as Elasticsearch
   `_id`, not just a source field. Re-ingestion must target the same index/ID.
4. Check the default `secrets/get` query, exact user and snapshot filtering, core/API
   group selection, scope, subresource wildcard grants and original name restrictions.
   A user with only `get pods` and `delete secrets` must **not** match `get secrets`.
    Confirm the selected-user status and tables distinguish known-empty and missing
    users; inspect historical records for expandable provenance.
5. Inspect actual Grafana variable interpolation and returned records; raw DSL support
   in upstream source is not proof of working panel configuration. Do not substitute
   flattened independent grant filters if this fails. Treat incompatibility as an
   unresolved delivery gate and report it.
6. Check Filebeat logs and its internal `http://filebeat:5066/stats` endpoint for
   delivery/queue/progress signals. No Prometheus exporter is installed; application
   `up` alone says nothing about shipping. Check volume free space and actual log
   rotation/renamed-file harvesting before relying on outage recovery.

No delivery acknowledgement timestamp or exact pending-snapshot count is claimed.
The setup's CA private key is sensitive; restrict backup/host access to its volume.

### Checking Filebeat and interpreting health panels

From the repository root, inspect shipping logs and internal statistics:

```sh
docker compose --env-file deployments/.env -f deployments/compose.yaml logs --since=15m filebeat
docker compose --env-file deployments/.env -f deployments/compose.yaml exec -T monitor wget -qO- http://filebeat:5066/stats
```

Compare statistics across collection cycles: `libbeat.output.events.acked` should
advance as new records arrive; inspect `failed`, `dropped`, pipeline `events.active`
and queue `filled.events` for delivery trouble. Cumulative counters reset on restart,
and old failures do not necessarily mean delivery is currently broken. Confirm a
new file snapshot actually arrives in Elasticsearch; a running container is not proof.

- **Application scrape availability:** Prometheus can scrape the monitor, not proof
  that collection or shipping works.
- **Observation age / File report age:** freshness of the last complete observation
  or file report, respectively; neither measures Elasticsearch acknowledgement.
- **Collection duration p95:** histogram estimate over 30 minutes, in source seconds
  with Grafana-scaled time units; includes successful and failed collection attempts.
- **Output failures / 15 minutes:** stdout/file write failures. The zero fallback
  also returns zero when telemetry is absent; check scrape availability separately.
- **Monitor application logs and errors:** historical monitor diagnostics in the
  selected range, not Kubernetes user activity or logs from every stack component.
- **Elasticsearch query check — access records:** record count within the selected
  range. A count, even zero, confirms the query path worked; it is not cluster health,
  delivery freshness, or an uptime percentage.

No Filebeat or Elasticsearch Prometheus exporter is deployed. Exporters or another
metrics integration would add queue, delivery and Elasticsearch internals; they are
future operational improvements, not a prerequisite for the task's two RBAC metrics.
No full-stack alerting is configured.

## Records and metrics

Access records have `schema_version: v1`, `event_type: access_snapshot`, a random
snapshot ID, a SHA-256 event ID derived from snapshot/user, observation/start times,
expected-user count, `identity_basis: supplied_input_subjects`, username, supplied
groups, grants and non-resource grants. All users in one successful observation share
the snapshot ID/time. Diagnostic records use `event_type: operational` and separate
random IDs. Filebeat uses event identity for document deduplication and fixed index
routing; it may remove the field from `_source` while setting `_id`.

Required metric shapes are unchanged:

```text
k8s_namespace_sensitive_access_users_count{namespace,verb,resource}
k8s_cluster_wide_sensitive_access_users_count{resource,verb}
```

Counts are distinct monitored usernames. Namespaced grants from ClusterRoleBindings
also count for each configured sensitive namespace; RoleBindings never imply
cluster-wide access. Policy is initially restricted to core API resources because
these required labels cannot distinguish API groups. Detailed permissions, users,
restrictions and sources belong in records, not metric labels.

Operational metrics cover collection outcomes/durations, stage/request failures,
file/stdout outcomes and latency, loaded users, observation/file-report freshness,
and Go/process health. A process restart has no restored access snapshot: unknown
remains unknown until a complete observation succeeds.

Three provisioned dashboards separate service health, access investigation and
sensitive access. Time-range history means **observed at least once**, not
continuous authorization. The permission table's snapshot selector checks indexed
completeness; it does not establish an atomic Kubernetes observation or live access.
Result limits also cap visible coverage.
Access variables do not filter aggregate Prometheus counts by username.

The requested Elasticsearch and Prometheus dashboards are separate exports, plus
an additional service-health dashboard. Filebeat is the selected log-processing/
shipping application, not a second custom Go Elasticsearch client. The richer output
schema follows the task's allowance to change the example format.

Snapshot completeness, duplicate/inconsistent input rejection, empty/missing users,
special-character usernames, subresource wildcard matching and same-grant filter
correlation were exercised using temporary synthetic records in an isolated local
Elasticsearch index; that index was removed afterward. No cluster RBAC was modified.
Remaining limitations include missing domain regression tests by request, unverified
end-to-end prolonged-outage/replay/rotation scenarios, the default Fx global stop
deadline, finite query limits, manual Elasticsearch retention and fresh-volume plugin
download dependence. Earlier Git history contains inventory data even though current
inventory is ignored; review repository-sharing permissions before submission.

## Storage and operations

- Application logs: 20 MiB per active file, 10 rotated backups (about 220 MiB total),
  rename rotation, no compression. This is a target, not a hard filesystem quota.
- Docker logs: 10 MiB × 3 files per container. Stdout is another sensitive copy,
  **not** an automatic Filebeat fallback.
- Prometheus: 7 days / `1GB` retention-size setting, whichever is reached first under
  its supported cleanup behavior; not a hard total-volume cap.
- Elasticsearch: fixed indices, manual cleanup after 30 days for access history and
  7 days for diagnostics. No automatic expiry or rollover is configured.
- Logs, Filebeat state, Elasticsearch, Prometheus, Grafana and certificates persist
  on named volumes. Persistence is not backup; never use `down -v` as a routine stop.

For ordinary stop, use `docker compose --env-file deployments/.env -f deployments/compose.yaml down`
without `-v`. User/policy inputs are loaded once during service construction.
Changes to `policy` or other settings in the selected YAML file require a monitor
restart. Changing `APP_USER` or `APP_ENV` in `.env` requires container recreation
via Compose `up -d`; a restart alone retains the old environment values.

### Manual Elasticsearch cleanup

Deletion is irreversible. Review the target, cutoff, backups and required history
before executing. Use the setup container's trusted CA/admin credential without
printing it; first query `/_count` with the same range. Example request bodies:

```json
{"query":{"range":{"observed_at":{"lt":"now-30d"}}}}
```

Send this body to `POST /access-history-v1/_delete_by_query`. For diagnostics use
`now-7d` and `POST /access-monitor-logs-v1/_delete_by_query`. Inspect response
`failures`, `version_conflicts` and `deleted`, not merely the HTTP status. Filebeat
may reintroduce old events if their source files/state are replayed; stable IDs
prevent duplicates, not reappearance after intentional deletion. Deleted documents
do not immediately release all disk space. Do not delete indices/mappings or reset
registry state to implement ordinary retention.

Retention is a local demonstration, not production sizing or an outage SLA. Size from
measured bytes/time, available resources, recovery/history needs and acceptable loss.
Rotated-away, never-written, rejected or lost-volume records cannot be recovered by
Filebeat. Slow synchronous output may delay collection; an indefinite outage exceeds
any finite retention window.

### Troubleshooting

- **Startup exits:** verify paths, secret readability, strict input/schema, durations,
  and supported kubeconfig authentication; avoid logging configuration/credentials.
- **401/403:** correct the auditor's authentication/read permissions; repeated retries
  within the same cycle will not fix authorization.
- **Unresolved role/scope:** inspect sanitized diagnostics and discovery permissions;
  stale counts are intentional rather than publishing a false partial observation.
- **Fresh metrics, no history:** inspect file-output counters, Filebeat logs/registry,
  index provisioning/permissions and Elasticsearch disk/security. These are separate
  success boundaries.
- **Grafana query error:** inspect the provisioned data source/plugin and query
  inspector. Preserve nested-grant correlation; report the unsupported behavior.
- **Disk pressure:** check Docker/named-volume usage, source-log size and Filebeat
  progress; apply deliberate retention or add capacity. Do not delete state blindly.
