# Verified BitRiver appliance UI captures

These three PNGs were captured from the **actual control UI hosted by a
disposable Fedora bootc BitRiver Studio OS guest** running under
QEMU q35 with OVMF UEFI. A loopback-only SSH tunnel reached
the guest's real local API and a headless Chrome browser captured its
operator login, authenticated dashboard and source list.

- Source revision: 5b1e6ebc37f99ba2c5d29b90ccccbb707a552eb4
- Build run: https://github.com/ProhibitedTV/BitRiver-Studio-OS/actions/runs/44235484169
- Test-only guest: no physical capture card, real streaming or graphical
  Wayland/OBS session was certified
- No operator keys, test SSH keys or CI QCOW2 images are committed

These are **real running-appliance web UI** screenshots, not screenshots
of the guest's display or a physical/VirtualBox graphics certification.
