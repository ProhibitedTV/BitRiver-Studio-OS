#!/usr/bin/env bash
set -euo pipefail

DESTINATIONS_PATH="${1:?destinations sidecar path is required}"
SOURCE_URL="${2:?source url is required}"

if ! command -v jq >/dev/null 2>&1 || ! command -v ffmpeg >/dev/null 2>&1; then
  echo "jq and ffmpeg are required for media restream" >&2; exit 1
fi
if [[ ! -f "$DESTINATIONS_PATH" ]]; then
  echo "media destinations sidecar missing" >&2; exit 1
fi

OUTPUTS=()
has_private_key=0
while IFS= read -r destination; do
  [[ -n "$destination" ]] || continue
  id="$(jq -er '.id' <<< "$destination")"
  protocol="$(jq -er '.protocol' <<< "$destination")"
  target="$(jq -er '.targetUrl' <<< "$destination")"
  stream_key_file="$(jq -r '.streamKeyFile // ""' <<< "$destination")"
  if [[ -n "$stream_key_file" ]]; then
    # Keep the key off the versioned config API and MediaMTX sidecar.
    # No arbitrary read paths, symlinks, or shell/tee metacharacters.
    if [[ ! "$id" =~ ^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$ ||
          "$protocol" != "rtmp" ||
          "$stream_key_file" != "/var/lib/bitriver/secrets/stream-keys/$id" ||
          -L "$stream_key_file" || ! -f "$stream_key_file" || ! -r "$stream_key_file" ]]; then
      echo "RTMP credential reference is invalid or unreadable for destination $id" >&2; exit 1
    fi
    mode="$(stat -c '%a' "$stream_key_file")"
    if [[ "$mode" != "600" ]]; then
      echo "RTMP credential must have file permissions 0600 for destination $id" >&2; exit 1
    fi
    IFS= read -r secret < "$stream_key_file" || [[ -n "${secret:-}" ]]
    if [[ ! "${secret:-}" =~ ^[A-Fa-f0-9]{48}$ ]]; then
      echo "RTMP credential has an invalid BitRiver-Live key format for destination $id" >&2; exit 1
    fi
    if [[ ! "$target" =~ ^rtmps?://[^[:space:]\|\[\]?#]+/[^[:space:]\|\[\]?#]+$ ]]; then
      echo "Invalid RTMP server URL for destination $id" >&2; exit 1
    fi
    target="${target%/}/$secret"
    has_private_key=1
    unset secret
  fi
  case "$protocol" in
    rtmp) OUTPUTS+=("[f=flv:onfail=ignore]$target");;
    srt) OUTPUTS+=("[f=mpegts:onfail=ignore]$target");;
  esac
done < <(jq -c '.publishDestinations[] | select(.enabled == true)' "$DESTINATIONS_PATH")

if [[ "${#OUTPUTS[@]}" -eq 0 ]]; then
  exec sleep infinity
fi

TEE_TARGETS="$(printf '%s|' "${OUTPUTS[@]}")"
TEE_TARGETS="${TEE_TARGETS%|}"

# ffmpeg currently needs publish URLs on its process argv. Until a private
# output worker removes that exposure, do not run this on untrusted multi-user
# machines or claim stream-key isolation from local process inspection.
# Avoid printing credential-bearing ffmpeg diagnostics to the system journal.
if [[ "$has_private_key" -eq 1 ]]; then
  exec ffmpeg -nostdin -hide_banner -loglevel quiet -fflags +genpts \
    -i "$SOURCE_URL" -map 0:v? -map 0:a? -c copy -f tee "$TEE_TARGETS"
fi
exec ffmpeg -nostdin -hide_banner -loglevel warning -fflags +genpts \
  -i "$SOURCE_URL" -map 0:v? -map 0:a? -c copy -f tee "$TEE_TARGETS"
