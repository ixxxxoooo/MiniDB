import React, { useState, useEffect, useRef } from "react";
import { X, Loader2, Check, AlertCircle, Search, Pencil } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { useTranslation } from "@/i18n";
import { useUIStore } from "@/stores/ui";
import {
  type ConnectionConfig,
  type DatabaseDriver,
  type ConnectionTag,
  DEFAULT_PORTS,
  DRIVER_LABELS,
  CONNECTION_COLORS,
  TAG_COLORS,
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

// 默认表单初始值
const defaultForm = (): Partial<ConnectionConfig> => ({
  type: "mysql",
  host: "127.0.0.1",
  port: 3306,
  color: CONNECTION_COLORS[0],
  tag: "local" as ConnectionTag,
  name: "",
  user: "root",
  password: "",
  database: "",
  sslMode: "disable",
  group: "",
});

// Tag 选项列表
const TAG_OPTIONS: ConnectionTag[] = ["local", "test", "production"];

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

  const [form, setForm] = useState<Partial<ConnectionConfig>>(defaultForm());
  const [testStatus, setTestStatus] = useState<"idle" | "testing" | "success" | "error">("idle");
  const [testError, setTestError] = useState("");
  // 表单校验错误
  const [errors, setErrors] = useState<Record<string, boolean>>({});
  // 键盘导航：当前高亮的连接索引
  const [highlightIndex, setHighlightIndex] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (connection) {
      setForm({ ...connection, tag: connection.tag || "local" });
      setView("form");
    } else {
      setForm(defaultForm());
      setView(connections.length > 0 ? "list" : "form");
    }
    setTestStatus("idle");
    setTestError("");
    setErrors({});
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
    const items = listRef.current.querySelectorAll(":scope > [role='button']");
    const target = items[highlightIndex];
    if (target) target.scrollIntoView({ block: "nearest" });
  }, [highlightIndex]);

  if (!open) return null;

  const updateField = (field: string, value: unknown) => {
    setForm((prev) => ({ ...prev, [field]: value }));
    // 清除对应字段的错误
    if (errors[field]) {
      setErrors((prev) => ({ ...prev, [field]: false }));
    }
  };

  const handleDriverChange = (type: DatabaseDriver) => {
    setForm((prev) => ({ ...prev, type, port: DEFAULT_PORTS[type] }));
  };

  // 表单校验：除 password 外所有字段必填
  const validateForm = (): boolean => {
    const newErrors: Record<string, boolean> = {};
    if (!form.name?.trim()) newErrors.name = true;
    if (form.type !== "sqlite") {
      if (!form.host?.trim()) newErrors.host = true;
      if (!form.port) newErrors.port = true;
      if (!form.user?.trim()) newErrors.user = true;
    }
    if (!form.database?.trim()) newErrors.database = true;
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleTest = async () => {
    if (!validateForm()) return;
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
    if (!validateForm()) return;
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
      tag: (form.tag as ConnectionTag) || "local",
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
    setForm({ ...conn, tag: conn.tag || "local" });
    setView("form");
    setTestStatus("idle");
    setTestError("");
    setErrors({});
  };

  // 获取当前 tag 颜色信息
  const currentTag = (form.tag || "local") as ConnectionTag;
  const tagColor = TAG_COLORS[currentTag];

  // Tag 标签翻译
  const tagLabel = (tag: ConnectionTag) => {
    switch (tag) {
      case "local": return t("connection.tagLocal");
      case "test": return t("connection.tagTest");
      case "production": return t("connection.tagProduction");
    }
  };

  // 带错误提示的 label
  const FieldLabel = ({ label, error, required = true }: { label: string; error?: boolean; required?: boolean }) => (
    <div className="flex items-center gap-1 mb-0.5">
      <label className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)]">
        {label}
      </label>
      {required && <span className="text-[length:var(--size-font-2xs)] text-red-400">*</span>}
      {error && (
        <span className="text-[length:var(--size-font-2xs)] text-red-400 ml-auto">
          {t("connection.requiredField")}
        </span>
      )}
    </div>
  );

  return (
    <>
      <div className="fixed inset-0 z-50 bg-black/20 backdrop-blur-sm" onClick={onClose} />
      <div
        className={cn(
          "fixed z-50 top-[18%] left-1/2 -translate-x-1/2",
          "rounded-[var(--radius-panel)] shadow-lg border",
          "bg-[var(--surface)] border-[var(--border-color)]",
          view === "list"
            ? "w-[380px] max-h-[380px]"
            : (isCompact ? "w-[500px]" : "w-[540px]")
        )}
      >
        {/* ====== 历史连接列表视图 ====== */}
        {view === "list" && !isEdit && (
          <div className="flex flex-col h-full" style={{ maxHeight: "380px" }}>
            {/* 搜索栏 */}
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
              {filteredConns.map((conn, idx) => {
                const connTagColor = TAG_COLORS[conn.tag || "local"];
                return (
                  <div
                    key={conn.id}
                    role="button"
                    tabIndex={0}
                    className={cn(
                      "w-full flex items-center gap-2.5 px-3 py-1.5 text-left transition-colors group cursor-pointer",
                      idx === highlightIndex
                        ? "bg-[var(--accent)] text-white"
                        : "text-[var(--fg)] hover:bg-[var(--sidebar-hover)]"
                    )}
                    onClick={() => handleSelectExisting(conn)}
                    onMouseEnter={() => setHighlightIndex(idx)}
                    onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") handleSelectExisting(conn); }}
                  >
                    <DriverIcon driver={conn.type || "mysql"} className="w-5 h-5 rounded-[3px]" />
                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-medium truncate">{conn.name}</div>
                      <div className={cn(
                        "text-2xs truncate",
                        idx === highlightIndex ? "text-white/60" : "text-[var(--fg-muted)]"
                      )}>
                        {conn.host}:{conn.port}
                      </div>
                    </div>
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
                    <span
                      className="text-2xs flex-shrink-0 px-1.5 py-0.5 rounded-[var(--radius-sm)] font-medium"
                      style={{
                        backgroundColor: idx === highlightIndex ? "rgba(255,255,255,0.15)" : connTagColor.bg,
                        color: idx === highlightIndex ? "rgba(255,255,255,0.8)" : connTagColor.text,
                        border: `1px solid ${idx === highlightIndex ? "rgba(255,255,255,0.2)" : connTagColor.border}`,
                      }}
                    >
                      {tagLabel(conn.tag || "local")}
                    </span>
                  </div>
                );
              })}
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
            <div className="flex items-center justify-center border-b border-[var(--border-color)] px-3 py-2 relative">
              {!isEdit && connections.length > 0 && (
                <button
                  className="absolute left-3 text-xs text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors"
                  onClick={() => { setView("list"); setForm(defaultForm()); setTestStatus("idle"); setErrors({}); }}
                >
                  ← {t("common.back")}
                </button>
              )}
              <h2 className="text-sm font-semibold text-[var(--fg)]">
                {form.id
                  ? `${DRIVER_LABELS[form.type as DatabaseDriver] || ""} Connection`
                  : form.type ? `${DRIVER_LABELS[form.type as DatabaseDriver] || ""} Connection` : t("connection.newConnection")}
              </h2>
              <button
                className="absolute right-3 h-5 w-5 flex items-center justify-center rounded hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors"
                onClick={onClose}
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>

            <div className={cn("p-4", isCompact && "p-3")}>
              {/* 数据库类型选择器 */}
              <div className="flex gap-1.5 mb-4">
                {(Object.keys(DRIVER_LABELS) as DatabaseDriver[]).map((driver) => (
                  <button
                    key={driver}
                    className={cn(
                      "flex items-center gap-1.5 rounded-[var(--radius-btn)] border font-medium transition-all",
                      "py-1 px-2.5 text-xs",
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

              {/* 名称 + 颜色 + Tag：第一行 */}
              <div className="flex gap-3 mb-3">
                <div className="flex-1">
                  <FieldLabel label={t("connection.name")} error={errors.name} />
                  <Input
                    className={cn(
                      "h-[var(--size-input-sm)] text-[length:var(--size-font-sm)]",
                      errors.name && "border-red-400 focus:ring-red-400"
                    )}
                    value={form.name || ""}
                    onChange={(e) => updateField("name", e.target.value)}
                    placeholder={t("connection.namePlaceholder")}
                  />
                </div>
                {/* Tag 选择器 */}
                <div className="w-[130px]">
                  <FieldLabel label={t("connection.tag")} required={false} />
                  <div className="flex gap-1 h-[var(--size-input-sm)]">
                    {TAG_OPTIONS.map((tag) => {
                      const tc = TAG_COLORS[tag];
                      const selected = currentTag === tag;
                      return (
                        <button
                          key={tag}
                          className={cn(
                            "flex-1 rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)] font-medium transition-all border",
                            selected ? "ring-1 ring-offset-1" : "opacity-60 hover:opacity-100"
                          )}
                          style={{
                            backgroundColor: tc.bg,
                            color: tc.text,
                            borderColor: selected ? tc.text : tc.border,
                            ...(selected ? { ringColor: tc.text } : {}),
                          }}
                          onClick={() => updateField("tag", tag)}
                        >
                          {tagLabel(tag)}
                        </button>
                      );
                    })}
                  </div>
                </div>
              </div>

              {/* 颜色选择 */}
              <div className="flex items-center gap-1.5 mb-3">
                <span className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)] mr-1">
                  {t("settings.appearance") || "Color"}
                </span>
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

              {/* 主机 & 端口 */}
              {form.type !== "sqlite" && (
                <div className="flex gap-3 mb-3">
                  <div className="flex-1">
                    <FieldLabel label={t("connection.host")} error={errors.host} />
                    <Input
                      className={cn(
                        "h-[var(--size-input-sm)] text-[length:var(--size-font-sm)]",
                        errors.host && "border-red-400 focus:ring-red-400"
                      )}
                      value={form.host || ""}
                      onChange={(e) => updateField("host", e.target.value)}
                      placeholder={t("connection.hostPlaceholder")}
                    />
                  </div>
                  <div className="w-24">
                    <FieldLabel label={t("connection.port")} error={errors.port} />
                    <Input
                      className={cn(
                        "h-[var(--size-input-sm)] text-[length:var(--size-font-sm)]",
                        errors.port && "border-red-400 focus:ring-red-400"
                      )}
                      type="number"
                      value={form.port || ""}
                      onChange={(e) => updateField("port", Number(e.target.value))}
                    />
                  </div>
                </div>
              )}

              {/* 用户名 & 密码 */}
              {form.type !== "sqlite" && (
                <div className="flex gap-3 mb-3">
                  <div className="flex-1">
                    <FieldLabel label={t("connection.user")} error={errors.user} />
                    <Input
                      className={cn(
                        "h-[var(--size-input-sm)] text-[length:var(--size-font-sm)]",
                        errors.user && "border-red-400 focus:ring-red-400"
                      )}
                      value={form.user || ""}
                      onChange={(e) => updateField("user", e.target.value)}
                      placeholder={t("connection.userPlaceholder")}
                    />
                  </div>
                  <div className="flex-1">
                    <FieldLabel label={t("connection.password")} required={false} />
                    <Input
                      className="h-[var(--size-input-sm)] text-[length:var(--size-font-sm)]"
                      type="password"
                      value={form.password || ""}
                      onChange={(e) => updateField("password", e.target.value)}
                      placeholder={t("connection.passwordPlaceholder")}
                    />
                  </div>
                </div>
              )}

              {/* 数据库名 */}
              <div className="mb-3">
                <FieldLabel
                  label={form.type === "sqlite" ? t("connection.databaseFile") : t("connection.database")}
                  error={errors.database}
                />
                <Input
                  className={cn(
                    "h-[var(--size-input-sm)] text-[length:var(--size-font-sm)]",
                    errors.database && "border-red-400 focus:ring-red-400"
                  )}
                  value={form.database || ""}
                  onChange={(e) => updateField("database", e.target.value)}
                  placeholder={form.type === "sqlite" ? t("connection.databaseFilePlaceholder") : t("connection.databasePlaceholder")}
                />
              </div>

              {/* 测试状态提示 */}
              {testStatus !== "idle" && (
                <div className={cn(
                  "flex items-center gap-2 px-2.5 py-1.5 rounded-[var(--radius-sm)] text-xs mb-3",
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
            <div className="flex items-center justify-between border-t border-[var(--border-color)] px-4 py-2">
              <Button
                variant="outline"
                size="sm"
                className="h-[var(--size-btn-sm)] text-xs"
                onClick={handleTest}
              >
                {t("common.test")}
              </Button>
              <div className="flex gap-1.5">
                <Button variant="ghost" size="sm" className="h-[var(--size-btn-sm)] text-xs" onClick={onClose}>
                  {t("common.cancel")}
                </Button>
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
