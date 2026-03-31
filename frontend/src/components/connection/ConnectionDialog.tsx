import React, { useState, useEffect } from "react";
import { X, TestTube2, Loader2, Check, AlertCircle, Database, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
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
      if (!success) setTestError("连接失败");
    } catch (e: any) {
      setTestStatus("error");
      setTestError(e?.message || "连接测试失败");
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

  const filteredConns = connections.filter((c) =>
    !search || c.name.toLowerCase().includes(search.toLowerCase()) || c.host.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <>
      <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm" onClick={onClose} />
      <div
        className={cn(
          "fixed z-50 top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2",
          "rounded-xl shadow-lg border overflow-hidden",
          "bg-[var(--surface)] border-[var(--border-color)]",
          view === "list" ? "w-[480px] max-h-[70vh]" : "w-[520px] max-h-[85vh]"
        )}
      >
        {/* ====== 历史连接列表视图（参考 TablePlus） ====== */}
        {view === "list" && !isEdit && (
          <div className="flex flex-col h-full" style={{ maxHeight: "70vh" }}>
            <div className="flex items-center gap-2 px-4 py-3 border-b border-[var(--border-color)]">
              <button
                className="text-xl font-light text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors h-7 w-7 flex items-center justify-center rounded hover:bg-[var(--sidebar-hover)]"
                onClick={() => setView("form")}
                title="新建连接"
              >
                +
              </button>
              <div className="flex-1 flex items-center gap-1.5 bg-[var(--surface-secondary)] rounded-lg px-2.5 py-1.5">
                <Search className="h-3.5 w-3.5 text-[var(--fg-muted)]" />
                <input
                  className="flex-1 text-sm bg-transparent outline-none text-[var(--fg)] placeholder-[var(--fg-muted)]"
                  placeholder="Search for connection... (⌘F)"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  autoFocus
                />
              </div>
              <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onClose}>
                <X className="h-4 w-4" />
              </Button>
            </div>
            <div className="flex-1 overflow-y-auto">
              {filteredConns.map((conn) => (
                <button
                  key={conn.id}
                  className="w-full flex items-center gap-3 px-4 py-2.5 hover:bg-[var(--sidebar-hover)] transition-colors text-left"
                  onClick={() => handleSelectExisting(conn)}
                >
                  <div
                    className="w-8 h-8 rounded-lg flex items-center justify-center text-white text-xs font-bold flex-shrink-0"
                    style={{ backgroundColor: conn.color || "#007aff" }}
                  >
                    {(conn.type || "M").charAt(0).toUpperCase()}
                    <span className="text-2xs">{(conn.type || "mysql").charAt(1)}</span>
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium text-[var(--fg)] truncate">{conn.name}</div>
                    <div className="text-xs text-[var(--fg-muted)] truncate">
                      {conn.host}:{conn.port}
                      {conn.database && ` : ${conn.database}`}
                    </div>
                  </div>
                  <span className="text-xs text-[var(--success)]">(local)</span>
                </button>
              ))}
              {filteredConns.length === 0 && (
                <div className="text-center py-8 text-[var(--fg-muted)] text-sm">
                  {search ? "无匹配连接" : "暂无已保存的连接"}
                </div>
              )}
            </div>
            <div className="px-4 py-2 border-t border-[var(--border-color)] flex justify-center">
              <Button variant="ghost" size="sm" className="text-xs" onClick={() => setView("form")}>
                + 创建新连接
              </Button>
            </div>
          </div>
        )}

        {/* ====== 新建/编辑连接表单（参考 TablePlus 样式） ====== */}
        {(view === "form" || isEdit) && (
          <>
            <div className="flex items-center justify-between px-5 py-3 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-2">
                {!isEdit && connections.length > 0 && (
                  <Button variant="ghost" size="sm" className="text-xs" onClick={() => setView("list")}>
                    ← 返回
                  </Button>
                )}
                <h2 className="text-base font-semibold">
                  {form.type ? `${DRIVER_LABELS[form.type as DatabaseDriver] || ""} Connection` : isEdit ? "编辑连接" : "新建连接"}
                </h2>
              </div>
              <Button variant="ghost" size="icon" onClick={onClose}>
                <X className="h-4 w-4" />
              </Button>
            </div>

            <div className="p-5 space-y-3.5 overflow-y-auto">
              {/* 数据库类型 */}
              <div className="flex gap-2">
                {(Object.keys(DRIVER_LABELS) as DatabaseDriver[]).map((driver) => (
                  <button
                    key={driver}
                    className={cn(
                      "flex-1 py-2 px-3 rounded-lg border text-sm font-medium transition-all",
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
                  <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">Name</label>
                  <Input value={form.name || ""} onChange={(e) => updateField("name", e.target.value)} placeholder="My Database" />
                </div>
                <div className="flex gap-1.5 pb-0.5">
                  {CONNECTION_COLORS.map((color) => (
                    <button
                      key={color}
                      className={cn("w-5 h-5 rounded-full transition-all", form.color === color && "ring-2 ring-offset-1 ring-[var(--accent)] scale-110")}
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
                    <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">Host/IP</label>
                    <Input value={form.host || ""} onChange={(e) => updateField("host", e.target.value)} placeholder="127.0.0.1" />
                  </div>
                  <div className="w-24">
                    <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">Port</label>
                    <Input type="number" value={form.port || ""} onChange={(e) => updateField("port", Number(e.target.value))} />
                  </div>
                </div>
              )}

              {/* User & Password */}
              {form.type !== "sqlite" && (
                <div className="flex gap-3">
                  <div className="flex-1">
                    <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">User</label>
                    <Input value={form.user || ""} onChange={(e) => updateField("user", e.target.value)} placeholder="root" />
                  </div>
                  <div className="flex-1">
                    <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">Password</label>
                    <Input type="password" value={form.password || ""} onChange={(e) => updateField("password", e.target.value)} placeholder="••••••••" />
                  </div>
                </div>
              )}

              {/* Database */}
              <div>
                <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">
                  {form.type === "sqlite" ? "Database File" : "Database"}
                </label>
                <Input value={form.database || ""} onChange={(e) => updateField("database", e.target.value)}
                  placeholder={form.type === "sqlite" ? "/path/to/database.db" : "database name"} />
              </div>

              {/* 测试状态 */}
              {testStatus !== "idle" && (
                <div className={cn(
                  "flex items-center gap-2 px-3 py-2 rounded-lg text-sm",
                  { "bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400": testStatus === "testing",
                    "bg-green-50 dark:bg-green-900/20 text-green-600 dark:text-green-400": testStatus === "success",
                    "bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400": testStatus === "error" }
                )}>
                  {testStatus === "testing" && <Loader2 className="h-4 w-4 animate-spin" />}
                  {testStatus === "success" && <Check className="h-4 w-4" />}
                  {testStatus === "error" && <AlertCircle className="h-4 w-4" />}
                  <span>
                    {testStatus === "testing" && "正在测试连接..."}
                    {testStatus === "success" && "连接成功！"}
                    {testStatus === "error" && (testError || "连接失败")}
                  </span>
                </div>
              )}
            </div>

            {/* 底部按钮（参考 TablePlus：Save / Test / Connect） */}
            <div className="flex items-center justify-between px-5 py-3 border-t border-[var(--border-color)]">
              <Button variant="outline" size="sm" onClick={handleTest}>
                <TestTube2 className="h-3.5 w-3.5 mr-1.5" />Test
              </Button>
              <div className="flex gap-2">
                <Button variant="ghost" size="sm" onClick={onClose}>取消</Button>
                <Button size="sm" onClick={handleSave}>
                  {isEdit ? "Save" : "Connect"}
                </Button>
              </div>
            </div>
          </>
        )}
      </div>
    </>
  );
}
