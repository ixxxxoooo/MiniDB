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
  RotateCcw,
  ArrowRightToLine,
  Wrench,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTabsStore } from "@/stores/tabs";
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
  // SQL 执行失败时保存原始 SQL 和错误信息，用于 AI 修复
  failedSQL?: string;
  sqlError?: string;
  meta?: {
    tokenCount?: number;
    charCount?: number;
    answeredAt?: string;
    durationMs?: number;
  };
}

interface AIChatStreamEvent {
  requestId: string;
  type: "delta" | "done" | "error" | "status";
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

interface ThinkingTimelineItem {
  status: string;
  at: number;
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
// SQL 自动执行失败后最大自动修复重试次数
const MAX_AUTO_FIX_RETRIES = 3;

// 仅允许查询/分析语句自动执行，避免误执行有副作用的 SQL
function stripLeadingSQLComments(sql: string): string {
  let text = sql.trim();
  while (text) {
    const prev = text;
    text = text.replace(/^\s*\/\*[\s\S]*?\*\/\s*/g, "");
    text = text.replace(/^\s*--[^\n]*\n\s*/g, "");
    if (text === prev) break;
  }
  return text.trim();
}

function getSQLLeadingVerb(sql: string): string {
  const cleaned = stripLeadingSQLComments(sql).toLowerCase();
  const matched = cleaned.match(/^[a-z]+/);
  return matched?.[0] || "";
}

function checkAutoExecutableSQL(sql: string): { allowed: boolean; reason?: string } {
  const verb = getSQLLeadingVerb(sql);
  if (!verb) {
    return {
      allowed: false,
      reason: "SQL 为空或无法识别语句类型",
    };
  }

  const allowVerbs = new Set(["select", "show", "desc", "describe", "explain", "with"]);
  if (allowVerbs.has(verb)) {
    return { allowed: true };
  }

  const riskyVerbs = new Set([
    "insert", "update", "delete", "replace", "create", "alter", "drop", "truncate", "rename",
    "grant", "revoke", "call", "set", "use", "begin", "start", "commit", "rollback", "lock", "unlock",
  ]);
  if (riskyVerbs.has(verb)) {
    return {
      allowed: false,
      reason: `检测到 ${verb.toUpperCase()} 语句，可能修改数据或结构`,
    };
  }

  return {
    allowed: false,
    reason: `语句类型 ${verb.toUpperCase()} 不在自动执行白名单内`,
  };
}

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
  const [thinkingStatus, setThinkingStatus] = useState<string>("");
  const [thinkingTimeline, setThinkingTimeline] = useState<ThinkingTimelineItem[]>([]);
  const [showThinkingTimeline, setShowThinkingTimeline] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const shouldAutoScrollRef = useRef(true);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const resizingRef = useRef(false);
  const { t } = useTranslation();

  // 进度状态文案映射
  const statusTextMap: Record<string, string> = {
    loading_schema: t("ai.statusLoadingSchema"),
    calling_ai: t("ai.statusCallingAI"),
    executing_sql: t("ai.statusExecutingSQL"),
    auto_fixing: t("ai.statusAutoFixing"),
  };

  const recordThinkingStep = useCallback((step: string) => {
    if (!step) return;
    setThinkingTimeline((prev) => {
      const last = prev[prev.length - 1];
      // 去重：连续重复步骤不重复追加，避免“刚开始重复显示”
      if (last && last.status === step) return prev;
      return [...prev, { status: step, at: Date.now() }];
    });
  }, []);

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

