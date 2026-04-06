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

// 全局禁用全选快捷键（Ctrl/Command + A）
window.addEventListener(
  "keydown",
  (e) => {
    const isSelectAllKey =
      (e.key && e.key.toLowerCase() === "a") || e.code === "KeyA";
    if (!((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && isSelectAllKey)) return;
    const target = e.target instanceof HTMLElement ? e.target : null;
    const active = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const isEditable = (el: HTMLElement | null) => Boolean(
      el?.closest("input, textarea, select, [contenteditable='true'], .monaco-editor, .ProseMirror, [role='textbox']")
    );
    const inGrid = Boolean(target?.closest("[data-grid-root='true']") || active?.closest("[data-grid-root='true']"));
    const inTableList = Boolean(target?.closest("[data-table-list='true']") || active?.closest("[data-table-list='true']"));
    const inAIChat = Boolean(target?.closest(".ai-chat-selectable") || active?.closest(".ai-chat-selectable"));
    if (isEditable(target) || isEditable(active) || inGrid || inTableList || inAIChat) return;
    e.preventDefault();
  },
  { capture: true }
);

applyInitialThemeBeforeMount();

const container = document.getElementById("root");
const root = createRoot(container!);
root.render(<App />);
