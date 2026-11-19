#!/usr/bin/bash
# Live ISO diagnostics only: no disks, credentials, external network or writes.
# Emit fixed-status fields; never dump browser arguments, URLs or stderr.
set -u
browser_uid="$(id -u bitriver-installer 2>/dev/null || printf 'missing')"
for sample in $(seq 1 16); do
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
  browser_log_errors=unavailable
  if [[ -r /tmp/webui-browser.log ]]; then
    browser_log_errors="$(grep -Eic 'error|failed|denied|sandbox' \
      /tmp/webui-browser.log 2>/dev/null || true)"
    [[ -n "$browser_log_errors" ]] || browser_log_errors=0
  fi
  printf 'BITRIVER_ISO_PROBE sample=%s firefox_unprivileged=%s xwayland=%s cockpit_http=%s browser_error_count=%s\n' \
    "$sample" "$firefox" "$xwayland" "$http_code" "$browser_log_errors"
  sleep 20
done
