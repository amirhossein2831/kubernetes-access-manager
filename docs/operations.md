# Operations reference

This is the setup and troubleshooting companion. Start with the
[system design](system-design.md) for the architecture and
[decisions](decisions-and-tradeoffs.md) for its boundaries.

## Before running

- Docker Engine and Compose v2, free disk and memory for the stack. Elasticsearch
  uses a 512 MiB heap and a 1536 MiB container limit. If it reports an insufficient
  `vm.max_map_count`, have the operator tune the host for the pinned release.
- An authorized read-only kubeconfig. The monitor rejects exec-auth plugins,
  impersonation and insecure TLS. Referenced credential files must be readable
  inside the container; embedded credentials avoid extra mounts.
- User inventory in ignored `deployments/.env`; synthetic users remain in committed
  configuration. Never commit credentials or put kubeconfig into an image.

The kubeconfig identity needs cluster-wide `list` access to Roles, RoleBindings,
ClusterRoles, ClusterRoleBindings and Namespaces, plus `get` discovery access to
`/api`, `/api/*`, `/apis`, `/apis/*`. An authorized administrator must arrange these
permissions; the application does not create or bind RBAC. No Secret-content read
or workload mutation permissions are needed.

## First-time local setup

From the repository root, create dedicated local credentials:

```sh
cp deployments/.env.example deployments/.env
mkdir -p secrets
chmod 700 secrets
cp /secure/path/to/authorized-kubeconfig secrets/kubeconfig
chmod 600 secrets/kubeconfig
setfacl -m u:10001:r secrets/kubeconfig
openssl rand -hex 32 > secrets/elastic-password
openssl rand -hex 32 > secrets/filebeat-password
openssl rand -hex 32 > secrets/grafana-reader-password
openssl rand -hex 32 > secrets/grafana-admin-password
chmod 444 secrets/*-password
```

These are **first-time commands**, not a restart procedure. Do not overwrite existing
credentials: changing the bootstrap password file does not reset an existing
Elasticsearch volume. The protected directory limits host access while individually
mounted password files remain readable by non-root containers. Local Compose secrets
are not an encrypted secret store; host root/Docker administrators can read them.
Verify real file readability rather than relying on secret UID/mode remapping.

Edit `.env` using `.env.example` as the reference. Set `APP_USER` to the full inventory
JSON in the shown single-quoted format and configure `KUBECONFIG_FILE` and secret
paths. Relative paths resolve from `deployments/`; absolute paths avoid ambiguity.
The monitor runs as UID/GID 10001. Do not make original kubeconfig credentials
world-readable. The env file contains user data and credential paths, not passwords;
rendered Compose configuration exposes that inventory to authorized Docker users.

```sh
# Validate without printing the inventory or contacting Kubernetes.
docker compose --env-file deployments/.env -f deployments/compose.yaml config --quiet

# Starts collection against the configured cluster: use only when authorized.
docker compose --env-file deployments/.env -f deployments/compose.yaml up -d --build
docker compose --env-file deployments/.env -f deployments/compose.yaml ps -a
docker compose --env-file deployments/.env -f deployments/compose.yaml logs --tail=100 setup monitor filebeat grafana
```

Setup generates a local CA/certificate and provisions Elasticsearch indices and
scoped Filebeat/Grafana credentials. It should exit successfully, not run forever.
The monitor does not depend on Elasticsearch startup. Preserve certificate and data
volumes together; runtime ingestion/query services do not receive the `elastic`
administrator credential. Restrict access to the CA private-key volume.

Pinned stack: Elasticsearch/Filebeat 9.5.6, Grafana 13.2.4, Prometheus v3.14.0,
Elasticsearch plugin 12.9.5 and Prometheus plugin 13.2.4. Plugins install synchronously
and updates are disabled. Fresh plugin volumes need network access.

## Configuration

`APP_ENV` selects the required `configs/config.local.yaml`,
`config.development.yaml` or `config.production.yaml`; unset means `local`.
There is no base-file merge or `APP_CONFIG_DIR`. Production naming does not imply
production sizing. `APP_*` overrides YAML, with nested dots replaced by underscores.
Unknown/malformed settings and missing required values fail startup.

