# new-api-control certificate mount

Do not commit private keys or production certificates. At runtime this directory
must contain:

- `tls.crt`: server certificate for the hostname used by Integration;
- `tls.key`: its private key;
- `client-ca.crt`: the dedicated CA that issues only the Integration client
  certificate.

For local Kind integration, also place the following files in this directory.
They are ignored by Git and are copied into a Kubernetes Secret by the
`k8s-deploy/local/scripts/create-runtime-secrets.ps1` script:

- `server-ca.crt`: CA that issued `tls.crt`;
- `integration.crt`: Integration client certificate;
- `integration.key`: Integration client private key.

The local server certificate must contain the DNS SAN `new-api-control.local`.
The client certificate must contain the URI SAN shown below. Use separate
server and client CAs so trust in either direction stays narrowly scoped.

The Integration certificate identity must match
`NEW_API_CONTROL_ALLOWED_CLIENT_ID`. The default Compose value expects the URI
SAN `spiffe://beyondia.internal/new-api-integration`.
