export function stripStreamMetaBlocks(text: string): string {
  return text
    .replace(/```minidb-meta\s*[\s\S]*?```/gi, "")
    .replace(/```minidb-next-steps\s*[\s\S]*?```/gi, "")
    .replace(/<\s*[|｜]\s*DSML\s*[|｜]\s*(?:function_calls|tool_calls)\s*>[\s\S]*?<\s*\/\s*[|｜]\s*DSML\s*[|｜]\s*(?:function_calls|tool_calls)\s*>/gi, "")
    .replace(/<\s*[|｜]\s*DSML\s*[|｜]\s*(?:function_calls|tool_calls)\s*>[\s\S]*$/gi, "")
    .replace(/<\s*[|｜]\s*DSML\s*[|｜]\s*invoke\b[^>]*>[\s\S]*?<\s*\/\s*[|｜]\s*DSML\s*[|｜]\s*invoke\s*>/gi, "")
    .replace(/<\s*[|｜]\s*DSML\s*[|｜]\s*parameter\b[^>]*>[\s\S]*?<\s*\/\s*[|｜]\s*DSML\s*[|｜]\s*parameter\s*>/gi, "")
    .replace(/<\s*[|｜]\s*DSML\s*[|｜]\s*parameter\b[^>]*>[\s\S]*$/gi, "")
    .replace(/<\s*\/?\s*[|｜]\s*DSML\s*[|｜]\s*(?:invoke|parameter)[^>]*>/gi, "")
    .replace(/^\s*<\s*\/?\s*[|｜]\s*DSML\s*[|｜].*$/gim, "")
    .replace(/\n{3,}/g, "\n\n");
}

export interface NextStepMetaChoice {
  label: string;
  prompt: string;
}

export function extractNextStepMetaChoices(text: string): NextStepMetaChoice[] {
  if (!text) return [];
  const matches = [...text.matchAll(/```minidb-next-steps\s*([\s\S]*?)```/gi)];
  if (matches.length === 0) return [];
  // 取最后一个块，避免前面历史片段干扰
  const rawJSON = (matches[matches.length - 1][1] || "").trim();
  if (!rawJSON) return [];
  try {
    const parsed = JSON.parse(rawJSON) as { choices?: Array<{ label?: unknown; prompt?: unknown }> };
    if (!Array.isArray(parsed?.choices)) return [];
    const picked: NextStepMetaChoice[] = [];
    for (const item of parsed.choices) {
      const label = String(item?.label || "").trim();
      const prompt = String(item?.prompt || "").trim();
      if (!label || !prompt) continue;
      if (label.length > 28 || prompt.length > 120) continue;
      picked.push({ label, prompt });
      if (picked.length >= 4) break;
    }
    return picked;
  } catch {
    return [];
  }
}

const META_OPEN_MARKERS = [
  "```minidb-meta",
  "```minidb-next-steps",
];
const FENCE = "```";
const DSML_OPEN_MARKERS = [
  "<|DSML|function_calls>",
  "< | DSML | function_calls>",
  "<｜DSML｜function_calls>",
  "< ｜ DSML ｜ function_calls>",
  "<|DSML|tool_calls>",
  "< | DSML | tool_calls>",
  "<｜DSML｜tool_calls>",
  "< ｜ DSML ｜ tool_calls>",
];
const DSML_CLOSE_MARKERS = [
  "</|DSML|function_calls>",
  "</ | DSML | function_calls>",
  "</｜DSML｜function_calls>",
  "</ ｜ DSML ｜ function_calls>",
  "</|DSML|tool_calls>",
  "</ | DSML | tool_calls>",
  "</｜DSML｜tool_calls>",
  "</ ｜ DSML ｜ tool_calls>",
];

function longestMetaOpenPrefixSuffix(text: string): number {
  const markers = [...META_OPEN_MARKERS, ...DSML_OPEN_MARKERS, ...DSML_CLOSE_MARKERS];
  const maxMarkerLen = Math.max(...markers.map((m) => m.length));
  const max = Math.min(text.length, maxMarkerLen - 1);
  for (let len = max; len > 0; len--) {
    const suffix = text.slice(-len);
    for (const marker of markers) {
      if (marker.startsWith(suffix)) return len;
    }
  }
  return 0;
}

