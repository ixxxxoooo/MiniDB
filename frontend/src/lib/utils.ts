import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
}

export function formatDuration(ms: number): string {
  if (ms < 1) return "<1ms";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

export function generateId(): string {
  return Date.now().toString(36) + Math.random().toString(36).slice(2, 8);
}

export async function copyToClipboard(text: string): Promise<void> {
  const content = String(text ?? "");
  try {
    const runtimeClipboard = typeof window !== "undefined"
      ? (window as any)?.runtime?.ClipboardSetText
      : undefined;
    if (typeof runtimeClipboard === "function") {
      const ok = await Promise.resolve(runtimeClipboard(content));
      if (ok) return;
    }
  } catch {
    // ignore and continue fallback
  }

  try {
    if (typeof navigator !== "undefined" && navigator.clipboard && typeof navigator.clipboard.writeText === "function") {
      await navigator.clipboard.writeText(content);
      return;
    }
  } catch {
    // ignore and fallback to execCommand
  }

  if (typeof document === "undefined") {
    throw new Error("Clipboard API unavailable");
  }
  const textarea = document.createElement("textarea");
  textarea.value = content;
  textarea.setAttribute("readonly", "true");
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  textarea.style.left = "-9999px";
  textarea.style.top = "0";
  document.body.appendChild(textarea);
  textarea.focus();
  textarea.select();
  const ok = document.execCommand("copy");
  document.body.removeChild(textarea);
  if (!ok) {
    throw new Error("Copy failed");
  }
}

export function escapeSQL(value: unknown): string {
  if (value === null || value === undefined) return "NULL";
  if (typeof value === "number") return String(value);
  if (typeof value === "boolean") return value ? "1" : "0";
  return `'${String(value).replace(/'/g, "''")}'`;
}

export function rowToInsertSQL(
  table: string,
  row: Record<string, unknown>
): string {
  const columns = Object.keys(row).join(", ");
  const values = Object.values(row).map(escapeSQL).join(", ");
  return `INSERT INTO ${table} (${columns}) VALUES (${values});`;
}
