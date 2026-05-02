import { useLocation } from "react-router-dom";
import { isApiError } from "../api/errors";
import type { Diagnostic, ValidateResult } from "../api/types";

export interface DiagnosticTarget {
  path: string;
  pointer: string;
  index?: number;
  field?: string;
}

interface LocationState {
  diagnosticPointer?: string;
}

export function getValidateResult(error: unknown): ValidateResult | null {
  if (!isApiError(error)) return null;
  if (isValidateResult(error.details)) return error.details;
  if (isValidateResult(error.payload)) return error.payload;
  return null;
}

export function isValidateResult(value: unknown): value is ValidateResult {
  return (
    typeof value === "object" &&
    value !== null &&
    "valid" in value &&
    "errors" in value &&
    "warnings" in value &&
    "infos" in value &&
    Array.isArray((value as ValidateResult).errors) &&
    Array.isArray((value as ValidateResult).warnings) &&
    Array.isArray((value as ValidateResult).infos)
  );
}

export function diagnosticsFromResult(result: ValidateResult | undefined): Diagnostic[] {
  if (!result) return [];
  return [...result.errors, ...result.warnings, ...result.infos];
}

export function diagnosticTarget(diagnostic: Diagnostic): DiagnosticTarget {
  const pointer = diagnostic.locator?.json_pointer ?? "/config";
  return targetForJsonPointer(pointer);
}

export function targetForJsonPointer(pointer: string): DiagnosticTarget {
  const normalized = pointer.startsWith("/config") ? pointer : "/config";

  const groups = normalized.match(/^\/config\/groups\/(\d+)(?:\/(key|value\/match|value\/strategy))?/);
  if (groups) return { path: "/groups", pointer: normalized, index: Number(groups[1]), field: groups[2] };

  const routing = normalized.match(/^\/config\/routing\/(\d+)(?:\/(key|value(?:\/\d+)?))?/);
  if (routing) return { path: "/routing", pointer: normalized, index: Number(routing[1]), field: routing[2] };

  const rulesets = normalized.match(/^\/config\/rulesets\/(\d+)(?:\/(key|value(?:\/\d+)?))?/);
  if (rulesets) return { path: "/rulesets", pointer: normalized, index: Number(rulesets[1]), field: rulesets[2] };

  const rules = normalized.match(/^\/config\/rules\/(\d+)/);
  if (rules) return { path: "/rules", pointer: normalized, index: Number(rules[1]) };

  if (normalized.startsWith("/config/sources")) return { path: "/sources", pointer: normalized };
  if (normalized.startsWith("/config/filters")) return { path: "/filters", pointer: normalized };
  if (normalized.startsWith("/config/fallback")) return { path: "/settings", pointer: normalized, field: "fallback" };
  if (normalized.startsWith("/config/base_url")) return { path: "/settings", pointer: normalized, field: "base_url" };
  if (normalized.startsWith("/config/templates")) return { path: "/settings", pointer: normalized, field: "templates" };

  return { path: "/sources", pointer: normalized };
}

export function useDiagnosticPointer(): string | undefined {
  const location = useLocation();
  const state = location.state as LocationState | null;
  return typeof state?.diagnosticPointer === "string" ? state.diagnosticPointer : undefined;
}

export function focusClassName(activePointer: string | undefined, candidates: string[], baseClass = ""): string {
  const focused = Boolean(activePointer && candidates.some((candidate) => pointerMatches(activePointer, candidate)));
  return [baseClass, focused ? "diagnostic-focus" : ""].filter(Boolean).join(" ");
}

function pointerMatches(activePointer: string, candidate: string): boolean {
  return activePointer === candidate || activePointer.startsWith(`${candidate}/`);
}
