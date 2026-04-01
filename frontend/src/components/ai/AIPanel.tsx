import React, { useState, useRef, useEffect, useCallback } from "react";
import {
  Sparkles,
  X,
  Send,
  Loader2,
  Copy,
  Check,
  Play,
  Trash2,
  Plus,
  History,
  ChevronLeft,
  MessageSquare,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn, copyToClipboard } from "@/lib/utils";
import { useTranslation } from "@/i18n";
import * as AIService from "../../../wailsjs/go/services/AIService";
import * as QueryService from "../../../wailsjs/go/services/QueryService";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { format as formatSQL } from "sql-formatter";
import Prism from "prismjs";
import "prismjs/components/prism-sql";
import "prismjs/components/prism-javascript";
import "prismjs/components/prism-typescript";
import "prismjs/components/prism-json";
import "prismjs/components/prism-bash";
import "prismjs/components/prism-markup";

interface ChatMsg {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: number;
  errorType?: "request_failed";
  streaming?: boolean;
  meta?: {
    tokenCount?: number;
    charCount?: number;
    answeredAt?: string;
    durationMs?: number;
  };
}

interface AIChatStreamEvent {
  requestId: string;
  type: "delta" | "done" | "error";
  delta?: string;
  content?: string;
  error?: string;
}

interface ChatSession {
  id: string;
  title: string;
  messages: ChatMsg[];
  createdAt: number;
  updatedAt: number;
  connectionId?: string;
  database?: string;
}

interface AIPanelProps {
  open: boolean;
  onClose: () => void;
  currentConnectionId?: string;
  currentDatabase?: string;
  currentTable?: string;
  width: number;
  onWidthChange: (w: number) => void;
}

const STORAGE_KEY = "tableplus-ai-chat-sessions";
const MAX_SESSIONS = 50;
const MAX_CONTEXT_MESSAGES = 12;

