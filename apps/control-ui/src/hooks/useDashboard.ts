import { useCallback, useEffect, useState } from "react";

import type {
  AudioState,
  ConfigEnvelope,
  ShowState,
  SourceSummary,
  StorageSummary,
  SystemSummary
} from "@bitriver/shared";

import {
  activateScene,
  getActiveConfig,
  getAudio,
  getShow,
  getSources,
  getStorage,
  getSystem,
  muteChannel,
  setChannelGain,
  startRecording,
  startStream,
  stopRecording,
  stopStream,
  unmuteChannel
} from "../lib/api";

interface DashboardData {
  system?: SystemSummary;
  show?: ShowState;
  sources: SourceSummary[];
  audio?: AudioState;
  storage?: StorageSummary;
  activeConfig?: ConfigEnvelope;
}

export function useDashboard() {
  const [data, setData] = useState<DashboardData>({ sources: [] });
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [warning, setWarning] = useState<string | null>(null);
  const [lastUpdated, setLastUpdated] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setError(null);

    try {
      // A missing OBS or audio device must not hide a healthy headless OS.
      // Never synthesize "stopped"/"healthy" states for unreadable endpoints.
      const [system, show, sources, audio, storage, activeConfig] = await Promise.allSettled([
        getSystem(),
        getShow(),
        getSources(),
        getAudio(),
        getStorage(),
        getActiveConfig()
      ]);
      const failures: string[] = [];
      if (system.status === "rejected") failures.push("system");
      if (show.status === "rejected") failures.push("OBS show");
      if (sources.status === "rejected") failures.push("sources");
      if (audio.status === "rejected") failures.push("audio");
      if (storage.status === "rejected") failures.push("storage");
      if (activeConfig.status === "rejected") failures.push("settings");

      setData({
        system: system.status === "fulfilled" ? system.value : undefined,
        show: show.status === "fulfilled" ? show.value : undefined,
        sources: sources.status === "fulfilled" ? sources.value : [],
        audio: audio.status === "fulfilled" ? audio.value : undefined,
        storage: storage.status === "fulfilled" ? storage.value : undefined,
        activeConfig: activeConfig.status === "fulfilled" ? activeConfig.value : undefined
      });
      if (system.status === "rejected") {
        setError("Cannot load system status from the BitRiver appliance.");
      }
      setWarning(failures.length
        ? "Unavailable subsystems: " + failures.join(", ") + ". Controls for missing subsystems are disabled; check local services before broadcasting."
        : null);
      if (system.status === "fulfilled") {
        setLastUpdated(new Date().toISOString());
      }
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unknown request error");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
    const interval = window.setInterval(() => {
      void refresh();
    }, 5000);

    return () => {
      window.clearInterval(interval);
    };
  }, [refresh]);

  const runAction = useCallback(
    async (operation: () => Promise<unknown>) => {
      setBusy(true);
      setError(null);

      try {
        await operation();
        await refresh();
      } catch (requestError) {
        setError(requestError instanceof Error ? requestError.message : "Unknown action error");
      } finally {
        setBusy(false);
      }
    },
    [refresh]
  );

  return {
    ...data,
    loading,
    busy,
    error,
    warning,
    lastUpdated,
    refresh,
    activateScene: (sceneId: string) => runAction(() => activateScene(sceneId)),
    startStream: () => runAction(startStream),
    stopStream: () => runAction(stopStream),
    startRecording: () => runAction(startRecording),
    stopRecording: () => runAction(stopRecording),
    muteChannel: (channelId: string) => runAction(() => muteChannel(channelId)),
    unmuteChannel: (channelId: string) => runAction(() => unmuteChannel(channelId)),
    raiseGain: (channelId: string, currentGainDb: number) =>
      runAction(() => setChannelGain(channelId, Math.min(currentGainDb + 1.5, 12))),
    lowerGain: (channelId: string, currentGainDb: number) =>
      runAction(() => setChannelGain(channelId, Math.max(currentGainDb - 1.5, -60)))
  };
}
