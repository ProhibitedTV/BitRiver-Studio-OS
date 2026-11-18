import {
  controlApiRoutes,
  type AudioState,
  type ConfigEnvelope,
  type ControlConfig,
  type ValidateConfigResponse,
  type GainRequest,
  type OperationResponse,
  type ShowState,
  type SourceSummary,
  type StorageSummary,
  type SystemSummary
} from "@bitriver/shared";

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "";

let operatorCsrfToken = "";

export interface OperatorSession {
  enabled: boolean;
  authenticated: boolean;
  csrfToken?: string;
}

export function setOperatorCsrfToken(token: string) {
  operatorCsrfToken = token;
}

export async function getOperatorSession() {
  const session = await request<OperatorSession>("/api/v1/auth/session");
  setOperatorCsrfToken(session.csrfToken ?? "");
  return session;
}

export async function operatorLogin(key: string) {
  const session = await request<OperatorSession>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ key })
  });
  setOperatorCsrfToken(session.csrfToken ?? "");
  return session;
}

export async function operatorLogout() {
  const result = await request<{ authenticated: boolean }>("/api/v1/auth/logout", { method: "POST" });
  setOperatorCsrfToken("");
  return result;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${apiBaseUrl}${path}`, {
    ...init,
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      ...(init?.method && init.method.toUpperCase() !== "GET" && operatorCsrfToken ? { "X-BitRiver-CSRF": operatorCsrfToken } : {}),
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

// Draft-only calls. The API's existing operator cookie and CSRF protections
// apply; no streaming key is ever sent to these endpoints.
export function getDraftConfig() {
  return request<ConfigEnvelope>(controlApiRoutes.settingsDraft);
}

export function updateDraftConfig(config: ControlConfig, expectedGeneration: string) {
  return request<ConfigEnvelope>(controlApiRoutes.settingsDraft, {
    method: "PUT",
    headers: { "If-Match": JSON.stringify(expectedGeneration) },
    body: JSON.stringify(config)
  });
}

export function validateDraftConfig() {
  return request<ValidateConfigResponse>(controlApiRoutes.settingsValidate, { method: "POST" });
}

export function applyDraftConfig(expectedGeneration: string) {
  return request<OperationResponse>(controlApiRoutes.settingsApply, {
    method: "POST",
    headers: { "If-Match": JSON.stringify(expectedGeneration) }
  });
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
