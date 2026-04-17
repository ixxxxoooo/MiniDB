import React, { useState, useRef, useEffect, useCallback } from "react";
import {
  Sparkles,
  X,
  ArrowUp,
  Loader2,
  Square,
  Copy,
  Check,
  Play,
  Trash2,
  Plus,
  History,
  ChevronLeft,
  ChevronDown,
  ChevronRight,
  MessageSquare,
  RotateCcw,
  ArrowRightToLine,
  Wrench,
  Table2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTabsStore } from "@/stores/tabs";
import { createStreamMetaFilter, extractNextStepMetaChoices, stripStreamMetaBlocks } from "@/components/ai/streamMeta";
import { cn, copyToClipboard } from "@/lib/utils";
import { useTranslation } from "@/i18n";
import * as AIService from "../../../wailsjs/go/services/AIService";
import * as DatabaseService from "../../../wailsjs/go/services/DatabaseService";
import * as QueryService from "../../../wailsjs/go/services/QueryService";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
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
  progressStatus?: string;
  progressTimeline?: ThinkingTimelineItem[];
  toolTimeline?: ToolTimelineItem[];
  // ReAct 模式：AI 真实推理内容时间线
  thinkingTimeline?: ThinkingItem[];
  contentStartedAt?: number;
  // 结构化下一步建议（来自 tableplus-ai-next-steps 元数据块）
  nextStepChoices?: NextStepChoice[];
}

