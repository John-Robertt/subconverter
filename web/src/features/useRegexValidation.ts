import { useEffect, useState } from "react";
import { api } from "../api/client";
import type { Config } from "../api/types";
import { getErrorMessage } from "../api/errors";
import { getValidateResult } from "./diagnostics";

// Use Prepare's regex engine without fetching subscription resources.
export function useRegexValidation(config: Config | undefined, pointer: string, pattern: string) {
  const [result, setResult] = useState<{ config?: Config; pointer: string; message: string; invalid: boolean }>({ pointer, message: "", invalid: false });
  useEffect(() => {
    if (!config || !pattern) return;
    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      try {
        const validation = await api.validateConfig(config, controller.signal);
        const diagnostic = [...validation.errors, ...validation.warnings].find((item) => item.locator?.json_pointer === pointer && /regex/.test(item.code));
        if (!controller.signal.aborted) setResult({ config, pointer, message: diagnostic?.message ?? "", invalid: Boolean(diagnostic) });
      } catch (error) {
        if (controller.signal.aborted) return;
        const validation = getValidateResult(error);
        const diagnostic = validation?.errors.find((item) => item.locator?.json_pointer === pointer && /regex/.test(item.code));
        setResult({ config, pointer, message: validation ? diagnostic?.message ?? "" : `正则检查失败，可点击预览重试：${getErrorMessage(error)}`, invalid: Boolean(diagnostic) });
      }
    }, 300);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [config, pointer, pattern]);
  const pending = Boolean(config && pattern && (result.config !== config || result.pointer !== pointer));
  return { pending, message: pattern && !pending ? result.message : "", invalid: Boolean(pattern && !pending && result.invalid) };
}
