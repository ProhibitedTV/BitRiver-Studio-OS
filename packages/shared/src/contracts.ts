export const API_PREFIX = "/api/v1";

export type HealthState = "healthy" | "degraded" | "error";
export type StreamState = "stopped" | "starting" | "live";
export type RecordState = "stopped" | "recording";
export type SourceStatus = "ready" | "disconnected" | "warning";
export type OperationStatus = "accepted" | "completed" | "rejected";
export type DestinationMode = "external_rtmp" | "owncast";
export type SourceKind = "local_video" | "local_audio" | "network_ingest" | "browser_source";

export interface HealthCheck {
  key: string;
  state: HealthState;
  message: string;
}

export interface HealthSummary {
  overall: HealthState;
  checks: HealthCheck[];
}

export interface NetworkSummary {
  lanAddress: string;
  hostname: string;
}

export interface SystemSummary {
  applianceId: string;
  name: string;
  version: string;
  bootMode: string;
  startedAt: string;
  uptimeSeconds: number;
  health: HealthSummary;
  network: NetworkSummary;
}

export interface DestinationSummary {
  mode: DestinationMode;
  label: string;
  target: string;
  connected: boolean;
}

export interface SceneSummary {
  id: string;
  name: string;
}

export interface ShowState {
  activeSceneId: string;
  streamState: StreamState;
  recordState: RecordState;
  destination: DestinationSummary;
  scenes: SceneSummary[];
}

export interface SourceSummary {
  id: string;
  name: string;
  kind: SourceKind;
  status: SourceStatus;
  detail: string;
  bitrateKbps: number;
  publishUrl?: string;
}

export interface AudioChannelSummary {
  id: string;
  name: string;
  muted: boolean;
  gainDb: number;
  peakDb: number;
  source: string;
}

export interface AudioState {
  channels: AudioChannelSummary[];
}

export interface StorageSummary {
  recordingPath: string;
  totalGb: number;
  usedGb: number;
  freeGb: number;
  warning?: string;
}

export interface ConfigSystem {
  name: string;
  timezone: string;
  updateChannel: string;
}

export interface ConfigSource {
  id: string;
  name: string;
  kind: SourceKind;
  deviceRef?: string;
  protocols?: string[];
  publishUrl?: string;
}

export interface ConfigScene {
  id: string;
  name: string;
  sources: string[];
}

export interface ConfigAudioChannel {
  id: string;
  name: string;
  deviceRef: string;
}

export interface ConfigAudio {
  channels: ConfigAudioChannel[];
}

export interface ConfigDestination {
  mode: DestinationMode;
  targetLabel: string;
  targetUrl: string;
}

export interface ConfigRecording {
  container: "mkv" | "mp4";
  path: string;
}

export interface ConfigOwncast {
  enabled: boolean;
  publicUrl?: string;
}

export interface ControlConfig {
  system: ConfigSystem;
  sources: ConfigSource[];
  scenes: ConfigScene[];
  audio: ConfigAudio;
  destination: ConfigDestination;
  recording: ConfigRecording;
  owncast: ConfigOwncast;
}

export interface ConfigEnvelope {
  generation: string;
  updatedAt: string;
  config: ControlConfig;
}

export interface ValidationIssue {
  field: string;
  severity: "error" | "warning";
  message: string;
}

export interface ValidateConfigResponse {
  valid: boolean;
  issues: ValidationIssue[];
}

export interface OperationResponse {
  operationId: string;
  status: OperationStatus;
  message: string;
  appliedGeneration?: string;
}

export interface GainRequest {
  gainDb: number;
}

export interface ErrorResponse {
  error: string;
}

export const controlApiRoutes = {
  system: `${API_PREFIX}/system`,
  show: `${API_PREFIX}/show`,
  sources: `${API_PREFIX}/sources`,
  audio: `${API_PREFIX}/audio`,
  storage: `${API_PREFIX}/storage`,
  activeConfig: `${API_PREFIX}/config/active`,
  draftConfig: `${API_PREFIX}/config/draft`,
  validateConfig: `${API_PREFIX}/config/validate`,
  applyConfig: `${API_PREFIX}/config/apply`,
  streamStart: `${API_PREFIX}/show/stream/start`,
  streamStop: `${API_PREFIX}/show/stream/stop`,
  recordingStart: `${API_PREFIX}/show/recording/start`,
  recordingStop: `${API_PREFIX}/show/recording/stop`
} as const;
