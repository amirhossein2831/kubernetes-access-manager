# Decisions and tradeoffs

The goal was a small, explainable monitoring system—not a second authorization
server. These are the choices that shaped it.

## 1. Inspect RBAC instead of probing as users

**Decision:** read roles, bindings, namespace metadata and API discovery; resolve
grants locally.

**Why:** the task is about RBAC visibility. Read-only inspection avoids impersonation
privileges, authorization probes and workload/Secret-content reads. Provenance makes
the answer explainable.

**Tradeoff:** this is not an effective-access oracle. External authorizers,
authentication context, admission and changes between reads can alter real outcomes.

## 2. Treat supplied groups as the identity boundary

**Decision:** monitor only supplied usernames and group memberships; do not add
`system:authenticated` or discover users through an identity provider.

**Why:** Kubernetes RBAC does not provide a canonical user directory, and the task
supplies the inventory to monitor.

**Tradeoff:** incorrect or stale memberships produce an incomplete picture of actual
access. The output identifies its basis as supplied input subjects.

## 3. Use Filebeat rather than build a delivery worker

**Decision:** Go writes JSON; Filebeat ships it to Elasticsearch.

**Why:** retrying, harvesting rotated files and remembering progress are established
log-shipper responsibilities. This keeps cluster analysis separate from delivery
and avoids a custom Elasticsearch client/queue inside the collector.

**Tradeoff:** another process and persistent registry must be operated. Delivery is
not exactly once; stable document IDs deduplicate replay of the same event. Filebeat
cannot recover records never written or already rotated away during a long outage.
Stdout is a separate output, not an automatic shipping fallback.

## 4. Separate detailed history from aggregate metrics

**Decision:** Elasticsearch stores user grants; Prometheus stores distinct-user
exposure counts and operational metrics.

**Why:** searchable records fit historical investigation. Bounded metric labels fit
time-series monitoring without turning every user/rule into a metric series.

**Tradeoff:** the two views have different delivery timing. Username filters cannot
filter aggregate counts. Required metric labels omit API groups, so the sensitive
policy is limited to core resources.

## 5. Poll every five minutes, rather than maintain watches

**Decision:** perform a full collection immediately and periodically, with no
overlapping cycles and no catch-up bursts.

**Why:** it matches the task interval and makes recovery and reasoning straightforward.
Bounded requests, pagination and retries limit pressure on the API server.

**Tradeoff:** short-lived changes can be missed and full reads cost more as RBAC grows.
Watches could reduce latency, but introduce reconnect, cache and resynchronization
complexity. Separate lists are not a transactionally consistent snapshot.

## 6. Prefer a failed observation to false confidence

**Decision:** unresolved relevant references or unknown scope invalidate collection;
retain the last successful counts and timestamp.

**Why:** partial data could make exposure appear to disappear. Visible staleness is
more honest than a reassuring zero.

**Tradeoff:** a discovery problem can prevent fresh results even for otherwise
resolvable users. Operators must monitor freshness, not only process availability.
Metrics update before independent synchronous output attempts, so metric success and
record delivery can diverge; slow output can also delay the scheduler.

## 7. Preserve rich grants instead of flattening them

**Decision:** retain API group, scope, subresource, verbs, name restrictions and
provenance; index resource grants as nested objects.

**Why:** flattening can invent permissions by combining unrelated rules. Original
evidence is more valuable than a compact but ambiguous summary.

**Tradeoff:** larger records and more complex queries. Non-resource rules and source
provenance remain unindexed objects; the dashboard reads non-resource rules from
source rather than offering equivalent indexed investigation.

## 8. Show one complete snapshot, not a union of history

**Decision:** permission tables select one indexed complete snapshot, for one or
all users. Historical resource/verb matching is a separate time-range query.

**Why:** merging snapshots could make revoked and current grants look simultaneous.
All-users tables retain usernames to avoid merging different people's permissions.

**Tradeoff:** newest-first is not automatic “follow latest.” Completeness validation
lives in the dropdown and can be bypassed by URL selections. Candidate, bucket and
record limits mean the dashboard is not an unbounded export tool.

## 9. Keep the local stack reproducible, without calling it production-ready

**Decision:** Compose, pinned images/plugins, local Elasticsearch TLS, scoped users,
file-backed secrets and persistent volumes.

**Why:** reviewers can run the same architecture without a separate platform or
unnecessary application services.

**Tradeoff:** Docker administrators can inspect secrets and environment values;
single-node persistence is not backup or high availability. Plugin downloads need
network access on a fresh installation, and Elasticsearch expiry is manual.

## What we would improve next

1. Add domain regression coverage and automated cross-service tests for outage,
   replay, rotation and partial indexing. Current tests are not exhaustive.
2. Add shipping/Elasticsearch telemetry and freshness alerts; there are no exporters
   or full-stack alerts today.
3. Automate Elasticsearch lifecycle/retention and define tested backup/recovery.
4. Enforce selected-snapshot completeness at every table query, improve truncation
   visibility, and support larger inventories without bounded dropdown assumptions.
5. Revisit output backpressure, shutdown deadlines and collection strategy using
   measured load rather than treating the local demonstration as an outage SLA.