  /**
   * 执行 SQL 并在失败时自动反馈错误给 AI 进行流式修复重试
   * 修复过程也是流式输出，用户可以实时看到 AI 的分析和修复过程
   * 最多重试 MAX_AUTO_FIX_RETRIES 次
   */
  const executeAndAutoFix = useCallback(async (
    sql: string,
    initialContent: string,
    _sessionId: string,
    baseMessages: ChatMsg[],
    _placeholderId: string,
    _startedAt: number,
    updateStreamMessage: (
      updater: (content: string) => string,
      errorType?: ChatMsg["errorType"],
      streaming?: boolean,
      meta?: ChatMsg["meta"]
    ) => void,
    _flushTimer: ReturnType<typeof setTimeout> | null,
    flushBuffer: () => void,
  ): Promise<{ content: string; flushTimer: ReturnType<typeof setTimeout> | null }> => {
    let content = initialContent;
    let currentSQL = sql;
    let currentMessages = [...baseMessages];
    let flushTimer = _flushTimer;

    for (let attempt = 0; attempt <= MAX_AUTO_FIX_RETRIES; attempt++) {
      // 获取 SQL 执行错误信息的辅助函数
      const getErrorMsg = async (): Promise<string | null> => {
        try {
          const queryResult = await QueryService.ExecuteSQL(
            currentConnectionId || "", currentDatabase || "", currentSQL
          );
          if (queryResult?.error) return queryResult.error;
          // 执行成功，渲染结果
          if (queryResult?.rows && queryResult.rows.length > 0) {
            const cols = queryResult.columns?.map((c: any) => c.name) || Object.keys(queryResult.rows[0]);
            const header = "| " + cols.join(" | ") + " |";
            const sep = "| " + cols.map(() => "---").join(" | ") + " |";
            const rows = queryResult.rows.slice(0, 50).map((row: any) =>
              "| " + cols.map((c: string) => {
                const v = row[c]; return v === null || v === undefined ? "NULL" : String(v).substring(0, 80);
              }).join(" | ") + " |"
            );
            const successPrefix = attempt > 0
              ? `\n\n---\n\n**✅ ${t("ai.autoFixSuccess")}**\n\n`
              : "\n\n";
            content += `${successPrefix}**${t("ai.queryResult")}** (${queryResult.total} rows, ${queryResult.duration}ms):\n\n${header}\n${sep}\n${rows.join("\n")}`;
            if (queryResult.rows.length > 50) content += `\n| ... |`;
          } else {
            const successPrefix = attempt > 0
              ? `\n\n---\n\n**✅ ${t("ai.autoFixSuccess")}**\n\n`
              : "\n\n";
            content += `${successPrefix}**${t("ai.executeSuccess")}**, ${queryResult?.total || 0} rows (${queryResult?.duration || 0}ms)`;
          }
          return null;
        } catch (e: any) {
          return e?.message || String(e);
        }
      };

      const errorMsg = await getErrorMsg();
      if (errorMsg === null) break; // 执行成功

      // 执行失败，判断是否还能重试
      if (attempt >= MAX_AUTO_FIX_RETRIES) {
        content += `\n\n---\n\n**❌ ${t("ai.autoFixFailed")}**\n\n\`${errorMsg}\``;
        break;
      }

      setThinkingStatus("auto_fixing");
      recordThinkingStep("auto_fixing");

      // 显示醒目的错误和修复进度提示
      const fixAttemptLabel = t("ai.autoFixAttempt")
        .replace("{attempt}", String(attempt + 1))
        .replace("{max}", String(MAX_AUTO_FIX_RETRIES));
      content += `\n\n---\n\n**⚠️ ${t("ai.sqlErrorFeedback")}**\n\n\`${errorMsg}\`\n\n**🔧 ${fixAttemptLabel}**\n\n`;

      if (flushTimer) { clearTimeout(flushTimer); flushTimer = null; }
      flushBuffer();
      updateStreamMessage(() => content, undefined, true);

      // 构建修复请求上下文
      const assistantMsg: ChatMsg = {
        id: generateMessageId(), role: "assistant", content,
        timestamp: Date.now(), streaming: false,
      };
      const errorFeedbackMsg: ChatMsg = {
        id: generateMessageId(), role: "user",
        content: `[SQL_ERROR] 执行以下 SQL 时报错：\n\`\`\`sql\n${currentSQL}\n\`\`\`\n错误信息: ${errorMsg}\n\n请分析错误原因并生成修复后的 SQL。`,
        timestamp: Date.now(),
      };
      currentMessages = [...currentMessages, assistantMsg, errorFeedbackMsg];

      // 流式请求 AI 修复：用户可以实时看到 AI 的分析和修复过程
      const fixRequestId = generateRequestId();
      const fixContentRef = { value: "" };
      let fixResolve: (val: string) => void;
      const fixPromise = new Promise<string>((resolve) => { fixResolve = resolve; });

      // 流式事件监听：实时将 AI 修复输出追加到当前气泡中
      let fixStreamBuffer = "";
      let fixFlushTimer: ReturnType<typeof setTimeout> | null = null;
      const fixFlushBuffer = () => {
        if (!fixStreamBuffer) return;
        const delta = fixStreamBuffer;
        fixStreamBuffer = "";
        fixContentRef.value += delta;
        const snapshot = content + fixContentRef.value;
        updateStreamMessage(() => snapshot, undefined, true);
      };

      const offFixStream = EventsOn("ai:chat_stream", (event: AIChatStreamEvent) => {
        if (!event || event.requestId !== fixRequestId) return;
        if (event.type === "delta" && event.delta) {
          fixStreamBuffer += event.delta;
          if (!fixFlushTimer) {
            fixFlushTimer = setTimeout(() => {
              fixFlushBuffer();
              fixFlushTimer = null;
            }, 48);
          }
        }
      });

      // 发起 AI 修复请求
      try {
        const contextMsgs = buildContextMessages(currentMessages);
        const apiMsgs = contextMsgs.map((m) => ({ role: m.role, content: m.content }));
        const fixResult = await (AIService as any).ChatAIStream(
          currentConnectionId || "", currentDatabase || "",
          apiMsgs, fixRequestId
        );
        // 刷新剩余缓冲
        if (fixFlushTimer) { clearTimeout(fixFlushTimer); fixFlushTimer = null; }
        fixFlushBuffer();
        fixResolve!(fixResult?.content || fixContentRef.value);
      } catch {
        if (fixFlushTimer) { clearTimeout(fixFlushTimer); fixFlushTimer = null; }
        fixFlushBuffer();
        fixResolve!(fixContentRef.value);
      } finally {
        offFixStream();
      }

      let fixContent = await fixPromise;
      fixContent = fixContent.replace("[AUTO_EXECUTE]", "").replace(/\[AUTO_EXECUTE\]/g, "");
      const fixSqlMatch = fixContent.match(/```sql\n([\s\S]*?)```/);

      // 用最终的修复内容更新气泡
      content += fixContent;
      updateStreamMessage(() => content, undefined, true);

      if (fixSqlMatch) {
        currentSQL = fixSqlMatch[1].trim();
        continue;
      } else {
        break;
      }
    }

    return { content, flushTimer };
  }, [currentConnectionId, currentDatabase, t]);

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
    setThinkingTimeline([]);
    setShowThinkingTimeline(false);
    // 仅当用户当前已在底部附近时才自动跟随，避免打断手动浏览
    if (scrollRef.current) {
      const el = scrollRef.current;
      const distanceToBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
      shouldAutoScrollRef.current = distanceToBottom < 72;
    }

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
      // 处理进度状态事件
      if (event.type === "status" && event.delta) {
        setThinkingStatus(event.delta);
        recordThinkingStep(event.delta);
        return;
      }
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

