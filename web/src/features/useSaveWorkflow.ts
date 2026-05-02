import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api/client";
import { isApiError } from "../api/errors";
import type { Config } from "../api/types";
import { queryKeys } from "../app/queryKeys";
import { useConfirm } from "../state/confirm";
import { useConfigState } from "../state/config";
import { useToast } from "../state/toast";

const firstSaveStorageKey = "subconverter.firstSaveConfirmed";
const reloadRetryDelayMs = 1500;

class ReloadAfterSaveError extends Error {
  savedRevision: string;
  savedDraft: Config;
  reloadError: unknown;

  constructor(savedRevision: string, savedDraft: Config, reloadError: unknown) {
    super("配置已保存，但 reload 未完成");
    this.name = "ReloadAfterSaveError";
    this.savedRevision = savedRevision;
    this.savedDraft = savedDraft;
    this.reloadError = reloadError;
  }
}

export function useSaveWorkflow() {
  const queryClient = useQueryClient();
  const confirm = useConfirm();
  const { pushToast } = useToast();
  const { draft, baseRevision, replaceDraft, forceReadonly, resetDraft } = useConfigState();

  const validateMutation = useMutation({
    mutationFn: () => {
      if (!draft) throw new Error("配置尚未加载");
      return api.validateConfig(draft);
    }
  });

  const reloadMutation = useMutation({
    mutationFn: reloadWithBackoff,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.status });
      void queryClient.invalidateQueries({ queryKey: ["previewNodes"] });
    }
  });

  const saveMutation = useMutation({
    mutationFn: async () => {
      if (!draft || !baseRevision) {
        throw new Error("配置尚未加载");
      }
      const savingDraft = draft;

      const validation = await api.validateConfig(savingDraft);
      if (!validation.valid) {
        const first = validation.errors[0] ?? validation.warnings[0] ?? validation.infos[0];
        const location = first?.locator?.json_pointer ?? first?.display_path ?? "/config";
        throw new Error(first ? `配置校验未通过：${first.code} ${first.message}（${location}）` : "配置校验未通过");
      }

      if (!localStorage.getItem(firstSaveStorageKey)) {
        const accepted = await confirm({
          title: "确认首次写回 YAML",
          message: "保存会把当前结构化配置写回 YAML，原文件中的注释、引号和部分格式风格可能丢失。",
          confirmLabel: "确认保存"
        });
        if (!accepted) {
          throw new Error("已取消保存");
        }
        localStorage.setItem(firstSaveStorageKey, "true");
      }

      const saved = await api.saveConfig(baseRevision, savingDraft);
      try {
        const reloadStatus = await reloadWithBackoff();
        return { saved, savedDraft: savingDraft, reloadStatus };
      } catch (reloadError) {
        throw new ReloadAfterSaveError(saved.config_revision, savingDraft, reloadError);
      }
    },
    onSuccess: async ({ saved, savedDraft }) => {
      replaceDraft(savedDraft, saved.config_revision);
      await queryClient.invalidateQueries({ queryKey: queryKeys.config });
      await queryClient.invalidateQueries({ queryKey: queryKeys.status });
      pushToast({ kind: "success", title: "配置已保存并生效" });
    },
    onError: async (error) => {
      if (error instanceof ReloadAfterSaveError) {
        replaceDraft(error.savedDraft, error.savedRevision);
        await queryClient.invalidateQueries({ queryKey: queryKeys.config });
        await queryClient.invalidateQueries({ queryKey: queryKeys.status });
        pushToast({
          kind: "warning",
          title: "配置已保存，reload 未完成",
          message: `配置文件已写回，但 RuntimeConfig 仍可能使用旧 revision。${formatError(error.reloadError)}`,
          persistent: true,
          action: {
            label: "重试 reload",
            onClick: () => void reloadOnly()
          }
        });
        return;
      }

      if (isApiError(error)) {
        if (error.code === "config_source_readonly") {
          forceReadonly();
        }
        pushToast(buildSaveErrorToast(error));
        return;
      }

      if (error instanceof Error) {
        pushToast({ kind: "error", title: "保存未完成", message: error.message, persistent: true });
        return;
      }

      pushToast({ kind: "error", title: "保存失败", message: "配置校验未通过或请求失败", persistent: true });
    }
  });

  async function reloadOnly() {
    try {
      await reloadMutation.mutateAsync();
      await queryClient.invalidateQueries({ queryKey: queryKeys.status });
      pushToast({ kind: "success", title: "RuntimeConfig 已重新加载" });
    } catch (error) {
      const message = isApiError(error) ? `${error.status} ${error.code}: ${error.message}` : error instanceof Error ? error.message : "reload 失败";
      pushToast({ kind: "error", title: "Reload 失败", message, persistent: true });
    }
  }

  async function reloadWithBackoff() {
    try {
      return await api.reload();
    } catch (error) {
      if (!isReloadInProgress(error)) {
        throw error;
      }

      pushToast({
        kind: "info",
        title: "Reload 正在执行",
        message: `${reloadRetryDelayMs / 1000} 秒后自动重试一次；如果仍失败，可稍后手动触发。`
      });
      await delay(reloadRetryDelayMs);
      return api.reload();
    }
  }

  function buildSaveErrorToast(error: { status: number; code: string; message: string }) {
    const common = `${error.status} ${error.code}: ${error.message}`;
    if (error.code === "config_revision_conflict") {
      return {
        kind: "error" as const,
        title: "配置文件已被外部修改",
        message: `${common}。当前草稿已保留；可继续手动合并，或重新加载配置源最新内容。`,
        persistent: true,
        action: {
          label: "重新加载配置",
          onClick: () => void reloadConfigFromServer()
        }
      };
    }
    if (error.code === "config_source_readonly") {
      return {
        kind: "error" as const,
        title: "配置源只读",
        message: `${common}。已进入只读查看模式，预览和 reload 仍可继续使用。`,
        persistent: true
      };
    }
    if (error.code === "config_file_not_writable") {
      return {
        kind: "error" as const,
        title: "配置文件不可写",
        message: `${common}。请检查文件权限、容器挂载或运行用户，当前草稿已保留。`,
        persistent: true
      };
    }
    if (error.status === 409) {
      return {
        kind: "error" as const,
        title: "未知保存冲突",
        message: `${common}。当前草稿已保留，请查看后端详情后重试。`,
        persistent: true
      };
    }
    return {
      kind: "error" as const,
      title: "保存失败",
      message: common,
      persistent: true
    };
  }

  async function reloadConfigFromServer() {
    const accepted = await confirm({
      title: "重新加载配置？",
      message: "这会丢弃当前草稿，并从配置源读取最新内容。若你还需要手动合并，请先取消。",
      confirmLabel: "重新加载",
      danger: true
    });
    if (!accepted) return;

    const snapshot = await queryClient.fetchQuery({ queryKey: queryKeys.config, queryFn: api.config });
    replaceDraft(snapshot.config, snapshot.config_revision);
    await queryClient.invalidateQueries({ queryKey: queryKeys.status });
    pushToast({ kind: "info", title: "已重新加载配置源内容" });
  }

  return {
    validateDraft: validateMutation.mutateAsync,
    isValidating: validateMutation.isPending,
    saveDraft: saveMutation.mutateAsync,
    isSaving: saveMutation.isPending,
    reloadOnly,
    isReloading: reloadMutation.isPending,
    resetDraft
  };
}

function isReloadInProgress(error: unknown) {
  return isApiError(error) && (error.status === 429 || error.code === "reload_in_progress");
}

function formatError(error: unknown) {
  if (isApiError(error)) {
    return `${error.status} ${error.code}: ${error.message}`;
  }
  return error instanceof Error ? error.message : "reload 失败";
}

function delay(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}
