import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "../api/client";

interface ResourceState {
  location: string;
  revision: number;
  content?: string;
  loading: boolean;
  error: string;
}

// Loading is independent of presentation; changed inputs invalidate stale responses.
export function useResourceContent(location: string) {
  const [revision, setRevision] = useState(0);
  const previousLocation = useRef(location);
  const [state, setState] = useState<ResourceState>();
  const reload = useCallback(() => setRevision((current) => current + 1), []);

  useEffect(() => {
    if (!location) return;
    let current = true;
    const controller = new AbortController();
    const delay = previousLocation.current === location ? 0 : 400;
    previousLocation.current = location;
    setState({ location, revision, loading: true, error: "" });
    const timer = window.setTimeout(async () => {
      try {
        const result = await api.readResource(location, controller.signal);
        if (current) setState({ location, revision, content: result.content, loading: false, error: "" });
      } catch (error) {
        if (current) setState({ location, revision, loading: false, error: error instanceof Error ? error.message : "读取失败" });
      }
    }, delay);
    return () => {
      current = false;
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [location, revision]);

  const matches = state?.location === location && state.revision === revision;
  return {
    content: matches ? state.content : undefined,
    error: matches ? state.error : "",
    loading: Boolean(location) && (!matches || state.loading),
    reload
  };
}
