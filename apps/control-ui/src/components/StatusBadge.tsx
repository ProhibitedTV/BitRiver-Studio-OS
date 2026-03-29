import type { HealthState, RecordState, SourceStatus, StreamState } from "@bitriver/shared";

type StatusTone = HealthState | StreamState | RecordState | SourceStatus;

interface StatusBadgeProps {
  label: string;
  tone: StatusTone;
}

const toneClassByState: Record<StatusTone, string> = {
  healthy: "status-badge--healthy",
  degraded: "status-badge--degraded",
  error: "status-badge--error",
  stopped: "status-badge--idle",
  starting: "status-badge--degraded",
  live: "status-badge--live",
  recording: "status-badge--live",
  ready: "status-badge--healthy",
  disconnected: "status-badge--error",
  warning: "status-badge--degraded"
};

export function StatusBadge({ label, tone }: StatusBadgeProps) {
  return <span className={`status-badge ${toneClassByState[tone]}`}>{label}</span>;
}
