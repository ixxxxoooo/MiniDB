import React, { useEffect, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { AlertCircle, ArrowUpCircle, CheckCircle2, Download, Loader2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useTranslation } from "@/i18n";
import { cn } from "@/lib/utils";
import { OpenURL } from "@/lib/wails/runtime";
import * as SettingsService from "@/lib/wails/services/SettingsService";
import { initUpdateListeners, useUpdateStore } from "@/stores/update";
import { useTabsStore } from "@/stores/tabs";
import { useUIStore } from "@/stores/ui";
import { requestConfirm } from "@/stores/confirm";
import { hasDirtyTabs } from "@/lib/tabGuards";
import type { TranslationKey } from "@/i18n";

/** 把后端消息码翻译成当前语言文案；未知码回退到通用提示。 */
export function useUpdateMessage() {
  const { t } = useTranslation();
  return (code: string, fallback?: string): string => {
    const key = `update.msg.${code}` as TranslationKey;
    const text = t(key);
    if (code && text !== key) return text;
    return fallback || t("update.msg.unknown" as TranslationKey);
  };
}

/** 安装更新：有未保存修改时先确认（安装会重启应用）。 */
export async function installUpdateWithGuard(): Promise<void> {
  const { t } = await import("@/i18n");
  if (hasDirtyTabs()) {
    const names = useTabsStore.getState().tabs.filter((tab) => tab.dirty).map((tab) => tab.title).join("、");
    const ok = await requestConfirm({
      title: t("update.installUnsavedTitle"),
      message: t("dialog.unsavedMessage", { names }),
      confirmText: t("dialog.discard"),
      danger: true,
    });
    if (!ok) return;
  }
  try {
    await SettingsService.InstallReadyUpdate();
  } catch (e) {
    // 失败原因（含 install_unsupported）已通过 update:error 事件反馈到界面
    console.error("安装更新失败:", e);
  }
}

/** 标题栏徽标：有可用/下载中/就绪的更新时出现，点击打开详情对话框。 */
export function UpdateBadge() {
  const { t } = useTranslation();
  const state = useUpdateStore((s) => s.state);
  const info = useUpdateStore((s) => s.info);
  const openDialog = useUpdateStore((s) => s.openDialog);

  if (state !== "available" && state !== "downloading" && state !== "ready") return null;
  const percent = Math.round(info.percentage || 0);
  const tip =
    state === "ready"
      ? t("update.badgeReady", { version: info.version || "" })
      : state === "downloading"
        ? t("update.badgeDownloading", { percent })
        : t("update.badgeAvailable", { version: info.version || "" });

  return (
    <Tooltip delayDuration={300}>
      <TooltipTrigger asChild>
        <button
          className="relative flex items-center justify-center h-[var(--size-btn)] w-[var(--size-btn)] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors"
          onClick={openDialog}
          aria-label={tip}
        >
          {state === "downloading" ? (
            <Loader2 className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--accent)] animate-spin" />
          ) : (
            <ArrowUpCircle className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--accent)]" />
          )}
          {state === "ready" && <span className="absolute top-1 right-1 h-1.5 w-1.5 rounded-full bg-[var(--success)]" />}
        </button>
      </TooltipTrigger>
      <TooltipContent side="bottom">{tip}</TooltipContent>
    </Tooltip>
  );
}

