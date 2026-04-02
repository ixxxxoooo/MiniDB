export function stripStreamMetaBlocks(text: string): string {
  return text
    .replace(/```tableplus-ai-meta\s*[\s\S]*?```/gi, "")
    .replace(/\n{3,}/g, "\n\n");
}

const META_OPEN = "```tableplus-ai-meta";
const FENCE = "```";

function longestMetaOpenPrefixSuffix(text: string): number {
  const max = Math.min(text.length, META_OPEN.length - 1);
  for (let len = max; len > 0; len--) {
    if (META_OPEN.startsWith(text.slice(-len))) {
      return len;
    }
  }
  return 0;
}

export function createStreamMetaFilter() {
  let visibleContent = "";
  let pendingRaw = "";
  let inMetaBlock = false;

  const flush = (chunk: string): string => {
    if (!chunk) {
      return visibleContent;
    }

    pendingRaw += chunk;
    let output = "";

    while (pendingRaw.length > 0) {
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

      const openIdx = pendingRaw.indexOf(META_OPEN);
      if (openIdx !== -1) {
        output += pendingRaw.slice(0, openIdx);
        pendingRaw = pendingRaw.slice(openIdx + META_OPEN.length);
        inMetaBlock = true;
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
