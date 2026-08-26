/**
 * Agent 事件协议 v2 —— 与 internal/agent/events.go 镜像。
 * 纯函数 reducer：把 v2 事件流收敛为 UI 时间线，可在 vitest 中单测。
 */
import { stripStreamMetaBlocks } from "@/components/ai/streamMeta";

export const AGENT_PROTOCOL_VERSION = 2;

export type AgentEventType =
  | "run.started"
  | "step.thinking"
  | "tool.requested"
  | "tool.result"
  | "answer.delta"
  | "run.done"
  | "run.error"
  | "approval.requested";

export interface AgentEvent {
  v: number;
  runId: string;
  seq: number;
  type: AgentEventType;
  phase?: string;
  payload: any;
}

export interface AgentToolResultPayload {
  callId: string;
  toolName: string;
  ok: boolean;
  kind: "rows" | "text" | "error";
  columns?: string[];
  rows?: Record<string, unknown>[];
  text?: string;
  truncated?: boolean;
  durationMs?: number;
  errorCode?: string;
  data?: Record<string, unknown>;
}

export interface AgentRunDonePayload {
  content: string;
  rounds: number;
  usage?: { promptTokens?: number; completionTokens?: number; totalTokens?: number; durationMs?: number };
  suggestions?: Array<{ label: string; prompt: string }>;
}

export interface AgentRunStartedPayload {
  model: string;
  schemaMode?: string;
  dbType?: string;
}

export interface AgentToolCallPayload {
  callId: string;
  toolName: string;
  arguments: Record<string, unknown>;
}

// ---- 纯函数 reducer：事件流 → 时间线条目 ----

export interface TimelineEntry {
  seq: number;
  type: AgentEventType;
  at: number;
  payload: any;
}

export function reduceAgentStream(entries: TimelineEntry[]): {
  thinking: string[];
  tools: AgentToolResultPayload[];
  answer: string;
  done?: AgentRunDonePayload;
  error?: { code?: string; message: string };
  started?: AgentRunStartedPayload;
} {
  const thinking: string[] = [];
  const tools: AgentToolResultPayload[] = [];
  let answer = "";
  let done: AgentRunDonePayload | undefined;
  let error: { code?: string; message: string } | undefined;
  let started: AgentRunStartedPayload | undefined;

  const sorted = [...entries].sort((a, b) => a.seq - b.seq);
  for (const entry of sorted) {
    switch (entry.type) {
      case "run.started":
        started = entry.payload as AgentRunStartedPayload;
        break;
      case "step.thinking":
        if (typeof entry.payload === "string") thinking.push(entry.payload);
        break;
      case "tool.requested":
        break;
      case "tool.result":
        tools.push(entry.payload as AgentToolResultPayload);
        break;
      case "answer.delta":
        if (typeof entry.payload === "string") answer += entry.payload;
        break;
      case "run.done":
        done = entry.payload as AgentRunDonePayload;
        break;
      case "run.error":
        error = entry.payload as { code?: string; message: string };
        break;
    }
  }
  return { thinking, tools, answer, done, error, started };
}

// ---- 把事件源统一为 TimelineEntry（供 hook 使用） ----

export function toTimelineEntry(ev: AgentEvent, at = Date.now()): TimelineEntry {
  return { seq: ev.seq, type: ev.type, at, payload: ev.payload };
}

/**
 * 把协议 v2 事件映射为兼容旧 AIStreamStepEvent 的（过渡层：让旧 reducer 仍能消费）。
 * 仅用于逐步迁移；新增 UI 直接消费 reduceAgentStream。
 */
export function agentEventToLegacy(ev: AgentEvent): {
  requestId: string;
  type: string;
  sequence?: number;
  delta?: string;
  content?: string;
  error?: string;
  toolName?: string;
  toolCallId?: string;
  toolState?: string;
  toolInput?: string;
  toolOutput?: string;
  durationMs?: number;
  thinkingContent?: string;
  columns?: string[];
  rows?: Record<string, unknown>[];
  truncated?: boolean;
} | null {
  const base = {
    requestId: ev.runId,
    sequence: ev.seq,
  };
  switch (ev.type) {
    case "step.thinking":
      return { ...base, type: "thinking", thinkingContent: String(ev.payload ?? "") };
    case "tool.requested": {
      const p = ev.payload as AgentToolCallPayload;
      return {
        ...base,
        type: "tool_start",
        toolName: p?.toolName,
        toolCallId: p?.callId,
        toolState: "running",
        toolInput: JSON.stringify(p?.arguments ?? {}),
      };
    }
    case "tool.result": {
      const p = ev.payload as AgentToolResultPayload;
      const isErr = p?.kind === "error" || !p?.ok;
      return {
        ...base,
        type: isErr ? "tool_error" : "tool_result",
        toolName: p?.toolName,
        toolCallId: p?.callId,
        toolState: isErr ? "error" : "success",
        toolOutput: p?.text ?? (p?.rows ? `rows=${p.rows.length}` : ""),
        durationMs: p?.durationMs,
        // 结构化表格结果透传给 steps reducer → 原生表格渲染
        columns: p?.columns,
        rows: p?.rows,
        truncated: p?.truncated,
      };
    }
    case "answer.delta":
      return { ...base, type: "delta", delta: String(ev.payload ?? "") };
    case "run.done":
      return { ...base, type: "done", content: (ev.payload as AgentRunDonePayload)?.content };
    case "run.error":
      // 后端已把 429 等转为友好中文消息，直接透传 message
      return { ...base, type: "error", error: (ev.payload as { message?: string })?.message ?? "请求失败" };
    default:
      return null;
  }
}

// 复用正文清理逻辑（后续随旧通道移除一起清理）
export { stripStreamMetaBlocks };