function generateSessionId() {
  return `session_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

function loadSessions(): ChatSession[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw);
  } catch {}
  return [];
}

function saveSessions(sessions: ChatSession[]) {
  try {
    const trimmed = sessions.slice(0, MAX_SESSIONS);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(trimmed));
  } catch {}
}

function generateTitle(msg: string): string {
  const trimmed = msg.trim().slice(0, 30);
  return trimmed + (msg.trim().length > 30 ? "..." : "");
}

function buildContextMessages(messages: ChatMsg[]) {
  // 仅保留最近若干条上下文，避免请求体过大导致响应慢或超出 token 限制
  if (messages.length <= MAX_CONTEXT_MESSAGES) {
    return messages;
  }
  return messages.slice(-MAX_CONTEXT_MESSAGES);
}

function generateRequestId() {
  return `req_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

function generateMessageId() {
  return `msg_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

function formatAnsweredAt(timestamp: number) {
  return new Date(timestamp).toLocaleTimeString("zh-CN", {
    hour12: false,
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function formatDuration(durationMs: number) {
  const totalSeconds = Math.max(0, Math.round(durationMs / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) {
    return `${hours}h ${minutes}m ${seconds}s`;
  }
  if (minutes > 0) {
    return `${minutes}m ${seconds}s`;
  }
  return `${seconds}s`;
}

function estimateTokenCount(text: string) {
  // 近似估算：英文约 4 chars/token，中文约 1.6 chars/token
  const cjkChars = (text.match(/[\u4e00-\u9fff]/g) || []).length;
  const otherChars = Math.max(0, text.length - cjkChars);
  return Math.max(1, Math.round(cjkChars / 1.6 + otherChars / 4));
}

function normalizeSessions(rawSessions: ChatSession[]): ChatSession[] {
  // 兼容历史本地会话数据，补齐消息 id，避免流式更新误命中
  return rawSessions.map((session) => ({
    ...session,
    messages: session.messages.map((msg) => ({
      ...msg,
      id: msg.id || generateMessageId(),
      streaming: false,
    })),
  }));
}

export function AIPanel({
  open,
  onClose,
  currentConnectionId,
  currentDatabase,
  width,
  onWidthChange,
}: AIPanelProps) {
  const [sessions, setSessions] = useState<ChatSession[]>(() => normalizeSessions(loadSessions()));
  const [activeSessionId, setActiveSessionId] = useState<string | null>(() => {
    const saved = normalizeSessions(loadSessions());
    return saved.length > 0 ? saved[0].id : null;
  });
  const [showHistory, setShowHistory] = useState(false);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [copiedMessageId, setCopiedMessageId] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const shouldAutoScrollRef = useRef(true);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const resizingRef = useRef(false);
  const { t } = useTranslation();

  const activeSession = sessions.find((s) => s.id === activeSessionId) || null;
  const messages = activeSession?.messages || [];

  useEffect(() => {
    saveSessions(sessions);
  }, [sessions]);

  useEffect(() => {
    if (!scrollRef.current || !shouldAutoScrollRef.current) return;
    scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [messages, loading]);

  const handleChatScroll = useCallback(() => {
    if (!scrollRef.current) return;
    const el = scrollRef.current;
    const distanceToBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
    // 用户离底部较近时才自动跟随，避免阅读中被“抢焦点”造成闪动
    shouldAutoScrollRef.current = distanceToBottom < 72;
  }, []);

  useEffect(() => {
    if (open && inputRef.current && !showHistory) inputRef.current.focus();
  }, [open, showHistory]);

  const handleResizeStart = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    resizingRef.current = true;
    const startX = e.clientX;
    const startWidth = width;
    const handleMove = (ev: MouseEvent) => {
      if (!resizingRef.current) return;
      const diff = startX - ev.clientX;
      const newWidth = Math.max(300, Math.min(800, startWidth + diff));
      onWidthChange(newWidth);
    };
    const handleUp = () => {
      resizingRef.current = false;
      document.removeEventListener("mousemove", handleMove);
      document.removeEventListener("mouseup", handleUp);
    };
    document.addEventListener("mousemove", handleMove);
    document.addEventListener("mouseup", handleUp);
  }, [width, onWidthChange]);

  const createNewSession = useCallback(() => {
    const session: ChatSession = {
      id: generateSessionId(),
      title: t("ai.newChat"),
      messages: [],
      createdAt: Date.now(),
      updatedAt: Date.now(),
      connectionId: currentConnectionId,
      database: currentDatabase,
    };
    setSessions((prev) => [session, ...prev]);
    setActiveSessionId(session.id);
    setShowHistory(false);
    setInput("");
  }, [currentConnectionId, currentDatabase, t]);

  const updateSessionMessages = useCallback((sessionId: string, msgs: ChatMsg[], title?: string) => {
    setSessions((prev) =>
      prev.map((s) =>
        s.id === sessionId
          ? { ...s, messages: msgs, updatedAt: Date.now(), ...(title ? { title } : {}) }
          : s
      )
    );
  }, []);

  const appendSessionMessage = useCallback((sessionId: string, msg: ChatMsg) => {
    setSessions((prev) =>
      prev.map((s) =>
        s.id === sessionId
          ? { ...s, messages: [...s.messages, msg], updatedAt: Date.now() }
          : s
      )
    );
  }, []);

  const deleteSession = useCallback((sessionId: string) => {
    setSessions((prev) => {
      const next = prev.filter((s) => s.id !== sessionId);
      if (activeSessionId === sessionId) {
        setActiveSessionId(next.length > 0 ? next[0].id : null);
      }
      return next;
    });
  }, [activeSessionId]);

  const requestAssistantReply = useCallback(async (sessionId: string, baseMessages: ChatMsg[]) => {
    const requestId = generateRequestId();
    const startedAt = Date.now();
    const placeholderId = generateMessageId();
    const placeholderTimestamp = Date.now();
    const streamBaseMessages: ChatMsg[] = [
      ...baseMessages,
      { id: placeholderId, role: "assistant", content: "", timestamp: placeholderTimestamp, streaming: true, meta: {} },
    ];

    updateSessionMessages(sessionId, streamBaseMessages);
    shouldAutoScrollRef.current = true;

    const updateStreamMessage = (
      updater: (content: string) => string,
      errorType?: ChatMsg["errorType"],
      streaming = true,
      meta?: ChatMsg["meta"]
    ) => {
      setSessions((prev) =>
        prev.map((session) => {
          if (session.id !== sessionId) return session;
          return {
            ...session,
            updatedAt: Date.now(),
            messages: session.messages.map((message) =>
              message.id === placeholderId
                ? { ...message, content: updater(message.content), errorType, streaming, meta: meta || message.meta }
                : message
            ),
          };
        })
      );
    };

    let streamBuffer = "";
    let flushTimer: ReturnType<typeof setTimeout> | null = null;
    const flushBuffer = () => {
      if (!streamBuffer) return;
      const delta = streamBuffer;
      streamBuffer = "";
      updateStreamMessage((prev) => prev + delta);
    };

    const offStream = EventsOn("ai:chat_stream", (event: AIChatStreamEvent) => {
      if (!event || event.requestId !== requestId) return;
      if (event.type === "delta" && event.delta) {
        streamBuffer += event.delta;
        if (!flushTimer) {
          flushTimer = setTimeout(() => {
            flushBuffer();
            flushTimer = null;
          }, 48);
        }
      }
      if (event.type === "error") {
        if (flushTimer) {
          clearTimeout(flushTimer);
          flushTimer = null;
        }
        flushBuffer();
        const errorText = `**${t("common.error")}**: ${event.error || t("ai.requestFailed")}`;
        const now = Date.now();
        updateStreamMessage(
          () => errorText,
          "request_failed",
          false,
          {
            tokenCount: estimateTokenCount(errorText),
            charCount: errorText.length,
            answeredAt: formatAnsweredAt(now),
            durationMs: now - startedAt,
          }
        );
      }
    });

    setLoading(true);
    try {
      const contextMessages = buildContextMessages(baseMessages);
      const apiMessages = contextMessages.map((m) => ({ role: m.role, content: m.content }));
      const result = await (AIService as any).ChatAIStream(
        currentConnectionId || "",
        currentDatabase || "",
        apiMessages,
        requestId
      );

      let aiContent = result?.content || t("ai.noContent");

      if (aiContent.includes("[AUTO_EXECUTE]") && currentConnectionId && currentDatabase) {
        aiContent = aiContent.replace("[AUTO_EXECUTE]", "");
        const sqlMatch = aiContent.match(/```sql\n([\s\S]*?)```/);
        if (sqlMatch) {
          const sql = sqlMatch[1].trim();
          try {
            const queryResult = await QueryService.ExecuteSQL(currentConnectionId, currentDatabase, sql);
            if (queryResult?.error) {
              aiContent += `\n\n**${t("ai.executeError")}**: ${queryResult.error}`;
            } else if (queryResult?.rows && queryResult.rows.length > 0) {
              const cols = queryResult.columns?.map((c: any) => c.name) || Object.keys(queryResult.rows[0]);
              const header = "| " + cols.join(" | ") + " |";
              const sep = "| " + cols.map(() => "---").join(" | ") + " |";
              const rows = queryResult.rows.slice(0, 50).map((row: any) =>
                "| " + cols.map((c: string) => {
                  const v = row[c];
                  return v === null || v === undefined ? "NULL" : String(v).substring(0, 80);
                }).join(" | ") + " |"
              );
              aiContent += `\n\n**${t("ai.queryResult")}** (${queryResult.total} rows, ${queryResult.duration}ms):\n\n${header}\n${sep}\n${rows.join("\n")}`;
              if (queryResult.rows.length > 50) {
                aiContent += `\n| ... |`;
              }
            } else {
              aiContent += `\n\n**${t("ai.executeSuccess")}**, ${queryResult?.total || 0} rows (${queryResult?.duration || 0}ms)`;
            }
          } catch (e: any) {
            aiContent += `\n\n**${t("ai.executeFailed")}**: ${e?.message || e}`;
          }
        }
      }

      if (flushTimer) {
        clearTimeout(flushTimer);
        flushTimer = null;
      }
      flushBuffer();
      const now = Date.now();
      updateStreamMessage(() => aiContent, undefined, false, {
        tokenCount: estimateTokenCount(aiContent),
        charCount: aiContent.length,
        answeredAt: formatAnsweredAt(now),
        durationMs: now - startedAt,
      });
    } catch (e: any) {
      if (flushTimer) {
        clearTimeout(flushTimer);
        flushTimer = null;
      }
      flushBuffer();
      const errorText = `**${t("common.error")}**: ${e?.message || t("ai.requestFailed")}`;
      const now = Date.now();
      updateStreamMessage(
        () => errorText,
        "request_failed",
        false,
        {
          tokenCount: estimateTokenCount(errorText),
          charCount: errorText.length,
          answeredAt: formatAnsweredAt(now),
          durationMs: now - startedAt,
        }
      );
    } finally {
      if (flushTimer) {
        clearTimeout(flushTimer);
        flushTimer = null;
      }
      offStream();
      setLoading(false);
    }
  }, [currentConnectionId, currentDatabase, t, updateSessionMessages]);

  const handleSend = async () => {
    const text = input.trim();
    if (!text || loading) return;

    let sessionId = activeSessionId;
    let currentMessages = [...messages];

    if (!sessionId) {
      const session: ChatSession = {
        id: generateSessionId(),
        title: generateTitle(text),
        messages: [],
        createdAt: Date.now(),
        updatedAt: Date.now(),
        connectionId: currentConnectionId,
        database: currentDatabase,
      };
      setSessions((prev) => [session, ...prev]);
      setActiveSessionId(session.id);
      sessionId = session.id;
      currentMessages = [];
    }

    const userMessage: ChatMsg = { id: generateMessageId(), role: "user", content: text, timestamp: Date.now() };
    const newMsgs = [...currentMessages, userMessage];

    const isFirst = currentMessages.length === 0;
    updateSessionMessages(sessionId, newMsgs, isFirst ? generateTitle(text) : undefined);
    shouldAutoScrollRef.current = true;
    setInput("");
    await requestAssistantReply(sessionId, newMsgs);
  };

  const handleCopy = async (messageId: string, content: string) => {
    await copyToClipboard(content);
    setCopiedMessageId(messageId);
    setTimeout(() => setCopiedMessageId(null), 2000);
  };

  const handleExecuteSQL = async (sql: string) => {
    if (!currentConnectionId || !currentDatabase || !activeSessionId) return;
    const currentSessionId = activeSessionId;
    setLoading(true);
    try {
      const result = await QueryService.ExecuteSQL(currentConnectionId, currentDatabase, sql);
      let content = "";
      if (result?.error) {
        content = `**${t("ai.executeError")}**: ${result.error}`;
      } else if (result?.rows && result.rows.length > 0) {
        const cols = result.columns?.map((c: any) => c.name) || Object.keys(result.rows[0]);
        const header = "| " + cols.join(" | ") + " |";
        const sep = "| " + cols.map(() => "---").join(" | ") + " |";
        const rows = result.rows.slice(0, 50).map((row: any) =>
          "| " + cols.map((c: string) => {
            const v = row[c];
            return v === null || v === undefined ? "NULL" : String(v).substring(0, 80);
          }).join(" | ") + " |"
        );
        content = `**${t("ai.queryResult")}** (${result.total} rows, ${result.duration}ms):\n\n${header}\n${sep}\n${rows.join("\n")}`;
      } else {
        content = `**${t("ai.executeSuccess")}**, ${result?.total || 0} rows (${result?.duration || 0}ms)`;
      }
      const now = Date.now();
      const execMsg: ChatMsg = {
        id: generateMessageId(),
        role: "assistant",
        content,
        timestamp: now,
        streaming: false,
        meta: {
          tokenCount: estimateTokenCount(content),
          charCount: content.length,
          answeredAt: formatAnsweredAt(now),
          durationMs: 0,
        },
      };
      appendSessionMessage(currentSessionId, execMsg);
    } catch (e: any) {
      const errText = `**${t("ai.executeFailed")}**: ${e?.message || e}`;
      const now = Date.now();
      const errMsg: ChatMsg = {
        id: generateMessageId(),
        role: "assistant",
        content: errText,
        timestamp: now,
        streaming: false,
        meta: {
          tokenCount: estimateTokenCount(errText),
          charCount: errText.length,
          answeredAt: formatAnsweredAt(now),
          durationMs: 0,
        },
      };
      appendSessionMessage(currentSessionId, errMsg);
    } finally {
      setLoading(false);
    }
  };

  const handleRetryAssistantMessage = useCallback((failedIdx: number) => {
    if (!activeSessionId || loading) return;
    const retryBaseMessages = messages.slice(0, failedIdx);
    const hasUserMessage = retryBaseMessages.some((m) => m.role === "user");
    if (!hasUserMessage) return;
    updateSessionMessages(activeSessionId, retryBaseMessages);
    shouldAutoScrollRef.current = true;
    requestAssistantReply(activeSessionId, retryBaseMessages);
  }, [activeSessionId, loading, messages, requestAssistantReply, updateSessionMessages]);

  const handleRetryFromUserMessage = useCallback((userIdx: number) => {
    if (!activeSessionId || loading) return;
    const baseMessages = messages.slice(0, userIdx + 1);
    const target = baseMessages[baseMessages.length - 1];
    if (!target || target.role !== "user") return;
    updateSessionMessages(activeSessionId, baseMessages);
    shouldAutoScrollRef.current = true;
    requestAssistantReply(activeSessionId, baseMessages);
  }, [activeSessionId, loading, messages, requestAssistantReply, updateSessionMessages]);

  const handleClearSession = useCallback(() => {
    if (activeSessionId) {
      updateSessionMessages(activeSessionId, []);
    }
  }, [activeSessionId, updateSessionMessages]);

  if (!open) return null;

  // 历史记录视图
  if (showHistory) {
    return (
      <div
        className={cn("flex flex-col border-l h-full relative", "bg-[var(--surface)] border-[var(--border-color)]")}
        style={{ width }}
      >
        {/* 美化拖拽条 */}
        <div
          className="absolute left-0 top-0 h-full w-[5px] cursor-col-resize z-10 group"
          onMouseDown={handleResizeStart}
        >
          <div className="absolute inset-y-0 left-1/2 -translate-x-1/2 w-px bg-transparent group-hover:bg-[var(--accent)]/40 transition-colors duration-200" />
          <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-1 h-6 rounded-full bg-transparent group-hover:bg-[var(--accent)]/50 transition-all duration-200" />
        </div>
        <div className="flex items-center justify-between px-3 py-2 border-b border-[var(--border-color)] flex-shrink-0">
          <div className="flex items-center gap-1.5">
            <button
              className="p-0.5 rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors"
              onClick={() => setShowHistory(false)}
            >
              <ChevronLeft className="h-4 w-4" />
            </button>
            <span className="text-sm font-medium">{t("ai.chatHistory")}</span>
            <span className="text-2xs text-[var(--fg-muted)]">({sessions.length})</span>
          </div>
          <Button variant="ghost" size="icon" className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)]" onClick={onClose}>
            <X className="h-3 w-3" />
          </Button>
        </div>

        <div className="flex-1 overflow-y-auto">
          {sessions.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-[var(--fg-muted)] text-sm">
              <MessageSquare className="h-8 w-8 mb-3 opacity-30" />
              <p>{t("ai.noHistory")}</p>
            </div>
          ) : (
            sessions.map((session) => (
              <div
                key={session.id}
                className={cn(
                  "flex items-center gap-2 px-3 py-2.5 border-b border-[var(--border-subtle)] cursor-pointer transition-colors group",
                  session.id === activeSessionId
                    ? "bg-[var(--accent)]/10 border-l-2 border-l-[var(--accent)]"
                    : "hover:bg-[var(--row-hover)]"
                )}
                onClick={() => {
                  setActiveSessionId(session.id);
                  setShowHistory(false);
                }}
              >
                <MessageSquare className="h-3.5 w-3.5 text-[var(--fg-muted)] flex-shrink-0" />
                <div className="flex-1 min-w-0">
                  <div className="text-xs font-medium truncate text-[var(--fg)]">
                    {session.title}
                  </div>
                  <div className="text-2xs text-[var(--fg-muted)] mt-0.5">
                    {session.messages.length} {t("ai.messages")} · {new Date(session.updatedAt).toLocaleString("zh-CN", { month: "numeric", day: "numeric", hour: "numeric", minute: "numeric" })}
                  </div>
                </div>
                <button
                  className="opacity-0 group-hover:opacity-100 p-1 rounded-[var(--radius-btn)] hover:bg-[var(--surface-secondary)] transition-opacity"
                  onClick={(e) => {
                    e.stopPropagation();
                    deleteSession(session.id);
                  }}
                  title={t("ai.deleteSession")}
                >
                  <Trash2 className="h-3 w-3 text-[var(--fg-muted)]" />
                </button>
              </div>
            ))
          )}
        </div>

        <div className="px-3 py-2 border-t border-[var(--border-color)]">
          <Button size="sm" className="w-full h-[var(--size-btn)] text-[length:var(--size-font-xs)]" onClick={createNewSession}>
            <Plus className="h-3 w-3 mr-1" /> {t("ai.newChat")}
          </Button>
        </div>
      </div>
    );
  }

  // 聊天主视图
  return (
    <div
      className={cn("flex flex-col border-l h-full relative", "bg-[var(--surface)] border-[var(--border-color)]")}
      style={{ width }}
    >
      {/* 美化拖拽条 */}
      <div
        className="absolute left-0 top-0 h-full w-[5px] cursor-col-resize z-10 group"
        onMouseDown={handleResizeStart}
      >
        <div className="absolute inset-y-0 left-1/2 -translate-x-1/2 w-px bg-transparent group-hover:bg-[var(--accent)]/40 transition-colors duration-200" />
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-1 h-6 rounded-full bg-transparent group-hover:bg-[var(--accent)]/50 transition-all duration-200" />
      </div>

      {/* 头部 */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-[var(--border-color)] flex-shrink-0">
        <div className="flex items-center gap-1.5">
          <Sparkles className="h-4 w-4 text-[var(--accent)]" />
          <span className="text-sm font-medium">{t("ai.title")}</span>
        </div>
        <div className="flex items-center gap-0.5">
          <Button variant="ghost" size="icon" className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)]" onClick={createNewSession} title={t("ai.newChat")}>
            <Plus className="h-3 w-3" />
          </Button>
          <Button variant="ghost" size="icon" className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)]" onClick={() => setShowHistory(true)} title={t("ai.chatHistory")}>
            <History className="h-3 w-3" />
          </Button>
          <Button variant="ghost" size="icon" className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)]" onClick={handleClearSession} title={t("ai.clearChat")}>
            <Trash2 className="h-3 w-3" />
          </Button>
          <Button variant="ghost" size="icon" className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)]" onClick={onClose}>
            <X className="h-3 w-3" />
          </Button>
        </div>
      </div>

      {/* 上下文信息 */}
      {currentDatabase && (
        <div className="px-3 py-1.5 text-2xs text-[var(--fg-muted)] border-b border-[var(--border-subtle)] flex-shrink-0 flex items-center gap-2">
          <span>{t("ai.database")}: {currentDatabase}</span>
          {activeSession && (
            <>
              <span className="text-[var(--border-color)]">|</span>
              <span className="truncate">{activeSession.title}</span>
            </>
          )}
        </div>
      )}

      {/* 对话区域 */}
      <div ref={scrollRef} onScroll={handleChatScroll} className="flex-1 overflow-y-auto p-3 space-y-3">
        {messages.length === 0 && !loading && (
          <div className="flex flex-col items-center justify-center h-full text-[var(--fg-muted)] text-sm">
            <Sparkles className="h-8 w-8 mb-3 opacity-30" />
            <p className="font-medium text-[var(--fg-secondary)]">{t("ai.dbAssistant")}</p>
            <p className="text-xs mt-1">{t("ai.dbAssistantDesc")}</p>
            <div className="mt-4 space-y-1 text-xs text-[var(--fg-muted)]">
              <p>{t("ai.tryLabel")}</p>
              <p className="italic">{t("ai.tryExample1")}</p>
              <p className="italic">{t("ai.tryExample2")}</p>
              <p className="italic">{t("ai.tryExample3")}</p>
            </div>
          </div>
        )}

        {messages.map((msg, idx) => (
          <div key={msg.id} className={cn("flex", msg.role === "user" ? "justify-end" : "justify-start")}>
            <div className={cn(
              "max-w-[92%] min-w-0",
              msg.role === "assistant" && "flex flex-col items-start"
            )}>
              <div className={cn(
                "px-3.5 py-2.5 text-[length:var(--size-font-xs)] leading-relaxed shadow-sm",
                "max-w-full min-w-0 overflow-hidden",
                msg.role === "user"
                  ? "bg-[var(--accent)] text-[var(--accent-fg)] rounded-2xl rounded-tr-sm"
                  : "bg-[var(--surface-elevated)] border border-[var(--border-subtle)] text-[var(--fg)] rounded-2xl rounded-tl-sm"
              )}>
              {msg.role === "assistant" ? (
                <div>
                  <MarkdownContent
                    content={msg.content}
                    onExecuteSQL={handleExecuteSQL}
                  />
                </div>
              ) : (
                <span className="whitespace-pre-wrap break-words">{msg.content}</span>
              )}
              </div>
              <div className="mt-1.5 flex items-center gap-1.5">
                {msg.role === "user" && (
                  <button
                    className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-[var(--radius-btn)] text-2xs text-[var(--accent)] hover:bg-[var(--sidebar-hover)] transition-colors disabled:opacity-60"
                    onClick={() => handleRetryFromUserMessage(idx)}
                    disabled={loading}
                    title={t("ai.retry")}
                  >
                    <Loader2 className={cn("h-3 w-3", loading && "animate-spin")} />
                    <span>{t("ai.retry")}</span>
                  </button>
                )}
                {(msg.role === "assistant" || msg.role === "user") && (
                  <button
                    className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-[var(--radius-btn)] text-2xs text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors"
                    onClick={() => handleCopy(msg.id, msg.content)}
                    title={t("common.copy")}
                  >
                    {copiedMessageId === msg.id ? <Check className="h-3 w-3 text-[var(--success)]" /> : <Copy className="h-3 w-3" />}
                    <span>{copiedMessageId === msg.id ? t("common.success") : t("common.copy")}</span>
                  </button>
                )}
                {msg.role === "assistant" && msg.errorType === "request_failed" && (
                  <button
                    className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-[var(--radius-btn)] text-2xs text-[var(--accent)] hover:bg-[var(--sidebar-hover)] transition-colors disabled:opacity-60"
                    onClick={() => handleRetryAssistantMessage(idx)}
                    disabled={loading}
                    title={t("ai.retry")}
                  >
                    <Loader2 className={cn("h-3 w-3", loading && "animate-spin")} />
                    <span>{t("ai.retry")}</span>
                  </button>
                )}
              </div>
              {msg.role === "assistant" && msg.meta && !msg.streaming && (
                <div className="mt-1 text-2xs text-[var(--fg-muted)] flex items-center gap-2">
                  <span>{t("ai.tokenCount")}: {msg.meta.tokenCount ?? 0}</span>
                  <span>{t("ai.charCount")}: {msg.meta.charCount ?? msg.content.length}</span>
                  <span>{t("ai.answerAt")}: {msg.meta.answeredAt || "-"}</span>
                  <span>{t("ai.duration")}: {formatDuration(msg.meta.durationMs || 0)}</span>
                </div>
              )}
            </div>
          </div>
        ))}

        {loading && !messages.some((m) => m.streaming) && (
          <div className="flex justify-start">
            <div className="bg-[var(--surface-elevated)] border border-[var(--border-subtle)] shadow-sm rounded-2xl rounded-tl-sm px-3.5 py-2.5 text-[length:var(--size-font-xs)] flex items-center gap-2 text-[var(--fg-secondary)]">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              {t("ai.thinking")}
            </div>
          </div>
        )}
      </div>

      {/* 输入区域 */}
      <div className="p-3 border-t border-[var(--border-color)] flex-shrink-0 bg-[var(--surface)]">
        <div className={cn(
          "relative flex items-end rounded-[var(--radius-input)] border border-[var(--border-color)] bg-[var(--surface)] transition-all",
          "focus-within:border-[var(--accent)] focus-within:ring-1 focus-within:ring-[var(--accent)] shadow-sm"
        )}>
          <textarea
            ref={inputRef}
            className={cn(
              "flex-1 max-h-32 min-h-[36px] resize-none bg-transparent px-3 py-2.5 text-[length:var(--size-font-xs)] leading-relaxed",
              "text-[var(--fg)] placeholder:text-[var(--fg-muted)] focus:outline-none scrollbar-auto-hide"
            )}
            placeholder={t("ai.placeholder")}
            value={input}
            rows={Math.min(5, Math.max(1, input.split("\n").length))}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              /* 回车 或 ⌘+回车 发送 */
              if (e.key === "Enter" && (e.metaKey || e.ctrlKey || !e.shiftKey)) {
                e.preventDefault();
                handleSend();
              }
            }}
          />
          <div className="p-1.5 flex-shrink-0 flex items-center justify-center">
            <Button
              size="icon"
              className={cn(
                "h-[var(--size-btn)] w-[var(--size-btn)] rounded-[var(--radius-btn)] transition-all duration-200",
                input.trim() && !loading
                  ? "bg-[var(--accent)] text-[var(--accent-fg)] hover:bg-[var(--accent-hover)] shadow-sm"
                  : "bg-[var(--surface-secondary)] text-[var(--fg-muted)] opacity-70 border border-[var(--border-color)]"
              )}
              onClick={handleSend}
              disabled={loading || !input.trim()}
            >
              <Send className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)]" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

function MarkdownContent({
  content,
  onExecuteSQL,
}: {
  content: string;
  onExecuteSQL?: (sql: string) => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="space-y-2 markdown-content max-w-full min-w-0 break-words">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          p: ({ children }) => <p className="whitespace-pre-wrap break-words">{children}</p>,
          ul: ({ children }) => <ul className="list-disc pl-5 space-y-1">{children}</ul>,
          ol: ({ children }) => <ol className="list-decimal pl-5 space-y-1">{children}</ol>,
          li: ({ children }) => <li className="break-words">{children}</li>,
          a: ({ href, children }) => (
            <a className="text-[var(--accent)] underline break-all" href={href} target="_blank" rel="noreferrer">
              {children}
            </a>
          ),
          table: ({ children }) => (
            <div className="w-full max-w-full overflow-x-auto border border-[var(--border-subtle)] rounded-[var(--radius-input)]">
              <table className="w-full text-left text-2xs">{children}</table>
            </div>
          ),
          th: ({ children }) => <th className="px-2 py-1 bg-[var(--surface)] border-b border-[var(--border-subtle)]">{children}</th>,
          td: ({ children }) => <td className="px-2 py-1 border-b border-[var(--border-subtle)] break-all">{children}</td>,
          code: ({ className, children, ...props }) => {
            const rawCode = String(children).replace(/\n$/, "");
            const matched = /language-(\w+)/.exec(className || "");
            const lang = (matched?.[1] || "").toLowerCase();
            const isInline = !className;

            if (isInline) {
              return (
                <code className="bg-[var(--surface)] px-1 py-0.5 rounded-[var(--radius-sm)] text-2xs font-mono break-all" {...props}>
                  {children}
                </code>
              );
            }

            let displayCode = rawCode;
            if (lang === "sql") {
              try {
                displayCode = formatSQL(rawCode, { language: "sql" });
              } catch {
                displayCode = rawCode;
              }
            }

            let html = "";
            try {
              const prismLang = Prism.languages[lang] ? lang : "sql";
              html = Prism.highlight(displayCode, Prism.languages[prismLang], prismLang);
            } catch {
              html = displayCode
                .replace(/&/g, "&amp;")
                .replace(/</g, "&lt;")
                .replace(/>/g, "&gt;");
            }

            const canExecute = lang === "sql" && onExecuteSQL;

            return (
              <div className="rounded-[var(--radius-input)] border border-[var(--border-color)] overflow-hidden my-2 max-w-full">
                <div className="flex items-center justify-between px-2 py-1 bg-[var(--surface)] text-2xs text-[var(--fg-muted)]">
                  <span>{lang || "code"}</span>
                  <div className="flex items-center gap-1">
                    {canExecute && (
                      <button
                        className="flex items-center gap-0.5 px-1.5 py-0.5 rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] text-[var(--accent)]"
                        onClick={() => onExecuteSQL(displayCode)}
                      >
                        <Play className="h-2.5 w-2.5" /> {t("ai.executeSQL")}
                      </button>
                    )}
                    <button
                      className="px-1.5 py-0.5 rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)]"
                      onClick={() => copyToClipboard(displayCode)}
                    >
                      <Copy className="h-2.5 w-2.5" />
                    </button>
                  </div>
                </div>
                <pre className="p-2 text-xs font-mono overflow-x-auto max-w-full bg-[var(--surface)]">
                  <code className="language-code" dangerouslySetInnerHTML={{ __html: html }} />
                </pre>
              </div>
            );
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
