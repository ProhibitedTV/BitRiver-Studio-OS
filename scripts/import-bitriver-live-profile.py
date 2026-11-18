#!/usr/bin/env python3
"""Prepare a BitRiver-Live publish destination without storing its secret in
Studio OS settings or outputting the key to the terminal.

This deliberately prepares a config fragment; it never starts broadcasting,
changes an active configuration, fetches URLs or logs into a creator account.
"""
from __future__ import annotations

import argparse
import getpass
import json
import os
from pathlib import Path
import re
import stat
import sys
from urllib.parse import urlsplit

SCHEMA = "bitriver.studio.ingest/v1"
SECRET_DIR = Path("/var/lib/bitriver/secrets/stream-keys")
DESTINATION_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$")
LIVE_KEY = re.compile(r"^[0-9a-fA-F]{48}$")
HEX_CHANNEL = re.compile(r"^[0-9a-fA-F]{32}$")


def profile_from_bytes(content: bytes) -> dict:
    if len(content) > 8192:
        raise ValueError("ingest profile exceeds the 8 KiB limit")
    try:
        profile = json.loads(content)
    except (ValueError, UnicodeDecodeError) as exc:
        raise ValueError("ingest profile must be UTF-8 JSON") from exc
    if not isinstance(profile, dict) or set(profile) != {
        "schemaVersion", "provider", "channelId", "rtmpServer", "viewerUrl"
    }:
        raise ValueError("invalid ingest-profile fields; secrets do not belong in exported profiles")
    if profile["schemaVersion"] != SCHEMA or profile["provider"] != "bitriver-live":
        raise ValueError("unsupported ingest-profile provider or schema")
    if not isinstance(profile["channelId"], str) or not HEX_CHANNEL.fullmatch(profile["channelId"]):
        raise ValueError("invalid BitRiver-Live channel id")
    endpoint = profile["rtmpServer"]
    if not isinstance(endpoint, str) or any(char in endpoint for char in "|[]?#\\ \r\n\t"):
        raise ValueError("invalid RTMP ingest endpoint")
    u = urlsplit(endpoint)
    if u.scheme not in ("rtmp", "rtmps") or not u.hostname or u.username or u.password or u.path != "/live":
        raise ValueError("expected a credential-free BitRiver-Live RTMP(S) server ending in /live")
    if not isinstance(profile["viewerUrl"], str) or len(profile["viewerUrl"]) > 2048:
        raise ValueError("invalid viewer URL")
    viewer = urlsplit(profile["viewerUrl"])
    if viewer.scheme not in ("http", "https") or not viewer.hostname or viewer.username or viewer.password or viewer.query or viewer.fragment:
        raise ValueError("invalid BitRiver-Live viewer URL")
    return profile


def destination_for(profile: dict, destination_id: str) -> dict:
    if not DESTINATION_ID.fullmatch(destination_id):
        raise ValueError("destination ID must be 1-64 alphanumeric characters, underscores or hyphens")
    return {
        "id": destination_id,
        "name": "BitRiver Live (" + profile["channelId"][:8] + ")",
        "protocol": "rtmp",
        "targetUrl": profile["rtmpServer"],
        "streamKeyFile": str(SECRET_DIR / destination_id),
        "enabled": False,
    }


def write_secret(directory: Path, destination_id: str, key: str) -> None:
    if not LIVE_KEY.fullmatch(key):
        raise ValueError("BitRiver-Live keys must be exactly 48 hexadecimal characters")
    if directory.is_symlink() or not directory.is_dir() or directory.stat().st_mode & 0o077:
        raise ValueError("stream-key directory must exist, not be a symlink, and be private (0700)")
    file = directory / destination_id
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL
    if hasattr(os, "O_NOFOLLOW"):
        flags |= os.O_NOFOLLOW
    fd = os.open(file, flags, 0o600)
    try:
        os.fchmod(fd, stat.S_IRUSR | stat.S_IWUSR)
        with os.fdopen(fd, "w", encoding="ascii") as output:
            output.write(key + "\n")
            output.flush()
            os.fsync(output.fileno())
    except BaseException:
        try:
            os.close(fd)
        except OSError:
            pass
        file.unlink(missing_ok=True)
        raise


def main() -> int:
    parser = argparse.ArgumentParser(description="Prepare an opt-in BitRiver-Live RTMP destination")
    parser.add_argument("--profile", required=True, type=Path, help="key-free Studio OS profile exported from the creator page")
    parser.add_argument("--id", default="bitriver-live", help="unique destination ID; separate names for multiple Live instances")
    parser.add_argument("--key-stdin", action="store_true", help="read stream key from stdin (avoid command-line arguments)")
    parser.add_argument("--dry-run", action="store_true", help="validate and print only a key-free destination fragment")
    args = parser.parse_args()
    try:
        profile = profile_from_bytes(args.profile.read_bytes())
        destination = destination_for(profile, args.id)
        if not args.dry_run:
            if os.geteuid() == 0:
                raise ValueError("run as the unprivileged bitriver service user, never root")
            secret = sys.stdin.readline().strip() if args.key_stdin else getpass.getpass("BitRiver-Live stream key (not echoed): ")
            if not LIVE_KEY.fullmatch(secret):
                raise ValueError("invalid BitRiver-Live stream key format (expected 48 hexadecimal characters)")
            write_secret(SECRET_DIR, args.id, secret)
            secret = ""
        print(json.dumps(destination, indent=2))
        print("\nDestination is DISABLED and has NOT been added to active settings.", file=sys.stderr)
        print("Review it, add to mediaRouter.publishDestinations in the draft settings, then explicitly validate/apply.", file=sys.stderr)
        return 0
    except (OSError, ValueError) as exc:
        print(f"Cannot prepare BitRiver-Live destination: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
