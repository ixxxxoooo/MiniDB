import { stripStreamMetaBlocks } from "@/components/ai/streamMeta";

export type AIStreamStepState = "running" | "done" | "success" | "error";

export type AIStreamStep =
  | {
    id: string;
    kind: "status";
    status: string;
    state: "running" | "done" | "error";
    at: number;
    updatedAt: number;
    sequence?: number;
  }
  | {
    id: string;
    kind: "thinking";
    content: string;
    state: "running" | "done";
    at: number;
    updatedAt: number;
    sequence?: number;
  }
  | {
    id: string;
    kind: "tool";
    toolCallId: string;
    toolName: string;
    state: "running" | "success" | "error";
    toolInput?: string;
    toolSql?: string;
    durationMs?: number;
    at: number;
    updatedAt: number;
    sequence?: number;
  }
  | {
    id: string;
    kind: "observation";
    toolCallId: string;
    toolName: string;
    state: "success" | "error";
    content: string;
    durationMs?: number;
    at: number;
    updatedAt: number;
    sequence?: number;
  }
  | {
    id: string;
    kind: "answer";
    content: string;
    state: "running" | "done";
    at: number;
    updatedAt: number;
    sequence?: number;
  };

export interface AIStreamStepEvent {
  type?: string;
  delta?: string;
  content?: string;
  error?: string;
  sequence?: number;
  toolName?: string;
  toolCallId?: string;
  toolState?: string;
  toolInput?: string;
  toolSql?: string;
  toolOutput?: string;
  durationMs?: number;
  thinkingContent?: string;
}

export function normalizeStreamThinkingContent(raw: string): string {
  if (!raw) return "";
  // 只剥离协议标签和 <think> 包裹，不做中文内容模板过滤
  // 真实推理模型返回的 reasoning 内容应原样展示
  return stripStreamMetaBlocks(raw)
    .replace(/<think>/gi, "")
    .replace(/<\/think>/gi, "")
    .replace(/\r\n/g, "\n");
}

export function hasAIStreamSteps(steps?: AIStreamStep[]): boolean {
  return Array.isArray(steps) && steps.length > 0;
}

function cleanToolOutput(raw?: string): string {
  return stripStreamMetaBlocks(String(raw || "")).trim();
}

function orderedSteps(steps: AIStreamStep[]): AIStreamStep[] {
  return [...steps].sort((a, b) => {
    const seqA = a.sequence ?? Number.MAX_SAFE_INTEGER;
    const seqB = b.sequence ?? Number.MAX_SAFE_INTEGER;
    if (seqA !== seqB) return seqA - seqB;
    if (a.at !== b.at) return a.at - b.at;
    return a.id.localeCompare(b.id);
  });
}

function finishRunningSteps(steps: AIStreamStep[], now: number): AIStreamStep[] {
  return steps.map((step) => {
    if (step.kind === "status" && step.state === "running") {
      return { ...step, state: "done", updatedAt: now };
    }
    if (step.kind === "thinking" && step.state === "running") {
      return { ...step, state: "done", updatedAt: now };
    }
    // 流结束时仍在 running 的 tool step 标记为 success（避免转圈不停）
    if (step.kind === "tool" && step.state === "running") {
      return { ...step, state: "success" as const, updatedAt: now };
    }
    if (step.kind === "answer" && step.state === "running") {
      return { ...step, state: "done", updatedAt: now };
    }
    return step;
  });
}

function closeRunningAnswerSteps(steps: AIStreamStep[], now: number): AIStreamStep[] {
  return steps.map((step) => {
    if (step.kind === "answer" && step.state === "running") {
      return { ...step, state: "done" as const, updatedAt: now };
    }
    return step;
  });
}

function ensureAnswerStep(steps: AIStreamStep[], event: AIStreamStepEvent, now: number): AIStreamStep[] {
  const existingIndex = (() => {
    for (let i = steps.length - 1; i >= 0; i--) {
      const step = steps[i];
      if (step.kind === "answer" && step.state === "running") return i;
    }
    return -1;
  })();

  if (existingIndex >= 0) {
    return orderedSteps(steps.map((step, index) =>
      index === existingIndex
        ? { ...step, updatedAt: now, sequence: step.sequence ?? event.sequence }
        : step
    ));
  }

  return orderedSteps([
    ...steps,
    {
      id: `answer:${event.sequence ?? now}`,
      kind: "answer",
      content: "",
      state: "running",
      at: now,
      updatedAt: now,
      sequence: event.sequence,
    },
  ]);
}

function appendAnswerDelta(steps: AIStreamStep[], event: AIStreamStepEvent, now: number): AIStreamStep[] {
  const delta = String(event.delta || "");
  const withAnswer = ensureAnswerStep(steps, event, now);
  let patched = false;
  const next = [...withAnswer].reverse().map((step) => {
    if (patched || step.kind !== "answer" || step.state !== "running") return step;
    patched = true;
    return {
      ...step,
      content: stripStreamMetaBlocks(`${step.content}${delta}`),
      updatedAt: now,
      sequence: step.sequence ?? event.sequence,
    };
  }).reverse();
  return orderedSteps(next);
}

function toolCallIdFor(event: AIStreamStepEvent): string {
  return String(event.toolCallId || event.toolName || "unknown_tool");
}

function inferToolState(event: AIStreamStepEvent): "running" | "success" | "error" {
  if (event.type === "tool_error" || event.toolState === "error") return "error";
  if (event.type === "tool_result" || event.toolState === "success") return "success";
  return "running";
}

function earliestSequence(current?: number, next?: number): number | undefined {
  if (current === undefined) return next;
  if (next === undefined) return current;
  return Math.min(current, next);
}

