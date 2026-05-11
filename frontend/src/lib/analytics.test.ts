import { describe, expect, it, vi, beforeEach } from "vitest";
import * as SettingsService from "@/lib/wails/services/SettingsService";
import {
  buildLaunchPayload,
  is51LAConfigured,
  resetAnalyticsRuntime,
  trackAppLaunch,
} from "./analytics";

vi.mock("@/lib/wails/services/SettingsService", () => ({
  GetAnalyticsConfig: vi.fn(),
}));

describe("analytics", () => {
  beforeEach(() => {
    resetAnalyticsRuntime();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it("requires a 51LA site id", () => {
    expect(is51LAConfigured()).toBe(true);
  });

  it("builds a launch payload with only the allowed fields", () => {
    const payload = buildLaunchPayload(
      {
        enabled: true,
        installationId: "minidb_test",
        appVersion: "1.2.3",
        os: "darwin",
        arch: "arm64",
      },
      new Date("2026-05-11T12:00:00.000Z"),
    );

    expect(payload).toEqual({
      event_name: "app_launch",
      installation_id: "minidb_test",
      app_version: "1.2.3",
      os: "darwin",
      arch: "arm64",
      locale: expect.any(String),
      time: "2026-05-11T12:00:00.000Z",
    });
    expect(Object.keys(payload).sort()).toEqual([
      "app_version",
      "arch",
      "event_name",
      "installation_id",
      "locale",
      "os",
      "time",
    ]);
  });

  it("does not touch the document when analytics is disabled", async () => {
    vi.mocked(SettingsService.GetAnalyticsConfig).mockResolvedValue({
      enabled: false,
      installationId: "minidb_test",
      appVersion: "1.2.3",
      os: "darwin",
      arch: "arm64",
    } as any);
    vi.stubGlobal("document", {
      createElement: vi.fn(() => {
        throw new Error("script should not be created");
      }),
    });

    await expect(trackAppLaunch()).resolves.toBeUndefined();
  });
});
