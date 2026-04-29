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

function isSeparatorLine(line: string): boolean {
  const cells = parsePipeCells(line);
  return cells.length >= 2 && cells.every(isSeparatorCell);
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

function formatTableRow(cells: string[], columnCount: number): string {
  const normalized = [...cells];
  while (normalized.length < columnCount) {
    normalized.push("");
  }
  return `| ${normalized.slice(0, columnCount).join(" | ")} |`;
}

function formatSeparatorRow(line: string, columnCount: number): string {
  const cells = parsePipeCells(line);
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

function normalizeTableBlock(lines: string[]): string[] | null {
  const trimmedLines = lines.map((line) => line.trim()).filter(Boolean);
  if (trimmedLines.length < 2) {
    return null;
  }

  const headerCells = parsePipeCells(trimmedLines[0]);
  if (headerCells.length < 2) {
    return null;
  }

  const separatorIndex = trimmedLines.findIndex((line, index) => index > 0 && isSeparatorLine(line));
  const hasExplicitTableShape = separatorIndex > 0 || (trimmedLines.length >= 3 && trimmedLines.every((line) => line.startsWith("|")));
  if (!hasExplicitTableShape) {
    return null;
  }

  const bodyLines = trimmedLines.filter((line, index) => index > 0 && index !== separatorIndex);
  const bodyCells = bodyLines.map(parsePipeCells).filter((cells) => cells.length > 0);

  const columnCount = Math.max(
    headerCells.length,
    ...bodyCells.map((cells) => cells.length),
  );

  if (columnCount < 2) {
    return null;
  }

  return [
    formatTableRow(headerCells, columnCount),
    separatorIndex > 0
      ? formatSeparatorRow(trimmedLines[separatorIndex], columnCount)
      : formatSeparatorRow("", columnCount),
    ...bodyCells.map((cells) => formatTableRow(cells, columnCount)),
  ];
}

export function normalizeAIMarkdown(content: string): string {
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
    while (cursor < lines.length && isTableCandidateLine(lines[cursor])) {
      tableBlock.push(lines[cursor]);
      cursor += 1;
    }

    const normalizedTable = normalizeTableBlock(tableBlock);
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
