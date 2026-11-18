# Screenshot source and verification

## Real captures

Files under [`real/`](real/) are produced by
[`readme-live-captures.yml`](../../.github/workflows/readme-live-captures.yml).

They are screenshots of the **real React control UI served by a booted Fedora
bootc BitRiver Studio OS** under a disposable QEMU/OVMF virtual machine. The
capture script creates an SSH tunnel to the guest loopback API and authenticates
using a one-time CI guest operator key without publishing that key. The capture
run writes [`real/PROVENANCE.md`](real/PROVENANCE.md) with the exact commit/run.

This validates a running appliance's **web UI** and its API, not the virtual
machine's graphical desktop, a physical GPU, an OBS stream or a user installer.

- `real/operator-login.png`: local operator sign-in served by the guest
- `real/appliance-dashboard.png`: authenticated dashboard served by the guest
- `real/appliance-sources.png`: authenticated sources/ingest section

## Design placeholders

`control-dashboard-placeholder.svg`, `ingest-management-placeholder.svg`,
`update-workflow-placeholder.svg`, and the [mockups](../assets/mockups/) folder
are **illustrations**, not screenshots. Do not pass them off as runtime evidence.

Never commit operator keys, API session cookies, real user data, VM SSH keys,
test QCOW2 disks, or live stream secrets alongside images.
