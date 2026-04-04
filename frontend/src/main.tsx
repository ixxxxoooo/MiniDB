import React from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "./globals.css";

function applyInitialThemeBeforeMount() {
  if (typeof window === "undefined") return;
  const root = document.documentElement;
  let theme: "light" | "dark" | "system" = "system";
  try {
    const raw = window.localStorage.getItem("tableplus-ai-theme");
    if (raw) {
      const parsed = JSON.parse(raw) as { state?: { theme?: "light" | "dark" | "system" } };
      if (parsed?.state?.theme === "light" || parsed?.state?.theme === "dark" || parsed?.state?.theme === "system") {
        theme = parsed.state.theme;
      }
    }
  } catch {
    // ignore parse errors
  }
  const resolved = theme === "system"
    ? (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light")
    : theme;
  root.classList.toggle("dark", resolved === "dark");
}

// 全局滚动条自动隐藏：滚动时显示，静止2秒后隐藏
const scrollTimers = new WeakMap<Element, number>();
document.addEventListener(
  "scroll",
  (e) => {
    const target = e.target;
    if (!target || target === document || !(target instanceof Element)) return;
    target.classList.add("is-scrolling");
    const prev = scrollTimers.get(target);
    if (prev) clearTimeout(prev);
    scrollTimers.set(
      target,
      window.setTimeout(() => {
        target.classList.remove("is-scrolling");
        scrollTimers.delete(target);
      }, 2000)
    );
  },
  true
);

applyInitialThemeBeforeMount();

const container = document.getElementById("root");
const root = createRoot(container!);
root.render(<App />);
