export const API_PREFIX = "/api/v1";

export type HealthState = "healthy" | "degraded" | "error";
export type StreamState = "stopped" | "starting" | "live";
export type RecordState = "stopped" | "recording";
export type SourceStatus = "ready" | "disconnected" | "warning";
export type OperationStatus = "accepted" | "completed" | "rejected";
export type DestinationMode = "external_rtmp" | "external_srt" | "owncast";
export type SourceKind = "local_video" | "local_audio" | "network_ingest" | "browser_source";
export type SetupState = "pending" | "in_progress" | "ready" | "degraded";

export interface HealthCheck {
  key: string;
  state: HealthState;
  message: string;
}

export interface HealthSummary {
  overall: HealthState;
  checks: HealthCheck[];
  updatedAt: string;
}

export interface ServiceStatus {
  name: string;
  active: boolean;
  state: string;
  detail: string;
  managedBy: string;
  restarting: boolean;
}

export interface NetworkSummary {
  lanAddress: string;
  hostname: string;
  interfaces: string[];
}

export interface CPUStatus {
  model: string;
  logicalCores: number;
  usagePercent: number;
  loadAverage: number;
}

export interface MemoryStatus {
  totalMiB: number;
  usedMiB: number;
  freeMiB: number;
}

export interface DiskStatus {
  path: string;
  totalGb: number;
  usedGb: number;
  freeGb: number;
  available: boolean;
  warning?: string;
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
  cpu: CPUStatus;
  memory: MemoryStatus;
  disk: DiskStatus;
  services: ServiceStatus[];
}

export interface DestinationSummary {
  id: string;
  mode: DestinationMode;
  label: string;
  target: string;
  connected: boolean;
  enabled: boolean;
  detail: string;
}

export interface OutputSummary {
  streamState: StreamState;
  destinations: DestinationSummary[];
}

export interface MediaDestinationSummary {
  id: string;
  name: string;
  protocol: "rtmp" | "srt";
  target: string;
  enabled: boolean;
  connected: boolean;
  detail: string;
}

export interface MediaPathSummary {
  name: string;
  ready: boolean;
  source: string;
  readers: number;
  bitrateKbps: number;
  detail: string;
  publishUrl?: string;
}

export interface MediaSessionSummary {
  id: string;
  protocol: "rtmp" | "srt" | "rtsp" | "webrtc";
  path: string;
  remoteAddr: string;
  state: string;
  startedAt?: string;
}

export interface MediaRouterSummary {
  healthy: boolean;
  detail: string;
  configGeneration?: string;
  configPath?: string;
  localIngestPath: string;
  localPreviewPath: string;
  recordingPath: string;
  webrtcPreviewEnabled: boolean;
  destinations: MediaDestinationSummary[];
  activePaths: MediaPathSummary[];
  activeSessions: MediaSessionSummary[];
  lastAppliedAt?: string;
  lastReloadedAt?: string;
  lastConfigValidatedAt?: string;
}

export interface SceneSummary {
  id: string;
  name: string;
  active: boolean;
}

export interface ScenesState {
  activeSceneId: string;
  items: SceneSummary[];
}

export interface ShowState {
  activeSceneId: string;
  streamState: StreamState;
  recordState: RecordState;
  destination: DestinationSummary;
  scenes: SceneSummary[];
}

export interface StreamSummary {
  state: StreamState;
  activeSceneId: string;
  destination: string;
  startedAt?: string;
}

export interface SourceSummary {
  id: string;
  name: string;
  kind: SourceKind;
  status: SourceStatus;
  available: boolean;
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
  available: boolean;
}

export interface AudioState {
  channels: AudioChannelSummary[];
}

export interface RecordingSummary {
  state: RecordState;
  recordingPath: string;
  currentFile?: string;
  totalGb: number;
  usedGb: number;
  freeGb: number;
  warning?: string;
}

export type StorageSummary = RecordingSummary;

export interface GPUDevice {
  pciAddress: string;
  vendor: "amd" | "intel" | "nvidia" | "other";
  vendorId: string;
  deviceId: string;
  kernelDriver: string;
}

export interface NetworkAdapter {
  name: string;
  wireless: boolean;
  kernelDriver: string;
}

