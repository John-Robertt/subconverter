import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { Navigate, Outlet, useLocation, useNavigate } from "react-router-dom";
import { api } from "../api/client";
import type { AuthStatus } from "../api/types";
import { queryKeys } from "./queryKeys";
import { LoadingState } from "../components/ui";
import { ConfigProvider } from "../state/config";
import { useToast } from "../state/toast";

export function ProtectedRoute() {
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { pushToast } = useToast();
  const authQuery = useQuery({
    queryKey: queryKeys.authStatus,
    queryFn: api.authStatus
  });

  useEffect(() => {
    function handleAuthRequired(event: Event) {
      const detail = (event as CustomEvent<{ code?: string }>).detail;
      queryClient.setQueryData<AuthStatus | undefined>(queryKeys.authStatus, (current) =>
        current ? { ...current, authed: false } : current
      );
      void queryClient.invalidateQueries({ queryKey: queryKeys.authStatus });
      if (detail?.code === "session_expired") {
        pushToast({
          scope: "auth",
          kind: "warning",
          title: "登录已过期",
          message: "会话已过期，请重新登录。"
        });
      }
      navigate(`/login?next=${encodeURIComponent(`${location.pathname}${location.search}`)}`, { replace: true });
    }

    window.addEventListener("subconverter:auth-required", handleAuthRequired);
    return () => window.removeEventListener("subconverter:auth-required", handleAuthRequired);
  }, [location.pathname, location.search, navigate, pushToast, queryClient]);

  if (authQuery.isLoading) {
    return <LoadingState message="正在校验登录状态" />;
  }

  if (authQuery.data?.setup_required || !authQuery.data?.authed) {
    const next = encodeURIComponent(`${location.pathname}${location.search}`);
    return <Navigate to={`/login?next=${next}`} replace />;
  }

  return (
    <ConfigProvider>
      <Outlet />
    </ConfigProvider>
  );
}
