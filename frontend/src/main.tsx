import React from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "./globals.css";

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

const container = document.getElementById("root");
const root = createRoot(container!);
root.render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