      // 自动执行 SQL 并支持失败后自动修复重试
      if (aiContent.includes("[AUTO_EXECUTE]") && currentConnectionId && currentDatabase) {
        aiContent = aiContent.replace("[AUTO_EXECUTE]", "");
        const sqlMatch = aiContent.match(/```sql\n([\s\S]*?)```/);
        if (sqlMatch) {
          const sql = sqlMatch[1].trim();
          const safeCheck = checkAutoExecutableSQL(sql);
          if (safeCheck.allowed) {
            setThinkingStatus("executing_sql");
            recordThinkingStep("executing_sql");
            const execResult = await executeAndAutoFix(
              sql, aiContent, sessionId, baseMessages, placeholderId, startedAt,
              updateStreamMessage, flushTimer, flushBuffer
            );
            aiContent = execResult.content;
            if (execResult.flushTimer !== undefined) flushTimer = execResult.flushTimer;
          } else {
            // 显示跳过自动执行原因，避免对库产生潜在影响
            aiContent += `\n\n---\n\n**⚠️ ${t("ai.autoExecuteSkippedUnsafe")}**\n\n${t("ai.autoExecuteSkippedUnsafeReason").replace("{reason}", safeCheck.reason || "")}`;
            console.warn("[AIPanel] 已跳过自动执行 SQL:", safeCheck.reason);
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
      setThinkingStatus("");
      setThinkingTimeline([]);
      setShowThinkingTimeline(false);
    }
  }, [currentConnectionId, currentDatabase, executeAndAutoFix, recordThinkingStep, t, updateSessionMessages]);

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

