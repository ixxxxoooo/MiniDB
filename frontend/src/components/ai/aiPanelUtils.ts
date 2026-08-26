import type { AIStreamStep } from "@/components/ai/streamSteps";
import { migratePersistedKey } from "@/stores/persistMigration";

// ====== 类型定义 ======

export interface NextStepChoice {
  label: string;
  prompt: string;
}

export interface ChatMsg {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: number;
  errorType?: "request_failed";
  streaming?: boolean;
  failedSQL?: string;
  sqlError?: string;
  meta?: {
    tokenCount?: number;
    charCount?: number;
    answeredAt?: string;
    durationMs?: number;
  };
  steps?: AIStreamStep[];
  nextStepChoices?: NextStepChoice[];
}

export interface AIChatStreamEvent {
  requestId: string;
  type: "delta" | "done" | "error" | "status" | "tool_start" | "tool_args" | "tool_sql" | "tool_result" | "tool_error" | "reasoning" | "thinking" | "answer_start" | "final_answer";
  phase?: "reasoning" | "tool" | "answer";
  sequence?: number;
  delta?: string;
  content?: string;
  error?: string;
  toolName?: string;
  toolCallId?: string;
  toolState?: string;
  toolInput?: string;
  toolSql?: string;
  toolOutput?: string;
  durationMs?: number;
  thinkingContent?: string;
}

export interface ChatSession {
  id: string;
  title: string;
  messages: ChatMsg[];
  createdAt: number;
  updatedAt: number;
  connectionId?: string;
  database?: string;
}

export type MentionHighlightVariant = "input" | "user" | "default";
export type MentionKind = "table" | "tool";
export type MentionScope = MentionKind | "mixed";

export interface MentionToken {
  kind: MentionKind;
  name: string;
  raw: string;
}

export interface MentionCandidate {
  value: string;
  display: string;
  kind: MentionKind;
  description?: string;
}

export interface MentionDeleteResult {
  next: string;
  caret: number;
}

export interface MentionRange {
  start: number;
  end: number;
  index: number;
}

// ====== 常量 ======

export const STORAGE_KEY = "minidb-chat-sessions";
migratePersistedKey(STORAGE_KEY, "tableplus-ai-chat-sessions");
export const MAX_SESSIONS = 50;
export const MAX_CONTEXT_MESSAGES = 12;
export const MAX_MENTION_CANDIDATES = 100;
export const INPUT_MIN_HEIGHT = 40;
export const INPUT_MAX_HEIGHT = 156;

// ====== 工具函数 ======

