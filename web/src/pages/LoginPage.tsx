import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Eye, EyeOff, KeyRound, LogIn, Moon, Sun } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Navigate, useNavigate, useSearchParams } from "react-router-dom";
import { api } from "../api/client";
import { getErrorMessage, isApiError } from "../api/errors";
import { queryKeys } from "../app/queryKeys";
import { Button, Field, TextInput } from "../components/ui";
import { useTheme } from "../state/theme";
import { useToast } from "../state/toast";

export function LoginPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { pushToast } = useToast();
  const { resolvedTheme, setPreference } = useTheme();
  const next = searchParams.get("next") || "/sources";
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [setupToken, setSetupToken] = useState("");
  const [remember, setRemember] = useState(true);
  const [showPassword, setShowPassword] = useState(false);
  const [lockedUntil, setLockedUntil] = useState("");

  const authQuery = useQuery({
    queryKey: queryKeys.authStatus,
    queryFn: api.authStatus,
    retry: false
  });

  const mode = authQuery.data?.setup_required ? "setup" : "login";
  const passwordStrength = useMemo(() => getPasswordStrength(password), [password]);
  const passwordError = useMemo(() => {
    if (mode === "setup" && password.length > 0 && password.length < 12) {
      return "密码至少 12 位";
    }
    if (mode === "setup" && confirmPassword.length > 0 && confirmPassword !== password) {
      return "两次密码不一致";
    }
    return "";
  }, [confirmPassword, mode, password]);

  const networkErrorRef = useRef<unknown>(null);
  const refetchAuth = authQuery.refetch;
  useEffect(() => {
    if (!authQuery.isError) {
      networkErrorRef.current = null;
      return;
    }
    if (networkErrorRef.current === authQuery.error) return;
    networkErrorRef.current = authQuery.error;
    pushToast({
      kind: "error",
      title: "后端不可达",
      message: getErrorMessage(authQuery.error),
      persistent: true,
      action: { label: "重试", onClick: () => void refetchAuth() }
    });
  }, [authQuery.error, authQuery.isError, pushToast, refetchAuth]);

  const mutation = useMutation({
    mutationFn: async () => {
      if (mode === "setup") {
        if (password.length < 12) throw new Error("密码至少 12 位");
        if (password !== confirmPassword) throw new Error("两次密码不一致");
        return api.setup({ username, password, setup_token: setupToken });
      }
      return api.login({ username, password, remember });
    },
    onSuccess: async (result) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.authStatus });
      navigate(result.redirect || next, { replace: true });
    },
    onError: (error) => {
      if (isApiError(error)) {
        const until = typeof error.payload === "object" && error.payload && "until" in error.payload ? String(error.payload.until ?? "") : "";
        if (error.code === "auth_locked") {
          setLockedUntil(until);
          pushToast({
            kind: "warning",
            title: "账号已临时锁定",
            message: until ? `连续登录失败 · 请于 ${until} 后重试，或联系管理员重置。` : error.message,
            persistent: true
          });
          return;
        }
        if (error.code === "invalid_credentials") {
          const remaining = typeof error.payload === "object" && error.payload && "remaining" in error.payload ? String(error.payload.remaining ?? "") : "";
          pushToast({
            kind: "error",
            title: "用户名或密码错误",
            message: remaining ? `还可尝试 ${remaining} 次。` : error.message,
            persistent: true
          });
          return;
        }
        pushToast({
          kind: "error",
          title: mode === "setup" ? "Setup 失败" : "登录失败",
          message: `${error.status} ${error.code}: ${error.message}`,
          persistent: true
        });
        return;
      }

      const message = error instanceof Error ? error.message : "登录失败";
      pushToast({ kind: "error", title: mode === "setup" ? "Setup 失败" : "登录失败", message, persistent: true });
    }
  });

  if (authQuery.data?.authed && !authQuery.data.setup_required) {
    return <Navigate to={next} replace />;
  }

  const isLocked = Boolean(lockedUntil || authQuery.data?.locked_until);

  return (
    <main className="login-screen">
      <div className="login-theme-switcher" aria-label="主题切换">
        <button className={resolvedTheme === "light" ? "active" : ""} type="button" title="浅色" onClick={() => setPreference("light")}>
          <Sun size={14} aria-hidden="true" />
        </button>
        <button className={resolvedTheme === "dark" ? "active" : ""} type="button" title="深色" onClick={() => setPreference("dark")}>
          <Moon size={14} aria-hidden="true" />
        </button>
      </div>

      <section className="login-panel" aria-label={mode === "setup" ? "首次部署管理员账号" : "登录管理后台"}>
        <div className="login-wordmark">
          <span className="brand-mark" aria-hidden="true">S</span>
          <strong>subconverter</strong>
        </div>

        {mode === "setup" ? (
          <div className="setup-notice">
            <div className="setup-notice-row">
              <span className="setup-notice-mark" aria-hidden="true">✓</span>
              <h2>首次创建管理员</h2>
            </div>
            <p>检测到尚未初始化。请从服务日志复制 Setup Token，凭据将写入 auth.yaml。</p>
          </div>
        ) : (
          <div className="login-title-block">
            <h1>登录管理后台</h1>
            <p>使用管理员账号继续</p>
          </div>
        )}

        <form
          className="form-stack"
          onSubmit={(event) => {
            event.preventDefault();
            void mutation.mutateAsync();
          }}
        >
          {mode === "setup" && authQuery.data?.setup_token_required ? (
            <Field label="Setup Token" hint="自动生成的 token 只会打印在服务日志中，前端不会通过 HTTP 获取。">
              <TextInput value={setupToken} onChange={(event) => setSetupToken(event.target.value)} autoComplete="one-time-code" type="password" />
            </Field>
          ) : null}

          <Field label="用户名">
            <TextInput value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" placeholder={mode === "setup" ? "设置管理员用户名" : ""} />
          </Field>

          <Field label={mode === "setup" ? "设置密码" : "密码"} error={passwordError}>
            <div className="password-field">
              <TextInput
                type={showPassword ? "text" : "password"}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                autoComplete={mode === "setup" ? "new-password" : "current-password"}
                placeholder={mode === "setup" ? "至少 12 位，含字母与数字" : ""}
              />
              <button
                type="button"
                className="password-toggle"
                aria-label={showPassword ? "隐藏密码" : "显示密码"}
                title={showPassword ? "隐藏密码" : "显示密码"}
                tabIndex={-1}
                onClick={() => setShowPassword((value) => !value)}
              >
                {showPassword ? <EyeOff size={16} aria-hidden="true" /> : <Eye size={16} aria-hidden="true" />}
              </button>
            </div>
            {mode === "setup" ? (
              <div className="password-strength" aria-label={`密码强度：${passwordStrength.label}`}>
                {[0, 1, 2, 3].map((index) => (
                  <span key={index} className={index < passwordStrength.score ? "active" : ""} />
                ))}
                <small>{passwordStrength.label}</small>
              </div>
            ) : null}
          </Field>

          {mode === "setup" ? (
            <Field label="确认密码">
              <TextInput
                type={showPassword ? "text" : "password"}
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                autoComplete="new-password"
              />
            </Field>
          ) : (
            <label className="checkbox-row">
              <input type="checkbox" checked={remember} onChange={(event) => setRemember(event.target.checked)} />
              <span>记住我（保持登录 7 天）</span>
            </label>
          )}

          <Button type="submit" variant="primary" loading={mutation.isPending || authQuery.isLoading} disabled={isLocked || Boolean(passwordError)} icon={mode === "setup" ? <KeyRound size={16} /> : <LogIn size={16} />}>
            {mutation.isPending ? (mode === "setup" ? "正在创建..." : "正在验证...") : mode === "setup" ? "创建管理员并登录" : "登录"}
          </Button>
        </form>
      </section>

      <footer className="login-footer">
        <span>subconverter v{authQuery.data ? "2.0" : "0.9.4"}</span>
        <span className="login-footer-dot" aria-hidden="true" />
        <a href="https://github.com/Stealthy-Dev/subconverter" target="_blank" rel="noreferrer">文档</a>
        <span className="login-footer-dot" aria-hidden="true" />
        <a href="https://github.com/Stealthy-Dev/subconverter" target="_blank" rel="noreferrer">GitHub</a>
      </footer>
    </main>
  );
}

function getPasswordStrength(password: string) {
  let score = 0;
  if (password.length >= 8) score += 1;
  if (password.length >= 12) score += 1;
  if (/[a-z]/i.test(password) && /\d/.test(password)) score += 1;
  if (/[^a-z0-9]/i.test(password)) score += 1;
  const labels = ["弱", "一般", "良好", "强", "很强"];
  return { score, label: labels[score] ?? "弱" };
}
