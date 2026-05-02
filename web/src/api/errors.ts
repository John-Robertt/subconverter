import type { ApiErrorPayload, ValidateResult } from "./types";

export class ApiError extends Error {
  status: number;
  code: string;
  details?: unknown;
  payload?: ApiErrorPayload | ValidateResult | unknown;

  constructor(status: number, code: string, message: string, details?: unknown, payload?: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.details = details;
    this.payload = payload;
  }
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError;
}

export function getErrorMessage(error: unknown): string {
  if (isApiError(error)) {
    return `${error.status} ${error.code}: ${error.message}`;
  }

  if (error instanceof Error) {
    return error.message;
  }

  return "未知错误";
}
