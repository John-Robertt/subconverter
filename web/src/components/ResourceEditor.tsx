import { useRef, useState } from "react";
import { useResourceContent } from "../features/useResourceContent";
import { ResourceBody } from "./ResourceBody";
import { Button, Field, SelectInput, TextInput } from "./ui";

interface ResourceValue {
  location: string;
  content?: string;
}

export function ResourceEditor({ label, location, content, readonly, onChange }: ResourceValue & {
  label: string;
  readonly: boolean;
  onChange: (value: ResourceValue) => void;
}) {
  const inline = content !== undefined;
  const resource = useResourceContent(inline ? "" : location.trim());
  const [expanded, setExpanded] = useState(false);
  const previous = useRef({ location, content });
  const body = inline ? content : resource.content;

  function useBody() {
    previous.current.location = location;
    onChange({ location: "", content: previous.current.content ?? resource.content ?? "" });
    setExpanded(true);
  }

  return (
    <div className="form-stack">
      <Field label={`${label}来源`}>
        <SelectInput aria-label={`${label}来源`} value={inline ? "content" : "location"} disabled={readonly}
          onChange={(event) => {
            if (event.target.value === "content") useBody();
            else {
              previous.current.content = content;
              onChange({ location: previous.current.location });
            }
          }}>
          <option value="location">URL / 文件路径</option>
          <option value="content">本地正文</option>
        </SelectInput>
      </Field>
      {!inline ? <Field label={label}>
        <TextInput aria-label={label} className="mono-input" value={location} disabled={readonly}
          onChange={(event) => { previous.current.content = undefined; onChange({ location: event.target.value }); }} />
      </Field> : null}
      <p className="field-hint" role="status">{inline ? "当前使用本地正文，由你手动维护。" : resource.loading ? "正在读取模板…" : resource.error ? "模板读取失败" : body !== undefined ? "模板内容已读取" : "请填写模板地址，或选择本地正文。"}</p>
      {resource.error && !inline ? <div className="field-error" role="alert">{resource.error}</div> : null}
      <ResourceBody label={`${label}正文`} content={body} readOnly={readonly || !inline} expanded={expanded}
        onToggle={() => setExpanded((current) => !current)} onChange={(text) => onChange({ location: "", content: text })}
        actions={inline ? null : <>
          <Button loading={resource.loading} disabled={!location.trim()} onClick={resource.reload}>重新读取</Button>
          <Button disabled={readonly || resource.loading} onClick={useBody}>{body === undefined ? "粘贴正文" : "编辑为本地副本"}</Button>
        </>} />
      {inline ? <p className="field-hint">保存后使用本地正文，不再跟随原地址更新；原文件保持不变。保存并热重载后生效。</p> : null}
    </div>
  );
}
