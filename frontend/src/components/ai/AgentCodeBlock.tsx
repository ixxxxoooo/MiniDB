import DOMPurify from "dompurify";
import Prism from "prismjs";
import "prismjs/components/prism-sql";
import "prismjs/components/prism-json";
import "prismjs/components/prism-javascript";
import "prismjs/components/prism-typescript";
import "prismjs/components/prism-bash";
import "prismjs/components/prism-markup";

const HIGHLIGHT_CACHE_MAX = 400;
const highlightCache = new Map<string, string>();

export function purgeHighlightCache() {
  highlightCache.clear();
}

/** 供 Markdown 代码块复用同一套高亮 + 缓存 */
export function getHighlightedHtml(code: string, language: string): string {
  const lang = Prism.languages[language] ? language : "plain";
  const cacheKey = `${lang}\u0000${code}`;
  const cached = highlightCache.get(cacheKey);
  if (cached) return cached;

  let html: string;
  if (lang === "plain") {
    html = DOMPurify.sanitize(code);
  } else {
    html = Prism.highlight(code, Prism.languages[lang], lang);
  }
  if (highlightCache.size >= HIGHLIGHT_CACHE_MAX) {
    const firstKey = highlightCache.keys().next().value;
    if (firstKey !== undefined) highlightCache.delete(firstKey);
  }
  highlightCache.set(cacheKey, html);
  return html;
}

/** 统一代码块渲染（prism 高亮 + sanitize 收口） */
export function AgentCodeBlock({ code, language }: { code: string; language: string }) {
  const html = getHighlightedHtml(code, language);
  return (
    <pre className="text-[11px] font-mono overflow-x-auto bg-[var(--surface-secondary)] rounded-[var(--radius-sm)] p-1.5 max-w-full">
      <code className={`language-${language}`} dangerouslySetInnerHTML={{ __html: html }} />
    </pre>
  );
}