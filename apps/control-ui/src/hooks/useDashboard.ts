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
  const [lastUpdated, setLastUpdated] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setError(null);

    try {
      const [system, show, sources, audio, storage, activeConfig] = await Promise.all([
        getSystem(),
        getShow(),
        getSources(),
        getAudio(),
        getStorage(),
        getActiveConfig()
      ]);

      setData({ system, show, sources, audio, storage, activeConfig });
      setLastUpdated(new Date().toISOString());
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