/** 右下角更新卡片 + 详情对话框；挂在应用根部，并负责订阅更新事件。 */
export function UpdateNotifier() {
  const { t } = useTranslation();
  const msg = useUpdateMessage();
  const state = useUpdateStore((s) => s.state);
  const info = useUpdateStore((s) => s.info);
  const card = useUpdateStore((s) => s.card);
  const messageCode = useUpdateStore((s) => s.messageCode);
  const dialogOpen = useUpdateStore((s) => s.dialogOpen);
  const { dismissCard, openDialog, closeDialog, download, skipVersion } = useUpdateStore.getState();
  const [installing, setInstalling] = useState(false);

  useEffect(() => initUpdateListeners(), []);
  useEffect(() => {
    if (state !== "installing") setInstalling(false);
  }, [state]);

  // 消息/错误卡片 6 秒后自动收起；available/ready 卡片需要用户处理
  useEffect(() => {
    if (!card || (card.kind !== "message" && card.kind !== "error")) return;
    const timer = window.setTimeout(() => dismissCard(), 6000);
    return () => window.clearTimeout(timer);
  }, [card, dismissCard]);

  // 对话框 ESC 关闭
  useEffect(() => {
    if (!dialogOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") closeDialog();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [dialogOpen, closeDialog]);

  const openReleases = () => {
    if (info.releaseUrl) void OpenURL(info.releaseUrl);
  };
  const handleInstall = async () => {
    setInstalling(true);
    await installUpdateWithGuard();
    setInstalling(false);
  };

  const percent = Math.round(info.percentage || 0);
  const canInstall = info.canInstall !== false;
  const isMessageOnly = card && (card.kind === "message" || card.kind === "error");

  return (
    <>
      {card && (
        <div className="fixed bottom-4 right-4 z-[210] w-[340px] max-w-[92vw] pointer-events-auto">
          <div
            className={cn(
              "rounded-[var(--radius-panel)] border shadow-lg p-3 text-xs animate-fade-in",
              "bg-[var(--surface-elevated)] border-[var(--border-color)] text-[var(--fg)]",
            )}
          >
            <div className="flex items-start gap-2">
              {card.kind === "error" ? (
                <AlertCircle className="h-4 w-4 text-[var(--danger)] flex-shrink-0 mt-0.5" />
              ) : card.kind === "message" ? (
                <CheckCircle2 className="h-4 w-4 text-[var(--success)] flex-shrink-0 mt-0.5" />
              ) : (
                <ArrowUpCircle className="h-4 w-4 text-[var(--accent)] flex-shrink-0 mt-0.5" />
              )}
              <div className="min-w-0 flex-1">
                {isMessageOnly ? (
                  <div className="leading-5">{msg(messageCode, info.error)}</div>
                ) : (
                  <>
                    <div className="font-medium">
                      {t(card.kind === "ready" ? "update.readyTitle" : "update.availableTitle", { version: info.version || "" })}
                    </div>
                    <div className="mt-0.5 text-[var(--fg-secondary)] leading-5">
                      {card.kind === "ready"
                        ? t("update.cardReady")
                        : state === "downloading"
                          ? t("update.cardDownloading", { percent })
                          : t("update.cardAvailable")}
                    </div>
                    {state === "downloading" && (
                      <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-[var(--surface)] border border-[var(--border-color)]">
                        <div className="h-full bg-[var(--accent)] transition-all" style={{ width: `${percent}%` }} />
                      </div>
                    )}
                    <div className="mt-2.5 flex items-center gap-1.5">
                      {card.kind === "ready" && canInstall && (
                        <Button size="sm" onClick={handleInstall} disabled={installing}>
                          {installing && <Loader2 className="h-3 w-3 mr-1 animate-spin" />}
                          {t("update.installNow")}
                        </Button>
                      )}
                      {card.kind === "ready" && !canInstall && (
                        <Button size="sm" onClick={openReleases}>{t("update.openReleases")}</Button>
                      )}
                      {card.kind === "available" && state === "available" && (
                        <Button size="sm" onClick={() => void download()}>
                          <Download className="h-3 w-3 mr-1" />
                          {t("update.download")}
                        </Button>
                      )}
                      <Button size="sm" variant="ghost" onClick={openDialog}>{t("update.viewDetails")}</Button>
                      {!info.mandatory && (
                        <Button size="sm" variant="ghost" onClick={dismissCard}>{t("update.later")}</Button>
                      )}
                    </div>
                  </>
                )}
              </div>
              {(isMessageOnly || !info.mandatory) && (
                <button className="text-[var(--fg-muted)] hover:text-[var(--fg)]" onClick={dismissCard} aria-label="close">
                  <X className="h-3 w-3" />
                </button>
              )}
            </div>
          </div>
        </div>
      )}

      {dialogOpen && (
        <>
          <div className="fixed inset-0 z-[300] bg-black/40 backdrop-blur-sm" onClick={() => !info.mandatory && closeDialog()} />
          <div
            role="dialog"
            aria-modal="true"
            className={cn(
              "fixed z-[301] top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[520px] max-w-[92vw] max-h-[80vh] flex flex-col",
              "rounded-[var(--radius-panel)] border shadow-lg",
              "bg-[var(--surface-elevated)] border-[var(--border-color)] text-[var(--fg)]",
            )}
          >
            <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-2">
                <ArrowUpCircle className="h-4 w-4 text-[var(--accent)]" />
                <span className="text-sm font-semibold">
                  {t(state === "ready" ? "update.readyTitle" : "update.availableTitle", { version: info.version || "" })}
                </span>
              </div>
              {!info.mandatory && (
                <button className="text-[var(--fg-muted)] hover:text-[var(--fg)]" onClick={closeDialog} aria-label="close">
                  <X className="h-3.5 w-3.5" />
                </button>
              )}
            </div>
            <div className="px-4 py-3 overflow-y-auto text-xs space-y-3">
              {info.releaseDate && (
                <div className="text-[var(--fg-muted)]">
                  {t("update.releaseDate")}: {new Date(info.releaseDate).toLocaleString()}
                </div>
              )}
              {info.mandatory && <div className="text-[var(--warning)]">{t("update.mandatory")}</div>}
              {state === "downloading" && (
                <div>
                  <div className="mb-1 text-[var(--fg-secondary)]">{t("update.cardDownloading", { percent })}</div>
                  <div className="h-1.5 overflow-hidden rounded-full bg-[var(--surface)] border border-[var(--border-color)]">
                    <div className="h-full bg-[var(--accent)] transition-all" style={{ width: `${percent}%` }} />
                  </div>
                </div>
              )}
              {(state === "error" || messageCode) && state !== "ready" && (
                <div className="flex items-start gap-1.5 text-[var(--danger)]">
                  <AlertCircle className="h-3.5 w-3.5 flex-shrink-0 mt-0.5" />
                  <span>{msg(messageCode, info.error)}</span>
                </div>
              )}
              {state === "ready" && !canInstall && (
                <div className="text-[var(--warning)]">{msg("install_unsupported")}</div>
              )}
              <div>
                <div className="font-medium mb-1">{t("update.releaseNotes")}</div>
                <div className="rounded-[var(--radius-btn)] border border-[var(--border-color)] bg-[var(--surface)] p-2.5 leading-5 max-h-[260px] overflow-y-auto markdown-body">
                  {info.releaseNotes ? (
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>{info.releaseNotes}</ReactMarkdown>
                  ) : (
                    <span className="text-[var(--fg-muted)]">{t("update.noNotes")}</span>
                  )}
                </div>
              </div>
            </div>
            <div className="flex items-center justify-between gap-2 px-4 py-3 border-t border-[var(--border-color)]">
              <Button size="sm" variant="ghost" onClick={openReleases}>{t("update.openReleases")}</Button>
              <div className="flex items-center gap-2">
                {!info.mandatory && state !== "installing" && (
                  <Button size="sm" variant="ghost" onClick={() => void skipVersion()}>{t("update.skipVersion")}</Button>
                )}
                {!info.mandatory && (
                  <Button size="sm" variant="outline" onClick={closeDialog}>{t("update.later")}</Button>
                )}
                {state === "available" && (
                  <Button size="sm" onClick={() => void download()}>
                    <Download className="h-3 w-3 mr-1" />
                    {t("update.download")}
                  </Button>
                )}
                {state === "ready" && canInstall && (
                  <Button size="sm" onClick={handleInstall} disabled={installing}>
                    {installing && <Loader2 className="h-3 w-3 mr-1 animate-spin" />}
                    {t("update.installNow")}
                  </Button>
                )}
              </div>
            </div>
          </div>
        </>
      )}
    </>
  );
}
