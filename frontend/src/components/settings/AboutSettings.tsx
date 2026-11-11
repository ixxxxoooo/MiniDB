import React, { useEffect, useMemo, useState } from "react";
import { AlertCircle, CheckCircle2, Download, ExternalLink, Github, Info, Loader2, Package, RefreshCw, User } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/i18n";
import { OpenURL } from "@/lib/wails/runtime";
import { useUpdateStore } from "@/stores/update";
import { useUIStore } from "@/stores/ui";
import { installUpdateWithGuard, useUpdateMessage } from "@/components/update/UpdateNotifier";
import * as SettingsService from "@/lib/wails/services/SettingsService";

interface AppInfo {
  name: string;
  company: string;
  version: string;
  commit: string;
  buildDate: string;
  repository: string;
  releaseUrl: string;
}

export function AboutSettings() {
  const { t } = useTranslation();
  const msg = useUpdateMessage();
  const [appInfo, setAppInfo] = useState<AppInfo | null>(null);
  const [installing, setInstalling] = useState(false);

  const updateState = useUpdateStore((s) => s.state);
  const info = useUpdateStore((s) => s.info);
  const settings = useUpdateStore((s) => s.settings);
  const messageCode = useUpdateStore((s) => s.messageCode);
  const { checkNow, download, openDialog, saveSettings } = useUpdateStore.getState();

  useEffect(() => {
    SettingsService.GetAppInfo()
      .then((i) => setAppInfo(i as AppInfo))
      .catch(() => setAppInfo(null));
  }, []);

  useEffect(() => {
    if (updateState !== "installing") setInstalling(false);
  }, [updateState]);

  const appName = appInfo?.name || "-";
  const companyName = appInfo?.company || "-";
  const versionText = appInfo?.version || "-";

  const infoItems = useMemo(() => [
    { label: t("about.appName"), value: appName, icon: Info },
    { label: t("about.version"), value: versionText, icon: Package },
    { label: t("about.author"), value: companyName, icon: User },
    {
      label: t("about.repository"),
      value: appInfo?.repository ? `https://github.com/${appInfo.repository}` : "-",
      icon: Github,
      href: appInfo?.repository ? `https://github.com/${appInfo.repository}` : undefined,
    },
  ], [appInfo?.repository, appName, companyName, t, versionText]);

  const handleInstallUpdate = async () => {
    setInstalling(true);
    await installUpdateWithGuard();
    setInstalling(false);
  };

  const openReleases = () => {
    const url = info.releaseUrl || appInfo?.releaseUrl;
    if (url) void OpenURL(url);
  };

  const busy = updateState === "checking" || updateState === "downloading" || updateState === "installing" || installing;
  const progress = Math.max(0, Math.min(100, info.percentage || 0));
  const canInstall = info.canInstall !== false;
  const updateMessage = (() => {
    if (updateState === "checking") return t("about.updateChecking");
    if (updateState === "available") return t("update.availableTitle", { version: info.version || "" });
    if (updateState === "downloading") return t("about.updateDownloading", { percent: `${Math.round(progress)}` });
    if (updateState === "ready") return t("about.updateReady", { version: info.version || "" });
    if (updateState === "installing") return t("about.updateInstalling");
    if (updateState === "error") return msg(messageCode, info.error);
    if (messageCode) return msg(messageCode);
    return t("about.updateHint");
  })();

  return (
    <div className="space-y-[var(--size-gap)]">
      <div>
        <h3 className="text-[length:var(--size-font-xs)] font-semibold mb-0.5">{t("about.title")}</h3>
        <p className="text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)]">{t("about.description")}</p>
      </div>

      <div className="rounded-[var(--radius-panel)] border border-[var(--border-color)] bg-[var(--surface-secondary)]/50 p-[var(--size-padding)]">
        <div className="text-[length:var(--size-font-sm)] font-semibold text-[var(--fg)]">{appName}</div>
        <div className="mt-1 text-[length:var(--size-font-xs)] text-[var(--fg-secondary)] leading-6">
          {t("about.summary")}
        </div>
      </div>

      <div className="rounded-[var(--radius-panel)] border border-[var(--border-color)] bg-[var(--surface-secondary)]/50 overflow-hidden">
        <div className="divide-y divide-[var(--border-color)]/70">
          {infoItems.map(({ label, value, icon: Icon, href }) => (
            <div
              key={label}
              className="flex items-center gap-[var(--size-gap)] px-[var(--size-padding)] py-[var(--size-padding-sm)]"
            >
              <div className="h-7 w-7 rounded-[var(--radius-btn)] border border-[var(--border-color)] bg-[var(--surface)] flex items-center justify-center flex-shrink-0">
                <Icon className="h-3.5 w-3.5 text-[var(--fg-secondary)]" />
              </div>
              <div className="min-w-[92px] text-[length:var(--size-font-2xs)] text-[var(--fg-muted)] flex-shrink-0">
                {label}
              </div>
              {href ? (
                <button
                  type="button"
                  className="text-left text-[length:var(--size-font-xs)] text-[var(--accent)] hover:underline break-all"
                  onClick={() => void OpenURL(href)}
                >
                  {value}
                </button>
              ) : (
                <div className="text-[length:var(--size-font-xs)] text-[var(--fg)] break-all">
                  {value}
                </div>
              )}
            </div>
          ))}
        </div>
      </div>

      <div className="rounded-[var(--radius-panel)] border border-[var(--border-color)] bg-[var(--surface-secondary)]/50 p-[var(--size-padding)]">
        <div className="flex items-center justify-between gap-2">
          <div>
            <div className="text-[length:var(--size-font-xs)] font-semibold text-[var(--fg)]">
              {t("about.updates")}
            </div>
            <div className="mt-0.5 text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)]">
              {updateMessage}
            </div>
          </div>
          <div className="flex items-center gap-1.5 flex-shrink-0">
            {updateState === "available" && (
              <Button size="sm" onClick={() => void download()}>
                <Download className="h-3.5 w-3.5 mr-1.5" />
                {t("update.download")}
              </Button>
            )}
            {updateState === "ready" && canInstall && (
              <Button size="sm" onClick={handleInstallUpdate} disabled={installing}>
                {installing ? <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" /> : <CheckCircle2 className="h-3.5 w-3.5 mr-1.5" />}
                {t("about.installUpdate")}
              </Button>
            )}
            {(updateState === "available" || updateState === "downloading" || updateState === "ready") && (
              <Button size="sm" variant="outline" onClick={openDialog}>{t("update.viewDetails")}</Button>
            )}
            <Button size="sm" variant="outline" onClick={() => void checkNow()} disabled={busy}>
              {busy && updateState !== "ready" ? <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5 mr-1.5" />}
              {t("about.checkUpdates")}
            </Button>
            <Button size="sm" variant="ghost" onClick={openReleases} disabled={!info.releaseUrl && !appInfo?.releaseUrl}>
              <ExternalLink className="h-3.5 w-3.5 mr-1.5" />
              {t("about.openReleases")}
            </Button>
          </div>
        </div>
        {updateState === "downloading" && (
          <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-[var(--surface)] border border-[var(--border-color)]">
            <div className="h-full bg-[var(--accent)] transition-all" style={{ width: `${progress}%` }} />
          </div>
        )}
        {updateState === "error" && (
          <div className="mt-2 flex items-start gap-1.5 text-[length:var(--size-font-2xs)] text-[var(--danger)] break-words">
            <AlertCircle className="h-3.5 w-3.5 flex-shrink-0 mt-0.5" />
            <span>{msg(messageCode, info.error)}</span>
          </div>
        )}
        {updateState === "ready" && !canInstall && (
          <div className="mt-2 text-[length:var(--size-font-2xs)] text-[var(--warning)]">{msg("install_unsupported")}</div>
        )}

        <div className="mt-3 pt-3 border-t border-[var(--border-color)]/70 space-y-2">
          <label className="flex items-start gap-2 cursor-pointer select-none">
            <input type="checkbox" className="mt-0.5" checked={settings.autoCheck} onChange={(e) => void saveSettings({ autoCheck: e.target.checked })} />
            <span>
              <span className="block text-[length:var(--size-font-xs)] font-medium">{t("update.autoCheck")}</span>
              <span className="block text-[length:var(--size-font-2xs)] text-[var(--fg-muted)]">{t("update.autoCheckDesc")}</span>
            </span>
          </label>
          <label className="flex items-start gap-2 cursor-pointer select-none">
            <input type="checkbox" className="mt-0.5" checked={settings.autoDownload} onChange={(e) => void saveSettings({ autoDownload: e.target.checked })} />
            <span>
              <span className="block text-[length:var(--size-font-xs)] font-medium">{t("update.autoDownload")}</span>
              <span className="block text-[length:var(--size-font-2xs)] text-[var(--fg-muted)]">{t("update.autoDownloadDesc")}</span>
            </span>
          </label>
          {settings.skippedVersion && (
            <div className="text-[length:var(--size-font-2xs)] text-[var(--fg-muted)]">{t("update.skippedHint", { version: settings.skippedVersion })}</div>
          )}
          <button
            type="button"
            className="text-[length:var(--size-font-2xs)] text-[var(--accent)] hover:underline"
            onClick={() => useUIStore.getState().setLogViewerOpen(true)}
          >
            {t("update.viewLogs")}
          </button>
        </div>
      </div>
    </div>
  );
}
