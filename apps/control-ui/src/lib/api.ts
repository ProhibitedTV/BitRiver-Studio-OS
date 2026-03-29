import {
  controlApiRoutes,
  type AudioState,
  type ConfigEnvelope,
  type GainRequest,
  type OperationResponse,
  type ShowState,
  type SourceSummary,
  type StorageSummary,
  type SystemSummary
} from "@bitriver/shared";

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {})
    }
  });

  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || `Request failed with status ${response.status}`);
  }

  return response.json() as Promise<T>;
}

export function getSystem() {
  return request<SystemSummary>(controlApiRoutes.system);
}

export function getShow() {
  return request<ShowState>(controlApiRoutes.show);
}

export function getSources() {
  return request<SourceSummary[]>(controlApiRoutes.sources);
}

export function getAudio() {
  return request<AudioState>(controlApiRoutes.audio);
}

export function getStorage() {
  return request<StorageSummary>(controlApiRoutes.storage);
}

export function getActiveConfig() {
  return request<ConfigEnvelope>(controlApiRoutes.activeConfig);
}

export function activateScene(sceneId: string) {
  return request<OperationResponse>(`${controlApiRoutes.show}/scene/${sceneId}/activate`, {
    method: "POST"
  });
}

export function startStream() {
  return request<OperationResponse>(controlApiRoutes.streamStart, { method: "POST" });
}

export function stopStream() {
  return request<OperationResponse>(controlApiRoutes.streamStop, { method: "POST" });
}

export function startRecording() {
  return request<OperationResponse>(controlApiRoutes.recordingStart, { method: "POST" });
}

export function stopRecording() {
  return request<OperationResponse>(controlApiRoutes.recordingStop, { method: "POST" });
}

export function muteChannel(channelId: string) {
  return request<OperationResponse>(`${controlApiRoutes.audio}/${channelId}/mute`, {
    method: "POST"
  });
}

export function unmuteChannel(channelId: string) {
  return request<OperationResponse>(`${controlApiRoutes.audio}/${channelId}/unmute`, {
    method: "POST"
  });
}

export function setChannelGain(channelId: string, gainDb: number) {
  const body: GainRequest = { gainDb };
  return request<OperationResponse>(`${controlApiRoutes.audio}/${channelId}/gain`, {
    method: "POST",
    body: JSON.stringify(body)
  });
}
