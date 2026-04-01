import React, { useState, useEffect, useRef } from "react";
import { X, TestTube2, Loader2, Check, AlertCircle, Search, Pencil } from "lucide-react";
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
import { DriverIcon } from "@/components/icons/DatabaseIcons";

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

  // 全局 ESC 关闭弹窗
  useEffect(() => {
    if (!open) return;
    const handleEsc = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
      }
    };
    window.addEventListener("keydown", handleEsc);
    return () => window.removeEventListener("keydown", handleEsc);
  }, [open, onClose]);

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
    const existingId = connection?.id || form.id;
    const conn: ConnectionConfig = {
      id: existingId || generateId(),
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
      createdAt: connection?.createdAt || form.createdAt || new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    onSave(conn);
    onClose();
  };

  const handleSelectExisting = (conn: ConnectionConfig) => {
    onSave(conn);
    onClose();
  };

  // 编辑已有连接：回填表单数据并切换到表单视图
  const handleEditExisting = (conn: ConnectionConfig) => {
    setForm({ ...conn });
    setView("form");
    setTestStatus("idle");
    setTestError("");
  };

  const spacing = isCompact ? "space-y-2" : "space-y-3";
  const padding = isCompact ? "p-3" : "p-4";
  const headerPadding = isCompact ? "px-3 py-1.5" : "px-3 py-2.5";

  return (
    <>
      <div className="fixed inset-0 z-50 bg-black/20 backdrop-blur-sm" onClick={onClose} />
      <div
        className={cn(
          "fixed z-50 top-[20%] left-1/2 -translate-x-1/2",
          "rounded-[var(--radius-panel)] shadow-lg border overflow-hidden",
          "bg-[var(--surface)] border-[var(--border-color)]",
          view === "list"
            ? "w-[380px] max-h-[380px]"
            : (isCompact ? "w-[440px]" : "w-[480px]")
        )}
      >
        {/* ====== 历史连接列表视图 ====== */}
        {view === "list" && !isEdit && (
          <div className="flex flex-col h-full" style={{ maxHeight: "380px" }}>
            {/* 搜索栏 - 与 DatabaseSwitcher 风格统一 */}
            <div className="flex items-center gap-2 px-3 py-2 border-b border-[var(--border-color)]">
              <button
                className="text-lg font-light text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors h-6 w-6 flex items-center justify-center rounded hover:bg-[var(--sidebar-hover)]"
                onClick={() => setView("form")}
                title={t("connection.newConnection")}
              >
                +
              </button>
              <Search className="h-3.5 w-3.5 text-[var(--fg-muted)]" />
              <input
                className="flex-1 bg-transparent text-sm text-[var(--fg)] placeholder:text-[var(--fg-muted)] focus:outline-none"
                placeholder={`${t("connection.searchConnections")} (⌘F)`}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                autoFocus
              />
              <button
                className="h-5 w-5 flex items-center justify-center rounded hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors"
                onClick={onClose}
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
            {/* 连接列表 */}
            <div ref={listRef} className="flex-1 overflow-y-auto py-0.5">
              {filteredConns.map((conn, idx) => (
                <button
                  key={conn.id}
                  className={cn(
                    "w-full flex items-center gap-2.5 px-3 py-1.5 text-left transition-colors group",
                    idx === highlightIndex
                      ? "bg-[var(--accent)] text-white"
                      : "text-[var(--fg)] hover:bg-[var(--sidebar-hover)]"
                  )}
                  onClick={() => handleSelectExisting(conn)}
                  onMouseEnter={() => setHighlightIndex(idx)}
                >
                  {/* 数据库类型图标 */}
                  <DriverIcon driver={conn.type || "mysql"} className="w-5 h-5 rounded-[3px]" />
                  {/* 连接信息 */}
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium truncate">{conn.name}</div>
                    <div className={cn(
                      "text-2xs truncate",
                      idx === highlightIndex ? "text-white/60" : "text-[var(--fg-muted)]"
                    )}>
                      {conn.host}:{conn.port}
                    </div>
                  </div>
                  {/* 编辑按钮 */}
                  <button
                    className={cn(
                      "opacity-0 group-hover:opacity-100 h-5 w-5 flex items-center justify-center rounded-[var(--radius-sm)] transition-all flex-shrink-0",
                      idx === highlightIndex ? "hover:bg-white/20" : "hover:bg-[var(--sidebar-hover)]"
                    )}
                    onClick={(e) => {
                      e.stopPropagation();
                      handleEditExisting(conn);
                    }}
                    title={t("common.edit")}
                  >
                    <Pencil className={cn("h-2.5 w-2.5", idx === highlightIndex ? "text-white/80" : "text-[var(--fg-secondary)]")} />
                  </button>
                  {/* local 标记 */}
                  <span className={cn(
                    "text-2xs flex-shrink-0",
                    idx === highlightIndex ? "text-white/60" : "text-[var(--success)]"
                  )}>(local)</span>
                </button>
              ))}
              {filteredConns.length === 0 && (
                <div className="text-center py-6 text-[var(--fg-muted)] text-sm">
                  {search ? t("connection.noMatch") : t("connection.noSaved")}
                </div>
              )}
            </div>
            {/* 底部新建连接 */}
            <div className="border-t border-[var(--border-color)] flex justify-center px-3 py-1.5">
              <button
                className="text-xs text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors"
                onClick={() => setView("form")}
              >
                + {t("connection.createNew")}
              </button>
            </div>
          </div>
        )}

        {/* ====== 新建/编辑连接表单 ====== */}
        {(view === "form" || isEdit) && (
          <>
            {/* 顶部标题栏 */}
            <div className={cn("flex items-center justify-between border-b border-[var(--border-color)]", headerPadding)}>
              <div className="flex items-center gap-2">
                {!isEdit && connections.length > 0 && (
                  <button
                    className="text-xs text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors"
                    onClick={() => { setView("list"); setForm({ type: "mysql", host: "127.0.0.1", port: 3306, color: CONNECTION_COLORS[0], name: "", user: "root", password: "", database: "", sslMode: "disable", group: "" }); setTestStatus("idle"); }}
                  >
                    ← {t("common.back")}
                  </button>
                )}
                <h2 className="text-sm font-semibold text-[var(--fg)]">
                  {form.id
                    ? `${t("connection.editConnection")} — ${DRIVER_LABELS[form.type as DatabaseDriver] || ""}`
                    : form.type ? `${DRIVER_LABELS[form.type as DatabaseDriver] || ""} Connection` : t("connection.newConnection")}
                </h2>
              </div>
              <button
                className="h-5 w-5 flex items-center justify-center rounded hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors"
                onClick={onClose}
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>

            <div className={cn(spacing, "overflow-y-auto", padding)}>
              {/* 数据库类型选择器 - 带图标 */}
              <div className="flex gap-1.5">
                {(Object.keys(DRIVER_LABELS) as DatabaseDriver[]).map((driver) => (
                  <button
                    key={driver}
                    className={cn(
                      "flex items-center gap-1.5 rounded-[var(--radius-btn)] border font-medium transition-all",
                      "py-1 px-2 text-xs",
                      form.type === driver
                        ? "border-[var(--accent)] bg-[var(--accent)]/10 text-[var(--accent)]"
                        : "border-[var(--border-color)] text-[var(--fg-secondary)] hover:border-[var(--fg-muted)]"
                    )}
                    onClick={() => handleDriverChange(driver)}
                  >
                    <DriverIcon driver={driver} className="w-4 h-4 rounded-[2px]" />
                    <span>{DRIVER_LABELS[driver]}</span>
                  </button>
                ))}
              </div>

              {/* 名称 & 颜色 */}
              <div className="flex gap-2.5 items-end">
                <div className="flex-1">
                  <label className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)] mb-0.5 block">{t("connection.name")}</label>
                  <Input className="h-[var(--size-input-sm)] text-[length:var(--size-font-sm)]" value={form.name || ""} onChange={(e) => updateField("name", e.target.value)} placeholder="My Database" />
                </div>
                <div className="flex gap-1 pb-0.5">
                  {CONNECTION_COLORS.map((color) => (
                    <button
                      key={color}
                      className={cn(
                        "w-4 h-4 rounded-full transition-all",
                        form.color === color && "ring-2 ring-offset-1 ring-[var(--accent)] scale-110"
                      )}
                      style={{ backgroundColor: color }}
                      onClick={() => updateField("color", color)}
                    />
                  ))}
                </div>
              </div>

              {/* 主机 & 端口 */}
              {form.type !== "sqlite" && (
                <div className="flex gap-2.5">
                  <div className="flex-1">
                    <label className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)] mb-0.5 block">{t("connection.host")}</label>
                    <Input className="h-[var(--size-input-sm)] text-[length:var(--size-font-sm)]" value={form.host || ""} onChange={(e) => updateField("host", e.target.value)} placeholder="127.0.0.1" />
                  </div>
                  <div className="w-20">
                    <label className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)] mb-0.5 block">{t("connection.port")}</label>
                    <Input className="h-[var(--size-input-sm)] text-[length:var(--size-font-sm)]" type="number" value={form.port || ""} onChange={(e) => updateField("port", Number(e.target.value))} />
                  </div>
                </div>
              )}

              {/* 用户名 & 密码 */}
              {form.type !== "sqlite" && (
                <div className="flex gap-2.5">
                  <div className="flex-1">
                    <label className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)] mb-0.5 block">{t("connection.user")}</label>
                    <Input className="h-[var(--size-input-sm)] text-[length:var(--size-font-sm)]" value={form.user || ""} onChange={(e) => updateField("user", e.target.value)} placeholder="root" />
                  </div>
                  <div className="flex-1">
                    <label className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)] mb-0.5 block">{t("connection.password")}</label>
                    <Input className="h-[var(--size-input-sm)] text-[length:var(--size-font-sm)]" type="password" value={form.password || ""} onChange={(e) => updateField("password", e.target.value)} placeholder="••••••••" />
                  </div>
                </div>
              )}

              {/* 数据库名 */}
              <div>
                <label className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)] mb-0.5 block">
                  {form.type === "sqlite" ? t("connection.databaseFile") : t("connection.database")}
                </label>
                <Input className="h-[var(--size-input-sm)] text-[length:var(--size-font-sm)]" value={form.database || ""} onChange={(e) => updateField("database", e.target.value)}
                  placeholder={form.type === "sqlite" ? "/path/to/database.db" : "database name"} />
              </div>

              {/* 测试状态提示 */}
              {testStatus !== "idle" && (
                <div className={cn(
                  "flex items-center gap-2 px-2.5 py-1 rounded-[var(--radius-sm)] text-xs",
                  { "bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400": testStatus === "testing",
                    "bg-green-50 dark:bg-green-900/20 text-green-600 dark:text-green-400": testStatus === "success",
                    "bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400": testStatus === "error" }
                )}>
                  {testStatus === "testing" && <Loader2 className="h-3 w-3 animate-spin" />}
                  {testStatus === "success" && <Check className="h-3 w-3" />}
                  {testStatus === "error" && <AlertCircle className="h-3 w-3" />}
                  <span>
                    {testStatus === "testing" && t("connection.testing")}
                    {testStatus === "success" && t("connection.testSuccess")}
                    {testStatus === "error" && (testError || t("connection.testFailed"))}
                  </span>
                </div>
              )}
            </div>

            {/* 底部操作栏 */}
            <div className={cn("flex items-center justify-between border-t border-[var(--border-color)]", headerPadding)}>
              <Button variant="outline" size="sm" className="h-[var(--size-btn-sm)] text-xs" onClick={handleTest}>
                <TestTube2 className="h-3 w-3 mr-1" />{t("common.test")}
              </Button>
              <div className="flex gap-1.5">
                <Button variant="ghost" size="sm" className="h-[var(--size-btn-sm)] text-xs" onClick={onClose}>{t("common.cancel")}</Button>
                <Button size="sm" className="h-[var(--size-btn-sm)] text-xs" onClick={handleSave}>
                  {(isEdit || form.id) ? t("common.save") : t("connection.saveAndConnect")}
                </Button>
              </div>
            </div>
          </>
        )}
      </div>
    </>
  );
}
