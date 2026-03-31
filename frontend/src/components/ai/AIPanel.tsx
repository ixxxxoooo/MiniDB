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
import * as AIService from "../../../wailsjs/go/services/AIService";
import * as QueryService from "../../../wailsjs/go/services/QueryService";

interface ChatMsg {
  role: "user" | "assistant";
  content: string;
  timestamp: number;
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

export function AIPanel({
  open,
  onClose,
  currentConnectionId,
  currentDatabase,
  width,
  onWidthChange,
}: AIPanelProps) {
  const [sessions, setSessions] = useState<ChatSession[]>(() => loadSessions());
  const [activeSessionId, setActiveSessionId] = useState<string | null>(() => {
    const saved = loadSessions();
    return saved.length > 0 ? saved[0].id : null;
  });
  const [showHistory, setShowHistory] = useState(false);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [copiedIdx, setCopiedIdx] = useState<number | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const resizingRef = useRef(false);

  const activeSession = sessions.find((s) => s.id === activeSessionId) || null;
  const messages = activeSession?.messages || [];

  useEffect(() => {
    saveSessions(sessions);
  }, [sessions]);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages, loading]);

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
      title: "新对话",
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
  }, [currentConnectionId, currentDatabase]);

  const updateSessionMessages = useCallback((sessionId: string, msgs: ChatMsg[], title?: string) => {
    setSessions((prev) =>
      prev.map((s) =>
        s.id === sessionId
          ? { ...s, messages: msgs, updatedAt: Date.now(), ...(title ? { title } : {}) }
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

  const handleSend = async () => {
    const text = input.trim();
    if (!text || loading) return;

    let sessionId = activeSessionId;
    let currentMessages = [...messages];

    // 如果没有当前会话，自动创建
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

    const userMsg: ChatMsg = { role: "user", content: text, timestamp: Date.now() };
    const newMsgs = [...currentMessages, userMsg];

    // 如果是会话的第一条消息，更新标题
    const isFirst = currentMessages.length === 0;
    updateSessionMessages(sessionId, newMsgs, isFirst ? generateTitle(text) : undefined);
    setInput("");
    setLoading(true);

    try {
      const apiMessages = newMsgs.map((m) => ({ role: m.role, content: m.content }));
      const result = await AIService.ChatAI(
        currentConnectionId || "",
        currentDatabase || "",
        apiMessages
      );

      let aiContent = result?.content || "AI 未返回内容";

      // 检测 [AUTO_EXECUTE] 标记，自动执行 SQL
      if (aiContent.includes("[AUTO_EXECUTE]") && currentConnectionId && currentDatabase) {
        aiContent = aiContent.replace("[AUTO_EXECUTE]", "");
        const sqlMatch = aiContent.match(/```sql\n([\s\S]*?)```/);
        if (sqlMatch) {
          const sql = sqlMatch[1].trim();
          try {
            const queryResult = await QueryService.ExecuteSQL(currentConnectionId, currentDatabase, sql);
            if (queryResult?.error) {
              aiContent += `\n\n**执行错误**: ${queryResult.error}`;
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
              aiContent += `\n\n**查询结果** (共 ${queryResult.total} 行, ${queryResult.duration}ms):\n\n${header}\n${sep}\n${rows.join("\n")}`;
              if (queryResult.rows.length > 50) {
                aiContent += `\n| ... 省略 ${queryResult.rows.length - 50} 行 |`;
              }
            } else {
              aiContent += `\n\n**执行成功**，影响 ${queryResult?.total || 0} 行 (${queryResult?.duration || 0}ms)`;
            }
          } catch (e: any) {
            aiContent += `\n\n**执行失败**: ${e?.message || e}`;
          }
        }
      }

      const assistantMsg: ChatMsg = { role: "assistant", content: aiContent, timestamp: Date.now() };
      const finalMsgs = [...newMsgs, assistantMsg];
      updateSessionMessages(sessionId, finalMsgs);
    } catch (e: any) {
      const errorMsg: ChatMsg = {
        role: "assistant",
        content: `**错误**: ${e?.message || "AI 请求失败"}`,
        timestamp: Date.now(),
      };
      updateSessionMessages(sessionId, [...newMsgs, errorMsg]);
    } finally {
      setLoading(false);
    }
  };

  const handleCopy = async (idx: number, content: string) => {
    await copyToClipboard(content);
    setCopiedIdx(idx);
    setTimeout(() => setCopiedIdx(null), 2000);
  };

  const handleExecuteSQL = async (sql: string) => {
    if (!currentConnectionId || !currentDatabase || !activeSessionId) return;
    setLoading(true);
    try {
      const result = await QueryService.ExecuteSQL(currentConnectionId, currentDatabase, sql);
      let content = "";
      if (result?.error) {
        content = `**执行错误**: ${result.error}`;
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
        content = `**查询结果** (${result.total} 行, ${result.duration}ms):\n\n${header}\n${sep}\n${rows.join("\n")}`;
      } else {
        content = `**执行成功**，影响 ${result?.total || 0} 行 (${result?.duration || 0}ms)`;
      }
      const execMsg: ChatMsg = { role: "assistant", content, timestamp: Date.now() };
      updateSessionMessages(activeSessionId, [...messages, execMsg]);
    } catch (e: any) {
      const errMsg: ChatMsg = {
        role: "assistant",
        content: `**执行失败**: ${e?.message || e}`,
        timestamp: Date.now(),
      };
      updateSessionMessages(activeSessionId, [...messages, errMsg]);
    } finally {
      setLoading(false);
    }
  };

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
        <div
          className="absolute left-0 top-0 h-full w-1.5 cursor-col-resize hover:bg-[var(--accent)] opacity-0 hover:opacity-40 transition-opacity z-10"
          onMouseDown={handleResizeStart}
        />
        <div className="flex items-center justify-between px-3 py-2 border-b border-[var(--border-color)] flex-shrink-0">
          <div className="flex items-center gap-1.5">
            <button
              className="p-0.5 rounded hover:bg-[var(--sidebar-hover)] transition-colors"
              onClick={() => setShowHistory(false)}
            >
              <ChevronLeft className="h-4 w-4" />
            </button>
            <span className="text-sm font-medium">会话历史</span>
            <span className="text-2xs text-[var(--fg-muted)]">({sessions.length})</span>
          </div>
          <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onClose}>
            <X className="h-3 w-3" />
          </Button>
        </div>

        <div className="flex-1 overflow-y-auto">
          {sessions.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-[var(--fg-muted)] text-sm">
              <MessageSquare className="h-8 w-8 mb-3 opacity-30" />
              <p>暂无历史会话</p>
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
                    {session.messages.length} 条消息 · {new Date(session.updatedAt).toLocaleString("zh-CN", { month: "numeric", day: "numeric", hour: "numeric", minute: "numeric" })}
                  </div>
                </div>
                <button
                  className="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-[var(--surface-secondary)] transition-opacity"
                  onClick={(e) => {
                    e.stopPropagation();
                    deleteSession(session.id);
                  }}
                  title="删除会话"
                >
                  <Trash2 className="h-3 w-3 text-[var(--fg-muted)]" />
                </button>
              </div>
            ))
          )}
        </div>

        <div className="px-3 py-2 border-t border-[var(--border-color)]">
          <Button size="sm" className="w-full h-8 text-xs" onClick={createNewSession}>
            <Plus className="h-3 w-3 mr-1" /> 新建对话
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
      <div
        className="absolute left-0 top-0 h-full w-1.5 cursor-col-resize hover:bg-[var(--accent)] opacity-0 hover:opacity-40 transition-opacity z-10"
        onMouseDown={handleResizeStart}
      />

      {/* 头部 */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-[var(--border-color)] flex-shrink-0">
        <div className="flex items-center gap-1.5">
          <Sparkles className="h-4 w-4 text-[var(--accent)]" />
          <span className="text-sm font-medium">AI 助手</span>
        </div>
        <div className="flex items-center gap-0.5">
          <Button variant="ghost" size="icon" className="h-6 w-6" onClick={createNewSession} title="新建对话">
            <Plus className="h-3 w-3" />
          </Button>
          <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => setShowHistory(true)} title="会话历史">
            <History className="h-3 w-3" />
          </Button>
          <Button variant="ghost" size="icon" className="h-6 w-6" onClick={handleClearSession} title="清空当前对话">
            <Trash2 className="h-3 w-3" />
          </Button>
          <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onClose}>
            <X className="h-3 w-3" />
          </Button>
        </div>
      </div>

      {/* 上下文信息 */}
      {currentDatabase && (
        <div className="px-3 py-1.5 text-2xs text-[var(--fg-muted)] border-b border-[var(--border-subtle)] flex-shrink-0 flex items-center gap-2">
          <span>数据库: {currentDatabase}</span>
          {activeSession && (
            <>
              <span className="text-[var(--border-color)]">|</span>
              <span className="truncate">{activeSession.title}</span>
            </>
          )}
        </div>
      )}

      {/* 对话区域 */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto p-3 space-y-3">
        {messages.length === 0 && !loading && (
          <div className="flex flex-col items-center justify-center h-full text-[var(--fg-muted)] text-sm">
            <Sparkles className="h-8 w-8 mb-3 opacity-30" />
            <p className="font-medium text-[var(--fg-secondary)]">AI 数据库助手</p>
            <p className="text-xs mt-1">输入自然语言查询数据、生成 SQL、分析数据</p>
            <div className="mt-4 space-y-1 text-xs text-[var(--fg-muted)]">
              <p>试试：</p>
              <p className="italic">"查询最近 7 天的订单数据"</p>
              <p className="italic">"解释这条 SQL: SELECT ..."</p>
              <p className="italic">"统计每个状态的数量"</p>
            </div>
          </div>
        )}

        {messages.map((msg, idx) => (
          <div key={`${msg.timestamp}-${idx}`} className={cn("flex", msg.role === "user" ? "justify-end" : "justify-start")}>
            <div className={cn(
              "max-w-[95%] rounded-lg px-3 py-2 text-xs leading-relaxed",
              msg.role === "user"
                ? "bg-[var(--accent)] text-white"
                : "bg-[var(--surface-secondary)] text-[var(--fg)]"
            )}>
              {msg.role === "assistant" ? (
                <div className="relative group">
                  <MarkdownContent
                    content={msg.content}
                    onExecuteSQL={handleExecuteSQL}
                  />
                  <button
                    className="absolute top-0 right-0 opacity-0 group-hover:opacity-100 transition-opacity p-1 rounded hover:bg-[var(--sidebar-hover)]"
                    onClick={() => handleCopy(idx, msg.content)}
                  >
                    {copiedIdx === idx ? <Check className="h-3 w-3 text-[var(--success)]" /> : <Copy className="h-3 w-3" />}
                  </button>
                </div>
              ) : (
                <span className="whitespace-pre-wrap">{msg.content}</span>
              )}
            </div>
          </div>
        ))}

        {loading && (
          <div className="flex justify-start">
            <div className="bg-[var(--surface-secondary)] rounded-lg px-3 py-2 text-xs flex items-center gap-2 text-[var(--fg-secondary)]">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              AI 正在思考...
            </div>
          </div>
        )}
      </div>

      {/* 输入区域 */}
      <div className="px-3 py-2 border-t border-[var(--border-color)] flex-shrink-0">
        <div className="flex gap-2">
          <textarea
            ref={inputRef}
            className={cn(
              "flex-1 resize-none rounded-md border px-3 py-2 text-xs",
              "bg-[var(--surface)] border-[var(--border-color)] text-[var(--fg)]",
              "placeholder:text-[var(--fg-muted)] focus:outline-none focus:ring-1 focus:ring-[var(--accent)]"
            )}
            placeholder="输入问题，如：查询所有状态为 active 的用户..."
            value={input}
            rows={2}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                handleSend();
              }
            }}
          />
          <Button
            size="icon"
            className="h-full w-9 flex-shrink-0"
            onClick={handleSend}
            disabled={loading || !input.trim()}
          >
            <Send className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>
    </div>
  );
}