| Setting | Override | Baseline |
| --- | --- | --- |
| Inventory / policy JSON | `APP_USER` / `APP_POLICY` | Synthetic inventory / policy in YAML |
| Collection interval / budget | `APP_POLL_INTERVAL` / `APP_POLL_TIMEOUT` | `5m` / `2m` |
| Kubeconfig | `APP_KUBERNETES_KUBECONFIG` | `/run/secrets/kubeconfig` |
| Request timeout / attempts | `APP_KUBERNETES_REQUEST_TIMEOUT` / `APP_KUBERNETES_ATTEMPTS` | `20s` / `3` |
| Backoff bounds | `APP_KUBERNETES_BACKOFF_MIN` / `APP_KUBERNETES_BACKOFF_MAX` | `500ms` / `5s` |
| QPS / burst / page size | `APP_KUBERNETES_QPS` / `APP_KUBERNETES_BURST` / `APP_KUBERNETES_PAGE_SIZE` | `5` / `10` / `500` |
| HTTP address / shutdown | `APP_HTTP_ADDR` / `APP_HTTP_SHUTDOWN_TIMEOUT` | `:8080` / `10s` |
| Output path | `APP_OUTPUT_PATH` | `/var/log/access-monitor/events.jsonl` |
| Rotation bytes / backups | `APP_OUTPUT_MAX_BYTES` / `APP_OUTPUT_BACKUPS` | `20971520` / `10` |
| Diagnostic level | `APP_LOG_LEVEL` | `info` |

User/policy JSON is validated at startup, limited to 8 MiB each, and loaded once.
Duplicate identities/usernames and trailing JSON are rejected; identity keys need
not be UUIDs. Compose forwards `APP_USER` and `APP_ENV`, but not `APP_POLICY`:
keep policy in the selected YAML. YAML changes require restarting the monitor;
environment changes require container recreation with Compose `up -d`.
401/403 responses are not retried within a collection cycle.

## Dashboards

Grafana defaults to `http://127.0.0.1:3000`; `GRAFANA_PORT` can change this. Sign in
as `admin` using the configured password file. Anonymous access/sign-up are disabled;
other service endpoints are internal by default.

| View | Path |
| --- | --- |
| User access | `/d/access-monitor` |
| Sensitive access | `/d/access-monitor-sensitive` |
| Service health | `/d/access-monitor-health` |

- Permission tables select one user or All users and one complete snapshot.
  No recorded permissions differs from a missing user row. API group remains a
  table column; historical matching searches all groups.
- Resource/verb/namespace choices come from history, not only the sensitivity policy.
  Namespace has All; custom resource/verb/namespace values support concrete queries
  covered by wildcard rules. These investigation filters do not narrow the
  all-permissions tables or aggregate Prometheus counts.
- The snapshot selector checks the latest 100 candidates within the range. It requires
  exact distinct-user/document counts and consistent positive `expected_users` on
  every record. Newer partial snapshots are excluded. No options means none qualified
  in that candidate window; narrow the range for older history.
- Fresh loads without saved variable selections choose the newest complete snapshot.
  Refresh preserves a selected snapshot; it does not follow latest. URL selections
  can bypass dropdown completeness checks—reselect from populated options.
- Dropdowns and terms columns are capped at 1,000 values/buckets. All-user raw tables
  return up to 1,000 records; historical matching returns up to 500. Reaching limits
  means completeness is not guaranteed. Non-resource rules are source-backed cells.
- Username URL values are numeric character keys, not raw names, to preserve literal
  quotes/backslashes/wildcards. Do not replace them manually.
- Grafana UI edits persist in its database, not provisioning JSON. File updates can
  overwrite UI edits; export changes you want to retain.

Dashboards are not an access-control boundary. Anyone allowed to query the data
source can potentially read beyond a panel filter. Protect both access-history and
diagnostic indices, stdout and file copies as sensitive data.

## Check health without confusing the boundaries

1. Check setup exit status and service logs. `/livez` proves responsiveness;
   `/readyz` proves initialization, not a fresh collection or delivery.
2. Inspect `up{job="access-monitor"}`, `access_monitor_has_observation`, observation
   age, file-report age and output failures. Counts are absent before first success.
3. Confirm a new file snapshot reaches `access-history-v1`. Diagnostics belong in
   `access-monitor-logs-v1`. Check nested `grants` mapping and event identity as `_id`.
