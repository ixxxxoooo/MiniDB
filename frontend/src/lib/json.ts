const MAX_NESTED_PARSE_DEPTH = 8;

function tryParseJSON(text: string): unknown | null {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

function isLikelyJSONObjectOrArray(text: string): boolean {
  const trimmed = text.trim();
  return trimmed.startsWith("{") || trimmed.startsWith("[");
}

function parseNestedJSON(value: unknown, depth = 0): unknown {
  if (depth >= MAX_NESTED_PARSE_DEPTH) return value;

  if (typeof value === "string") {
    if (!isLikelyJSONObjectOrArray(value)) return value;
    const parsed = tryParseJSON(value);
    if (parsed === null) return value;
    return parseNestedJSON(parsed, depth + 1);
  }

  if (Array.isArray(value)) {
    return value.map((item) => parseNestedJSON(item, depth + 1));
  }

  if (value && typeof value === "object") {
    const output: Record<string, unknown> = {};
    Object.entries(value as Record<string, unknown>).forEach(([key, item]) => {
      output[key] = parseNestedJSON(item, depth + 1);
    });
    return output;
  }

  return value;
}

export function formatJSONForPreview(value: unknown): string | null {
  if (value === null || value === undefined) return null;

  let parsed: unknown;
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (!(trimmed.startsWith("{") || trimmed.startsWith("[") || trimmed.startsWith("\""))) return null;
    const parsedValue = tryParseJSON(value);
    if (parsedValue === null) return null;
    parsed = parsedValue;
  } else if (typeof value === "object") {
    parsed = value;
  } else {
    return null;
  }

  const nestedParsed = parseNestedJSON(parsed);
  if (!nestedParsed || typeof nestedParsed !== "object") return null;
  return JSON.stringify(nestedParsed, null, 2);
}
