# Task validation — 2026-11-04

Reviewed against the supplied Security Engineer task PDF (sections 3–4), current
source and Compose deployment at `e9d0175`. This report deliberately excludes real
usernames, groups, cluster addresses and credentials. Validation was read-only:
no RBAC/workload changes, outage injection, exporter installation or history rewrite.

## Requirement coverage

| Task requirement | Result and evidence |
| --- | --- |
| Go/Python application using supplied kubeconfig | Implemented in Go with client-go; authorized live collection succeeded. |
| Collect every five minutes | Immediate run followed by configured `5m` schedule; successive live snapshots observed. Collection failures skip publishing incomplete results. |
| Monitor only supplied JSON users, including users with no grants | Inventory parsed once at startup; resolver iterates supplied users and emits empty grant arrays when unmatched. Eight configured users matched the latest complete snapshot. An empty-user live fixture was not manufactured. |
| Per-user JSON to stdout and file | Latest checked snapshot has eight records/eight unique users in both sinks. |
| User, resources, verbs, namespace/cluster scope and timestamp | Records retain native Kubernetes verbs, API groups, subresources, scopes, name restrictions and role/binding provenance. Output schema differs from the explicitly optional example. |
| Process logs and store them in Elasticsearch | Filebeat ships JSON, routes access/operational records separately and sets stable document IDs. Latest file snapshot matches eight indexed records, identical usernames and event IDs. |
| Select resource/verb and inspect historical matching users | Nested Elasticsearch query works for tested core `secrets/get` selection; time range and user/snapshot selection supported. Dropdown coverage and wildcard limitations below apply. |
| Table of all permissions for one user | Partial: all-permissions records are available and expandable, but the panel is a logs view, not a dedicated table. |
| Namespace-sensitive metric with exact labels | Present: `k8s_namespace_sensitive_access_users_count{namespace,verb,resource}`; 30 configured policy series returned. |
| Cluster-wide sensitive metric with exact labels | Present: `k8s_cluster_wide_sensitive_access_users_count{resource,verb}`; 19 configured policy series returned. |
| Sensitive policy and namespaces chosen by implementer | YAML policy covers `default` and `kube-system`, secrets and selected other core resources/verbs. Counts deduplicate monitored usernames. |
| Elasticsearch access dashboard and Prometheus risk dashboard | Both views work, but packaged as one combined dashboard rather than two separately requested dashboards. |
| All services configured through Compose | Monitor, setup, Elasticsearch, Filebeat, Prometheus and Grafana configured; setup exited 0, five long-running services running, Elasticsearch container healthy. |
| Code and documentation in Git repository | Present locally. Remote submission/push was not performed or validated in this review. |

## Fresh verification evidence

- `go test ./...`, `go test -race ./...`, `go vet ./...` and a build to
  `/tmp/opencode/access-monitor-validated` exited successfully. Some test results
  were served from Go's test cache. Config, domain and main have no tests by request.
- Compose `config --quiet` succeeded without exposing rendered environment values.
- `/livez` returned `ok`; `/readyz` returned `ready`. These are not delivery probes.
- Snapshot `650b34cbf67641ccee041e44580c15d5`, observed at
  `2026-11-04T16:42:22.716779117Z`: expected users 8; stdout records 8; file records 8;
  indexed records 8; matching user sets and stable Elasticsearch document IDs.
- Elasticsearch `grants` mapping is `nested`.
- Filebeat stats: 130 acknowledged, 0 output failures, 0 dropped, 0 active pipeline
  events and empty queue. Counters describe that process lifetime, not future health.
- Grafana data-source health checks: Prometheus, access history and operational logs
  all HTTP 200/OK. All 15 data-panel queries succeeded after applying dashboard
  variable/time interpolation. Raw access queries were tested with All and a
  nonexistent username: records for All, empty results for the nonexistent user.
  Default matching query returned 102 records; all-permissions query returned 136
  within the selected six-hour range at verification time.
- Live metrics showed 16 successful and 2 failed collections, followed by recovery.
  Earlier failures were logged as Kubernetes roles `transport_error`; the underlying
  transport cause is not established. Output counters showed 130 successful writes
  for each sink at inspection time.

## Remaining gaps and limitations

1. **Submission shape:** split dashboard exports and a dedicated permissions table
   remain absent. The review documents them rather than silently changing features.
2. **Query coverage:** resource/verb/namespace dropdowns use policy labels, not the
   full grant inventory. Matching handles exact resource and `*`, but not Kubernetes
   `*/subresource` wildcard expansion. Name-restricted grants require interpretation;
   matching a recorded rule is not a live authorization decision. Raw quoted variable
   interpolation was not adversarially tested for arbitrary special characters.
3. **Semantic regression coverage:** domain tests were removed. Existing service,
   metrics, transport, scheduling and logger tests do not exhaustively prove resolver
   correctness. No claim of exhaustive RBAC edge-case validation is made.
4. **Recovery:** forced rotation/renamed-file harvesting, prolonged output outages,
   registry replay, duplicate replay and loss/restart fault scenarios were not freshly
   exercised end to end. Unit tests are not substitutes for those scenarios.
5. **Observability:** no Filebeat/Elasticsearch Prometheus exporters, full-stack alerts,
   acknowledgement timestamp or exact pending-snapshot metric. The Elasticsearch
   stat panel counts records; it does not measure cluster health or shipping freshness.
6. **Lifecycle:** Fx's global stop timeout remains its default; arbitrary configured
   shutdown budgets are not propagated to that global deadline. Blocking synchronous
   output can exceed the collection work budget.
7. **Deployment:** plugin downloads on a fresh volume depend on network access;
   successful validation used the persistent installed plugin copies. No fresh-volume
   reinstall, production hardening certification or cluster version-skew certification
   was performed. Elasticsearch retention is manual, not automatic expiry.
8. **Identity/privacy:** supplied groups are trusted input, not discovered membership;
   no automatic `system:authenticated` group is invented. Actual inventory now resides
   in ignored `.env`, but earlier Git history contains inventory data; this review did
   not rewrite history. Review sharing permissions before submission.

The main collection → output → Filebeat → Elasticsearch → Grafana path is verified
for the sampled live snapshots. These results do not waive the listed task gaps or
establish continuous delivery, effective authorization, or production readiness.