interface AIChatStreamEvent {
  requestId: string;
  type: "delta" | "done" | "error" | "status" | "tool_start" | "tool_sql" | "tool_result" | "tool_error" | "thinking" | "final_answer";
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
  // AI 在工具调用间输出的真实推理/分析内容
  thinkingContent?: string;
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

interface ToolTimelineItem {
  type: "tool_start" | "tool_sql" | "tool_result" | "tool_error";
  toolName?: string;
  toolCallId?: string;
  toolState?: string;
  toolInput?: string;
  toolSql?: string;
  toolOutput?: string;
  durationMs?: number;
  at: number;
}

// ReAct 模式：AI 真实推理内容
interface ThinkingItem {
  content: string;
  at: number;
}

interface NextStepChoice {
  label: string;
  prompt: string;
}

const HIGHLIGHT_CACHE_MAX = 400;
const highlightCache = new Map<string, string>();

function escapeHtml(text: string): string {
  return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function getHighlightedHtml(code: string, language: string): string {
  const prismLang = Prism.languages[language] ? language : "sql";
  const cacheKey = `${prismLang}\u0000${code}`;
  const cached = highlightCache.get(cacheKey);
  if (cached !== undefined) {
    return cached;
  }

  let html = "";
  try {
    html = Prism.highlight(code, Prism.languages[prismLang], prismLang);
  } catch {
    html = escapeHtml(code);
  }

  if (highlightCache.size >= HIGHLIGHT_CACHE_MAX) {
    const firstKey = highlightCache.keys().next().value;
    if (firstKey !== undefined) {
      highlightCache.delete(firstKey);
    }
  }
  highlightCache.set(cacheKey, html);
  return html;
}

type MentionHighlightVariant = "input" | "user" | "default";
type MentionKind = "table" | "tool";

interface MentionToken {
  kind: MentionKind;
  name: string;
  raw: string;
}

interface MentionCandidate {
  value: string;
  display: string;
  kind: MentionKind;
}

interface MentionDeleteResult {
  next: string;
  caret: number;
}

interface MentionRange {
  start: number;
  end: number;
  index: number;
}

function normalizeThinkingContent(raw: string): string {
  if (!raw) return "";
  const text = raw
    .replace(/<think>/gi, "")
    .replace(/<\/think>/gi, "")
    .replace(/\r\n/g, "\n")
    .trim();
  // 过滤固定模板/过渡噪音，避免“伪过程感”
  const filteredLines = text
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.length > 0)
    .filter((line) => !/信息已足够.*整理最终回答/.test(line))
    .filter((line) => !/^问题复述[：:]/.test(line))
    .filter((line) => !/^我的理解[：:]/.test(line))
    .filter((line) => !/^分析[：:]/.test(line))
    .filter((line) => !/^\<\s*\/?\s*[|｜]\s*DSML\s*[|｜]/i.test(line))
    .filter((line) => !/invoke name=|parameter name=|function_calls/i.test(line));
  return filteredLines.join("\n").trim();
}

function extractNextStepChoices(rawContent: string): NextStepChoice[] {
  return extractNextStepMetaChoices(rawContent || "");
}

function parseMentionToken(text: string): MentionToken | null {
  const matched = text.match(/^@(tool|table):([^\s]+)$/);
  if (!matched) return null;
  return {
    kind: matched[1] as MentionKind,
    name: matched[2],
    raw: matched[0],
  };
}

function deleteMentionTokenByKey(value: string, cursor: number, key: "Backspace" | "Delete"): MentionDeleteResult | null {
  const mentionPattern = /@(?:tool|table):[^\s]+/g;
  let matched: RegExpExecArray | null = null;
  while ((matched = mentionPattern.exec(value)) !== null) {
    const start = matched.index;
    const token = matched[0];
    const end = start + token.length;

    if (key === "Backspace") {
      // 光标在 mention 末尾：一次删整个 mention
      if (cursor === end) {
        return {
          next: value.slice(0, start) + value.slice(end),
          caret: start,
        };
      }
      // 光标在 mention 后一个空格：一次删“空格 + mention”
      if (cursor === end + 1 && value[cursor - 1] === " ") {
        return {
          next: value.slice(0, start) + value.slice(cursor),
          caret: start,
        };
      }
    } else {
      // 光标在 mention 开头：一次删 mention（并吞掉后面的一个空格，避免多余空白）
      if (cursor === start) {
        const cutEnd = value[end] === " " ? end + 1 : end;
        return {
          next: value.slice(0, start) + value.slice(cutEnd),
          caret: start,
        };
      }
      // 光标在 mention 前一个空格：一次删“空格 + mention”
      if (cursor + 1 === start && value[cursor] === " ") {
        return {
          next: value.slice(0, cursor) + value.slice(end),
          caret: cursor,
        };
      }
    }
  }
  return null;
}

function deleteMentionByOccurrence(value: string, occurrence: number): MentionDeleteResult | null {
  if (occurrence < 0) return null;
  const mentionPattern = /@(?:tool|table):[^\s]+/g;
  let matched: RegExpExecArray | null = null;
  let idx = 0;
  while ((matched = mentionPattern.exec(value)) !== null) {
    if (idx !== occurrence) {
      idx++;
      continue;
    }
    let start = matched.index;
    let end = start + matched[0].length;
    // 优先吞右侧空格，次选吞左侧空格，避免残留双空格
    if (value[end] === " ") {
      end += 1;
    } else if (start > 0 && value[start - 1] === " ") {
      start -= 1;
    }
    return {
      next: value.slice(0, start) + value.slice(end),
      caret: start,
    };
  }
  return null;
}

function findMentionRangeAtPosition(value: string, pos: number): MentionRange | null {
  const mentionPattern = /@(?:tool|table):[^\s]+/g;
  let matched: RegExpExecArray | null = null;
  let idx = 0;
  while ((matched = mentionPattern.exec(value)) !== null) {
    const start = matched.index;
    const end = start + matched[0].length;
    if (pos >= start && pos <= end) {
      return { start, end, index: idx };
    }
    idx++;
  }
  return null;
}

function findMentionByExactRange(value: string, start: number, end: number): MentionRange | null {
  const mentionPattern = /@(?:tool|table):[^\s]+/g;
  let matched: RegExpExecArray | null = null;
  let idx = 0;
  while ((matched = mentionPattern.exec(value)) !== null) {
    const tokenStart = matched.index;
    const tokenEnd = tokenStart + matched[0].length;
    if (tokenStart === start && tokenEnd === end) {
      return { start: tokenStart, end: tokenEnd, index: idx };
    }
    idx++;
  }
  return null;
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
const MAX_MENTION_CANDIDATES = 100;
const INPUT_MIN_HEIGHT = 40; // 紧凑单行高度（含内边距）
const INPUT_MAX_HEIGHT = 156;

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
  const [inputScrollTop, setInputScrollTop] = useState(0);
  const [loading, setLoading] = useState(false);
  const [copiedMessageId, setCopiedMessageId] = useState<string | null>(null);
  const [dismissedSuggestionMap, setDismissedSuggestionMap] = useState<Record<string, boolean>>({});
  const [expandedToolCallMap, setExpandedToolCallMap] = useState<Record<string, boolean>>({});
  const [mentionVisible, setMentionVisible] = useState(false);
  const [mentionQuery, setMentionQuery] = useState("");
  const [mentionType, setMentionType] = useState<MentionKind>("table");
  const [mentionIndex, setMentionIndex] = useState(0);
  const [selectedInputMentionOccurrence, setSelectedInputMentionOccurrence] = useState<number | null>(null);
  const [toolSuggestions, setToolSuggestions] = useState<Array<{ name: string; description?: string }>>([]);
  const [tableSuggestions, setTableSuggestions] = useState<string[]>([]);
  const allTabs = useTabsStore((s) => s.tabs);
  // 表名缓存：按 连接ID+数据库 缓存，减少重复请求
  const tableSuggestionsCacheRef = useRef<Record<string, string[]>>({});
  const activeStreamRequestRef = useRef<string | null>(null);
  const stopCurrentStreamRef = useRef<(() => void) | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const shouldAutoScrollRef = useRef(true);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const resizingRef = useRef(false);
  const resizeRafRef = useRef<number | null>(null);
  const pendingWidthRef = useRef<number | null>(null);
  const { t } = useTranslation();

  const activeSession = sessions.find((s) => s.id === activeSessionId) || null;
  const messages = activeSession?.messages || [];
  const inputPlaceholder = t("ai.placeholder");
  const scoreMention = useCallback((name: string, query: string) => {
    const q = query.trim().toLowerCase();
    const lower = name.toLowerCase();
    if (!q) return 1;
    let score = 0;
    if (lower === q) score += 120;
    if (lower.startsWith(q)) score += 80;
    if (lower.includes(q)) score += 40;
    // 子序列匹配：提升模糊输入命中率（如输入 usr 命中 user_profile）
    let i = 0;
    for (const ch of lower) {
      if (i < q.length && ch === q[i]) i++;
    }
    if (i > 1) score += i * 6;
    return score;
  }, []);
  const rankMentionItems = useCallback((items: string[], query: string) => {
    const q = query.trim().toLowerCase();
    if (!q) return items.slice(0, MAX_MENTION_CANDIDATES);
    return items
      .map((name) => {
        return { name, score: scoreMention(name, q) };
      })
      .filter((item) => item.score > 0)
      .sort((a, b) => b.score - a.score || a.name.localeCompare(b.name))
      .slice(0, MAX_MENTION_CANDIDATES)
      .map((item) => item.name);
  }, [scoreMention]);
  const mentionCandidates: MentionCandidate[] = mentionType === "tool"
    ? rankMentionItems(toolSuggestions.map((item) => item.name), mentionQuery)
      .map((name) => ({
        value: `@tool:${name}`,
        display: name,
        kind: "tool" as const,
      }))
    : [];
  const openedTableNamesInWorkspace = React.useMemo(() => {
    if (!currentConnectionId || !currentDatabase) return [];
    const names = allTabs
      .filter((tab) => tab.connectionId === currentConnectionId && tab.database === currentDatabase)
      .map((tab) => String(tab.table || "").trim())
      .filter(Boolean);
    return Array.from(new Set(names));
  }, [allTabs, currentConnectionId, currentDatabase]);
  const tableMentionGroups = React.useMemo(() => {
    const ranked = rankMentionItems(tableSuggestions, mentionQuery);
    if (ranked.length === 0) {
      return { opened: [] as string[], other: [] as string[] };
    }
    const openedSet = new Set(openedTableNamesInWorkspace.map((name) => name.toLowerCase()));
    const opened: string[] = [];
    const other: string[] = [];
    ranked.forEach((name) => {
      if (openedSet.has(name.toLowerCase())) {
        opened.push(name);
      } else {
        other.push(name);
      }
    });
    return { opened, other };
  }, [openedTableNamesInWorkspace, mentionQuery, rankMentionItems, tableSuggestions]);
  const tableMentionCandidates: MentionCandidate[] = React.useMemo(
    () => [...tableMentionGroups.opened, ...tableMentionGroups.other]
      .map((name) => ({
        value: `@table:${name}`,
        display: name,
        kind: "table" as const,
      })),
    [tableMentionGroups]
  );
  const finalMentionCandidates = mentionType === "tool" ? mentionCandidates : tableMentionCandidates;

  useEffect(() => {
    saveSessions(sessions);
  }, [sessions]);

  useEffect(() => {
    // 加载可用工具清单，支持 @tool 联想
    (async () => {
      try {
        const tools = await (AIService as any).ListTools();
        const normalized = (tools || []).map((tool: any) => ({
          name: String(tool?.name || ""),
          description: String(tool?.description || ""),
        })).filter((item: any) => item.name);
        setToolSuggestions(normalized);
      } catch {
        // 忽略失败，避免阻断输入体验
      }
    })();
  }, []);

  useEffect(() => {
    // 按当前库加载表名，用于 @table 联想
    (async () => {
      if (!currentConnectionId || !currentDatabase) {
        setTableSuggestions([]);
        return;
      }
      const cacheKey = `${currentConnectionId}::${currentDatabase}`;
      const cached = tableSuggestionsCacheRef.current[cacheKey];
      if (cached && cached.length > 0) {
        setTableSuggestions(cached);
        return;
      }
      try {
        const tables = await (DatabaseService as any).GetTables(currentConnectionId, currentDatabase);
        const names = (tables || []).map((t: any) => String(t?.name || "")).filter(Boolean);
        tableSuggestionsCacheRef.current[cacheKey] = names;
        setTableSuggestions(names);
      } catch {
        setTableSuggestions([]);
      }
    })();
  }, [currentConnectionId, currentDatabase]);

  useEffect(() => {
    if (!scrollRef.current || !shouldAutoScrollRef.current) return;
    scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [messages, loading]);

  useEffect(() => {
    const el = inputRef.current;
    if (!el) return;
    el.style.height = "auto";
    const nextHeight = Math.max(INPUT_MIN_HEIGHT, Math.min(INPUT_MAX_HEIGHT, el.scrollHeight));
    el.style.height = `${nextHeight}px`;
  }, [input]);

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
      pendingWidthRef.current = newWidth;
      if (resizeRafRef.current !== null) return;
      resizeRafRef.current = requestAnimationFrame(() => {
        resizeRafRef.current = null;
        if (pendingWidthRef.current !== null) {
          onWidthChange(pendingWidthRef.current);
        }
      });
    };
    const handleUp = () => {
      resizingRef.current = false;
      if (resizeRafRef.current !== null) {
        cancelAnimationFrame(resizeRafRef.current);
        resizeRafRef.current = null;
      }
      if (pendingWidthRef.current !== null) {
        onWidthChange(pendingWidthRef.current);
        pendingWidthRef.current = null;
      }
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
      {
        id: placeholderId,
        role: "assistant",
        content: "",
        timestamp: placeholderTimestamp,
        streaming: true,
        meta: {},
        progressTimeline: [],
        toolTimeline: [],
        thinkingTimeline: [],
      },
    ];
    activeStreamRequestRef.current = requestId;
    setLoading(true);

    updateSessionMessages(sessionId, streamBaseMessages);
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
      meta?: ChatMsg["meta"],
      nextStepChoices?: NextStepChoice[]
    ) => {
      setSessions((prev) =>
        prev.map((session) => {
          if (session.id !== sessionId) return session;
          return {
            ...session,
            updatedAt: Date.now(),
            messages: session.messages.map((message) =>
              message.id === placeholderId
                ? {
                  ...message,
                  content: updater(message.content),
                  errorType,
                  streaming,
                  meta: meta || message.meta,
                  nextStepChoices: nextStepChoices ?? message.nextStepChoices,
                }
                : message
            ),
          };
        })
      );
    };