4. Exercise `secrets/get`, a selected user/snapshot and a subresource query. A rule
   for `get pods` plus another for `delete secrets` must not match `get secrets`.
   Inspect actual returned data and interpolation, not just HTTP success.
5. Inspect Filebeat progress across cycles:

```sh
docker compose --env-file deployments/.env -f deployments/compose.yaml logs --since=15m filebeat
docker compose --env-file deployments/.env -f deployments/compose.yaml exec -T monitor wget -qO- http://filebeat:5066/stats
```

Look for advancing `libbeat.output.events.acked`, failed/dropped events, active
pipeline events and queue fill. Counters reset on restart; old failures alone do not
prove current trouble. Also check disk and rotated-file harvesting. Never mutate
cluster RBAC/workloads just to manufacture test fixtures.

| Signal | What it does **not** prove |
| --- | --- |
| Application scrape availability | Collection or shipping success |
| Observation / file-report age | Elasticsearch acknowledgement |
| Output failures showing zero | Telemetry exists: the query has a zero fallback |
| Elasticsearch record-count query | Cluster health, freshness or complete delivery |
| Application logs panel | Kubernetes user activity or every stack component's logs |

Collection p95 is a histogram estimate over 30 minutes in source seconds, scaled by
Grafana. No Filebeat/Elasticsearch exporter, delivery-acknowledgement timestamp,
exact pending-snapshot count or full-stack alerting is provided.

## Storage, restarts and retention

| Store | Current policy |
| --- | --- |
| Application files | 20 MiB active file + 10 rename-rotated backups; no compression |
| Docker logs | 10 MiB × 3 files per container |
| Prometheus | 7 days / `1GB` retention-size setting |
| Elasticsearch | Manual cleanup: 30 days access history, 7 days diagnostics |

These are operating policies, not hard total-disk quotas or an outage SLA. Persistent
volumes are not backups. Stop normally without deleting volumes:

```sh
docker compose --env-file deployments/.env -f deployments/compose.yaml down
```

**Do not use `down -v` for routine stops.** Never reset Filebeat registry state or
delete indices/mappings as ordinary retention.

For Elasticsearch cleanup, first review backups and query `_count` using the same
cutoff. Only after explicit approval, use `_delete_by_query` on the intended index:

```json
{"query":{"range":{"observed_at":{"lt":"now-30d"}}}}
```

Access endpoint: `POST /access-history-v1/_delete_by_query`. Diagnostics use
`now-7d` and `POST /access-monitor-logs-v1/_delete_by_query`. Use trusted CA/admin
credentials without printing them. Inspect failures, conflicts and deleted counts.
Deletion is irreversible and does not immediately release all disk space. Replaying
source files can reintroduce intentionally deleted records despite stable IDs.

## Troubleshooting

| Symptom | Check first |
| --- | --- |
| Startup exits | Paths, UID 10001 readability, input schema and supported kubeconfig auth |
| Kubernetes 401/403 | Auditor credentials and read/discovery permissions |
| Unresolved role/scope | Sanitized collection diagnostics and discovery access; stale counts are intentional |
| Fresh metrics, missing history | File writes, Filebeat registry/queue, index provisioning, Elasticsearch permissions/disk |
| Grafana query errors | Pinned plugin/data source and query inspector; preserve same-grant correlation |
| Disk pressure | Named-volume usage, rotation and shipping progress; apply deliberate retention |

## Development and verification

Use Go 1.24.13 (module minimum 1.24.0); dependencies are pinned, including client-go
0.33.7. Check client/server version skew before using another cluster.

```sh
go test ./...
go test -race ./...
go vet ./...
go build -o bin/access-monitor ./cmd/access-monitor
```

Tests use temporary files and local HTTP servers, not the supplied cluster. Domain,
configuration and entrypoint coverage is incomplete; no automated external-service
E2E suite is shipped. Local verification exercised a complete eight-user pipeline,
18 dashboard queries, and isolated synthetic completeness/matching cases. Prolonged
outage/replay/rotation recovery is not fully verified; the Fx global shutdown deadline
also remains a limitation.

Current inventory is ignored, but earlier Git history contains inventory data.
Review repository-sharing permissions before distributing the repository.
