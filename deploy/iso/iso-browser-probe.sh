#!/usr/bin/bash
# Live ISO diagnostics only: no disks, credentials, external network or writes.
# Emit fixed-status fields; never dump browser arguments, URLs or stderr.
set -u
browser_uid="$(id -u bitriver-installer 2>/dev/null || printf 'missing')"
# The old Fedora 44 WebUI helper places its Firefox profile under
# /run/user/0, while the browser itself runs as bitriver-installer.
# A root-only parent or inaccessible X display is a plausible blank-screen
# cause. Detect access failures; do NOT relax directory or X11 permissions.
for sample in $(seq 1 22); do
  firefox=no
  xwayland=no
  for proc in /proc/[0-9]*; do
    [[ -r "$proc/comm" ]] || continue
    proc_name=""
    IFS= read -r proc_name < "$proc/comm" || true
    [[ "$proc_name" == Xwayland ]] && xwayland=yes
    if [[ "$proc_name" == firefox* && "$browser_uid" != missing ]]; then
      uid="$(stat -c %u "$proc" 2>/dev/null || printf unknown)"
      [[ "$uid" == "$browser_uid" ]] && firefox=yes
    fi
  done
  http_code=unavailable
  if command -v curl >/dev/null 2>&1; then
    http_code="$(curl --noproxy '*' -m 2 -s -o /dev/null -w '%{http_code}' \
      http://127.0.0.1/cockpit/@localhost/anaconda-webui/index.html 2>/dev/null || printf 'unavailable')"
  fi
  # Query Xwayland using the same unprivileged account as Firefox.
  # Do not emit window names, titles, cookies, environment or raw output.
  x11_connectable=unavailable
  x11_windows=unavailable
  profile_traversable=unavailable
  root_runtime_mode=unavailable
  if [[ "$browser_uid" != missing ]] && command -v runuser >/dev/null 2>&1; then
    if [[ -d /run/user/0 ]]; then
      root_runtime_mode="$(stat -c %a /run/user/0 2>/dev/null || printf unavailable)"
    fi
    profile_traversable=no
    if runuser -u bitriver-installer -- test -x /run/user/0/anaconda/firefox-profile 2>/dev/null; then
      profile_traversable=yes
    fi
    if command -v xdpyinfo >/dev/null 2>&1; then
      x11_connectable=no
      if timeout 3 runuser -u bitriver-installer -- env DISPLAY=:1 xdpyinfo >/dev/null 2>&1; then
        x11_connectable=yes
        if command -v xwininfo >/dev/null 2>&1; then
          # Count is derived in-memory; full window info is never logged.
          windows="$(timeout 3 runuser -u bitriver-installer -- env DISPLAY=:1 xwininfo -root -tree 2>/dev/null || true)"
          count="$(printf '%s\n' "$windows" | sed -nE 's/^[[:space:]]*([0-9]+) children:.*/\1/p' | head -n 1)"
          if [[ "$count" =~ ^[0-9]+$ ]]; then x11_windows="$count"; fi
          unset windows count
        fi
      fi
    fi
  fi
  browser_log_errors=unavailable
  if [[ -r /tmp/webui-browser.log ]]; then
    browser_log_errors="$(grep -Eic 'error|failed|denied|sandbox' \
      /tmp/webui-browser.log 2>/dev/null || true)"
    [[ -n "$browser_log_errors" ]] || browser_log_errors=0
  fi
  printf 'BITRIVER_ISO_PROBE sample=%s firefox_unprivileged=%s xwayland=%s cockpit_http=%s browser_error_count=%s x11_connectable=%s x11_windows=%s root_runtime_mode=%s firefox_profile_traversable=%s\n' \
    "$sample" "$firefox" "$xwayland" "$http_code" "$browser_log_errors" \
    "$x11_connectable" "$x11_windows" "$root_runtime_mode" "$profile_traversable"
  sleep 20
done
