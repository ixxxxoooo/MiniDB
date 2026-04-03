import { useUIStore } from "@/stores/ui";

// 统一标签页区域的错误反馈：控制台中文日志 + Toast 提示
export function reportTabError(options: {
  logTitle: string;
  toastMessage: string;
  error: unknown;
}) {
  const { logTitle, toastMessage, error } = options;
  console.error(logTitle, error);
  const message = error instanceof Error ? error.message : String(error ?? "未知错误");
  useUIStore.getState().addToast("error", `${toastMessage}: ${message}`);
}
