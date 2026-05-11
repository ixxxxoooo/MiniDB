import * as SettingsService from "@/lib/wails/services/SettingsService";
import type { AnalyticsConfig } from "../../bindings/minidb/services/models";

const SCRIPT_ID = "LA_COLLECT";
const DEFAULT_51LA_SDK_URL = "https://sdk.51.la/js-sdk-pro.min.js";
const SITE_ID = "3PtsQa0O5ZcDnEIg";
const CK = "3PtsQa0O5ZcDnEIg";
const EVENT_NAME = "app_launch";

type LaunchPayload = {
  event_name: typeof EVENT_NAME;
  installation_id: string;
  app_version: string;
  os: string;
  arch: string;
  locale: string;
  time: string;
};

type LAClient = {
  init?: (options: Record<string, unknown>) => void;
  track?: (eventName: string, props?: Record<string, unknown>) => void;
  send?: (eventName: string, props?: Record<string, unknown>) => void;
  collect?: (eventName: string, props?: Record<string, unknown>) => void;
};

declare global {
  interface Window {
    LA?: LAClient;
  }
}

let launchTracked = false;
let scriptPromise: Promise<void> | null = null;
let initialized = false;

export function is51LAConfigured(): boolean {
  return Boolean(SITE_ID && CK);
}

export function buildLaunchPayload(config: AnalyticsConfig, now = new Date()): LaunchPayload {
  return {
    event_name: EVENT_NAME,
    installation_id: config.installationId,
    app_version: config.appVersion || "unknown",
    os: config.os || "unknown",
    arch: config.arch || "unknown",
    locale: typeof navigator === "undefined" ? "unknown" : navigator.language || "unknown",
    time: now.toISOString(),
  };
}

export async function trackAppLaunch(): Promise<void> {
  if (launchTracked) return;

  let config: AnalyticsConfig | null;
  try {
    config = await SettingsService.GetAnalyticsConfig();
  } catch {
    return;
  }

  if (!config?.enabled || !config.installationId || !is51LAConfigured()) {
    return;
  }

  try {
    await load51LAScript();
    init51LA();
    sendLaunchEvent(buildLaunchPayload(config));
    launchTracked = true;
  } catch {
    // 统计不可用不应影响应用启动或使用。
  }
}

export function resetAnalyticsRuntime(removeScript = false) {
  launchTracked = false;
  initialized = false;
  scriptPromise = null;
  if (removeScript && typeof document !== "undefined") {
    document.getElementById(SCRIPT_ID)?.remove();
  }
}

async function load51LAScript(): Promise<void> {
  if (typeof document === "undefined") return;
  if (window.LA) return;
  if (scriptPromise) return scriptPromise;

  const existing = document.getElementById(SCRIPT_ID);
  if (existing) return;

  scriptPromise = new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.id = SCRIPT_ID;
    script.src = DEFAULT_51LA_SDK_URL;
    script.async = true;
    script.charset = "UTF-8";
    script.onload = () => resolve();
    script.onerror = () => reject(new Error("Failed to load 51LA analytics SDK"));
    document.head.appendChild(script);
  });
  return scriptPromise;
}

function init51LA() {
  if (initialized || !window.LA?.init) return;
  window.LA.init({ id: SITE_ID, ck: CK, hashMode: true });
  initialized = true;
}

function sendLaunchEvent(payload: LaunchPayload) {
  const la = window.LA;
  const props = { ...payload };
  if (typeof la?.track === "function") {
    la.track(EVENT_NAME, props);
  } else if (typeof la?.send === "function") {
    la.send(EVENT_NAME, props);
  } else if (typeof la?.collect === "function") {
    la.collect(EVENT_NAME, props);
  }
}
