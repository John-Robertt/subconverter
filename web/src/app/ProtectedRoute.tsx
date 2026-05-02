import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { Navigate, Outlet, useLocation, useNavigate } from "react-router-dom";
import { api } from "../api/client";
import { queryKeys } from "./queryKeys";
import { LoadingState } from "../components/ui";
import { ConfigProvider } from "../state/config";

export function ProtectedRoute() {
  const location = useLocation();
  const navigate = useNavigate();
  const authQuery = useQuery({
    queryKey: queryKeys.authStatus,
    queryFn: api.authStatus
  });

  useEffect(() => {
    function handleAuthRequired() {
      navigate(`/login?next=${encodeURIComponent(`${location.pathname}${location.search}`)}`, { replace: true });
    }

    window.addEventListener("subconverter:auth-required", handleAuthRequired);
    return () => window.removeEventListener("subconverter:auth-required", handleAuthRequired);
  }, [location.pathname, location.search, navigate]);

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
