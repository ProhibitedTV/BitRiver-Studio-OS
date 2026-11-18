import type { ConfigMediaDestination } from "@bitriver/shared";

export interface BitRiverLiveProfile {
  schemaVersion: "bitriver.studio.ingest/v1";
  provider: "bitriver-live";
  channelId: string;
  rtmpServer: string;
  viewerUrl: string;
}

const expected = ["schemaVersion", "provider", "channelId", "rtmpServer", "viewerUrl"];
const idPattern = /^[a-fA-F0-9]{32}$/;
const managedSecretsDir = "/var/lib/bitriver/secrets/stream-keys/";

// Parse an explicitly selected LOCAL download. Never fetch a remote URL or
// accept unknown fields: a stream key / session token is not a profile field.
export function parseBitRiverLiveProfile(raw: string): BitRiverLiveProfile {
  if (new TextEncoder().encode(raw).length > 8192) {
    throw new Error("Ingest profile exceeds the 8 KiB size limit.");
  }
  let candidate: unknown;
  try {
    candidate = JSON.parse(raw);
  } catch {
    throw new Error("Select a valid JSON profile exported by BitRiver-Live.");
  }
  if (!candidate || typeof candidate !== "object" || Array.isArray(candidate)) {
    throw new Error("Profile must be a JSON object.");
  }
  const data = candidate as Record<string, unknown>;
  if (
    Object.keys(data).length !== expected.length ||
    !expected.every((key) => Object.prototype.hasOwnProperty.call(data, key)) ||
    data.schemaVersion !== "bitriver.studio.ingest/v1" ||
    data.provider !== "bitriver-live" ||
    typeof data.channelId !== "string" || !idPattern.test(data.channelId) ||
    typeof data.rtmpServer !== "string" || typeof data.viewerUrl !== "string"
  ) {
    throw new Error("Unexpected profile fields or version. Export again from your BitRiver-Live creator page.");
  }
  if (/[|\[\]#?\\\s]/.test(data.rtmpServer)) {
    throw new Error("RTMP server contains an invalid delimiter or credential.");
  }
  let ingest: URL;
  let viewer: URL;
  try {
    ingest = new URL(data.rtmpServer);
    viewer = new URL(data.viewerUrl);
  } catch {
    throw new Error("Invalid ingest server or viewer URL.");
  }
  if (
    !["rtmp:", "rtmps:"].includes(ingest.protocol) || !ingest.hostname ||
    ingest.username || ingest.password || ingest.pathname !== "/live" ||
    ingest.search || ingest.hash
  ) {
    throw new Error("An RTMP(S) server ending in /live without credentials is required.");
  }
  if (
    !["https:", "http:"].includes(viewer.protocol) || !viewer.hostname ||
    viewer.username || viewer.password || viewer.search || viewer.hash ||
    data.viewerUrl.length > 2048
  ) {
    throw new Error("The viewer URL cannot include credentials or query parameters.");
  }
  return data as unknown as BitRiverLiveProfile;
}

export function destinationForLiveProfile(profile: BitRiverLiveProfile): ConfigMediaDestination {
  const id = "live-" + profile.channelId.slice(0, 12).toLowerCase();
  return {
    id,
    name: "BitRiver Live (" + profile.channelId.slice(0, 8) + ")",
    protocol: "rtmp",
    targetUrl: profile.rtmpServer,
    streamKeyFile: managedSecretsDir + id,
    enabled: false
  };
}