export function createStreamMetaFilter() {
  let visibleContent = "";
  let pendingRaw = "";
  let inMetaBlock = false;
  let inDSMLBlock = false;

  const findDSMLOpen = (raw: string) =>
    raw.search(/<\s*[|｜]\s*DSML\s*[|｜]\s*(?:function_calls|tool_calls)\s*>/i);
  const findDSMLClose = (raw: string) =>
    raw.search(/<\s*\/\s*[|｜]\s*DSML\s*[|｜]\s*(?:function_calls|tool_calls)\s*>/i);
  const findAnyDSMLClose = (raw: string) =>
    raw.search(/<\s*\/\s*[|｜]\s*DSML\s*[|｜]\s*[a-z_]+\s*>/i);

  const dsmlOpenLenAt = (raw: string, start: number): number => {
    const m = raw.slice(start).match(/^<\s*[|｜]\s*DSML\s*[|｜]\s*(?:function_calls|tool_calls)\s*>/i);
    return m ? m[0].length : 0;
  };
  const dsmlCloseLenAt = (raw: string, start: number): number => {
    const m = raw.slice(start).match(/^<\s*\/\s*[|｜]\s*DSML\s*[|｜]\s*(?:function_calls|tool_calls)\s*>/i);
    return m ? m[0].length : 0;
  };
  const anyDsmlCloseLenAt = (raw: string, start: number): number => {
    const m = raw.slice(start).match(/^<\s*\/\s*[|｜]\s*DSML\s*[|｜]\s*[a-z_]+\s*>/i);
    return m ? m[0].length : 0;
  };

  const flush = (chunk: string): string => {
    if (!chunk) {
      return visibleContent;
    }

    pendingRaw += chunk;
    let output = "";

    while (pendingRaw.length > 0) {
      if (inDSMLBlock) {
        const closeIdx = (() => {
          const rootCloseIdx = findDSMLClose(pendingRaw);
          if (rootCloseIdx !== -1) return rootCloseIdx;
          return findAnyDSMLClose(pendingRaw);
        })();
        if (closeIdx === -1) {
          pendingRaw = "";
          break;
        }
        const closeLen = dsmlCloseLenAt(pendingRaw, closeIdx) || anyDsmlCloseLenAt(pendingRaw, closeIdx) || 0;
        pendingRaw = pendingRaw.slice(closeIdx + closeLen);
        if (/^\s*$/.test(pendingRaw)) {
          pendingRaw = "";
        }
        inDSMLBlock = false;
        continue;
      }

      if (inMetaBlock) {
        const closeIdx = pendingRaw.indexOf(FENCE);
        if (closeIdx === -1) {
          pendingRaw = "";
          break;
        }
        pendingRaw = pendingRaw.slice(closeIdx + FENCE.length);
        inMetaBlock = false;
        continue;
      }

      let openIdx = -1;
      let openMarker = "";
      for (const marker of META_OPEN_MARKERS) {
        const idx = pendingRaw.indexOf(marker);
        if (idx !== -1 && (openIdx === -1 || idx < openIdx)) {
          openIdx = idx;
          openMarker = marker;
        }
      }
      if (openIdx !== -1) {
        output += pendingRaw.slice(0, openIdx);
        pendingRaw = pendingRaw.slice(openIdx + openMarker.length);
        inMetaBlock = true;
        continue;
      }

      const dsmlOpenIdx = findDSMLOpen(pendingRaw);
      if (dsmlOpenIdx !== -1) {
        output += pendingRaw.slice(0, dsmlOpenIdx);
        const openLen = dsmlOpenLenAt(pendingRaw, dsmlOpenIdx) || 0;
        pendingRaw = pendingRaw.slice(dsmlOpenIdx + openLen);
        inDSMLBlock = true;
        continue;
      }

      const prefixHoldLen = longestMetaOpenPrefixSuffix(pendingRaw);
      if (prefixHoldLen > 0) {
        output += pendingRaw.slice(0, pendingRaw.length - prefixHoldLen);
        pendingRaw = pendingRaw.slice(pendingRaw.length - prefixHoldLen);
        break;
      }

      output += pendingRaw;
      pendingRaw = "";
      break;
    }

    visibleContent = stripStreamMetaBlocks(visibleContent + output);
    return visibleContent;
  };

  const getVisibleContent = () => visibleContent;

  return {
    flush,
    getVisibleContent,
  };
}
