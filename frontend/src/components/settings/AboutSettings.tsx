import React, { useEffect, useMemo, useState } from "react";
import { AlertCircle, CheckCircle2, ExternalLink, Github, Info, Loader2, Package, RefreshCw, User } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/i18n";
import { EventsOn, OpenURL } from "@/lib/wails/runtime";
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

type UpdateState = "idle" | "checking" | "downloading" | "ready" | "installing" | "error";

interface UpdatePayload {
  state: UpdateState;
  version?: string;
  releaseDate?: string;
  releaseNotes?: string;
  downloaded?: number;
  total?: number;
  percentage?: number;
  error?: string;
  message?: string;
  prompt?: boolean;
  promptKind?: string;
  releaseUrl?: string;
}

export function AboutSettings() {
  const { t } = useTranslation();
  const [appInfo, setAppInfo] = useState<AppInfo | null>(null);
  const [updateState, setUpdateState] = useState<UpdateState>("idle");
  const [updatePayload, setUpdatePayload] = useState<UpdatePayload | null>(null);
  const [installing, setInstalling] = useState(false);

  useEffect(() => {
    SettingsService.GetAppInfo()
      .then((info) => setAppInfo(info as AppInfo))
      .catch(() => setAppInfo(null));
    SettingsService.GetUpdateStatus()
      .then((status) => {
        const payload = status as UpdatePayload;
        setUpdateState(payload.state || "idle");
        setUpdatePayload(payload);
      })
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    const offState = EventsOn<UpdatePayload>("update:state", (payload) => {
      setUpdateState(payload.state || "idle");
      setUpdatePayload((prev) => ({ ...prev, ...payload }));
      if (payload.state !== "installing") setInstalling(false);
    });
    const offProgress = EventsOn<UpdatePayload>("update:progress", (payload) => {
      setUpdateState("downloading");
      setUpdatePayload((prev) => ({ ...prev, ...payload, state: "downloading" }));
    });
    const offReady = EventsOn<UpdatePayload>("update:ready", (payload) => {
      setUpdateState("ready");
      setUpdatePayload((prev) => ({ ...prev, ...payload, state: "ready" }));
      setInstalling(false);
    });
    const offError = EventsOn<UpdatePayload>("update:error", (payload) => {
      setUpdateState("error");
      setUpdatePayload((prev) => ({ ...prev, ...payload, state: "error" }));
      setInstalling(false);
    });
    return () => {
      offState();
      offProgress();
      offReady();
      offError();
    };
  }, []);

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

  const handleCheckUpdate = async () => {
    setUpdateState("checking");
    setUpdatePayload({ state: "checking" });
    try {
      await SettingsService.CheckForUpdates();
    } catch (err) {
      setUpdateState("error");
      setUpdatePayload({ state: "error", error: err instanceof Error ? err.message : String(err) });
    }
  };

  const handleInstallUpdate = async () => {
    setInstalling(true);
    setUpdateState("installing");
    try {
      await SettingsService.InstallReadyUpdate();
    } catch (err) {
      setInstalling(false);
      setUpdateState("error");
      setUpdatePayload((prev) => ({ ...prev, state: "error", error: err instanceof Error ? err.message : String(err) }));
    }
  };

  const openReleases = () => {
    const url = updatePayload?.releaseUrl || appInfo?.releaseUrl;
    if (url) void OpenURL(url);
  };

  const busy = updateState === "checking" || updateState === "downloading" || updateState === "installing" || installing;
  const progress = Math.max(0, Math.min(100, updatePayload?.percentage || 0));
  const updateMessage = (() => {
    if (updateState === "checking") return t("about.updateChecking");
    if (updateState === "downloading") return t("about.updateDownloading").replace("{percent}", `${Math.round(progress)}`);
    if (updateState === "ready") return t("about.updateReady").replace("{version}", updatePayload?.version || "");
    if (updateState === "installing") return t("about.updateInstalling");
    if (updateState === "error") return updatePayload?.error || t("about.updateError");
    return updatePayload?.message || t("about.updateHint");
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
            {updateState === "ready" && (
              <Button size="sm" onClick={handleInstallUpdate} disabled={installing}>
                {installing ? <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" /> : <CheckCircle2 className="h-3.5 w-3.5 mr-1.5" />}
                {t("about.installUpdate")}
              </Button>
            )}
            <Button size="sm" variant="outline" onClick={handleCheckUpdate} disabled={busy}>
              {busy && updateState !== "ready" ? <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5 mr-1.5" />}
              {t("about.checkUpdates")}
            </Button>
            <Button size="sm" variant="ghost" onClick={openReleases} disabled={!updatePayload?.releaseUrl && !appInfo?.releaseUrl}>
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
            <span>{updatePayload?.error || t("about.updateError")}</span>
          </div>
        )}
        {(updateState === "ready" || updateState === "downloading") && updatePayload?.releaseNotes && (
          <div className="mt-2 max-h-28 overflow-y-auto rounded-[var(--radius-btn)] border border-[var(--border-color)] bg-[var(--surface)] p-2 text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)] whitespace-pre-wrap">
            {updatePayload.releaseNotes}
          </div>
        )}
      </div>
    </div>
  );
}