  const { tabs, activeTabId, addTab, updateTab } = useTabsStore();

  const handleApplyAndRunSQL = useCallback((sql: string) => {
    // 始终使用 AI 面板当前的连接和库，避免复用指向旧连接的 tab
    const connId = currentConnectionId;
    const dbName = currentDatabase;
    if (!connId || !dbName) return;

    const activeWsTab = tabs.find(t => t.id === activeTabId);
    let targetTabId = "";

    // 仅当当前活跃 tab 是 query 类型且连接+库完全匹配时才复用
    if (activeWsTab && activeWsTab.type === "query" && activeWsTab.connectionId === connId && activeWsTab.database === dbName) {
      targetTabId = activeWsTab.id;
    }

    if (targetTabId) {
      updateTab(targetTabId, { sql });
      useTabsStore.getState().setActiveTab(targetTabId);
      setTimeout(() => window.dispatchEvent(new CustomEvent("tableplus-ai:run-sql", { detail: { tabId: targetTabId, sql } })), 50);
    } else {
      // 不再搜索其他 tab，直接创建新 tab 确保连接正确
      const newId = addTab({
        type: "query",
        title: t("tabs.newQuery"),
        connectionId: connId,
        database: dbName,
        closable: true,
        sql,
      });
      // 新建 tab 后先激活，再触发执行，避免事件在组件尚未挂载时丢失
      useTabsStore.getState().setActiveTab(newId);
      setTimeout(() => window.dispatchEvent(new CustomEvent("tableplus-ai:run-sql", { detail: { tabId: newId, sql } })), 160);
    }
  }, [currentConnectionId, currentDatabase, tabs, activeTabId, updateTab, addTab, t]);

