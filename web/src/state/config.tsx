import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { api } from "../api/client";
import type { Config, StatusResponse } from "../api/types";
import { queryKeys } from "../app/queryKeys";
import { cloneConfig, ensureConfig, isConfigChanged } from "../features/configModel";

interface ConfigContextValue {
  draft: Config | undefined;
  baseConfig: Config | undefined;
  baseRevision: string | undefined;
  status: StatusResponse | undefined;
  isLoading: boolean;
  isConfigLoading: boolean;
  isStatusLoading: boolean;
  isDraftDirty: boolean;
  isReadonly: boolean;
  externalRevisionChanged: boolean;
  configError: unknown;
  statusError: unknown;
  updateDraft: (updater: (draft: Config) => Config) => void;
  replaceDraft: (config: Config, revision?: string) => void;
  resetDraft: () => void;
  forceReadonly: () => void;
}

const ConfigContext = createContext<ConfigContextValue | undefined>(undefined);

export function ConfigProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState<Config | undefined>();
  const [baseConfig, setBaseConfig] = useState<Config | undefined>();
  const [baseRevision, setBaseRevision] = useState<string | undefined>();
  const [forcedReadonly, setForcedReadonly] = useState(false);
  const baseConfigRef = useRef<Config | undefined>(undefined);

  const statusQuery = useQuery({
    queryKey: queryKeys.status,
    queryFn: api.status,
    refetchInterval: 30000
  });

  const configQuery = useQuery({
    queryKey: queryKeys.config,
    queryFn: api.config
  });

  useEffect(() => {
    if (!configQuery.data) return;
    const nextConfig = ensureConfig(configQuery.data.config);
    const previousBase = baseConfigRef.current;
    baseConfigRef.current = nextConfig;
    setBaseConfig(nextConfig);
    setBaseRevision(configQuery.data.config_revision);
    setDraft((current) => {
      if (current && previousBase && isConfigChanged(current, previousBase)) {
        return current;
      }
      return cloneConfig(nextConfig);
    });
  }, [configQuery.data]);

  const updateDraft = useCallback((updater: (draft: Config) => Config) => {
    setDraft((current) => ensureConfig(updater(ensureConfig(current))));
  }, []);

  const replaceDraft = useCallback(
    (config: Config, revision?: string) => {
      const next = ensureConfig(config);
      baseConfigRef.current = next;
      setDraft(cloneConfig(next));
      setBaseConfig(cloneConfig(next));
      if (revision) {
        setBaseRevision(revision);
      }
      void queryClient.invalidateQueries({ queryKey: queryKeys.config });
      void queryClient.invalidateQueries({ queryKey: queryKeys.status });
    },
    [queryClient]
  );

  const resetDraft = useCallback(() => {
    if (baseConfig) {
      setDraft(cloneConfig(baseConfig));
      baseConfigRef.current = baseConfig;
    }
  }, [baseConfig]);

  const isDraftDirty = isConfigChanged(draft, baseConfig);
  const status = statusQuery.data;
  const isReadonly = forcedReadonly || status?.capabilities.config_write === false || status?.config_source.writable === false;
  const externalRevisionChanged = Boolean(isDraftDirty && baseRevision && status?.config_revision && status.config_revision !== baseRevision);

  const value = useMemo<ConfigContextValue>(
    () => ({
      draft,
      baseConfig,
      baseRevision,
      status,
      isLoading: configQuery.isLoading || statusQuery.isLoading,
      isConfigLoading: configQuery.isLoading,
      isStatusLoading: statusQuery.isLoading,
      isDraftDirty,
      isReadonly,
      externalRevisionChanged,
      configError: configQuery.error,
      statusError: statusQuery.error,
      updateDraft,
      replaceDraft,
      resetDraft,
      forceReadonly: () => setForcedReadonly(true)
    }),
    [
      baseConfig,
      baseRevision,
      configQuery.error,
      configQuery.isLoading,
      draft,
      externalRevisionChanged,
      isDraftDirty,
      isReadonly,
      status,
      statusQuery.error,
      statusQuery.isLoading,
      updateDraft,
      replaceDraft,
      resetDraft
    ]
  );

  return <ConfigContext.Provider value={value}>{children}</ConfigContext.Provider>;
}

export function useConfigState() {
  const value = useContext(ConfigContext);
  if (!value) {
    throw new Error("useConfigState must be used inside ConfigProvider");
  }
  return value;
}
