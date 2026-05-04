import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { api } from "../api/client";
import { isApiError } from "../api/errors";
import type { ValidateResult } from "../api/types";
import { queryKeys } from "../app/queryKeys";
import { useConfirm } from "../state/confirm";
import { useConfigState } from "../state/config";
import { useToast } from "../state/toast";

const firstSaveStorageKey = "subconverter.firstSaveConfirmed";
const reloadRetryDelayMs = 1500;

class ValidationFailedError extends Error {
  result: ValidateResult;
  constructor(result: ValidateResult) {
    super("配置校验未通过");
    this.name = "ValidationFailedError";
    this.result = result;
  }
}

export function useSaveWorkflow() {
  const queryClient = useQueryClient();
  const confirm = useConfirm();
  const navigate = useNavigate();
  const { pushToast } = useToast();
  const { draft, baseRevision, replaceDraft, forceReadonly, resetDraft } = useConfigState();

  const reloadMutation = useMutation({
    mutationFn: reloadWithBackoff,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.status });
      void queryClient.invalidateQueries({ queryKey: ["previewNodes"] });
      void queryClient.invalidateQueries({ queryKey: ["previewGroups"] });
      void queryClient.invalidateQueries({ queryKey: ["generatePreview"] });
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
        throw new ValidationFailedError(validation);
      }

      if (!localStorage.getItem(firstSaveStorageKey)) {
        const accepted = await confirm({
          title: "将草稿写入 YAML 文件？",
          message: "保存会用当前草稿覆盖 YAML 文件，原文件中的注释、引号和部分格式风格可能丢失。保存后不会自动热重载，请按需点击右上角\"热重载\"。",
          confirmLabel: "确认保存"
        });
        if (!accepted) {
          throw new Error("已取消保存");
        }
        localStorage.setItem(firstSaveStorageKey, "true");
      }

      const saved = await api.saveConfig(baseRevision, savingDraft);
      return { saved, savedDraft: savingDraft };
    },
    onSuccess: async ({ saved, savedDraft }) => {
      replaceDraft(savedDraft, saved.config_revision);
      await queryClient.invalidateQueries({ queryKey: queryKeys.config });
      await queryClient.invalidateQueries({ queryKey: queryKeys.status });
      pushToast({
        kind: "success",
        title: "草稿已写入 YAML 文件",
        message: "如需立即生效，请点击右上角\"热重载\"。"
      });
    },
    onError: async (error) => {
      if (error instanceof ValidationFailedError) {
        navigate("/validate", { state: { validateResult: error.result } });
        const total = error.result.errors.length + error.result.warnings.length + error.result.infos.length;
        pushToast({
          kind: "warning",
          title: "保存被阻断：静态校验未通过",
          message: `发现 ${total} 个诊断项，已跳转到校验页查看详情。`
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

      pushToast({ kind: "error", title: "保存失败", message: "保存请求失败", persistent: true });
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

function delay(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}
