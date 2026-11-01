import { useEffect, useRef, useState } from "react";
import { api } from "../api/client";
import { Button, Field, SelectInput, TextArea, TextInput } from "./ui";

interface ResourceValue {
  location: string;
  content?: string;
}

// Reads an editable draft copy without changing the referenced resource.
export function ResourceEditor({ label, location, content, readonly, onChange }: ResourceValue & {
  label: string;
  readonly: boolean;
  onChange: (value: ResourceValue) => void;
}) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const previous = useRef({ location, content: content ?? "" });
  const request = useRef(0);
  const inline = content !== undefined;
  useEffect(() => () => { request.current++; }, []);

  async function readContent() {
    const id = ++request.current;
    setLoading(true);
    setError("");
    try {
      const result = await api.readResource(location);
      if (request.current === id) {
        previous.current.location = location;
        onChange({ location: "", content: result.content });
      }
    } catch (err) {
      if (request.current === id) setError(err instanceof Error ? err.message : "读取失败");
    } finally {
      if (request.current === id) setLoading(false);
    }
  }

  return (
    <div className="form-stack">
      <Field label={`${label}来源`}>
        <SelectInput aria-label={`${label}来源`} value={inline ? "content" : "location"} disabled={readonly || loading}
          onChange={(event) => {
            setError("");
            if (inline) previous.current.content = content;
            else previous.current.location = location;
            onChange(event.target.value === "content"
              ? { location: "", content: previous.current.content }
              : { location: previous.current.location });
          }}>
          <option value="location">URL / 文件路径</option>
          <option value="content">直接编辑正文</option>
        </SelectInput>
      </Field>
      {inline ? (
        <Field label={`${label}正文`} hint="每次保存生成带时间戳的版本文件并保留旧版，热重载后生效。订阅正文副本不会自动更新。">
          <TextArea aria-label={`${label}正文`} className="mono-input" rows={14} spellCheck={false}
            value={content} disabled={readonly || loading} onChange={(event) => onChange({ location: "", content: event.target.value })} />
        </Field>
      ) : (
        <Field label={label}>
          <TextInput aria-label={label} className="mono-input" value={location} disabled={readonly || loading}
            onChange={(event) => onChange({ location: event.target.value })} />
          <Button disabled={readonly || loading || !location.trim()} onClick={() => void readContent()}>
            {loading ? "读取中…" : "读取并编辑正文"}
          </Button>
        </Field>
      )}
      {error ? <div className="field-error" role="alert">{error}</div> : null}
    </div>
  );
}
