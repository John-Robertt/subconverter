import { ApiError } from "./errors";
import type {
  AuthStatus,
  Config,
  ConfigSnapshot,
  GenerateFormat,
  GenerateLinkResponse,
  GroupPreviewResponse,
  LoginRequest,
  LoginResponse,
  NodePreviewResponse,
  ReloadResult,
  SetupRequest,
  StatusResponse,
  ValidateResult
} from "./types";

const JSON_CONTENT_TYPE = "application/json";

export interface RequestOptions extends Omit<RequestInit, "body"> {
  body?: unknown;
  skipAuthRedirect?: boolean;
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { body, headers, skipAuthRedirect: _skipAuthRedirect, ...init } = options;
  const requestHeaders = new Headers(headers);

  if (body !== undefined && !requestHeaders.has("Content-Type")) {
    requestHeaders.set("Content-Type", JSON_CONTENT_TYPE);
  }

  const response = await fetch(path, {
    ...init,
    credentials: "include",
    headers: requestHeaders,
    body: body === undefined ? undefined : JSON.stringify(body)
  });

  const parsed = await parseResponse(response);

  if (!response.ok) {
    const error = normalizeError(response.status, parsed);
    if (!options.skipAuthRedirect && (error.code === "auth_required" || error.code === "session_expired")) {
      window.dispatchEvent(new CustomEvent("subconverter:auth-required", { detail: error }));
    }
    throw error;
  }

  return parsed as T;
}

async function parseResponse(response: Response): Promise<unknown> {
  const contentType = response.headers.get("Content-Type") ?? "";
  if (response.status === 204) {
    return {};
  }

  if (contentType.includes(JSON_CONTENT_TYPE)) {
    return response.json();
  }

  const text = await response.text();
  return text.length > 0 ? text : {};
}

function normalizeError(status: number, parsed: unknown): ApiError {
  if (isObject(parsed) && "error" in parsed && isObject(parsed.error)) {
    const error = parsed.error;
    const code = stringValue(error.code) || `http_${status}`;
    const message = stringValue(error.message) || defaultErrorMessage(status);
    return new ApiError(status, code, message, error.details, error);
  }

  if (isValidateResult(parsed)) {
    const first = parsed.errors[0] ?? parsed.warnings[0] ?? parsed.infos[0];
    return new ApiError(status, first?.code ?? "validation_failed", first?.message ?? "配置校验失败", parsed, parsed);
  }

  if (typeof parsed === "string" && parsed.length > 0) {
    return new ApiError(status, `http_${status}`, parsed, parsed, parsed);
  }

  return new ApiError(status, `http_${status}`, defaultErrorMessage(status), parsed, parsed);
}

function defaultErrorMessage(status: number): string {
  if (status === 401) return "登录状态不可用";
  if (status === 409) return "配置保存冲突";
  if (status === 429) return "操作正在执行，请稍后重试";
  if (status === 502) return "远程来源拉取失败";
  return "请求失败";
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function stringValue(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}

function isValidateResult(value: unknown): value is ValidateResult {
  return (
    isObject(value) &&
    typeof value.valid === "boolean" &&
    Array.isArray(value.errors) &&
    Array.isArray(value.warnings) &&
    Array.isArray(value.infos)
  );
}

export const api = {
  authStatus: () => apiRequest<AuthStatus>("/api/auth/status", { skipAuthRedirect: true }),
  login: (body: LoginRequest) => apiRequest<LoginResponse>("/api/auth/login", { method: "POST", body, skipAuthRedirect: true }),
  setup: (body: SetupRequest) => apiRequest<LoginResponse>("/api/auth/setup", { method: "POST", body, skipAuthRedirect: true }),
  logout: () => apiRequest<{ success: boolean }>("/api/auth/logout", { method: "POST", skipAuthRedirect: true }),
  config: () => apiRequest<ConfigSnapshot>("/api/config"),
  saveConfig: (config_revision: string, config: Config) =>
    apiRequest<{ config_revision: string }>("/api/config", { method: "PUT", body: { config_revision, config } }),
  validateConfig: (config: Config) => apiRequest<ValidateResult>("/api/config/validate", { method: "POST", body: { config } }),
  reload: () => apiRequest<ReloadResult>("/api/reload", { method: "POST" }),
  status: () => apiRequest<StatusResponse>("/api/status"),
  healthz: () => apiRequest<string>("/healthz"),
  previewNodes: () => apiRequest<NodePreviewResponse>("/api/preview/nodes"),
  previewNodesDraft: (config: Config) => apiRequest<NodePreviewResponse>("/api/preview/nodes", { method: "POST", body: { config } }),
  previewGroups: () => apiRequest<GroupPreviewResponse>("/api/preview/groups"),
  previewGroupsDraft: (config: Config) => apiRequest<GroupPreviewResponse>("/api/preview/groups", { method: "POST", body: { config } }),
  generatePreview: (format: GenerateFormat) => apiRequest<string>(`/api/generate/preview?${buildGenerateQuery(format)}`),
  generatePreviewDraft: (format: GenerateFormat, config: Config) =>
    apiRequest<string>(`/api/generate/preview?${buildGenerateQuery(format)}`, { method: "POST", body: { config } }),
  generateLink: (format: GenerateFormat, filename: string, includeToken = true) =>
    apiRequest<GenerateLinkResponse>(`/api/generate/link?${buildGenerateQuery(format, filename, includeToken)}`)
};

export function buildGeneratePath(format: GenerateFormat, filename: string): string {
  return `/generate?${buildGenerateQuery(format, filename)}`;
}

function buildGenerateQuery(format: GenerateFormat, filename?: string, includeToken?: boolean): string {
  const params = new URLSearchParams({ format });
  const trimmedFilename = filename?.trim();
  if (trimmedFilename) {
    params.set("filename", trimmedFilename);
  }
  if (typeof includeToken === "boolean") {
    params.set("include_token", String(includeToken));
  }
  return params.toString();
}
