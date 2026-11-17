# Operator authentication (pre-release)

The appliance control plane now requires a local operator session. This is a
**first security layer**, not a claim that remote multi-user administration,
TLS, unattended remote recovery or a production-ready first-boot claim flow are complete.

## Appliance boot

The first-boot script generates a random 128-bit control key and stores only
its SHA-256 digest as controlKeySha256 in the root-owned/bootstrap-managed
admin-credentials.json record (alongside the legacy password hash). The
**plaintext operator key** is written to a mode-0600 local recovery notice:

    /var/lib/bitriver/bootstrap/one-time-admin-password.txt

Access this notice from a trusted local shell with appropriate administrator
privileges. It is never returned by the API. Existing pre-release records
without a control key receive a **new key** from the first-boot migration.
A valid existing key persists across reboot. Keep recovery notices private.

The systemd control API service passes the credentials path and enables the
appliance adapter. Startup fails if the key digest is missing or invalid.

## Rotate a compromised operator key

On the appliance's **trusted local shell**, run:

    sudo /usr/local/bin/bitriver-operator-key rotate

The command generates a fresh 256-bit random key, atomically replaces only the
key digest in the existing credentials file, preserves the file's owner and
private permissions, and prints the **new key once** to local standard output.
Do not run it from CI logs, shared terminals, or session-recording software.
Save the new key in a trusted password manager; it cannot be recovered from
the stored digest.

The old first-boot notice is deleted after successful rotation because it
contains the revoked key. If cleanup fails, the command prints a warning;
delete that obsolete notice manually. If the new key is lost, run the command
locally again. This requires administrative access to the appliance.

The running control API re-reads the digest for every authenticated request:
existing sessions are revoked as soon as the new digest is observed; the
previous key no longer permits login. A missing or corrupt credentials file
causes the API to **fail closed** with HTTP 503 rather than falling back to
the old key. A service restart is not required to activate the new key.

This is **offline recovery**, not web-based password change, user claiming,
or cloud account recovery. It is intended only for trusted local operators.

## Sign in and protected routes

The local Chromium kiosk opens http://127.0.0.1:8080/ and shows the operator
key entry screen. After sign in:

- The server sets a random, opaque, HttpOnly, SameSite=Strict session cookie.
- Sessions expire after eight hours (or sooner on logout/restart).
- Mutating API routes require an independent X-BitRiver-CSRF token.
- The API checks request Origin when present, including WebSocket upgrades.
- All /api/ endpoints except the login and session-status routes are private.
- /healthz stays public for the local health supervisor.
- Eight failed logins per minute are throttled globally.
- Session secrets are kept in server memory, never localStorage.

API endpoints:

    GET  /api/v1/auth/session
    POST /api/v1/auth/login   {"key":"the-control-key"}
    POST /api/v1/auth/logout  X-BitRiver-CSRF: <session token>

Development mode without a credentials file remains intentionally open for
localhost-only smoke tests. **Do not expose development mode over a network.**
The bitriverd CLI refuses non-loopback HTTP bindings for both modes.

## Access from another machine (temporary development workflow)

The appliance binds **127.0.0.1:8080**, not 0.0.0.0. Remote LAN access via
raw HTTP is disabled because it would expose the operator key and session
cookies in transit. For a trusted development workstation with secure shell
access, use an SSH tunnel:

    ssh -L 18080:127.0.0.1:8080 <appliance-admin>@<appliance-ip>

Then browse to http://127.0.0.1:18080/ on the developer machine. The tunnel
protects the TCP hop, but this is only an interim workflow; direct browser
access from arbitrary LAN clients still needs an HTTPS front end and an
explicit trust boundary before an official release.

## Remaining release blockers

- Web-based key change, a secure first-boot claim flow, and full recovery UX.
- Audit local key rotation on real hardware, including backup and file owner handling.
- Production HTTPS certificate management and secure cookie policy across
  a trusted TLS termination boundary.
- Session rate limits across restart and security audit of origin/Host policies.
- Authenticated media ingest and OBS/media router service exposure.
- Physical-machine security review, image rollback, and full boot tests.

Track this work in issue #3. A passing CI run does **not** close that issue.
