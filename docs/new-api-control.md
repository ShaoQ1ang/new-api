# IAM control plane

`new-api-control` is a separate process and container. It is the only supported
entry point for IAM-managed users and API keys. The public new-api HTTP session
and administrator endpoints are deliberately not reused.

## Security boundary

- TLS 1.3 and a verified client certificate are mandatory. The client
  certificate must also match `NEW_API_CONTROL_ALLOWED_CLIENT_ID` by URI SAN,
  DNS SAN, or Common Name.
- Use a dedicated client CA whose only leaf is the cluster-side
  `new-api-integration` workload. Do not reuse a broad corporate or public CA.
- Restrict TCP/3010 at the host firewall to the cluster egress addresses. mTLS
  remains mandatory even on a private network.
- The service never logs API-key secrets. Responses use `Cache-Control:
  no-store`, request bodies are limited to 64 KiB, and server timeouts are
  bounded.
- The supplied Compose service runs as UID 65532 with a read-only root
  filesystem, no additional privileges or Linux capabilities, a PID limit,
  and a dedicated in-memory writable log directory.
- Run with `NODE_TYPE=slave`. Schema migration remains owned by the normal
  new-api process, preventing two containers from racing over migrations.

## Consistency and concurrency

`iam_identity_links` and `iam_api_key_links` keep stable IAM-to-new-api
mappings, last-applied lifecycle versions and event IDs. Mutations run in a
database transaction and lock the mapping row with `FOR UPDATE` on MySQL and
PostgreSQL. SQLite relies on its single-writer conflict behavior.

Older events are ignored, exact retries are idempotent, and conflicting events
at the same version fail closed. Identity disable immediately disables managed
tokens; re-enable restores only keys whose desired state is still `ACTIVE`.
Identity deletion permanently revokes all linked managed keys while retaining
the mapping tombstones for audit and late-event rejection. An API key that has
reached `REVOKED` cannot be recreated by replaying a create request at either an
older or current version.

## Database portability

The two link tables are migrated with GORM alongside the existing models and
use types supported by SQLite, MySQL, and PostgreSQL. Existing `users` and
`tokens` gain a `management_source` column. The reserved username prefix
`iam_` prevents local users from colliding with deterministic IAM usernames.
The Token model also rejects local create/update/delete and full-key export for
IAM-managed resources, so legacy UI or administrator routes cannot bypass the
control plane. Runtime quota/accounting updates remain available.

## Endpoints

All business endpoints live below `/internal/v1`:

- `POST /internal/v1/identities/apply`
- `POST /internal/v1/api-keys/create`
- `POST /internal/v1/api-keys/revoke`
- `GET /internal/v1/api-keys/{id}`
- `GET /internal/v1/chat-models`

Health endpoints are `/health/live` and `/health/ready`; when called over the
network they are protected by the same mTLS listener.