export interface HardwareReport {
  detectedAt: string;
  architecture: string;
  cpuModel: string;
  logicalCores: number;
  memoryMiB: number;
  displayAttached: boolean;
  bootFirmware: "uefi" | "bios" | "unknown";
  gpus: GPUDevice[];
  networkAdapters: NetworkAdapter[];
  compatibilityNotes: string[];
  videoDevices: string[];
  audioCaptureDevices: string[];
  networkInterfaces: string[];
  capabilities: {
    kioskEligible: boolean;
    obsEligible: boolean;
    recommendedForMvp: boolean;
  };
}

export interface SetupStatus {
  state: SetupState;
  firstBootComplete: boolean;
  applianceId: string;
  hostname: string;
  adminUsername: string;
  credentialsReady: boolean;
  requiresPasswordRotation: boolean;
  initialConfigWritten: boolean;
  hardwareDetected: boolean;
  displayMode: "headless" | "kiosk";
  startedAt: string;
  completedAt?: string;
  issues: string[];
  hardware: HardwareReport;
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

export interface ConfigMediaDestination {
  id: string;
  name: string;
  protocol: "rtmp" | "srt";
  targetUrl: string;
  streamKeyFile?: string;
  enabled: boolean;
}

export interface ConfigMediaRouter {
  localIngestPath: string;
  localPreviewPath: string;
  recordingPath: string;
  publishDestinations: ConfigMediaDestination[];
  webrtcPreviewEnabled: boolean;
}

export interface ControlConfig {
  system: ConfigSystem;
  sources: ConfigSource[];
  scenes: ConfigScene[];
  audio: ConfigAudio;
  destination: ConfigDestination;
  recording: ConfigRecording;
  owncast: ConfigOwncast;
  mediaRouter: ConfigMediaRouter;
}

export interface ConfigEnvelope {
  schemaVersion: string;
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

export interface APIError {
  code: string;
  message: string;
  details?: Record<string, string>;
}

export interface UpdateStatus {
  state: string;
  currentVersion: string;
  stagedVersion?: string;
  deploymentSource: string;
  rebootRequired: boolean;
  lastCheckedAt?: string;
  message?: string;
}

export interface Snapshot {
  system: SystemSummary;
  health: HealthSummary;
  stream: StreamSummary;
  outputs: OutputSummary;
  recordings: RecordingSummary;
  mediaRouter: MediaRouterSummary;
  scenes: ScenesState;
  sources: SourceSummary[];
  audio: AudioState;
  settings: ConfigEnvelope;
  update: UpdateStatus;
  setup: SetupStatus;
}

export const controlApiRoutes = {
  system: `${API_PREFIX}/system`,
  health: `${API_PREFIX}/health`,
  setupStatus: `${API_PREFIX}/setup/status`,
  stream: `${API_PREFIX}/stream`,
  streamStart: `${API_PREFIX}/stream/start`,
  streamStop: `${API_PREFIX}/stream/stop`,
  outputs: `${API_PREFIX}/outputs`,
  mediaRouter: `${API_PREFIX}/media-router`,
  recordings: `${API_PREFIX}/recordings`,
  recordingStart: `${API_PREFIX}/recordings/start`,
  recordingStop: `${API_PREFIX}/recordings/stop`,
  scenes: `${API_PREFIX}/scenes`,
  sources: `${API_PREFIX}/sources`,
  audio: `${API_PREFIX}/audio`,
  audioRoutes: `${API_PREFIX}/audio/routes`,
  settingsActive: `${API_PREFIX}/settings/active`,
  settingsDraft: `${API_PREFIX}/settings/draft`,
  settingsValidate: `${API_PREFIX}/settings/validate`,
  settingsApply: `${API_PREFIX}/settings/apply`,
  update: `${API_PREFIX}/update`,
  websocket: `${API_PREFIX}/ws`,
  openapi: `${API_PREFIX}/openapi.yaml`,
  show: `${API_PREFIX}/show`,
  storage: `${API_PREFIX}/storage`,
  activeConfig: `${API_PREFIX}/settings/active`,
  draftConfig: `${API_PREFIX}/settings/draft`,
  validateConfig: `${API_PREFIX}/settings/validate`,
  applyConfig: `${API_PREFIX}/settings/apply`
} as const;
