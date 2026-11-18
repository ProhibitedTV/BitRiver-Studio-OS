import { StatusBadge } from "./components/StatusBadge";
import { CreatorAssets } from "./components/CreatorAssets";
import { useDashboard } from "./hooks/useDashboard";

function formatTimestamp(value: string | null) {
  if (!value) {
    return "waiting for first poll";
  }

  return new Date(value).toLocaleTimeString();
}

export function App() {
  const dashboard = useDashboard();

  const streamLive = dashboard.show?.streamState === "live";
  const recording = dashboard.show?.recordState === "recording";

  return (
    <div className="shell">
      <div className="shell__glow shell__glow--one" />
      <div className="shell__glow shell__glow--two" />

      <header className="topbar">
        <div>
          <p className="eyebrow">BitRiver Studio OS</p>
          <h1>LAN-Controlled Broadcast Appliance</h1>
          <p className="lede">
            Immutable appliance UX for switching, ingest, recording, and safe operations.
          </p>
        </div>

        <div className="topbar__meta">
          <div className="meta-pill">
            <span>Host</span>
            <strong>{dashboard.system?.network.hostname ?? "bitriver.local"}</strong>
          </div>
          <div className="meta-pill">
            <span>LAN</span>
            <strong>{dashboard.system?.network.lanAddress ?? "127.0.0.1"}</strong>
          </div>
          <div className="meta-pill">
            <span>Version</span>
            <strong>{dashboard.system?.version ?? "loading"}</strong>
          </div>
        </div>
      </header>

      {dashboard.error ? <div className="alert" role="alert">{dashboard.error}</div> : null}
      {dashboard.warning ? (
        <div className="status-notice" role="status">
          <strong>Limited hardware / service readiness</strong>
          <p>{dashboard.warning}</p>
        </div>
      ) : null}

      <main className="grid">
        <section className="panel panel--hero">
          <div className="panel__header">
            <div>
              <p className="eyebrow">Show Control</p>
              <h2>{dashboard.show?.activeSceneId ?? (dashboard.loading ? "Loading scene graph" : "OBS not connected")}</h2>
            </div>
            <div className="panel__status-row">
              <StatusBadge label={dashboard.show?.streamState ?? "unknown"} tone={dashboard.show?.streamState ?? "degraded"} />
              <StatusBadge
                label={dashboard.show?.recordState ?? "unknown"}
                tone={dashboard.show?.recordState ?? "degraded"}
              />
            </div>
          </div>

          <div className="program-card">
            <div className="program-card__screen">
              <div className="scanline" />
              <div className="program-card__overlay">
                <p>Program Output</p>
                <strong>{dashboard.show?.destination.label ?? "Destination pending"}</strong>
              </div>
            </div>

            <div className="program-card__controls">
              <button
                className={`button ${streamLive ? "button--danger" : "button--primary"}`}
                disabled={dashboard.busy || dashboard.loading || !dashboard.show}
                onClick={() => (streamLive ? dashboard.stopStream() : dashboard.startStream())}
              >
                {streamLive ? "Stop Stream" : "Start Stream"}
              </button>
              <button
                className={`button ${recording ? "button--danger" : "button--secondary"}`}
                disabled={dashboard.busy || dashboard.loading || !dashboard.show}
                onClick={() => (recording ? dashboard.stopRecording() : dashboard.startRecording())}
              >
                {recording ? "Stop Recording" : "Start Recording"}
              </button>
              <button className="button button--ghost" disabled={dashboard.busy} onClick={() => void dashboard.refresh()}>
                Refresh
              </button>
            </div>
          </div>
        </section>

        <section className="panel">
          <div className="panel__header">
            <div>
              <p className="eyebrow">System</p>
              <h2>{dashboard.system?.name ?? "studio-a"}</h2>
            </div>
            <StatusBadge label={dashboard.system?.health.overall ?? "unknown"} tone={dashboard.system?.health.overall ?? "degraded"} />
          </div>

          <div className="metrics">
            <div className="metric">
              <span>Boot Mode</span>
              <strong>{dashboard.system?.bootMode ?? "development"}</strong>
            </div>
            <div className="metric">
              <span>Uptime</span>
              <strong>{dashboard.system?.uptimeSeconds ?? 0}s</strong>
            </div>
            <div className="metric">
              <span>Last Poll</span>
              <strong>{formatTimestamp(dashboard.lastUpdated)}</strong>
            </div>
          </div>

          <div className="list">
            {dashboard.system?.health.checks.map((check) => (
              <div className="list__row" key={check.key}>
                <div>
                  <strong>{check.key}</strong>
                  <p>{check.message}</p>
                </div>
                <StatusBadge label={check.state} tone={check.state} />
              </div>
            ))}
          </div>
        </section>

        <section className="panel">
          <div className="panel__header">
            <div>
              <p className="eyebrow">Storage</p>
              <h2>{dashboard.storage?.recordingPath ?? "/data/recordings"}</h2>
            </div>
          </div>

          <div className="metrics metrics--triple">
            <div className="metric">
              <span>Total</span>
              <strong>{dashboard.storage?.totalGb ?? 0} GB</strong>
            </div>
            <div className="metric">
              <span>Used</span>
              <strong>{dashboard.storage?.usedGb ?? 0} GB</strong>
            </div>
            <div className="metric">
              <span>Free</span>
              <strong>{dashboard.storage?.freeGb ?? 0} GB</strong>
            </div>
          </div>

          <div className="callout">
            <strong>Update posture</strong>
            <p>Stage updates off-air, validate on reboot, and keep rollback one boot away.</p>
          </div>
        </section>

        <section className="panel panel--wide">
          <div className="panel__header">
            <div>
              <p className="eyebrow">Scene Rail</p>
              <h2>Production switching</h2>
            </div>
          </div>

          <div className="scene-rail">
            {dashboard.show?.scenes.map((scene) => {
              const active = dashboard.show?.activeSceneId === scene.id;

              return (
                <button
                  className={`scene-button ${active ? "scene-button--active" : ""}`}
                  disabled={dashboard.busy}
                  key={scene.id}
                  onClick={() => dashboard.activateScene(scene.id)}
                >
                  <span>{scene.name}</span>
                  <small>{scene.id}</small>
                </button>
              );
            })}
          </div>
        </section>

        <section className="panel panel--wide">
          <div className="panel__header">
            <div>
              <p className="eyebrow">Sources</p>
              <h2>Local devices and ingest slots</h2>
            </div>
          </div>

          <div className="source-grid">
            {dashboard.sources.map((source) => (
              <article className="source-card" key={source.id}>
                <div className="source-card__header">
                  <div>
                    <strong>{source.name}</strong>
                    <p>{source.kind}</p>
                  </div>
                  <StatusBadge label={source.status} tone={source.status} />
                </div>
                <p className="source-card__detail">{source.detail}</p>
                {source.publishUrl ? <code>{source.publishUrl}</code> : null}
              </article>
            ))}
          </div>
        </section>

        <CreatorAssets />

        <section className="panel panel--wide">
          <div className="panel__header">
            <div>
              <p className="eyebrow">Audio</p>
              <h2>PipeWire-backed mixer surface</h2>
            </div>
          </div>

          <div className="audio-list">
            {dashboard.audio?.channels.map((channel) => (
              <div className="audio-row" key={channel.id}>
                <div className="audio-row__meta">
                  <strong>{channel.name}</strong>
                  <p>{channel.source}</p>
                </div>
                <div className="audio-row__levels">
                  <span>{channel.gainDb.toFixed(1)} dB</span>
                  <span>Peak {channel.peakDb.toFixed(1)} dB</span>
                </div>
                <div className="audio-row__actions">
                  <button
                    className="button button--ghost"
                    disabled={dashboard.busy}
                    onClick={() => dashboard.lowerGain(channel.id, channel.gainDb)}
                  >
                    -1.5 dB
                  </button>
                  <button
                    className="button button--ghost"
                    disabled={dashboard.busy}
                    onClick={() => dashboard.raiseGain(channel.id, channel.gainDb)}
                  >
                    +1.5 dB
                  </button>
                  <button
                    className={`button ${channel.muted ? "button--danger" : "button--secondary"}`}
                    disabled={dashboard.busy}
                    onClick={() =>
                      channel.muted ? dashboard.unmuteChannel(channel.id) : dashboard.muteChannel(channel.id)
                    }
                  >
                    {channel.muted ? "Unmute" : "Mute"}
                  </button>
                </div>
              </div>
            ))}
          </div>
        </section>

        <section className="panel">
          <div className="panel__header">
            <div>
              <p className="eyebrow">Config</p>
              <h2>{dashboard.activeConfig?.generation ?? "draft-next"}</h2>
            </div>
          </div>
          <div className="list">
            <div className="list__row">
              <div>
                <strong>Destination</strong>
                <p>{dashboard.activeConfig?.config.destination.targetLabel ?? "Twitch"}</p>
              </div>
              <span>{dashboard.activeConfig?.config.destination.mode ?? "external_rtmp"}</span>
            </div>
            <div className="list__row">
              <div>
                <strong>Recording</strong>
                <p>{dashboard.activeConfig?.config.recording.path ?? "/data/recordings"}</p>
              </div>
              <span>{dashboard.activeConfig?.config.recording.container ?? "mkv"}</span>
            </div>
            <div className="list__row">
              <div>
                <strong>Update Channel</strong>
                <p>{dashboard.activeConfig?.config.system.updateChannel ?? "stable"}</p>
              </div>
              <span>{dashboard.activeConfig?.config.system.timezone ?? "America/Denver"}</span>
            </div>
          </div>
        </section>

        <section className="panel">
          <div className="panel__header">
            <div>
              <p className="eyebrow">Runbook</p>
              <h2>Operator posture</h2>
            </div>
          </div>

          <div className="runbook">
            <p>1. Verify local devices and ingest slots before arming stream or recording.</p>
            <p>2. Start recording first, then bring the destination live.</p>
            <p>3. Stage image updates only after the show, then reboot into validation.</p>
          </div>
        </section>
      </main>
    </div>
  );
}