function reduceToolEvent(steps: AIStreamStep[], event: AIStreamStepEvent, now: number): AIStreamStep[] {
  const baseSteps = closeRunningAnswerSteps(steps, now);
  const toolCallId = toolCallIdFor(event);
  const toolId = `tool:${toolCallId}`;
  const observationId = `observation:${toolCallId}`;
  const toolName = String(event.toolName || "unknown");
  const state = inferToolState(event);
  let foundTool = false;

  let next = baseSteps.map((step) => {
    if (step.kind !== "tool" || step.id !== toolId) return step;
    foundTool = true;
    return {
      ...step,
      toolName: event.toolName || step.toolName,
      state,
      toolInput: event.toolInput || step.toolInput,
      toolSql: event.toolSql || step.toolSql,
      durationMs: event.durationMs ?? step.durationMs,
      updatedAt: now,
      sequence: earliestSequence(step.sequence, event.sequence),
    };
  });

  if (!foundTool) {
    next = [
      ...next,
      {
        id: toolId,
        kind: "tool",
        toolCallId,
        toolName,
        state,
        toolInput: event.toolInput,
        toolSql: event.toolSql,
        durationMs: event.durationMs,
        at: now,
        updatedAt: now,
        sequence: event.sequence,
      },
    ];
  }

  if (event.type === "tool_result" || event.type === "tool_error") {
    const content = cleanToolOutput(event.toolOutput);
    const observationState = event.type === "tool_error" ? "error" : "success";
    let foundObservation = false;
    next = next.map((step) => {
      if (step.kind !== "observation" || step.id !== observationId) return step;
      foundObservation = true;
      return {
        ...step,
        toolName,
        state: observationState,
        content,
        durationMs: event.durationMs ?? step.durationMs,
        updatedAt: now,
        sequence: step.sequence ?? event.sequence,
      };
    });
    if (!foundObservation) {
      next = [
        ...next,
        {
          id: observationId,
          kind: "observation",
          toolCallId,
          toolName,
          state: observationState,
          content,
          durationMs: event.durationMs,
          at: now,
          updatedAt: now,
          sequence: event.sequence,
        },
      ];
    }
  }

  return orderedSteps(next);
}

function reduceThinkingEvent(steps: AIStreamStep[], event: AIStreamStepEvent, now: number): AIStreamStep[] {
  const content = normalizeStreamThinkingContent(event.delta || event.thinkingContent || "");
  if (!content) return steps;

  const baseSteps = closeRunningAnswerSteps(steps, now);
  const last = baseSteps[baseSteps.length - 1];
  let next: AIStreamStep[];
  if (last?.kind === "thinking" && last.state === "running") {
    next = [
      ...baseSteps.slice(0, -1),
      {
        ...last,
        content: `${last.content}${content}`,
        updatedAt: now,
      },
    ];
  } else {
    next = [
      ...baseSteps,
      {
        id: `thinking:${event.sequence ?? now}`,
        kind: "thinking",
        content,
        state: "running",
        at: now,
        updatedAt: now,
        sequence: event.sequence,
      },
    ];
  }

  return orderedSteps(next);
}

function reduceStatusEvent(steps: AIStreamStep[], event: AIStreamStepEvent, now: number): AIStreamStep[] {
  const status = String(event.delta || event.content || event.error || "").trim();
  if (!status) return steps;
  const last = steps[steps.length - 1];
  if (last?.kind === "status" && last.status === status) {
    return steps;
  }
  const previousDone = steps.map((step) => {
    if (step.kind === "status" && step.state === "running") {
      return { ...step, state: "done" as const, updatedAt: now };
    }
    return step;
  });
  const state = event.type === "error" ? "error" : "running";
  return orderedSteps([
    ...previousDone,
    {
      id: `status:${status}:${event.sequence ?? now}`,
      kind: "status",
      status,
      state,
      at: now,
      updatedAt: now,
      sequence: event.sequence,
    },
  ]);
}

function applyDoneContent(steps: AIStreamStep[], event: AIStreamStepEvent, now: number): AIStreamStep[] {
  const content = stripStreamMetaBlocks(String(event.content || "")).trim();
  const doneSteps = finishRunningSteps(steps, now);
  if (!content) {
    return orderedSteps(doneSteps);
  }

  let patched = false;
  const next = [...doneSteps].reverse().map((step) => {
    if (patched || step.kind !== "answer") return step;
    patched = true;
    return {
      ...step,
      content,
      state: "done" as const,
      updatedAt: now,
    };
  }).reverse();

  if (patched) {
    return orderedSteps(next);
  }
  return orderedSteps([
    ...doneSteps,
    {
      id: `answer:${event.sequence ?? now}`,
      kind: "answer",
      content,
      state: "done",
      at: now,
      updatedAt: now,
      sequence: event.sequence,
    },
  ]);
}

export function reduceAIStreamSteps(steps: AIStreamStep[], event: AIStreamStepEvent, now = Date.now()): AIStreamStep[] {
  switch (event.type) {
    case "status":
      return reduceStatusEvent(steps, event, now);
    case "reasoning":
    case "thinking":
      return reduceThinkingEvent(steps, event, now);
    case "tool_start":
    case "tool_args":
    case "tool_sql":
    case "tool_result":
    case "tool_error":
      return reduceToolEvent(steps, event, now);
    case "answer_start":
    case "final_answer":
      return ensureAnswerStep(steps, event, now);
    case "delta":
      return appendAnswerDelta(steps, event, now);
    case "done":
      return applyDoneContent(steps, event, now);
    case "error":
      return orderedSteps(finishRunningSteps(reduceStatusEvent(steps, event, now), now));
    default:
      return steps;
  }
}
