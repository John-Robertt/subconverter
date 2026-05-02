export const queryKeys = {
  authStatus: ["authStatus"] as const,
  status: ["status"] as const,
  config: ["config"] as const,
  previewNodes: (runtimeConfigRevision: string | undefined) => ["previewNodes", runtimeConfigRevision ?? "unknown"] as const,
  previewGroups: (runtimeConfigRevision: string | undefined) => ["previewGroups", runtimeConfigRevision ?? "unknown"] as const,
  generatePreview: (runtimeConfigRevision: string | undefined, format: string) => ["generatePreview", runtimeConfigRevision ?? "unknown", format] as const
};
