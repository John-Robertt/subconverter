import { useMutation } from "@tanstack/react-query";
import { Search } from "lucide-react";
import { useMemo } from "react";
import { api } from "../api/client";
import { getErrorMessage } from "../api/errors";
import { Button, Chip, EmptyState, ErrorState, Field, LoadingState, RailPanel, SplitWorkbench, TextInput } from "../components/ui";
import { focusClassName, useDiagnosticPointer } from "../features/diagnostics";
import { useConfigState } from "../state/config";

const templates = [
  ["流量信息", "剩余|流量|套餐|到期|Expire"],
  ["官网链接", "官网|website|官方"],
  ["测试节点", "测试|Test|Demo"],
  ["IPv6", "IPv6|v6"],
  ["高倍率", "×|x[2-9]|高倍|高级"]
];

export function FiltersPage() {
  const { draft, updateDraft, isReadonly } = useConfigState();
  const activePointer = useDiagnosticPointer();
  const exclude = draft?.filters?.exclude ?? "";
  const regexState = useMemo(() => {
    if (!exclude) return { valid: true, message: "" };
    try {
      new RegExp(exclude);
      return { valid: true, message: "" };
    } catch (error) {
      return { valid: false, message: error instanceof Error ? error.message : "正则语法错误" };
    }
  }, [exclude]);
  const previewMutation = useMutation({
    mutationFn: () => {
      if (!draft) throw new Error("配置尚未加载");
      return api.previewNodesDraft(draft);
    }
  });

  const preview = previewMutation.data;
  const filtered = preview?.filtered_count ?? preview?.nodes.filter((node) => node.filtered).length ?? 0;
  const total = preview?.total ?? preview?.nodes.length ?? 0;
  const active = preview?.active_count ?? Math.max(0, total - filtered);

  function appendTemplate(pattern: string) {
    updateDraft((config) => ({
      ...config,
      filters: {
        ...config.filters,
        exclude: exclude ? `${exclude}|${pattern}` : pattern
      }
    }));
  }

  return (
    <SplitWorkbench
      rail={
        <RailPanel eyebrow="Preview" title="草稿节点预览">
          {previewMutation.isPending ? <LoadingState message="正在拉取订阅并应用草稿过滤器" /> : null}
          {previewMutation.error ? <ErrorState message={getErrorMessage(previewMutation.error)} /> : null}
          {preview ? (
            <div className="preview-node-list">
              {preview.nodes.map((node) => (
                <div key={`${node.kind}-${node.type}-${node.name}`} className={node.filtered ? "preview-node filtered" : "preview-node"}>
                  <Chip tone={node.filtered ? "error" : "success"}>{node.filtered ? "剔除" : "保留"}</Chip>
                  <span>{node.name}</span>
                </div>
              ))}
            </div>
          ) : (
            !previewMutation.isPending && <EmptyState title="尚未运行预览" message="点击草稿节点预览后，这里会展示后端返回的 active / filtered 节点。" />
          )}
        </RailPanel>
      }
    >
      <div className="page-stack">
        <section className={focusClassName(activePointer, ["/config/filters"], "content-panel editor-panel")}>
          <div className="section-heading row">
            <div>
              <h3>排除规则</h3>
              <p>用正则匹配会被剔除的节点名，比如流量信息、官网、套餐到期等占位条目。</p>
            </div>
            <Button
              variant="secondary"
              icon={<Search size={16} aria-hidden="true" />}
              loading={previewMutation.isPending}
              disabled={!draft}
              onClick={() => previewMutation.mutate()}
            >
              草稿节点预览
            </Button>
          </div>

          <Field label="exclude 正则" hint="本地校验正则语法；点击预览才会实际拉取订阅来源。" error={regexState.valid ? undefined : regexState.message}>
            <TextInput
              className="text-input mono-input"
              value={exclude}
              disabled={isReadonly}
              onChange={(event) => updateDraft((config) => ({ ...config, filters: { ...config.filters, exclude: event.target.value } }))}
              placeholder="过期|剩余流量|到期"
            />
          </Field>

          <div className="stats-grid three">
            <div className="big-stat big-stat-neutral">
              <span>原始节点</span>
              <strong>{total || "-"}</strong>
            </div>
            <div className="big-stat big-stat-error">
              <span>剔除</span>
              <strong>{preview ? filtered : "-"}</strong>
            </div>
            <div className="big-stat big-stat-success">
              <span>保留</span>
              <strong>{preview ? active : "-"}</strong>
            </div>
          </div>
        </section>

        <section className="template-strip">
          <div className="section-heading">
            <h3>常用模板</h3>
            <p>模板只拼接到当前草稿，不会自动请求远程来源。</p>
          </div>
          <div className="template-chips">
            {templates.map(([name, pattern]) => (
              <button key={name} type="button" disabled={isReadonly} onClick={() => appendTemplate(pattern)}>
                + {name}
              </button>
            ))}
          </div>
        </section>
      </div>
    </SplitWorkbench>
  );
}
