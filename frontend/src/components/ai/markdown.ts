function parsePipeCells(line: string): string[] {
  let trimmed = line.trim();
  if (trimmed.startsWith("|")) {
    trimmed = trimmed.slice(1);
  }
  if (trimmed.endsWith("|")) {
    trimmed = trimmed.slice(0, -1);
  }

  const cells = trimmed.split("|").map((cell) => cell.trim());
  while (cells.length > 0 && cells[0] === "") {
    cells.shift();
  }
  while (cells.length > 0 && cells[cells.length - 1] === "") {
    cells.pop();
  }
  return cells;
}

function isSeparatorCell(cell: string): boolean {
  return /^:?-{3,}:?$/.test(cell.trim());
}

function parseSeparatorCells(line: string): string[] | null {
  const cells = parsePipeCells(line);
  return cells.length >= 2 && cells.every(isSeparatorCell) ? cells : null;
}

function isSeparatorLine(line: string): boolean {
  return parseSeparatorCells(line) !== null;
}

function isTableCandidateLine(line: string): boolean {
  const trimmed = line.trim();
  if (!trimmed || !trimmed.includes("|")) {
    return false;
  }
  if (/^>/.test(trimmed)) {
    return false;
  }
  return parsePipeCells(trimmed).length >= 2 || isSeparatorLine(trimmed);
}

function isCompletePipeTableLine(line: string): boolean {
  const trimmed = line.trim();
  return trimmed.startsWith("|") && trimmed.endsWith("|") && parsePipeCells(trimmed).length >= 2;
}

function isTableContinuationLine(line: string): boolean {
  const trimmed = line.trim();
  if (!trimmed || /^>/.test(trimmed)) {
    return false;
  }
  if (isTableCandidateLine(trimmed)) {
    return true;
  }
  // 流式输出时，一行表格数据可能还没吐完，例如 "| 订单数"。
  // 只在已经进入表格块后接受这种半截行，后续会按列数补齐。
  return trimmed.startsWith("|");
}

function formatTableRow(cells: string[], columnCount: number): string {
  return `| ${padCells(cells, columnCount).join(" | ")} |`;
}

function formatSeparatorRow(cells: string[], columnCount: number): string {
  const normalized = Array.from({ length: columnCount }, (_, index) => {
    const cell = cells[index] ?? "---";
    const trimmed = cell.trim();
    if (!trimmed) return "---";
    if (/^:-{3,}:$/.test(trimmed)) return ":---:";
    if (/^:-{3,}$/.test(trimmed)) return ":---";
    if (/^-{3,}:$/.test(trimmed)) return "---:";
    return "---";
  });
  return `| ${normalized.join(" | ")} |`;
}

export interface NormalizeAIMarkdownOptions {
  streaming?: boolean;
}

interface ParsedTableBlock {
  header: string[];
  rows: string[][];
  separator: string[];
  columnCount: number;
}

function padCells(cells: string[], columnCount: number): string[] {
  const normalized = [...cells];
  while (normalized.length < columnCount) {
    normalized.push("");
  }
  return normalized.slice(0, columnCount);
}

function parseTableBlock(lines: string[], options: NormalizeAIMarkdownOptions = {}): ParsedTableBlock | null {
  const trimmedLines = lines.map((line) => line.trim()).filter(Boolean);
  const headerCells = parsePipeCells(trimmedLines[0]);
  if (headerCells.length < 2) {
    return null;
  }

  const separatorIndex = trimmedLines.findIndex((line, index) => index > 0 && parseSeparatorCells(line));
  const hasExplicitTableShape =
    separatorIndex > 0 ||
    (options.streaming && isCompletePipeTableLine(trimmedLines[0])) ||
    (trimmedLines.length >= 3 && trimmedLines.every((line) => line.startsWith("|")));
  if (!hasExplicitTableShape) {
    return null;
  }

  const bodyLines = trimmedLines.filter((line, index) => index > 0 && index !== separatorIndex);
  const bodyCells = bodyLines.map(parsePipeCells).filter((cells) => cells.length > 0);
  const separatorCells = separatorIndex > 0 ? parseSeparatorCells(trimmedLines[separatorIndex]) ?? [] : [];

  const columnCount = Math.max(
    headerCells.length,
    ...bodyCells.map((cells) => cells.length),
  );

  if (columnCount < 2) {
    return null;
  }

  return {
    header: padCells(headerCells, columnCount),
    rows: bodyCells.map((cells) => padCells(cells, columnCount)),
    separator: padCells(separatorCells, columnCount),
    columnCount,
  };
}

function normalizeTableBlock(lines: string[], options: NormalizeAIMarkdownOptions = {}): string[] | null {
  const table = parseTableBlock(lines, options);
  if (!table) {
    return null;
  }

  return [
    formatTableRow(table.header, table.columnCount),
    formatSeparatorRow(table.separator, table.columnCount),
    ...table.rows.map((row) => formatTableRow(row, table.columnCount)),
  ];
}

export function normalizeAIMarkdown(content: string, options: NormalizeAIMarkdownOptions = {}): string {
  const normalizedContent = content.replace(/\r\n/g, "\n");
  const lines = normalizedContent.split("\n");
  const output: string[] = [];
  let inFence = false;

  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index];

    if (/^\s*```/.test(line)) {
      inFence = !inFence;
      output.push(line);
      continue;
    }

    if (inFence) {
      output.push(line);
      continue;
    }

    if (!isTableCandidateLine(line)) {
      output.push(line);
      continue;
    }

    const tableBlock: string[] = [line];
    let cursor = index + 1;
    while (cursor < lines.length && isTableContinuationLine(lines[cursor])) {
      tableBlock.push(lines[cursor]);
      cursor += 1;
    }

    const normalizedTable = normalizeTableBlock(tableBlock, options);
    if (!normalizedTable) {
      output.push(line);
      continue;
    }

    if (output.length > 0 && output[output.length - 1].trim() !== "") {
      output.push("");
    }
    output.push(...normalizedTable);
    if (cursor < lines.length && lines[cursor].trim() !== "") {
      output.push("");
    }
    index = cursor - 1;
  }

  return output.join("\n");
}