    const appendProgressStatus = (status: string) => {
      if (!status) return;
      setSessions((prev) =>
        prev.map((session) => {
          if (session.id !== sessionId) return session;
          return {
            ...session,
            updatedAt: Date.now(),
            messages: session.messages.map((message) => {
              if (message.id !== placeholderId) return message;
              const prevTimeline = message.progressTimeline || [];
              const last = prevTimeline[prevTimeline.length - 1];
              const nextTimeline = last && last.status === status
                ? prevTimeline
                : [...prevTimeline, { status, at: Date.now() }];
              return { ...message, progressStatus: status, progressTimeline: nextTimeline };
            }),
          };
        })
      );
    };

    let lastProcessEventKind: "thinking" | "tool" | null = null;

    const appendToolEvent = (event: ToolTimelineItem) => {
      setSessions((prev) =>
        prev.map((session) => {
          if (session.id !== sessionId) return session;
          return {
            ...session,
            updatedAt: Date.now(),
            messages: session.messages.map((message) =>
              message.id === placeholderId
                ? { ...message, toolTimeline: [...(message.toolTimeline || []), event] }
                : message
            ),
          };
        })
      );
      // 标记工具事件边界，避免将“工具前后”的思考片段合并到同一条
      lastProcessEventKind = "tool";
    };

    // 追加推理事件到消息的 thinkingTimeline
    const appendThinkingEvent = (item: ThinkingItem) => {
      const normalized = normalizeThinkingContent(item.content);
      if (!normalized) return;
      setSessions((prev) =>
        prev.map((session) => {
          if (session.id !== sessionId) return session;
          return {
            ...session,
            updatedAt: Date.now(),
            messages: session.messages.map((message) => {
              if (message.id !== placeholderId) return message;
              const timeline = message.thinkingTimeline || [];
              const last = timeline[timeline.length - 1];
              // reasoning_content 常按 token 切片，短时间内连续片段合并成一条，避免“碎片瀑布”
              if (last && lastProcessEventKind === "thinking" && item.at - last.at < 420) {
                const joiner = /\s$/.test(last.content) || /^\s/.test(normalized) ? "" : " ";
                const merged: ThinkingItem = {
                  ...last,
                  content: `${last.content}${joiner}${normalized}`,
                  at: item.at,
                };
                return { ...message, thinkingTimeline: [...timeline.slice(0, -1), merged] };
              }
              return { ...message, thinkingTimeline: [...timeline, { ...item, content: normalized }] };
            }),
          };
        })
      );
      lastProcessEventKind = "thinking";
    };

    // 设置内容开始时间戳（仅首次设置生效）
    const setMessageContentStartedAt = (at: number) => {
      setSessions((prev) =>
        prev.map((session) => {
          if (session.id !== sessionId) return session;
          return {
            ...session,
            updatedAt: Date.now(),
            messages: session.messages.map((message) =>
              message.id === placeholderId
                ? { ...message, contentStartedAt: message.contentStartedAt || at }
                : message
            ),
          };
        })
      );
    };

    let streamBuffer = "";
    let flushTimer: ReturnType<typeof setTimeout> | null = null;
    let contentStartedAtLocal = 0;
    let offStream: (() => void) | null = null;
    const streamMetaFilter = createStreamMetaFilter();
    const flushBuffer = () => {
      if (!streamBuffer) return;
      const delta = streamBuffer;
      streamBuffer = "";
      const visible = streamMetaFilter.flush(delta);
      updateStreamMessage(() => visible);
    };

    const cleanupStreamResources = () => {
      if (flushTimer) {
        clearTimeout(flushTimer);
        flushTimer = null;
      }
      flushBuffer();
      if (offStream) {
        offStream();
        offStream = null;
      }
      if (stopCurrentStreamRef.current === handleStopStreaming) {
        stopCurrentStreamRef.current = null;
      }
    };

    const settleLoading = () => {
      if (activeStreamRequestRef.current === requestId) {
        activeStreamRequestRef.current = null;
        setLoading(false);
      }
    };

    const handleStopStreaming = () => {
      if (activeStreamRequestRef.current !== requestId) return;
      cleanupStreamResources();
      appendProgressStatus("cancelled");
      const now = Date.now();
      updateStreamMessage(
        (content) => content.trim() ? content : `*${t("common.cancelled")}*`,
        undefined,
        false,
        {
          tokenCount: 0,
          charCount: 0,
          answeredAt: formatAnsweredAt(now),
          durationMs: now - startedAt,
        }
      );
      settleLoading();
    };

    stopCurrentStreamRef.current = handleStopStreaming;

