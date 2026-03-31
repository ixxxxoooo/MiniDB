import React, { useState, useEffect } from "react";
import { X, TestTube2, Loader2, Check, AlertCircle } from "lucide-react";
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
  const [form, setForm] = useState<Partial<ConnectionConfig>>({
    type: "mysql",
    host: "127.0.0.1",
    port: 3306,
    color: CONNECTION_COLORS[0],
  });
  const [testStatus, setTestStatus] = useState<
    "idle" | "testing" | "success" | "error"
  >("idle");
  const [testError, setTestError] = useState("");

  useEffect(() => {
    if (connection) {
      setForm(connection);
    } else {
      setForm({
        type: "mysql",
        host: "127.0.0.1",
        port: 3306,
        color: CONNECTION_COLORS[0],
        name: "",
        user: "root",
        password: "",
        database: "",
        sslMode: "disable",
        group: "",
      });
    }
    setTestStatus("idle");
    setTestError("");
  }, [connection, open]);

  if (!open) return null;

  const updateField = (field: string, value: unknown) => {
    setForm((prev) => ({ ...prev, [field]: value }));
  };

  const handleDriverChange = (type: DatabaseDriver) => {
    setForm((prev) => ({
      ...prev,
      type,
      port: DEFAULT_PORTS[type],
    }));
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

  return (
    <>
      {/* 遮罩 */}
      <div
        className="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm"
        onClick={onClose}
      />

      {/* 对话框 */}
      <div
        className={cn(
          "fixed z-50 top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2",
          "w-[520px] max-h-[85vh] rounded-xl shadow-lg border overflow-hidden",
          "bg-[var(--surface)] border-[var(--border-color)]"
        )}
      >
        {/* 头部 */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-[var(--border-color)]">
          <h2 className="text-base font-semibold">
            {isEdit ? "编辑连接" : "新建连接"}
          </h2>
          <Button variant="ghost" size="icon" onClick={onClose}>
            <X className="h-4 w-4" />
          </Button>
        </div>

        {/* 内容 */}
        <div className="p-5 space-y-4 overflow-y-auto">
          {/* 数据库类型选择 */}
          <div>
            <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">
              数据库类型
            </label>
            <div className="flex gap-2">
              {(Object.keys(DRIVER_LABELS) as DatabaseDriver[]).map(
                (driver) => (
                  <button
                    key={driver}
                    className={cn(
                      "flex-1 py-2 px-3 rounded-lg border text-sm font-medium transition-colors",
                      form.type === driver
                        ? "border-[var(--accent)] bg-[var(--accent)]/10 text-[var(--accent)]"
                        : "border-[var(--border-color)] text-[var(--fg-secondary)] hover:border-[var(--fg-muted)]"
                    )}
                    onClick={() => handleDriverChange(driver)}
                  >
                    {DRIVER_LABELS[driver]}
                  </button>
                )
              )}
            </div>
          </div>

          {/* 连接名称 */}
          <div>
            <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">
              连接名称
            </label>
            <Input
              value={form.name || ""}
              onChange={(e) => updateField("name", e.target.value)}
              placeholder="My Database"
            />
          </div>

          {/* Host & Port */}
          {form.type !== "sqlite" && (
            <div className="flex gap-3">
              <div className="flex-1">
                <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">
                  主机
                </label>
                <Input
                  value={form.host || ""}
                  onChange={(e) => updateField("host", e.target.value)}
                  placeholder="127.0.0.1"
                />
              </div>
              <div className="w-24">
                <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">
                  端口
                </label>
                <Input
                  type="number"
                  value={form.port || ""}
                  onChange={(e) => updateField("port", Number(e.target.value))}
                />
              </div>
            </div>
          )}

          {/* User & Password */}
          {form.type !== "sqlite" && (
            <div className="flex gap-3">
              <div className="flex-1">
                <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">
                  用户名
                </label>
                <Input
                  value={form.user || ""}
                  onChange={(e) => updateField("user", e.target.value)}
                  placeholder="root"
                />
              </div>
              <div className="flex-1">
                <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">
                  密码
                </label>
                <Input
                  type="password"
                  value={form.password || ""}
                  onChange={(e) => updateField("password", e.target.value)}
                  placeholder="••••••••"
                />
              </div>
            </div>
          )}

          {/* Database */}
          <div>
            <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">
              {form.type === "sqlite" ? "数据库文件路径" : "数据库名"}
            </label>
            <Input
              value={form.database || ""}
              onChange={(e) => updateField("database", e.target.value)}
              placeholder={form.type === "sqlite" ? "/path/to/database.db" : "mydb"}
            />
          </div>

          {/* 标识色 */}
          <div>
            <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">
              标识颜色
            </label>
            <div className="flex gap-2">
              {CONNECTION_COLORS.map((color) => (
                <button
                  key={color}
                  className={cn(
                    "w-6 h-6 rounded-full transition-transform",
                    form.color === color && "ring-2 ring-offset-2 ring-[var(--accent)] scale-110"
                  )}
                  style={{ backgroundColor: color }}
                  onClick={() => updateField("color", color)}
                />
              ))}
            </div>
          </div>

          {/* 测试状态 */}
          {testStatus !== "idle" && (
            <div
              className={cn(
                "flex items-center gap-2 px-3 py-2 rounded-lg text-sm",
                {
                  "bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400":
                    testStatus === "testing",
                  "bg-green-50 dark:bg-green-900/20 text-green-600 dark:text-green-400":
                    testStatus === "success",
                  "bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400":
                    testStatus === "error",
                }
              )}
            >
              {testStatus === "testing" && (
                <Loader2 className="h-4 w-4 animate-spin" />
              )}
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

        {/* 底部按钮 */}
        <div className="flex items-center justify-between px-5 py-4 border-t border-[var(--border-color)]">
          <Button variant="outline" size="sm" onClick={handleTest}>
            <TestTube2 className="h-3.5 w-3.5 mr-1.5" />
            测试连接
          </Button>
          <div className="flex gap-2">
            <Button variant="ghost" size="sm" onClick={onClose}>
              取消
            </Button>
            <Button size="sm" onClick={handleSave}>
              {isEdit ? "保存" : "创建"}
            </Button>
          </div>
        </div>
      </div>
    </>
  );
}