  const handleExecuteSQL = useCallback(async (sql: string) => {
    if (!currentConnectionId || !currentDatabase || !activeSessionId) return;
    const currentSessionId = activeSessionId;
    setLoading(true);
    try {
      const result = await QueryService.ExecuteSQL(currentConnectionId, currentDatabase, sql);
      let content = "";
      let failedSQL: string | undefined;
      let sqlError: string | undefined;

      if (result?.error) {
        content = `**${t("ai.executeError")}**: ${result.error}`;
        failedSQL = sql;
        sqlError = result.error;
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
        failedSQL,
        sqlError,
        meta: {
          tokenCount: estimateTokenCount(content),
          charCount: content.length,
          answeredAt: formatAnsweredAt(now),
          durationMs: 0,
        },
      };
      appendSessionMessage(currentSessionId, execMsg);
    } catch (e: any) {
      const errorMessage = e?.message || String(e);
      const errText = `**${t("ai.executeFailed")}**: ${errorMessage}`;
      const now = Date.now();
      const errMsg: ChatMsg = {
        id: generateMessageId(),
        role: "assistant",
        content: errText,
        timestamp: now,
        streaming: false,
        failedSQL: sql,
        sqlError: errorMessage,
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
  }, [currentConnectionId, currentDatabase, activeSessionId, t, appendSessionMessage]);

  // 手动执行 SQL 失败后点击「AI 修复」按钮：将错误信息作为用户消息发给 AI
  const handleFixWithAI = useCallback(async (failedSQL: string, sqlError: string) => {
    if (!activeSessionId || loading) return;
    const errorFeedbackContent = `[SQL_ERROR] 执行以下 SQL 时报错：\n\`\`\`sql\n${failedSQL}\n\`\`\`\n错误信息: ${sqlError}\n\n请分析错误原因并生成修复后的 SQL。`;
    const feedbackMsg: ChatMsg = {
      id: generateMessageId(),
      role: "user",
      content: errorFeedbackContent,
      timestamp: Date.now(),
    };
    const newMsgs = [...messages, feedbackMsg];
    updateSessionMessages(activeSessionId, newMsgs);
    shouldAutoScrollRef.current = true;
    await requestAssistantReply(activeSessionId, newMsgs);
  }, [activeSessionId, loading, messages, requestAssistantReply, updateSessionMessages]);

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

  const renderThinkingProgress = useCallback(() => {
    if (!thinkingStatus && thinkingTimeline.length === 0) return null;

    const currentText = statusTextMap[thinkingStatus] || t("ai.thinking");
    const firstAt = thinkingTimeline[0]?.at || Date.now();

    return (
      <div className="mt-1.5 rounded-[var(--radius-input)] border border-[var(--border-subtle)] bg-[var(--surface)] px-2.5 py-2">
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2 text-2xs text-[var(--fg-secondary)]">
            <Loader2 className="h-3.5 w-3.5 animate-spin text-[var(--accent)]" />
            <span>{currentText}</span>
          </div>
          <button
            className="text-2xs text-[var(--accent)] hover:underline"
            onClick={() => setShowThinkingTimeline((prev) => !prev)}
            type="button"
          >
            {showThinkingTimeline ? t("ai.hideProcess") : t("ai.viewProcess")}
          </button>
        </div>

        {showThinkingTimeline && thinkingTimeline.length > 0 && (
          <div className="mt-2 space-y-1.5 text-2xs text-[var(--fg-secondary)]">
            {thinkingTimeline.map((item, idx) => {
              const next = thinkingTimeline[idx + 1];
              const elapsed = item.at - firstAt;
              const cost = next ? next.at - item.at : null;
              const isCurrent = item.status === thinkingStatus;
              return (
                <div key={`${item.status}-${item.at}-${idx}`} className="flex items-center justify-between gap-2">
                  <div className="flex items-center gap-1.5 min-w-0">
                    <span className={cn(
                      "inline-flex h-3.5 w-3.5 items-center justify-center rounded-full border flex-shrink-0",
                      isCurrent
                        ? "border-[var(--accent)] text-[var(--accent)]"
                        : "border-[var(--border-subtle)] text-[var(--fg-secondary)]"
                    )}>
                      {isCurrent ? <Loader2 className="h-2 w-2 animate-spin" /> : <Check className="h-2 w-2" />}
                    </span>
                    <span className="truncate">{statusTextMap[item.status] || item.status}</span>
                  </div>
                  <span className="text-[var(--fg-muted)] whitespace-nowrap">
                    +{elapsed}ms{cost !== null ? ` · ${cost}ms` : ""}
                  </span>
                </div>
              );
            })}
          </div>
        )}
      </div>
    );
  }, [showThinkingTimeline, statusTextMap, t, thinkingStatus, thinkingTimeline]);

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
          <div key={msg.id} className={cn("flex group/msg", msg.role === "user" ? "justify-end" : "justify-start")}>
            <div className={cn(
              "max-w-[92%] min-w-0 flex flex-col",
              msg.role === "user" ? "items-end" : "items-start"
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
                  {msg.streaming && !msg.content ? (
                    <div className="py-1.5">{renderThinkingProgress()}</div>
                  ) : (
                    <>
                      <MarkdownContent
                        content={msg.content}
                        onExecuteSQL={handleExecuteSQL}
                        onApplyAndRunSQL={handleApplyAndRunSQL}
                      />
                      {msg.streaming && renderThinkingProgress()}
                    </>
                  )}
                </div>
              ) : (
                <span className="whitespace-pre-wrap break-words">{msg.content}</span>
              )}
              </div>
              {/* SQL 执行失败时「AI 修复」按钮 — 始终可见，独立一行，醒目强调 */}
              {msg.role === "assistant" && msg.failedSQL && msg.sqlError && !loading && (
                <div className="mt-2">
                  <button
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-[var(--radius-btn)] text-xs font-medium bg-[var(--accent)] text-[var(--accent-fg)] hover:bg-[var(--accent-hover)] shadow-sm transition-all"
                    onClick={() => handleFixWithAI(msg.failedSQL!, msg.sqlError!)}
                    title={t("ai.fixWithAI")}
                  >
                    <Wrench className="h-3.5 w-3.5" />
                    <span>{t("ai.fixWithAI")}</span>
                  </button>
                </div>
              )}
              <div className="mt-1.5 flex flex-wrap items-center gap-2 opacity-0 group-hover/msg:opacity-100 transition-opacity">
                {msg.role === "user" && (
                  <button
                    className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-[var(--radius-btn)] text-2xs text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] hover:text-[var(--accent)] transition-colors disabled:opacity-60"
                    onClick={() => handleRetryFromUserMessage(idx)}
                    disabled={loading}
                    title={t("ai.retry")}
                  >
                    <RotateCcw className="h-3 w-3" />
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
                    className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-[var(--radius-btn)] text-2xs text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] hover:text-[var(--accent)] transition-colors disabled:opacity-60"
                    onClick={() => handleRetryAssistantMessage(idx)}
                    disabled={loading}
                    title={t("ai.retry")}
                  >
                    <RotateCcw className="h-3 w-3" />
                    <span>{t("ai.retry")}</span>
                  </button>
                )}
                