    offStream = EventsOn("ai:chat_stream", (event: AIChatStreamEvent) => {
      if (!event || event.requestId !== requestId || activeStreamRequestRef.current !== requestId) return;
      // 处理进度状态事件
      if (event.type === "status" && event.delta) {
        appendProgressStatus(event.delta);
        return;
      }
      if (event.type === "tool_start" || event.type === "tool_sql" || event.type === "tool_result" || event.type === "tool_error") {
        const toolType: ToolTimelineItem["type"] = event.type;
        appendToolEvent({
          type: toolType,
          toolName: event.toolName,
          toolCallId: event.toolCallId,
          toolState: event.toolState,
          toolInput: event.toolInput,
          toolSql: event.toolSql,
          toolOutput: event.toolOutput,
          durationMs: event.durationMs,
          at: Date.now(),
        });
        return;
      }
      // 处理推理事件
      if (event.type === "thinking" && event.thinkingContent) {
        appendThinkingEvent({ content: event.thinkingContent, at: Date.now() });
        return;
      }
      // 处理最终回答标记事件：记录内容开始时间
      if (event.type === "final_answer") {
        contentStartedAtLocal = Date.now();
        setMessageContentStartedAt(contentStartedAtLocal);
        return;
      }
      if (event.type === "delta" && event.delta) {
        // 首个 delta 标记内容开始时间（兜底逻辑，final_answer 事件未到达时使用）
        if (!contentStartedAtLocal) {
          contentStartedAtLocal = Date.now();
          setMessageContentStartedAt(contentStartedAtLocal);
        }
        streamBuffer += event.delta;
        if (!flushTimer) {
          flushTimer = setTimeout(() => {
            flushBuffer();
            flushTimer = null;
          }, 16);
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
    try {
      const contextMessages = buildContextMessages(baseMessages);
      const apiMessages = contextMessages.map((m) => ({ role: m.role, content: m.content }));
      const result = await (AIService as any).ChatAIStream(
        currentConnectionId || "",
        currentDatabase || "",
        apiMessages,
        requestId
      );

      if (activeStreamRequestRef.current !== requestId) return;
      cleanupStreamResources();
      const rawFinal = String(result?.content || "");
      const structuredNextSteps = extractNextStepMetaChoices(rawFinal);
      const streamedVisible = streamMetaFilter.getVisibleContent();
      const cleanedFinal = stripStreamMetaBlocks(rawFinal);
      const aiContent = (streamedVisible.length >= cleanedFinal.length ? streamedVisible : cleanedFinal) || t("ai.noContent");
      const now = Date.now();
      // 主流程结束时显式写入 done，避免 UI 仍显示“进行中”状态
      appendProgressStatus("done");
      updateStreamMessage(() => aiContent, undefined, false, {
        tokenCount: estimateTokenCount(aiContent),
        charCount: aiContent.length,
        answeredAt: formatAnsweredAt(now),
        durationMs: now - startedAt,
      }, structuredNextSteps);
    } catch (e: any) {
      if (activeStreamRequestRef.current !== requestId) return;
      cleanupStreamResources();
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
      cleanupStreamResources();
      settleLoading();
    }
  }, [currentConnectionId, currentDatabase, t, updateSessionMessages]);

  const sendUserMessage = useCallback(async (rawText: string, opts?: { clearInput?: boolean }) => {
    const text = rawText.trim();
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
    if (opts?.clearInput) setInput("");
    await requestAssistantReply(sessionId, newMsgs);
  }, [activeSessionId, currentConnectionId, currentDatabase, loading, messages, requestAssistantReply, updateSessionMessages]);

  const handleSend = useCallback(async () => {
    await sendUserMessage(input, { clearInput: true });
  }, [input, sendUserMessage]);
  const handleStopStreaming = useCallback(() => {
    stopCurrentStreamRef.current?.();
  }, []);
  const isStreaming = loading && !!activeStreamRequestRef.current;

  const latestAssistantMessage = React.useMemo(
    () => [...messages].reverse().find((msg) => msg.role === "assistant") || null,
    [messages]
  );
  const latestNextStepChoices = React.useMemo(() => {
    if (!latestAssistantMessage || latestAssistantMessage.streaming) return [];
    return latestAssistantMessage.nextStepChoices || extractNextStepChoices(latestAssistantMessage.content);
  }, [latestAssistantMessage]);
  const showNextStepPicker = !!latestAssistantMessage
    && !latestAssistantMessage.streaming
    && !loading
    && latestNextStepChoices.length > 0
    && !dismissedSuggestionMap[latestAssistantMessage.id];

  const handleDismissNextStepPicker = useCallback(() => {
    if (!latestAssistantMessage) return;
    setDismissedSuggestionMap((prev) => ({ ...prev, [latestAssistantMessage.id]: true }));
  }, [latestAssistantMessage]);

  const handlePickNextStep = useCallback(async (choice: NextStepChoice) => {
    if (!latestAssistantMessage) return;
    setDismissedSuggestionMap((prev) => ({ ...prev, [latestAssistantMessage.id]: true }));
    await sendUserMessage(choice.prompt, { clearInput: true });
  }, [latestAssistantMessage, sendUserMessage]);

  const handleCopy = async (messageId: string, content: string) => {
    await copyToClipboard(content);
    setCopiedMessageId(messageId);
    setTimeout(() => setCopiedMessageId(null), 2000);
  };

  const addTab = useTabsStore((s) => s.addTab);
  const updateTab = useTabsStore((s) => s.updateTab);

  const handleApplyAndRunSQL = useCallback((sql: string) => {
    // 始终使用 AI 面板当前的连接和库，避免复用指向旧连接的 tab
    const connId = currentConnectionId;
    const dbName = currentDatabase;
    if (!connId || !dbName) return;

    const { tabs, activeTabId, setActiveTab } = useTabsStore.getState();
    const activeWsTab = tabs.find(t => t.id === activeTabId);
    let targetTabId = "";

    // 仅当当前活跃 tab 是 query 类型且连接+库完全匹配时才复用
    if (activeWsTab && activeWsTab.type === "query" && activeWsTab.connectionId === connId && activeWsTab.database === dbName) {
      targetTabId = activeWsTab.id;
    }

    if (targetTabId) {
      updateTab(targetTabId, { sql });
      setActiveTab(targetTabId);
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
      setActiveTab(newId);
      setTimeout(() => window.dispatchEvent(new CustomEvent("tableplus-ai:run-sql", { detail: { tabId: newId, sql } })), 160);
    }
  }, [currentConnectionId, currentDatabase, updateTab, addTab, t]);

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

  const applyMentionCandidate = useCallback((candidate: string) => {
    const el = inputRef.current;
    const cursor = el?.selectionStart ?? input.length;
    const head = input.slice(0, cursor);
    const tail = input.slice(cursor);
    const matched = head.match(/(^|\s)@([a-zA-Z0-9_:\-]*)$/);
    if (!matched) return;

    const start = head.length - matched[0].length + matched[1].length;
    const next = head.slice(0, start) + candidate + " " + tail;
    setInput(next);
    setSelectedInputMentionOccurrence(null);
    setMentionVisible(false);
    setMentionQuery("");
    setMentionIndex(0);
    requestAnimationFrame(() => {
      if (!inputRef.current) return;
      const pos = start + candidate.length + 1;
      inputRef.current.focus();
      inputRef.current.setSelectionRange(pos, pos);
    });
  }, [input]);

  const handleInputChange = useCallback((value: string) => {
    setInput(value);
    setSelectedInputMentionOccurrence(null);

    const el = inputRef.current;
    const cursor = el?.selectionStart ?? value.length;
    const head = value.slice(0, cursor);
    const matched = head.match(/(^|\s)@([a-zA-Z0-9_:\-]*)$/);
    if (!matched) {
      setMentionVisible(false);
      return;
    }

    const query = matched[2] || "";
    if (query.startsWith("tool:")) {
      setMentionType("tool");
      setMentionQuery(query.replace(/^tool:/, ""));
    } else if (query.includes(":")) {
      // 对未知命名空间（如 table:）默认走表联想
      const parts = query.split(":");
      setMentionType(parts[0] === "tool" ? "tool" : "table");
      setMentionQuery(parts.slice(1).join(":"));
    } else {
      setMentionType("table");
      setMentionQuery(query);
    }
    setMentionVisible(true);
    setMentionIndex(0);
  }, []);
  const handleRemoveInputMention = useCallback((occurrence: number) => {
    const deleted = deleteMentionByOccurrence(input, occurrence);
    if (!deleted) return;
    setInput(deleted.next);
    setSelectedInputMentionOccurrence(null);
    setMentionVisible(false);
    requestAnimationFrame(() => {
      if (!inputRef.current) return;
      inputRef.current.focus();
      inputRef.current.setSelectionRange(deleted.caret, deleted.caret);
    });
  }, [input]);

  const renderExecutionFlow = useCallback((msg: ChatMsg) => {
    const msgTools = msg.toolTimeline || [];
    const msgThinking = msg.thinkingTimeline || [];
    const hasFlow = msg.streaming || msg.content || msgTools.length > 0 || msgThinking.length > 0;
    if (!hasFlow) return null;
    const hasContentStarted = !!msg.contentStartedAt || !!msg.content;

    const eventTextMap: Record<ToolTimelineItem["type"], string> = {
      tool_start: t("ai.toolEventStart"),
      tool_sql: t("ai.toolEventSQL"),
      tool_result: t("ai.toolEventResult"),
      tool_error: t("ai.toolEventError"),
    };

    const inferState = (item: ToolTimelineItem): "running" | "success" | "error" => {
      if (item.toolState === "running" || item.toolState === "success" || item.toolState === "error") {
        return item.toolState;
      }
      if (item.type === "tool_error") return "error";
      if (item.type === "tool_result") return "success";
      return "running";
    };

    type ToolCallGroup = {
      callId: string;
      toolName: string;
      state: "running" | "success" | "error";
      lastType: ToolTimelineItem["type"];
      firstAt: number;
      durationMs?: number;
      toolInput?: string;
      toolSql?: string;
      toolOutput?: string;
      events: ToolTimelineItem[];
    };

    const groups: ToolCallGroup[] = [];
    const indexMap: Record<string, number> = {};
    msgTools.forEach((item, idx) => {
      const fallbackCallID = `call_${item.toolName || "unknown"}_${idx}`;
      const callId = item.toolCallId || fallbackCallID;
      let group = groups[indexMap[callId]];
      if (!group) {
        group = {
          callId,
          toolName: item.toolName || t("ai.toolUnknown"),
          state: inferState(item),
          lastType: item.type,
          firstAt: item.at,
          events: [],
        };
        indexMap[callId] = groups.length;
        groups.push(group);
      }
      group.events.push(item);
      group.lastType = item.type;
      group.state = inferState(item);
      if (item.toolName) group.toolName = item.toolName;
      if (item.toolInput) group.toolInput = item.toolInput;
      if (item.toolSql) group.toolSql = item.toolSql;
      if (item.toolOutput) group.toolOutput = item.toolOutput;
      if (item.durationMs) group.durationMs = item.durationMs;
    });

    type ProcessNode =
      | { kind: "thinking"; at: number; content: string }
      | { kind: "tool"; at: number; call: ToolCallGroup };
    type TimelineNode =
      | ProcessNode
      | { kind: "answer"; at: number };

    const processNodes: ProcessNode[] = [
      ...msgThinking.map((item) => ({ kind: "thinking" as const, at: item.at, content: item.content })),
      ...groups.map((call) => ({ kind: "tool" as const, at: call.firstAt, call })),
    ].sort((a, b) => a.at - b.at);
    const lastProcessAt = processNodes.length > 0 ? processNodes[processNodes.length - 1].at : 0;
    const answerAt = (() => {
      if (lastProcessAt > 0) {
        // 保证“思考/工具”先于正文节点展示，避免正文提前插入把工具过程挤到下面
        return Math.max(msg.contentStartedAt || 0, lastProcessAt + 1);
      }
      if (msg.contentStartedAt) return msg.contentStartedAt;
      if (msg.content) return msg.timestamp;
      return 0;
    })();
    const timelineNodes: TimelineNode[] = [
      ...processNodes,
      ...(hasContentStarted ? [{ kind: "answer" as const, at: answerAt }] : []),
    ].sort((a, b) => a.at - b.at);

    const visibleNodes = timelineNodes;
    const isWaitingForAI = msg.streaming && visibleNodes.length === 0;
    const latestThinkingIdx = (() => {
      for (let i = visibleNodes.length - 1; i >= 0; i--) {
        if (visibleNodes[i].kind === "thinking") return i;
      }
      return -1;
    })();

    return (
      <div className="space-y-2">
        <div className="space-y-2">
          {isWaitingForAI && (
            <div className="flex items-center gap-2 text-2xs text-[var(--fg-muted)] py-1">
              <Loader2 className="h-2.5 w-2.5 animate-spin text-[var(--accent)]" />
              <span>{t("ai.thinking")}</span>
            </div>
          )}
          {visibleNodes.map((node, idx) => {
            if (node.kind === "thinking") {
              const activeThinking = msg.streaming && idx === latestThinkingIdx;
              return (
                <div key={`thinking-${node.at}-${idx}`} className="py-1">
                  <div className={cn(
                    "text-2xs leading-relaxed whitespace-pre-wrap",
                    activeThinking ? "ai-processing-sweep-base font-medium" : "text-[var(--fg-secondary)]"
                  )}>
                    {node.content}
                    {activeThinking && (
                      <span className="ai-processing-sweep-overlay" aria-hidden>{node.content}</span>
                    )}
                  </div>
                </div>
              );
            }
            if (node.kind === "answer") {
              return msg.content ? (
                <MarkdownContent
                  key={`answer-${msg.id}`}
                  content={msg.content}
                  onExecuteSQL={handleExecuteSQL}
                  onApplyAndRunSQL={handleApplyAndRunSQL}
                />
              ) : (
                <div key={`answer-loading-${msg.id}`} className="flex items-center gap-2 text-2xs text-[var(--fg-muted)] py-1">
                  <Loader2 className="h-3 w-3 animate-spin text-[var(--accent)]" />
                  <span>{t("ai.finalAnswerLabel")}...</span>
                </div>
              );
            }

            const call = node.call;
            const callKey = `${msg.id}:${call.callId}`;
            const done = call.state === "success" || call.state === "error";
            const defaultExpanded = !done;
            const expanded = Object.prototype.hasOwnProperty.call(expandedToolCallMap, callKey)
              ? !!expandedToolCallMap[callKey]
              : defaultExpanded;
            return (
              <div key={call.callId} className="rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--surface)]/80">
                <button
                  className="w-full px-2 py-1.5 flex items-center justify-between gap-2 hover:bg-[var(--surface-secondary)]/80 transition-colors"
                  onClick={() => setExpandedToolCallMap((prev) => ({ ...prev, [callKey]: !expanded }))}
                  style={{ transform: "none", opacity: 1 }}
                  type="button"
                >
                  <div className="min-w-0 flex items-center gap-1.5 text-left">
                    <span className={cn(
                      "inline-flex h-3.5 w-3.5 items-center justify-center rounded-full border flex-shrink-0",
                      call.state === "error"
                        ? "border-[var(--danger)] text-[var(--danger)]"
                        : call.state === "success"
                          ? "border-[var(--success)] text-[var(--success)]"
                          : "border-[var(--accent)] text-[var(--accent)]"
                    )}>
                      {call.state === "running" ? <Loader2 className="h-2 w-2 animate-spin" /> : call.state === "error" ? <X className="h-2 w-2" /> : <Check className="h-2 w-2" />}
                    </span>
                    <span className="font-medium truncate">{call.toolName}</span>
                    <span className="text-[var(--fg-muted)] truncate">{eventTextMap[call.lastType]}</span>
                  </div>
                  <div className="flex items-center gap-1.5 text-[var(--fg-muted)]">
                    <span>{call.durationMs ? `${call.durationMs}ms` : "-"}</span>
                    {expanded ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
                  </div>
                </button>
                {expanded && (
                  <div className="px-2 pb-2 space-y-1.5">
                    {call.toolInput ? (
                      <div>
                        <div className="text-[var(--fg-muted)] mb-0.5">{t("ai.toolEventStart")}</div>
                        <ToolHighlightedCode code={call.toolInput} language="json" />
                      </div>
                    ) : null}
                    {call.toolSql ? (
                      <div>
                        <div className="text-[var(--fg-muted)] mb-0.5">{t("ai.toolEventSQL")}</div>
                        <ToolHighlightedCode code={call.toolSql} language="sql" />
                      </div>
                    ) : null}
                    {call.toolOutput ? (
                      <div>
                        <div className="text-[var(--fg-muted)] mb-0.5">{call.lastType === "tool_error" ? t("ai.toolEventError") : t("ai.toolEventResult")}</div>
                        <div className="text-[length:var(--size-font-2xs)] leading-relaxed">
                          <MarkdownContent content={call.toolOutput} />
                        </div>
                      </div>
                    ) : null}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>
    );
  }, [expandedToolCallMap, handleApplyAndRunSQL, handleExecuteSQL, t]);

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
                  data-ui-title-tooltip={t("ai.deleteSession")}
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
          <Button variant="ghost" size="icon" className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)]" onClick={createNewSession} data-ui-title-tooltip={t("ai.newChat")}>
            <Plus className="h-3 w-3" />
          </Button>
          <Button variant="ghost" size="icon" className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)]" onClick={() => setShowHistory(true)} data-ui-title-tooltip={t("ai.chatHistory")}>
            <History className="h-3 w-3" />
          </Button>
          <Button variant="ghost" size="icon" className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)]" onClick={handleClearSession} data-ui-title-tooltip={t("ai.clearChat")}>
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
                "max-w-full min-w-0 overflow-hidden ai-chat-selectable",
                msg.role === "user"
                  ? "bg-[var(--accent)] text-[var(--accent-fg)] rounded-2xl rounded-tr-sm"
                  : cn(
                    "bg-[var(--surface-elevated)] border border-[var(--border-subtle)] text-[var(--fg)] rounded-2xl rounded-tl-sm",
                    msg.streaming && "ai-streaming-bubble"
                  )
              )}>
              {msg.role === "assistant" ? (
                <div>
                  {(msg.streaming || msg.progressTimeline?.length || msg.toolTimeline?.length || msg.thinkingTimeline?.length)
                    ? renderExecutionFlow(msg)
                    : (
                      <MarkdownContent
                        content={msg.content}
                        onExecuteSQL={handleExecuteSQL}
                        onApplyAndRunSQL={handleApplyAndRunSQL}
                      />
                    )}
                </div>
              ) : (
                <MentionHighlightedText text={msg.content} variant="user" />
              )}
              </div>
              {/* SQL 执行失败时「AI 修复」按钮 — 始终可见，独立一行，醒目强调 */}
              {msg.role === "assistant" && msg.failedSQL && msg.sqlError && !loading && (
                <div className="mt-2">
                  <button
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-[var(--radius-btn)] text-xs font-medium bg-[var(--accent)] text-[var(--accent-fg)] hover:bg-[var(--accent-hover)] shadow-sm transition-all"
                    onClick={() => handleFixWithAI(msg.failedSQL!, msg.sqlError!)}
                    data-ui-title-tooltip={t("ai.fixWithAI")}
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
                    data-ui-title-tooltip={t("ai.retry")}
                  >
                    <RotateCcw className="h-3 w-3" />
                    <span>{t("ai.retry")}</span>
                  </button>
                )}
                {(msg.role === "assistant" || msg.role === "user") && (
                  <button
                    className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-[var(--radius-btn)] text-2xs text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors"
                    onClick={() => handleCopy(msg.id, msg.content)}
                    data-ui-title-tooltip={t("common.copy")}
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
                    data-ui-title-tooltip={t("ai.retry")}
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

      {showNextStepPicker && (
        <div className="px-3 pt-2 pb-1 flex-shrink-0">
          <div className="rounded-[var(--radius-input)] border border-[var(--accent)]/20 bg-gradient-to-br from-[var(--surface-elevated)] via-[var(--surface-elevated)] to-[var(--accent)]/5 shadow-sm">
            <div className="px-2.5 py-2 flex items-center justify-between text-2xs border-b border-[var(--border-subtle)]/80">
              <span className="inline-flex items-center gap-1.5 font-medium text-[var(--fg)]">
                <Sparkles className="h-3.5 w-3.5 text-[var(--accent)]" />
                <span>{t("ai.nextStepPickerTitle")}</span>
              </span>
              <button
                type="button"
                className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[var(--fg-muted)] hover:bg-[var(--surface-secondary)] transition-colors"
                onClick={handleDismissNextStepPicker}
              >
                <X className="h-3 w-3" />
                <span>{t("ai.nextStepFinish")}</span>
              </button>
            </div>
            <div className="p-2.5 flex flex-wrap gap-2">
              {latestNextStepChoices.map((choice, idx) => (
                <button
                  key={`${choice.label}-${idx}`}
                  type="button"
                  className="group inline-flex items-center gap-1.5 px-3 py-1.5 rounded-[10px] text-xs border border-[var(--border-color)] bg-[var(--surface)] text-[var(--fg)] hover:border-[var(--accent)] hover:bg-[var(--accent)]/5 hover:text-[var(--accent)] transition-colors"
                  onClick={() => handlePickNextStep(choice)}
                >
                  <ChevronRight className="h-3.5 w-3.5 text-[var(--accent)]/80 group-hover:text-[var(--accent)]" />
                  {choice.label}
                </button>
              ))}
              <button
                type="button"
                className="px-3 py-1.5 rounded-[10px] text-xs border border-[var(--border-color)] text-[var(--fg-muted)] hover:bg-[var(--surface-secondary)] transition-colors"
                onClick={handleDismissNextStepPicker}
              >
                {t("ai.nextStepFinish")}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 输入区域 */}
      <div className="p-3 border-t border-[var(--border-color)] flex-shrink-0 bg-[var(--surface)]">
        <div className={cn(
          "ai-input-shell relative flex items-center rounded-[var(--radius-input)] border border-[var(--border-color)] bg-[var(--surface)] transition-all shadow-sm"
        )}>
          <div className="relative flex-1 min-w-0">
            <div
              className="pointer-events-none absolute inset-0 px-3 py-1.5 text-[length:var(--size-font-sm)] leading-[1.45] overflow-hidden"
              aria-hidden
            >
              <div style={{ transform: `translateY(-${inputScrollTop}px)` }}>
                {input ? (
                  <MentionHighlightedText
                    text={input}
                    variant="input"
                    onRemoveMention={handleRemoveInputMention}
                    selectedOccurrence={selectedInputMentionOccurrence}
                  />
                ) : (
                  <span className="text-[var(--fg-muted)]">{inputPlaceholder}</span>
                )}
              </div>
            </div>
            <textarea
              ref={inputRef}
              className={cn(
                "relative z-[1] w-full min-h-[40px] max-h-[156px] resize-none overflow-y-auto bg-transparent px-3 py-1.5 text-[length:var(--size-font-sm)] leading-[1.45]",
                "text-transparent [caret-color:var(--fg)] placeholder:text-[var(--fg-muted)] focus:outline-none scrollbar-auto-hide"
              )}
              style={{ WebkitTextFillColor: "transparent" }}
              placeholder={inputPlaceholder}
              value={input}
              rows={2}
              onScroll={(e) => setInputScrollTop((e.target as HTMLTextAreaElement).scrollTop)}
              onChange={(e) => handleInputChange(e.target.value)}
              onMouseUp={() => {
                const el = inputRef.current;
                if (!el) return;
                const start = el.selectionStart ?? 0;
                const end = el.selectionEnd ?? 0;
                if (start !== end) return;
                const mention = findMentionRangeAtPosition(input, start);
                if (!mention) {
                  setSelectedInputMentionOccurrence(null);
                  return;
                }
                requestAnimationFrame(() => {
                  if (!inputRef.current) return;
                  inputRef.current.focus();
                  inputRef.current.setSelectionRange(mention.start, mention.end);
                  setSelectedInputMentionOccurrence(mention.index);
                });
              }}
              onSelect={() => {
                const el = inputRef.current;
                if (!el) return;
                const start = el.selectionStart ?? 0;
                const end = el.selectionEnd ?? 0;
                const mention = findMentionByExactRange(input, start, end);
                setSelectedInputMentionOccurrence(mention ? mention.index : null);
              }}
              onKeyDown={(e) => {
                const isComposing = Boolean((e.nativeEvent as KeyboardEvent).isComposing);
                if (!isComposing && (e.key === "Backspace" || e.key === "Delete")) {
                  const el = inputRef.current;
                  const start = el?.selectionStart ?? 0;
                  const end = el?.selectionEnd ?? 0;
                  if (start === end) {
                    const deleted = deleteMentionTokenByKey(input, start, e.key);
                    if (deleted) {
                      e.preventDefault();
                      setInput(deleted.next);
                      setSelectedInputMentionOccurrence(null);
                      requestAnimationFrame(() => {
                        if (!inputRef.current) return;
                        inputRef.current.focus();
                        inputRef.current.setSelectionRange(deleted.caret, deleted.caret);
                      });
                      return;
                    }
                  }
                }
                if (mentionVisible && finalMentionCandidates.length > 0) {
                  if (e.key === "ArrowDown") {
                    e.preventDefault();
                    setMentionIndex((prev) => (prev + 1) % finalMentionCandidates.length);
                    return;
                  }
                  if (e.key === "ArrowUp") {
                    e.preventDefault();
                    setMentionIndex((prev) => (prev - 1 + finalMentionCandidates.length) % finalMentionCandidates.length);
                    return;
                  }
                  if (e.key === "Tab" || e.key === "Enter") {
                    e.preventDefault();
                    const picked = finalMentionCandidates[Math.max(0, Math.min(mentionIndex, finalMentionCandidates.length - 1))];
                    if (picked) applyMentionCandidate(picked.value);
                    return;
                  }
                  if (e.key === "Escape") {
                    e.preventDefault();
                    setMentionVisible(false);
                    return;
                  }
                }
                // 回车发送，Shift+Enter 换行（同时兼容 Cmd/Ctrl+Enter）
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  handleSend();
                }
              }}
            />
          </div>
          {mentionVisible && finalMentionCandidates.length > 0 && (
            <div className="absolute left-2 bottom-[calc(100%+6px)] w-[230px] rounded-[var(--radius-menu)] border border-[var(--border-color)] bg-[var(--surface-elevated)] shadow-lg p-1 max-h-80 overflow-y-auto z-20">
              {mentionType === "table" && tableMentionGroups.opened.length > 0 && tableMentionGroups.other.length > 0 ? (
                <>
                  {tableMentionCandidates.slice(0, tableMentionGroups.opened.length).map((item, idx) => (
                    <button
                      key={`${item.value}-${idx}`}
                      type="button"
                      className={cn(
                        "w-full text-left px-2 py-2 rounded-[var(--radius-sm)] text-xs transition-colors",
                        idx === mentionIndex
                          ? "bg-[var(--accent)]/10 text-[var(--accent)]"
                          : "text-[var(--fg)] hover:bg-[var(--sidebar-hover)]"
                      )}
                      onPointerDown={(ev) => {
                        ev.preventDefault();
                        applyMentionCandidate(item.value);
                      }}
                    >
                      <div className="flex items-center gap-1.5 min-w-0">
                        <Table2 className="h-3.5 w-3.5 text-[var(--accent)]/85 shrink-0" />
                        <div className="truncate">{item.display}</div>
                      </div>
                    </button>
                  ))}
                  <div className="my-1 h-px bg-[var(--border-subtle)]" />
                  {tableMentionCandidates.slice(tableMentionGroups.opened.length).map((item, idx) => {
                    const visualIndex = idx + tableMentionGroups.opened.length;
                    return (
                      <button
                        key={`${item.value}-${visualIndex}`}
                        type="button"
                        className={cn(
                          "w-full text-left px-2 py-2 rounded-[var(--radius-sm)] text-xs transition-colors",
                          visualIndex === mentionIndex
                            ? "bg-[var(--accent)]/10 text-[var(--accent)]"
                            : "text-[var(--fg)] hover:bg-[var(--sidebar-hover)]"
                        )}
                        onPointerDown={(ev) => {
                          ev.preventDefault();
                          applyMentionCandidate(item.value);
                        }}
                      >
                        <div className="flex items-center gap-1.5 min-w-0">
                          <Table2 className="h-3.5 w-3.5 text-[var(--accent)]/85 shrink-0" />
                          <div className="truncate">{item.display}</div>
                        </div>
                      </button>
                    );
                  })}
                </>
              ) : (
                finalMentionCandidates.map((item, idx) => (
                  <button
                    key={`${item.value}-${idx}`}
                    type="button"
                    className={cn(
                      "w-full text-left px-2 py-2 rounded-[var(--radius-sm)] text-xs transition-colors",
                      idx === mentionIndex
                        ? "bg-[var(--accent)]/10 text-[var(--accent)]"
                        : "text-[var(--fg)] hover:bg-[var(--sidebar-hover)]"
                    )}
                    onPointerDown={(ev) => {
                      ev.preventDefault();
                      applyMentionCandidate(item.value);
                    }}
                  >
                    <div className="flex items-center gap-1.5 min-w-0">
                      {item.kind === "tool" ? (
                        <Wrench className="h-3.5 w-3.5 text-[var(--accent)]/85 shrink-0" />
                      ) : (
                        <Table2 className="h-3.5 w-3.5 text-[var(--accent)]/85 shrink-0" />
                      )}
                      <div className="truncate">{item.display}</div>
                    </div>
                  </button>
                ))
              )}
            </div>
          )}
          <div className="px-1.5 py-1.5 flex-shrink-0 flex items-center justify-center self-center">
            <Button
              variant="ghost"
              size="icon"
              className={cn(
                "h-9 w-9 rounded-full transition-all duration-200 border",
                isStreaming
                  ? "!bg-[var(--danger)] !text-white !border-[var(--danger)] hover:brightness-95 shadow-sm"
                  : !loading && input.trim()
                    ? "!bg-[var(--accent)] !text-[var(--accent-fg)] !border-[var(--accent)] hover:!bg-[var(--accent-hover)] shadow-sm"
                    : "!bg-[var(--surface-secondary)] !text-[var(--fg-muted)] !border-[var(--border-color)] opacity-70"
              )}
              onClick={isStreaming ? handleStopStreaming : handleSend}
              disabled={isStreaming ? false : (loading || !input.trim())}
              data-ui-title-tooltip={isStreaming ? t("common.stop") : undefined}
            >
              {isStreaming ? (
                <Square className="h-3.5 w-3.5 fill-current" />
              ) : (
                <ArrowUp className="h-4 w-4" />
              )}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

function MermaidPreview({ code }: { code: string }) {
  const { t } = useTranslation();
  const [svg, setSvg] = useState("");
  const [errorMsg, setErrorMsg] = useState("");
  const [isRendering, setIsRendering] = useState(true);
  const renderIdRef = useRef(`ai-mermaid-${Math.random().toString(36).slice(2, 10)}`);

  useEffect(() => {
    let disposed = false;
    setIsRendering(true);
    setErrorMsg("");

    (async () => {
      try {
        const mermaidModule = await import("mermaid");
        const mermaid = mermaidModule.default;
        mermaid.initialize({
          startOnLoad: false,
          securityLevel: "loose",
          suppressErrorRendering: true,
        });
        const { svg: renderedSvg } = await mermaid.render(renderIdRef.current, code);
        if (disposed) return;
        setSvg(renderedSvg);
        // 关键日志：记录 mermaid 渲染成功，便于排查图表问题
        console.info("[AIPanel] Mermaid 图表渲染成功");
      } catch (error: any) {
        if (disposed) return;
        const msg = String(error?.message || "Mermaid 渲染失败");
        setSvg("");
        setErrorMsg(msg);
        // 关键日志：记录 mermaid 渲染失败原因，便于快速定位问题
        console.warn("[AIPanel] Mermaid 图表渲染失败:", msg);
      } finally {
        if (!disposed) setIsRendering(false);
      }
    })();

    return () => {
      disposed = true;
    };
  }, [code]);

  if (isRendering) {
    return (
      <div className="px-2 py-2 text-2xs text-[var(--fg-muted)]">
        {t("common.loading")}
      </div>
    );
  }

  if (errorMsg) {
    return (
      <div className="space-y-1.5 px-2 py-2 text-2xs text-[var(--danger)]">
        <div>{t("ai.mermaidRenderFailed")}</div>
        <pre className="overflow-x-auto rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--surface-secondary)] p-2 text-[var(--fg-secondary)]">
          {errorMsg}
        </pre>
      </div>
    );
  }

  return (
    <div
      className="overflow-x-auto px-2 py-2 bg-[var(--surface)]"
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}

function MentionHighlightedText({
  text,
  variant = "default",
  onRemoveMention,
  selectedOccurrence,
}: {
  text: string;
  variant?: MentionHighlightVariant;
  onRemoveMention?: (occurrence: number) => void;
  selectedOccurrence?: number | null;
}) {
  const parts = text.split(/(@(?:tool|table):[^\s]+)/g);
  const baseCls = variant === "input" ? "text-[var(--fg)]" : "";
  let occurrence = -1;

  return (
    <span className={cn("whitespace-pre-wrap break-words", baseCls)}>
      {parts.map((part, idx) => {
        const token = parseMentionToken(part);
        if (token) {
          occurrence += 1;
          return (
            <MentionPill
              key={`mention-${idx}`}
              token={token}
              variant={variant}
              occurrence={occurrence}
              onRemoveMention={onRemoveMention}
              selected={selectedOccurrence === occurrence}
            />
          );
        }
        return <span key={`text-${idx}`}>{part}</span>;
      })}
    </span>
  );
}

function MentionPill({
  token,
  variant,
  occurrence,
  onRemoveMention,
  selected = false,
}: {
  token: MentionToken;
  variant: MentionHighlightVariant;
  occurrence: number;
  onRemoveMention?: (occurrence: number) => void;
  selected?: boolean;
}) {
  if (variant === "input") {
    // 输入框高亮：视觉恢复为胶囊（含图标 + padding），
    // 但用“占位原文 + 绝对定位皮肤”方式避免光标定位漂移。
    return (
      <span title={token.raw} className="group/mention relative inline-block align-baseline">
        <span className="invisible whitespace-pre">{token.raw}</span>
        <span
          aria-hidden
          className={cn(
            "pointer-events-none absolute -inset-x-[1px] inset-y-[-1px]",
            "inline-flex items-center gap-1.5 px-2 rounded-[6px]",
            "text-[var(--accent)] text-[length:var(--size-font-sm)] leading-[1.55] font-medium",
            selected
              ? "bg-gradient-to-r from-[color-mix(in_srgb,var(--accent)_26%,transparent)] to-[color-mix(in_srgb,var(--accent)_50%,transparent)]"
              : "bg-gradient-to-r from-[color-mix(in_srgb,var(--accent)_16%,transparent)] to-[color-mix(in_srgb,var(--accent)_34%,transparent)]"
          )}
        >
          <span className="relative h-4 w-4 shrink-0">
            {token.kind === "tool" ? (
              <Wrench className="absolute inset-0 h-4 w-4 transition-opacity group-hover/mention:opacity-0" />
            ) : (
              <Table2 className="absolute inset-0 h-4 w-4 transition-opacity group-hover/mention:opacity-0" />
            )}
            <X className="absolute inset-0 h-4 w-4 opacity-0 transition-opacity group-hover/mention:opacity-100" />
          </span>
          <span className="truncate">{token.name}</span>
        </span>
        {onRemoveMention ? (
          <button
            type="button"
            className={cn(
              "absolute left-0 top-1/2 -translate-y-1/2",
              "h-3.5 w-3.5 opacity-0 transition-opacity",
              "pointer-events-none group-hover/mention:opacity-100 group-hover/mention:pointer-events-auto"
            )}
            onMouseDown={(e) => {
              e.preventDefault();
              e.stopPropagation();
              onRemoveMention(occurrence);
            }}
            aria-label="Remove mention"
          >
            <X className="h-2.5 w-2.5 mx-auto" />
          </button>
        ) : null}
      </span>
    );
  }

  const isUser = variant === "user";
  return (
    <span
      title={token.raw}
      className={cn(
        "inline-flex items-center gap-1 px-1.5 py-[1px] rounded-[7px] border align-baseline font-medium",
        isUser
          ? "bg-white/20 border-white/35 text-white"
          : "bg-[var(--accent)]/14 border-[var(--accent)]/40 text-[var(--accent)]"
      )}
    >
      {token.kind === "tool" ? (
        <Wrench className="h-3 w-3 shrink-0" />
      ) : (
        <Table2 className="h-3 w-3 shrink-0" />
      )}
      <span className="break-all">{token.name}</span>
    </span>
  );
}

/** 工具调用详情中的语法高亮代码块（JSON / SQL 等） */
const ToolHighlightedCode = React.memo(function ToolHighlightedCode({ code, language }: { code: string; language: string }) {
  const html = getHighlightedHtml(code, language);
  return (
    <pre className="text-[11px] font-mono overflow-x-auto bg-[var(--surface-secondary)] rounded-[var(--radius-sm)] p-1.5 max-w-full">
      <code className={`language-${language}`} dangerouslySetInnerHTML={{ __html: html }} />
    </pre>
  );
});

const MarkdownContent = React.memo(function MarkdownContent({
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

      const displayCode = rawCode;
      const isMermaid = lang === "mermaid";

      let html = "";
      if (!isMermaid) {
        html = getHighlightedHtml(displayCode, lang);
      }

      const canExecute = lang === "sql" && onExecuteSQL;

      return (
        <div className="rounded-[var(--radius-input)] border border-[var(--border-color)] overflow-hidden my-2 max-w-full">
          <div className="flex items-center justify-between px-2 py-1 bg-[var(--surface)] text-2xs text-[var(--fg-muted)]">
            <span>{isMermaid ? t("ai.mermaidPreviewLabel") : (lang || "code")}</span>
            <div className="flex items-center gap-1">
              {canExecute && onApplyAndRunSQL && (
                <button
                  type="button"
                  className="flex items-center gap-0.5 px-1.5 py-0.5 rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] text-[var(--accent)] transition-colors"
                  onClick={() => onApplyAndRunSQL(rawCode)}
                >
                  <ArrowRightToLine className="h-2.5 w-2.5" /> <span>{t("ai.applyAndExecute")}</span>
                </button>
              )}
              {canExecute && (
                <button
                  type="button"
                  className="flex items-center gap-0.5 px-1.5 py-0.5 rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors text-[var(--fg-secondary)]"
                  onClick={() => onExecuteSQL(rawCode)}
                >
                  <Play className="h-2.5 w-2.5" /> {t("ai.executeSQL")}
                </button>
              )}
              <button
                type="button"
                className="px-1.5 py-0.5 rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)]"
                onClick={() => copyToClipboard(displayCode)}
              >
                <Copy className="h-2.5 w-2.5" />
              </button>
            </div>
          </div>
          {isMermaid ? (
            <MermaidPreview code={displayCode} />
          ) : (
            <pre className="p-2 text-xs font-mono overflow-x-auto max-w-full bg-[var(--surface)]">
              <code className="language-code" dangerouslySetInnerHTML={{ __html: html }} />
            </pre>
          )}
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
});