export function generateSessionId() {
  return `session_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

export function loadSessions(): ChatSession[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return normalizeSessions(JSON.parse(raw));
  } catch {}
  return [];
}

export function saveSessions(sessions: ChatSession[]) {
  try {
    const trimmed = sessions.slice(0, MAX_SESSIONS);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(trimmed));
  } catch {}
}

export function generateTitle(msg: string): string {
  const trimmed = msg.trim().slice(0, 30);
  return trimmed + (msg.trim().length > 30 ? "..." : "");
}

export function buildContextMessages(messages: ChatMsg[]) {
  if (messages.length <= MAX_CONTEXT_MESSAGES) {
    return messages;
  }
  return messages.slice(-MAX_CONTEXT_MESSAGES);
}

export function generateRequestId() {
  return `req_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

export function generateMessageId() {
  return `msg_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

export function formatAnsweredAt(timestamp: number) {
  return new Date(timestamp).toLocaleTimeString("zh-CN", {
    hour12: false,
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

export function formatDuration(durationMs: number) {
  const totalSeconds = Math.max(0, Math.round(durationMs / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) return `${hours}h ${minutes}m ${seconds}s`;
  if (minutes > 0) return `${minutes}m ${seconds}s`;
  return `${seconds}s`;
}

export function estimateTokenCount(text: string) {
  const cjkChars = (text.match(/[\u4e00-\u9fff]/g) || []).length;
  const otherChars = Math.max(0, text.length - cjkChars);
  return Math.max(1, Math.round(cjkChars / 1.6 + otherChars / 4));
}

export function parseToolInput(input?: string): Record<string, any> {
  if (!input) return {};
  try {
    const parsed = JSON.parse(input);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : {};
  } catch {
    return {};
  }
}

export function formatToolTarget(value: unknown): string {
  if (Array.isArray(value)) {
    return value.map((item) => String(item || "").trim()).filter(Boolean).slice(0, 3).join(", ");
  }
  return String(value || "").trim();
}

export function compactSQL(sql?: string): string {
  const firstLine = String(sql || "").replace(/\s+/g, " ").trim();
  if (!firstLine) return "";
  return firstLine.length > 48 ? `${firstLine.slice(0, 48)}...` : firstLine;
}

// ====== Mention 操作 ======

export function parseMentionToken(text: string): MentionToken | null {
  const matched = text.match(/^@(tool|table):([^\s]+)$/);
  if (!matched) return null;
  return { kind: matched[1] as MentionKind, name: matched[2], raw: matched[0] };
}

export function deleteMentionTokenByKey(value: string, cursor: number, key: "Backspace" | "Delete"): MentionDeleteResult | null {
  const mentionPattern = /@(?:tool|table):[^\s]+/g;
  let matched: RegExpExecArray | null = null;
  while ((matched = mentionPattern.exec(value)) !== null) {
    const start = matched.index;
    const token = matched[0];
    const end = start + token.length;
    if (key === "Backspace") {
      if (cursor === end) return { next: value.slice(0, start) + value.slice(end), caret: start };
      if (cursor === end + 1 && value[cursor - 1] === " ") return { next: value.slice(0, start) + value.slice(cursor), caret: start };
    } else {
      if (cursor === start) {
        const cutEnd = value[end] === " " ? end + 1 : end;
        return { next: value.slice(0, start) + value.slice(cutEnd), caret: start };
      }
      if (cursor + 1 === start && value[cursor] === " ") return { next: value.slice(0, cursor) + value.slice(end), caret: cursor };
    }
  }
  return null;
}

export function deleteMentionByOccurrence(value: string, occurrence: number): MentionDeleteResult | null {
  if (occurrence < 0) return null;
  const mentionPattern = /@(?:tool|table):[^\s]+/g;
  let matched: RegExpExecArray | null = null;
  let idx = 0;
  while ((matched = mentionPattern.exec(value)) !== null) {
    if (idx !== occurrence) { idx++; continue; }
    let start = matched.index;
    let end = start + matched[0].length;
    if (value[end] === " ") end += 1;
    else if (start > 0 && value[start - 1] === " ") start -= 1;
    return { next: value.slice(0, start) + value.slice(end), caret: start };
  }
  return null;
}

export function findMentionRangeAtPosition(value: string, pos: number): MentionRange | null {
  const mentionPattern = /@(?:tool|table):[^\s]+/g;
  let matched: RegExpExecArray | null = null;
  let idx = 0;
  while ((matched = mentionPattern.exec(value)) !== null) {
    const start = matched.index;
    const end = start + matched[0].length;
    if (pos >= start && pos <= end) return { start, end, index: idx };
    idx++;
  }
  return null;
}

export function findMentionByExactRange(value: string, start: number, end: number): MentionRange | null {
  const mentionPattern = /@(?:tool|table):[^\s]+/g;
  let matched: RegExpExecArray | null = null;
  let idx = 0;
  while ((matched = mentionPattern.exec(value)) !== null) {
    const tokenStart = matched.index;
    const tokenEnd = tokenStart + matched[0].length;
    if (tokenStart === start && tokenEnd === end) return { start: tokenStart, end: tokenEnd, index: idx };
    idx++;
  }
  return null;
}

export function normalizeSessions(rawSessions: unknown): ChatSession[] {
  if (!Array.isArray(rawSessions)) return [];
  return rawSessions.flatMap((candidate) => {
    if (!candidate || typeof candidate !== "object") return [];
    const session = candidate as Partial<ChatSession>;
    if (typeof session.id !== "string" || !session.id) return [];
    const messages = Array.isArray(session.messages)
      ? session.messages.flatMap((candidateMessage) => {
        if (!candidateMessage || typeof candidateMessage !== "object") return [];
        const msg = candidateMessage as Partial<ChatMsg>;
        if ((msg.role !== "user" && msg.role !== "assistant") || typeof msg.content !== "string") return [];
        return [{ ...msg, id: typeof msg.id === "string" && msg.id ? msg.id : generateMessageId(), timestamp: typeof msg.timestamp === "number" ? msg.timestamp : Date.now(), streaming: false } as ChatMsg];
      })
      : [];
    const now = Date.now();
    return [{
      ...session,
      id: session.id,
      title: typeof session.title === "string" ? session.title : "",
      messages,
      createdAt: typeof session.createdAt === "number" ? session.createdAt : now,
      updatedAt: typeof session.updatedAt === "number" ? session.updatedAt : now,
    } as ChatSession];
  });
}