                {msg.role === "assistant" && msg.meta && !msg.streaming && (
                  <>
                    <div className="w-px h-3 bg-[var(--border-subtle)] mx-0.5"></div>
                    <div className="text-2xs text-[var(--fg-muted)] flex items-center gap-2">
                      <span>{t("ai.tokenCount")}: {msg.meta.tokenCount ?? 0}</span>
                      <span>{t("ai.charCount")}: {msg.meta.charCount ?? msg.content.length}</span>
                      <span>{t("ai.answerAt")}: {msg.meta.answeredAt || "-"}</span>
                      <span>{t("ai.duration")}: {formatDuration(msg.meta.durationMs || 0)}</span>
                    </div>
                  </>
                )}
              </div>
            </div>
          </div>
        ))}
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
  onApplyAndRunSQL,
}: {
  content: string;
  onExecuteSQL?: (sql: string) => void;
  onApplyAndRunSQL?: (sql: string) => void;
}) {
  const { t } = useTranslation();

  const components = React.useMemo(() => ({
    p: ({ children }: any) => <p className="whitespace-pre-wrap break-words">{children}</p>,
    ul: ({ children }: any) => <ul className="list-disc pl-5 space-y-1">{children}</ul>,
    ol: ({ children }: any) => <ol className="list-decimal pl-5 space-y-1">{children}</ol>,
    li: ({ children }: any) => <li className="break-words">{children}</li>,
    a: ({ href, children }: any) => (
      <a className="text-[var(--accent)] underline break-all" href={href} target="_blank" rel="noreferrer">
        {children}
      </a>
    ),
    table: ({ children }: any) => (
      <div className="w-full max-w-full overflow-x-auto border border-[var(--border-subtle)] rounded-[var(--radius-input)]">
        <table className="min-w-max text-left text-2xs">{children}</table>
      </div>
    ),
    th: ({ children }: any) => <th className="px-2 py-1 bg-[var(--surface)] border-b border-[var(--border-subtle)] whitespace-nowrap">{children}</th>,
    td: ({ children }: any) => <td className="px-2 py-1 border-b border-[var(--border-subtle)] whitespace-nowrap">{children}</td>,
    code: ({ className, children, ...props }: any) => {
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
              {canExecute && onApplyAndRunSQL && (
                <button
                  className="flex items-center gap-0.5 px-1.5 py-0.5 rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] text-[var(--accent)] transition-colors"
                  onClick={() => onApplyAndRunSQL(rawCode)}
                >
                  <ArrowRightToLine className="h-2.5 w-2.5" /> <span>应用并执行</span>
                </button>
              )}
              {canExecute && (
                <button
                  className="flex items-center gap-0.5 px-1.5 py-0.5 rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors text-[var(--fg-secondary)]"
                  onClick={() => onExecuteSQL(rawCode)}
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
  }), [onExecuteSQL, onApplyAndRunSQL, t]);

  return (
    <div className="space-y-2 markdown-content max-w-full min-w-0 break-words">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={components}
      >
        {content}
      </ReactMarkdown>

    </div>
  );
}
