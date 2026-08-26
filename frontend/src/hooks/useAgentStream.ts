/**
 * useAgentStream —— Agent 会话流式请求 Hook（协议 v2 消费者）。
 * 职责：发起 ChatAIStream run、订阅 ai:agent_events 事件、维护时间线、取消。
 * 替代 AIPanel 内联的 EventsOn + reduceAIStreamSteps 逻辑（供 Phase3 拆分使用）。
 */
import { useCallback, useEffect, useRef, useState } from "react";
import * as AIService from "@/lib/wails/services/AIService";
import { ChatMessage } from "../../bindings/minidb/internal/ai/models";
import { EventsOn } from "@/lib/wails/runtime";
import {
  type AgentEvent,
  type AgentRunStartedPayload,
  type AgentToolResultPayload,
  type AgentRunDonePayload,
  toTimelineEntry,
  reduceAgentStream,
  type TimelineEntry,
} from "@/components/ai/agentEvents";

export interface AgentStreamState {
  runId: string | null;
  loading: boolean;
  error: string | null;
  started: AgentRunStartedPayload | null;
  thinking: string[];
  tools: AgentToolResultPayload[];
  answer: string;
  done: AgentRunDonePayload | null;
  /** 按 seq 排序的完整事件时间线 */
  timeline: TimelineEntry[];
  hasStreamed: boolean;
}

const initialState: AgentStreamState = {
  runId: null,
  loading: false,
  error: null,
  started: null,
  thinking: [],
  tools: [],
  answer: "",
  done: null,
  timeline: [],
  hasStreamed: false,
};

export function useAgentStream(connId: string, dbName: string) {
  const [state, setState] = useState<AgentStreamState>(initialState);
  const entriesRef = useRef<TimelineEntry[]>([]);
  const runIdRef = useRef<string | null>(null);
  const listenersRef = useRef<Set<() => void>>(new Set());

  const reset = useCallback(() => {
    entriesRef.current = [];
    runIdRef.current = null;
    setState(initialState);
  }, []);

  const start = useCallback(
    async (messages: Array<{ role: string; content: string }>, sessionId = "") => {
      const runId = `run_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
      runIdRef.current = runId;
      entriesRef.current = [];
      setState({ ...initialState, runId, loading: true });

      const requestId = runId;
      const off = EventsOn<AgentEvent>("ai:agent_events", (ev) => {
        if (!ev || ev.runId !== requestId) return;
        entriesRef.current = [...entriesRef.current, toTimelineEntry(ev)];
        const reduced = reduceAgentStream(entriesRef.current);
        setState({
          runId,
          loading: !reduced.done && !reduced.error,
          error: reduced.error?.message ?? null,
          started: reduced.started ?? null,
          thinking: reduced.thinking,
          tools: reduced.tools,
          answer: reduced.answer,
          done: reduced.done ?? null,
          timeline: entriesRef.current,
          hasStreamed: true,
        });
      });
      listenersRef.current.add(off);

      try {
        const chatMessages = messages.map((m) => new ChatMessage({ role: m.role, content: m.content }));
        await AIService.ChatAIStream(connId, dbName, chatMessages, requestId, sessionId);
      } catch (e: any) {
        setState((prev) => ({
          ...prev,
          loading: false,
          error: e?.message ?? "请求失败",
        }));
      }
    },
    [connId, dbName]
  );

  const stop = useCallback(async () => {
    const runId = runIdRef.current;
    if (!runId) return;
    try {
      await AIService.CancelChatStream(runId);
    } catch (e) {
      console.warn("[useAgentStream] cancel failed:", e);
    }
    setState((prev) => (prev.runId === runId ? { ...prev, loading: false } : prev));
  }, []);

  useEffect(() => {
    const listeners = listenersRef.current;
    return () => {
      for (const off of listeners) {
        try {
          off();
        } catch {
          /* ignore */
        }
      }
      listeners.clear();
    };
  }, []);

  return { ...state, start, stop, reset };
}