// Markdown 渲染组件
function MarkdownContent({ content, onExecuteSQL }: { content: string; onExecuteSQL?: (sql: string) => void }) {
  const parts = content.split(/(```[\s\S]*?```)/g);

  return (
    <div className="space-y-2">
      {parts.map((part, i) => {
        if (part.startsWith("```")) {
          const match = part.match(/```(\w*)\n?([\s\S]*?)```/);
          if (match) {
            const lang = match[1];
            const code = match[2].trim();
            const isSQL = lang.toLowerCase() === "sql";
            return (
              <div key={i} className="rounded border border-[var(--border-color)] overflow-hidden my-2">
                <div className="flex items-center justify-between px-2 py-1 bg-[var(--surface)] text-2xs text-[var(--fg-muted)]">
                  <span>{lang || "code"}</span>
                  <div className="flex items-center gap-1">
                    {isSQL && onExecuteSQL && (
                      <button
                        className="flex items-center gap-0.5 px-1.5 py-0.5 rounded hover:bg-[var(--sidebar-hover)] text-[var(--accent)]"
                        onClick={() => onExecuteSQL(code)}
                      >
                        <Play className="h-2.5 w-2.5" /> 执行
                      </button>
                    )}
                    <button
                      className="px-1.5 py-0.5 rounded hover:bg-[var(--sidebar-hover)]"
                      onClick={() => copyToClipboard(code)}
                    >
                      <Copy className="h-2.5 w-2.5" />
                    </button>
                  </div>
                </div>
                <pre className="p-2 text-xs font-mono overflow-x-auto bg-[var(--surface)]">{code}</pre>
              </div>
            );
          }
        }

        return (
          <div key={i} className="whitespace-pre-wrap">
            {part.split("\n").map((line, li) => {
              if (line.startsWith("### ")) return <h4 key={li} className="font-bold text-sm mt-2 mb-1">{line.slice(4)}</h4>;
              if (line.startsWith("## ")) return <h3 key={li} className="font-bold text-sm mt-2 mb-1">{line.slice(3)}</h3>;
              if (line.startsWith("# ")) return <h2 key={li} className="font-bold mt-2 mb-1">{line.slice(2)}</h2>;
              if (line.startsWith("**") && line.endsWith("**")) return <p key={li} className="font-bold">{line.slice(2, -2)}</p>;
              if (line.startsWith("- ")) return <p key={li} className="ml-2">• {renderInline(line.slice(2))}</p>;
              if (/^\d+\.\s/.test(line)) return <p key={li} className="ml-2">{renderInline(line)}</p>;
              if (line.startsWith("|") && line.endsWith("|")) {
                if (line.includes("---")) return null;
                const cells = line.split("|").filter(Boolean).map(s => s.trim());
                return (
                  <div key={li} className="flex font-mono text-2xs">
                    {cells.map((cell, ci) => (
                      <span key={ci} className="flex-1 px-1 py-0.5 border-b border-[var(--border-subtle)] truncate">{cell}</span>
                    ))}
                  </div>
                );
              }
              if (!line.trim()) return <br key={li} />;
              return <p key={li}>{renderInline(line)}</p>;
            })}
          </div>
        );
      })}
    </div>
  );
}

function renderInline(text: string) {
  return text.replace(/\*\*(.*?)\*\*/g, "").split(/(`[^`]+`)/).map((part, i) => {
    if (part.startsWith("`") && part.endsWith("`")) {
      return <code key={i} className="bg-[var(--surface)] px-1 py-0.5 rounded text-2xs font-mono">{part.slice(1, -1)}</code>;
    }
    const boldParts = part.split(/\*\*(.*?)\*\*/g);
    if (boldParts.length > 1) {
      return boldParts.map((bp, bi) => bi % 2 === 1 ? <strong key={`${i}-${bi}`}>{bp}</strong> : <span key={`${i}-${bi}`}>{bp}</span>);
    }
    return <span key={i}>{part}</span>;
  });
}
