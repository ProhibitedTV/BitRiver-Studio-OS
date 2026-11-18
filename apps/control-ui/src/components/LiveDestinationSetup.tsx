import { useState } from "react";
import type { ConfigMediaDestination, ValidationIssue } from "@bitriver/shared";
import {
  applyDraftConfig,
  getDraftConfig,
  updateDraftConfig,
  validateDraftConfig
} from "../lib/api";
import { destinationForLiveProfile, parseBitRiverLiveProfile } from "../lib/bitriver-live-profile";
import type { BitRiverLiveProfile } from "../lib/bitriver-live-profile";

interface Props {
  onAir: boolean;
  recording: boolean;
  activeDestinations: ConfigMediaDestination[];
  onApplied: () => Promise<unknown>;
}

export function LiveDestinationSetup({ onAir, recording, activeDestinations, onApplied }: Props) {
  const [profile, setProfile] = useState<BitRiverLiveProfile | null>(null);
  const [fileName, setFileName] = useState("");
  const [busy, setBusy] = useState(false);
  const [stagedGeneration, setStagedGeneration] = useState("");
  const [validatedGeneration, setValidatedGeneration] = useState("");
  const [issues, setIssues] = useState<ValidationIssue[]>([]);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const destination = profile ? destinationForLiveProfile(profile) : null;
  const safeToApply = !onAir && !recording;

  async function chooseProfile(file?: File) {
    setProfile(null);
    setStagedGeneration("");
    setValidatedGeneration("");
    setIssues([]);
    setMessage("");
    setError("");
    setFileName("");
    if (!file) return;
    if (file.size > 8192) {
      setError("Profile must be no more than 8 KiB.");
      return;
    }
    try {
      const parsed = parseBitRiverLiveProfile(await file.text());
      setProfile(parsed);
      setFileName(file.name);
      setMessage("Profile parsed locally. No data was uploaded and no destination was enabled.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to read profile.");
    }
  }

  async function stage() {
    if (!destination) return;
    setBusy(true);
    setError("");
    setMessage("");
    try {
      // Re-read immediately before patching so we preserve the other staged
      // settings. Never patch active settings or enable publishing from import.
      const draft = await getDraftConfig();
      if (draft.config.mediaRouter.publishDestinations.some((entry) => entry.id === destination.id)) {
        throw new Error("That channel already has a destination in the draft. No settings were changed.");
      }
      const next = {
        ...draft.config,
        mediaRouter: {
          ...draft.config.mediaRouter,
          publishDestinations: [...draft.config.mediaRouter.publishDestinations, destination]
        }
      };
      const updated = await updateDraftConfig(next, draft.generation);
      setStagedGeneration(updated.generation);
      setValidatedGeneration("");
      setIssues([]);
      setMessage("Staged a DISABLED BitRiver-Live destination. Existing destinations are unchanged.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not stage destination.");
    } finally {
      setBusy(false);
    }
  }

  async function validate() {
    if (!destination || !stagedGeneration) return;
    setBusy(true);
    setError("");
    setMessage("");
    setValidatedGeneration("");
    try {
      const current = await getDraftConfig();
      const selected = current.config.mediaRouter.publishDestinations.find((entry) => entry.id === destination.id);
      if (current.generation !== stagedGeneration || !selected ||
          selected.enabled || selected.targetUrl !== destination.targetUrl ||
          selected.streamKeyFile !== destination.streamKeyFile) {
        throw new Error("The draft changed since import. Review or re-stage it before validation.");
      }
      const result = await validateDraftConfig();
      setIssues(result.issues);
      if (result.valid) {
        setValidatedGeneration(current.generation);
        setMessage("Draft validated. This destination is still disabled; applying cannot start a BitRiver-Live stream.");
      } else {
        setError("Draft validation returned issues. Nothing has been applied.");
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not validate draft.");
    } finally {
      setBusy(false);
    }
  }

  async function apply() {
    if (!destination || !validatedGeneration || !safeToApply) return;
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const latest = await getDraftConfig();
      const selected = latest.config.mediaRouter.publishDestinations.find((entry) => entry.id === destination.id);
      if (latest.generation !== validatedGeneration || !selected ||
          selected.enabled || selected.targetUrl !== destination.targetUrl ||
          selected.streamKeyFile !== destination.streamKeyFile) {
        throw new Error("The draft was modified since validation. Re-validate before applying.");
      }
      // The UI blocks active streams/recordings and the server independently
      // rechecks OBS status before it applies MediaMTX changes.
      await applyDraftConfig(validatedGeneration);
      setStagedGeneration("");
      setValidatedGeneration("");
      setIssues([]);
      setMessage("Applied the disabled destination. No broadcast was started. Install your key locally, then enable the destination deliberately.");
      await onApplied();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not apply draft.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="panel panel--wide" aria-labelledby="bitriver-live-setup-heading">
      <div className="panel__header">
        <div>
          <p className="eyebrow">Creator ecosystem</p>
          <h2 id="bitriver-live-setup-heading">Connect BitRiver-Live</h2>
          <p className="creator-assets__hint">
            Import a public connection profile from any BitRiver-Live instance you own.
            The stream key is never part of the download or the Studio OS settings.
          </p>
        </div>
      </div>

      <div className="live-setup__steps">
        <div>
          <strong>1. Download from BitRiver-Live</strong>
          <p>In your channel's Creator Live page, choose <em>Download Studio OS profile</em>.
          Transfer the JSON file to this computer.</p>
        </div>
        <div>
          <label htmlFor="bitriver-live-profile"><strong>2. Select connection profile</strong></label>
          <input
            id="bitriver-live-profile"
            type="file"
            accept=".json,application/json"
            disabled={busy}
            onChange={(event) => { void chooseProfile(event.currentTarget.files?.[0]); }}
          />
          {fileName && <p>Selected: {fileName}</p>}
        </div>
      </div>

      {destination && profile && (
        <div className="live-setup__review">
          <strong>Review the destination before staging</strong>
          <p>Channel: <code>{profile.channelId}</code></p>
          <p>RTMP server (not a stream key): <code>{profile.rtmpServer}</code></p>
          <p>Viewer: <code>{profile.viewerUrl}</code></p>
          <p>Destination ID: <code>{destination.id}</code></p>
          <p>New destination state: <strong>Disabled</strong></p>
          <button className="button button--secondary" disabled={busy || !!stagedGeneration} onClick={() => { void stage(); }}>
            Stage disabled destination
          </button>
        </div>
      )}

      {!!stagedGeneration && destination && (
        <div className="live-setup__review">
          <strong>3. Validate, then explicitly apply</strong>
          <p>Staged generation: <code>{stagedGeneration}</code></p>
          <div className="live-setup__actions">
            <button className="button button--secondary" disabled={busy} onClick={() => { void validate(); }}>
              Validate staged settings
            </button>
            <button
              className="button button--primary"
              disabled={busy || !validatedGeneration || !safeToApply}
              onClick={() => { void apply(); }}
            >
              Apply disabled destination
            </button>
          </div>
          {!safeToApply && <p role="status">Stop streaming and recording before applying changes; MediaMTX may restart.</p>}
        </div>
      )}

      {issues.length > 0 && (
        <div className="live-setup__issues" role="status">
          <strong>Validation findings</strong>
          <ul>{issues.map((issue, index) => (
            <li key={index}>{issue.severity}: {issue.field} — {issue.message}</li>
          ))}</ul>
        </div>
      )}
      {error && <p className="live-setup__error" role="alert">{error}</p>}
      {message && <p className="live-setup__notice" role="status">{message}</p>}

      <div className="live-setup__review">
        <strong>4. Supply the key locally (not in this browser)</strong>
        <p>As the unprivileged BitRiver service user, use the OS import helper with
        your saved profile and the destination ID. It prompts privately for your
        48-character BitRiver-Live stream key:</p>
        <code>{`sudo -u bitriver python3 /usr/local/libexec/bitriver/import-bitriver-live-profile.py --profile /path/to/downloaded-profile.json --id ${destination?.id ?? "live-CHANNELID"}`}</code>
        <p>After installing the key, enable the destination explicitly through the
        authenticated settings API. This early UI does not provision keys,
        enable outputs or start publishing automatically.</p>
      </div>

      <div className="live-setup__review">
        <strong>Currently applied outputs</strong>
        {activeDestinations.length === 0 ? <p>No configured destinations.</p> : (
          <ul>{activeDestinations.map((entry) => (
            <li key={entry.id}>
              {entry.name} — {entry.enabled ? "enabled" : "disabled"} ({entry.protocol})
            </li>
          ))}</ul>
        )}
      </div>
    </section>
  );
}
