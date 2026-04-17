import React, { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { cn } from "@/lib/utils";
import { TOOLTIP_DELAY_MS } from "@/components/ui/tooltip";

const TITLE_DATA_ATTR = "data-ui-title-tooltip";

interface TooltipState {
  text: string;
  x: number;
  y: number;
  target: Element | null;
}

function migrateTitleAttr(el: HTMLElement) {
  if (el.hasAttribute("data-native-tooltip")) return;
  const title = el.getAttribute("title");
  if (title === null) return;
  const trimmed = title.trim();
  if (trimmed) {
    el.setAttribute(TITLE_DATA_ATTR, trimmed);
  } else {
    el.removeAttribute(TITLE_DATA_ATTR);
  }
  el.removeAttribute("title");
}

function migrateTree(node: Node) {
  if (!(node instanceof HTMLElement)) return;
  migrateTitleAttr(node);
  node.querySelectorAll<HTMLElement>("[title]").forEach(migrateTitleAttr);
}

export function TitleTooltipBridge() {
  const [tooltip, setTooltip] = useState<TooltipState | null>(null);
  const visibleTooltipRef = useRef<TooltipState | null>(null);

  useEffect(() => {
    visibleTooltipRef.current = tooltip;
  }, [tooltip]);

  useEffect(() => {
    if (typeof document === "undefined") return;

    migrateTree(document.body);

    const observer = new MutationObserver((mutations) => {
      for (const mutation of mutations) {
        if (mutation.type === "attributes") {
          const target = mutation.target;
          if (target instanceof HTMLElement) {
            migrateTitleAttr(target);
          }
          continue;
        }

        for (const node of mutation.addedNodes) {
          migrateTree(node);
        }
      }
    });

    observer.observe(document.body, {
      subtree: true,
      childList: true,
      attributes: true,
      attributeFilter: ["title"],
    });

    let showTimer: number | null = null;
    let pendingTooltip: TooltipState | null = null;

    const clearShowTimer = () => {
      if (showTimer !== null) {
        window.clearTimeout(showTimer);
        showTimer = null;
      }
    };

    const hide = () => {
      clearShowTimer();
      pendingTooltip = null;
      setTooltip(null);
    };

    const onPointerMove = (e: PointerEvent) => {
      const target = e.target instanceof Element
        ? (e.target.closest(`[${TITLE_DATA_ATTR}]`) as Element | null)
        : null;
      if (!target) {
        hide();
        return;
      }
      const text = target.getAttribute(TITLE_DATA_ATTR) || "";
      if (!text) {
        hide();
        return;
      }

      const nextTooltip: TooltipState = {
        text,
        x: e.clientX,
        y: e.clientY,
        target,
      };

      setTooltip((prev) => {
        if (!prev || prev.target !== target || prev.text !== text) return prev;
        if (prev.x === e.clientX && prev.y === e.clientY) return prev;
        return { ...prev, x: e.clientX, y: e.clientY };
      });

      if (visibleTooltipRef.current && visibleTooltipRef.current.target === target && visibleTooltipRef.current.text === text) {
        return;
      }

      const pendingTargetChanged =
        !pendingTooltip || pendingTooltip.target !== target || pendingTooltip.text !== text;
      pendingTooltip = nextTooltip;
      if (pendingTargetChanged) {
        clearShowTimer();
        showTimer = window.setTimeout(() => {
          showTimer = null;
          if (!pendingTooltip) return;
          setTooltip({ ...pendingTooltip });
        }, TOOLTIP_DELAY_MS);
      }
    };

    const onPointerLeaveWindow = (e: PointerEvent) => {
      if (!e.relatedTarget) hide();
    };

    window.addEventListener("pointermove", onPointerMove, true);
    window.addEventListener("pointerdown", hide, true);
    window.addEventListener("wheel", hide, true);
    window.addEventListener("blur", hide);
    window.addEventListener("pointerout", onPointerLeaveWindow);

    return () => {
      observer.disconnect();
      clearShowTimer();
      window.removeEventListener("pointermove", onPointerMove, true);
      window.removeEventListener("pointerdown", hide, true);
      window.removeEventListener("wheel", hide, true);
      window.removeEventListener("blur", hide);
      window.removeEventListener("pointerout", onPointerLeaveWindow);
    };
  }, []);

  const pos = useMemo(() => {
    if (!tooltip) return null;
    const margin = 10;
    const offset = 14;
    // 按文本长度估算实际宽度，避免靠右时被错误地大幅左移
    const estimatedWidth = Math.min(
      360,
      Math.max(64, Math.round(tooltip.text.length * 7.2) + 20),
    );
    const fallbackHeight = 38;
    const left = Math.max(
      margin,
      Math.min(tooltip.x + offset, window.innerWidth - estimatedWidth - margin),
    );
    const top = Math.max(
      margin,
      Math.min(tooltip.y + offset, window.innerHeight - fallbackHeight - margin),
    );
    return { left, top };
  }, [tooltip]);

  if (!tooltip || !pos || typeof document === "undefined") return null;

  return createPortal(
    <div
      className={cn(
        "fixed z-[10000] pointer-events-none max-w-[360px] overflow-hidden px-2.5 py-1.5",
        "rounded-[var(--radius-btn)] border text-[11px] leading-[1.35] select-none",
        "bg-[var(--surface-elevated)] text-[var(--fg)] border-[var(--border-color)]",
        "shadow-[var(--shadow-lg)] animate-fade-in",
        "whitespace-pre-wrap break-all",
      )}
      style={{ left: pos.left, top: pos.top }}
    >
      {tooltip.text}
    </div>,
    document.body,
  );
}
