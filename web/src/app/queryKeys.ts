export const queryKeys = {
  authStatus: ["authStatus"] as const,
  status: ["status"] as const,
  config: ["config"] as const,
  previewNodes: (runtimeConfigRevision: string | undefined) => ["previewNodes", runtimeConfigRevision ?? "unknown"] as const
};
