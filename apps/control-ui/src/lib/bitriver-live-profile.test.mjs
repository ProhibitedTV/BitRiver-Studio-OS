import assert from "node:assert/strict";
import test from "node:test";
import { destinationForLiveProfile, parseBitRiverLiveProfile } from "./bitriver-live-profile.ts";

const profile = {
  schemaVersion: "bitriver.studio.ingest/v1",
  provider: "bitriver-live",
  channelId: "0123456789abcdef0123456789abcdef",
  rtmpServer: "rtmp://ingest.example.test:1935/live",
  viewerUrl: "https://watch.example.test/viewer/channel"
};

test("imports BitRiver-Live public metadata without enabling anything", () => {
  const parsed = parseBitRiverLiveProfile(JSON.stringify(profile));
  assert.deepEqual(parsed, profile);
  const destination = destinationForLiveProfile(parsed);
  assert.deepEqual(destination, {
    id: "live-0123456789ab",
    name: "BitRiver Live (01234567)",
    protocol: "rtmp",
    targetUrl: "rtmp://ingest.example.test:1935/live",
    streamKeyFile: "/var/lib/bitriver/secrets/stream-keys/live-0123456789ab",
    enabled: false
  });
  assert.equal(Object.hasOwn(destination, "streamKey"), false);
  assert.equal(Object.hasOwn(parsed, "streamKey"), false);
});

test("rejects unexpected fields including private stream keys", () => {
  for (const field of ["streamKey", "token", "password", "cookie", "metadata"]) {
    assert.throws(
      () => parseBitRiverLiveProfile(JSON.stringify({ ...profile, [field]: "secret" })),
      /Unexpected profile fields/
    );
  }
});

test("rejects URLs containing credentials, query parameters or unsafe delimiters", () => {
  const unsafeIngest = [
    "rtmp://ingest.example.test:1935/live/ABCDEF",
    "rtmp://user:secret@ingest.example.test/live",
    "rtmp://ingest.example.test/live?token=secret",
    "rtmp://ingest.example.test/live#stream",
    "rtmp://ingest.example.test/live|evil",
    "http://ingest.example.test/live",
    "rtmp://ingest.example.test/other"
  ];
  for (const rtmpServer of unsafeIngest) {
    assert.throws(() => parseBitRiverLiveProfile(JSON.stringify({ ...profile, rtmpServer })), Error, rtmpServer);
  }
  for (const viewerUrl of ["https://user:pass@watch.example.test/", "https://watch.example.test/?auth=hidden", "javascript:alert(1)"]) {
    assert.throws(() => parseBitRiverLiveProfile(JSON.stringify({ ...profile, viewerUrl })), Error, viewerUrl);
  }
});

test("rejects wrong schema, channel ID, malformed or oversized profile", () => {
  assert.throws(() => parseBitRiverLiveProfile("[]"));
  assert.throws(() => parseBitRiverLiveProfile("{not-json}"));
  assert.throws(() => parseBitRiverLiveProfile(JSON.stringify({ ...profile, schemaVersion: "v2" })));
  assert.throws(() => parseBitRiverLiveProfile(JSON.stringify({ ...profile, channelId: "../evil" })));
  assert.throws(() => parseBitRiverLiveProfile(" ".repeat(8193)));
});
