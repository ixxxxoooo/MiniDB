import React, { useState, useEffect, useRef } from "react";
import { X, TestTube2, Loader2, Check, AlertCircle, Database, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { useTranslation } from "@/i18n";
import { useUIStore } from "@/stores/ui";
import {
  type ConnectionConfig,
  type DatabaseDriver,
  DEFAULT_PORTS,
  DRIVER_LABELS,
  CONNECTION_COLORS,
} from "@/types/connection";
import { generateId } from "@/lib/utils";
import { useConnectionStore } from "@/stores/connection";

interface ConnectionDialogProps {
  open: boolean;
  connection?: ConnectionConfig | null;
  onClose: () => void;
  onSave: (conn: ConnectionConfig) => void;
  onTest: (conn: ConnectionConfig) => Promise<boolean>;
}

export function ConnectionDialog({
  open,
  connection,
  onClose,
  onSave,
  onTest,
}: ConnectionDialogProps) {
  const isEdit = !!connection;
  const { connections } = useConnectionStore();
  const { t } = useTranslation();
  const { layoutMode } = useUIStore();
  const isCompact = layoutMode === "compact";
  const [view, setView] = useState<"list" | "form">(isEdit ? "form" : "list");
  const [search, setSearch] = useState("");

  const [form, setForm] = useState<Partial<ConnectionConfig>>({
    type: "mysql",
    host: "127.0.0.1",
    port: 3306,
    color: CONNECTION_COLORS[0],
  });
  const [testStatus, setTestStatus] = useState<"idle" | "testing" | "success" | "error">("idle");
  const [testError, setTestError] = useState("");
  // 键盘导航：当前高亮的连接索引
  const [highlightIndex, setHighlightIndex] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (connection) {
      setForm(connection);
      setView("form");
    } else {
      setForm({
        type: "mysql", host: "127.0.0.1", port: 3306, color: CONNECTION_COLORS[0],
        name: "", user: "root", password: "", database: "", sslMode: "disable", group: "",
      });
      setView(connections.length > 0 ? "list" : "form");
    }
    setTestStatus("idle");
    setTestError("");
    setSearch("");
  }, [connection, open, connections.length]);

  const filteredConns = connections.filter((c) =>
    !search || c.name.toLowerCase().includes(search.toLowerCase()) || c.host.toLowerCase().includes(search.toLowerCase())
  );

  // 搜索变化时重置高亮索引
  useEffect(() => {
    setHighlightIndex(0);
  }, [search]);

  // 列表视图键盘导航：上下选择、回车确认
  useEffect(() => {
    if (!open || view !== "list" || isEdit) return;
    const handleKeyNav = (e: KeyboardEvent) => {
      if (filteredConns.length === 0) return;
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setHighlightIndex((prev) => Math.min(prev + 1, filteredConns.length - 1));
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setHighlightIndex((prev) => Math.max(prev - 1, 0));
      } else if (e.key === "Enter") {
        e.preventDefault();
        const conn = filteredConns[highlightIndex];
        if (conn) {
          onSave(conn);
          onClose();
        }
      }
    };
    window.addEventListener("keydown", handleKeyNav);
    return () => window.removeEventListener("keydown", handleKeyNav);
  }, [open, view, isEdit, filteredConns, highlightIndex, onSave, onClose]);

  // 高亮项自动滚动到可视区
  useEffect(() => {
    if (!listRef.current) return;
    const items = listRef.current.querySelectorAll(":scope > button");
    const target = items[highlightIndex];
    if (target) target.scrollIntoView({ block: "nearest" });
  }, [highlightIndex]);

  if (!open) return null;

  const updateField = (field: string, value: unknown) => {
    setForm((prev) => ({ ...prev, [field]: value }));
  };

  const handleDriverChange = (type: DatabaseDriver) => {
    setForm((prev) => ({ ...prev, type, port: DEFAULT_PORTS[type] }));
  };

  const handleTest = async () => {
    setTestStatus("testing");
    setTestError("");
    try {
      const success = await onTest(form as ConnectionConfig);
      setTestStatus(success ? "success" : "error");
      if (!success) setTestError(t("connection.testFailed"));
    } catch (e: any) {
      setTestStatus("error");
      setTestError(e?.message || t("connection.testFailed"));
    }
  };

  const handleSave = () => {
    const conn: ConnectionConfig = {
      id: connection?.id || generateId(),
      name: form.name || `${form.host}:${form.port}`,
      type: form.type as DatabaseDriver,
      host: form.host || "127.0.0.1",
      port: form.port || 3306,
      user: form.user || "",
      password: form.password || "",
      database: form.database || "",
      sslMode: form.sslMode || "disable",
      color: form.color || CONNECTION_COLORS[0],
      group: form.group || "",
      createdAt: connection?.createdAt || new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    onSave(conn);
    onClose();
  };

  const handleSelectExisting = (conn: ConnectionConfig) => {
    onSave(conn);
    onClose();
  };

  const spacing = isCompact ? "space-y-2.5" : "space-y-3.5";
  const padding = isCompact ? "p-3.5" : "p-5";
  const headerPadding = isCompact ? "px-3.5 py-2" : "px-5 py-3";

  return (
    <>
      <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm" onClick={onClose} />
      <div
        className={cn(
          "fixed z-50 top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2",
          "rounded-[var(--radius-panel)] shadow-lg border overflow-hidden",
          "bg-[var(--surface)] border-[var(--border-color)]",
          view === "list"
            ? (isCompact ? "w-[420px] max-h-[60vh]" : "w-[480px] max-h-[70vh]")
            : (isCompact ? "w-[460px] max-h-[80vh]" : "w-[520px] max-h-[85vh]")
        )}
      >
        {/* ====== 历史连接列表视图 ====== */}
        {view === "list" && !isEdit && (
          <div className="flex flex-col h-full" style={{ maxHeight: isCompact ? "60vh" : "70vh" }}>
            <div className={cn("flex items-center gap-2 border-b border-[var(--border-color)]", headerPadding)}>
              <button
                className="text-xl font-light text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors h-7 w-7 flex items-center justify-center rounded hover:bg-[var(--sidebar-hover)]"
                onClick={() => setView("form")}
                title={t("connection.newConnection")}
              >
                +
              </button>
              <div className="flex-1 flex items-center gap-1.5 bg-[var(--surface-secondary)] rounded-lg px-2.5 py-1.5">
                <Search className="h-3.5 w-3.5 text-[var(--fg-muted)]" />
                <input
                  className="flex-1 text-sm bg-transparent outline-none text-[var(--fg)] placeholder-[var(--fg-muted)]"
                  placeholder={t("connection.searchConnections")}
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  autoFocus
                />
              </div>
              <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onClose}>
                <X className="h-4 w-4" />
              </Button>
            </div>
            <div ref={listRef} className="flex-1 overflow-y-auto">
              {filteredConns.map((conn, idx) => (
                <button
                  key={conn.id}
                  className={cn(
                    "w-full flex items-center gap-3 transition-colors text-left",
                    isCompact ? "px-3.5 py-2" : "px-4 py-2.5",
                    idx === highlightIndex
                      ? "bg-[var(--row-selected)] ring-1 ring-inset ring-[var(--accent)]"
                      : "hover:bg-[var(--sidebar-hover)]"
                  )}
                  onClick={() => handleSelectExisting(conn)}
                  onMouseEnter={() => setHighlightIndex(idx)}
                >
                  <div
                    className={cn(
                      "rounded-lg flex items-center justify-center text-white font-bold flex-shrink-0",
                      isCompact ? "w-7 h-7 text-2xs" : "w-8 h-8 text-xs"
                    )}
                    style={{ backgroundColor: conn.color || "#007aff" }}
                  >
                    {(conn.type || "M").charAt(0).toUpperCase()}
                    <span className="text-2xs">{(conn.type || "mysql").charAt(1)}</span>
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className={cn("font-medium text-[var(--fg)] truncate", isCompact ? "text-xs" : "text-sm")}>{conn.name}</div>
                    <div className="text-2xs text-[var(--fg-muted)] truncate">
                      {conn.host}:{conn.port}
                      {conn.database && ` : ${conn.database}`}
                    </div>
                  </div>
                  <span className="text-2xs text-[var(--success)]">(local)</span>
                </button>
              ))}
              {filteredConns.length === 0 && (
                <div className="text-center py-8 text-[var(--fg-muted)] text-sm">
                  {search ? t("connection.noMatch") : t("connection.noSaved")}
                </div>
              )}
            </div>
            <div className={cn("border-t border-[var(--border-color)] flex justify-center", isCompact ? "px-3 py-1.5" : "px-4 py-2")}>
              <Button variant="ghost" size="sm" className="text-xs" onClick={() => setView("form")}>
                + {t("connection.createNew")}
              </Button>
            </div>
          </div>
        )}

        {/* ====== 新建/编辑连接表单 ====== */}
        {(view === "form" || isEdit) && (
          <>
            <div className={cn("flex items-center justify-between border-b border-[var(--border-color)]", headerPadding)}>
              <div className="flex items-center gap-2">
                {!isEdit && connections.length > 0 && (
                  <Button variant="ghost" size="sm" className="text-xs" onClick={() => setView("list")}>
                    ← {t("common.back")}
                  </Button>
                )}
                <h2 className={cn("font-semibold", isCompact ? "text-sm" : "text-base")}>
                  {form.type ? `${DRIVER_LABELS[form.type as DatabaseDriver] || ""} Connection` : isEdit ? t("connection.editConnection") : t("connection.newConnection")}
                </h2>
              </div>
              <Button variant="ghost" size="icon" className={isCompact ? "h-6 w-6" : undefined} onClick={onClose}>
                <X className={cn(isCompact ? "h-3.5 w-3.5" : "h-4 w-4")} />
              </Button>
            </div>

            <div className={cn(spacing, "overflow-y-auto", padding)}>
              {/* 数据库类型 */}
              <div className="flex gap-2">
                {(Object.keys(DRIVER_LABELS) as DatabaseDriver[]).map((driver) => (
                  <button
                    key={driver}
                    className={cn(
                      "flex-1 rounded-lg border font-medium transition-all",
                      isCompact ? "py-1.5 px-2 text-xs" : "py-2 px-3 text-sm",
                      form.type === driver
                        ? "border-[var(--accent)] bg-[var(--accent)]/10 text-[var(--accent)] shadow-sm"
                        : "border-[var(--border-color)] text-[var(--fg-secondary)] hover:border-[var(--fg-muted)]"
                    )}
                    onClick={() => handleDriverChange(driver)}
                  >
                    {DRIVER_LABELS[driver]}
                  </button>
                ))}
              </div>

              {/* Name & Color */}
              <div className="flex gap-3 items-end">
                <div className="flex-1">
                  <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1 block">{t("connection.name")}</label>
                  <Input className={isCompact ? "h-7 text-xs" : undefined} value={form.name || ""} onChange={(e) => updateField("name", e.target.value)} placeholder="My Database" />
                </div>
                <div className="flex gap-1.5 pb-0.5">
                  {CONNECTION_COLORS.map((color) => (
                    <button
                      key={color}
                      className={cn(
                        "rounded-full transition-all",
                        isCompact ? "w-4 h-4" : "w-5 h-5",
                        form.color === color && "ring-2 ring-offset-1 ring-[var(--accent)] scale-110"
                      )}
                      style={{ backgroundColor: color }}
                      onClick={() => updateField("color", color)}
                    />
                  ))}
                </div>
              </div>

              {/* Host & Port */}
              {form.type !== "sqlite" && (
                <div className="flex gap-3">
                  <div className="flex-1">
                    <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1 block">{t("connection.host")}</label>
                    <Input className={isCompact ? "h-7 text-xs" : undefined} value={form.host || ""} onChange={(e) => updateField("host", e.target.value)} placeholder="127.0.0.1" />
                  </div>
                  <div className="w-24">
                    <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1 block">{t("connection.port")}</label>
                    <Input className={isCompact ? "h-7 text-xs" : undefined} type="number" value={form.port || ""} onChange={(e) => updateField("port", Number(e.target.value))} />
                  </div>
                </div>
              )}

              {/* User & Password */}
              {form.type !== "sqlite" && (
                <div className="flex gap-3">
                  <div className="flex-1">
                    <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1 block">{t("connection.user")}</label>
                    <Input className={isCompact ? "h-7 text-xs" : undefined} value={form.user || ""} onChange={(e) => updateField("user", e.target.value)} placeholder="root" />
                  </div>
                  <div className="flex-1">
                    <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1 block">{t("connection.password")}</label>
                    <Input className={isCompact ? "h-7 text-xs" : undefined} type="password" value={form.password || ""} onChange={(e) => updateField("password", e.target.value)} placeholder="••••••••" />
                  </div>
                </div>
              )}

              {/* Database */}
              <div>
                <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1 block">
                  {form.type === "sqlite" ? t("connection.databaseFile") : t("connection.database")}
                </label>
                <Input className={isCompact ? "h-7 text-xs" : undefined} value={form.database || ""} onChange={(e) => updateField("database", e.target.value)}
                  placeholder={form.type === "sqlite" ? "/path/to/database.db" : "database name"} />
              </div>

              {/* 测试状态 */}
              {testStatus !== "idle" && (
                <div className={cn(
                  "flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs",
                  { "bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400": testStatus === "testing",
                    "bg-green-50 dark:bg-green-900/20 text-green-600 dark:text-green-400": testStatus === "success",
                    "bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400": testStatus === "error" }
                )}>
                  {testStatus === "testing" && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                  {testStatus === "success" && <Check className="h-3.5 w-3.5" />}
                  {testStatus === "error" && <AlertCircle className="h-3.5 w-3.5" />}
                  <span>
                    {testStatus === "testing" && t("connection.testing")}
                    {testStatus === "success" && t("connection.testSuccess")}
                    {testStatus === "error" && (testError || t("connection.testFailed"))}
                  </span>
                </div>
              )}
            </div>

            {/* 底部按钮 */}
            <div className={cn("flex items-center justify-between border-t border-[var(--border-color)]", headerPadding)}>
              <Button variant="outline" size="sm" className={isCompact ? "h-7 text-xs" : undefined} onClick={handleTest}>
                <TestTube2 className="h-3.5 w-3.5 mr-1.5" />{t("common.test")}
              </Button>
              <div className="flex gap-2">
                <Button variant="ghost" size="sm" className={isCompact ? "h-7 text-xs" : undefined} onClick={onClose}>{t("common.cancel")}</Button>
                <Button size="sm" className={isCompact ? "h-7 text-xs" : undefined} onClick={handleSave}>
                  {isEdit ? t("common.save") : t("connection.saveAndConnect")}
                </Button>
              </div>
            </div>
          </>
        )}
      </div>
    </>
  );
}
