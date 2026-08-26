/**
 * 历史兼容：剥离旧链路模型正文中的元数据/DSML 控制块。
 * 新 Agent 协议（v2）已把结构化数据（suggestions/工具结果）移到事件通道，
 * 本模块仅用于清洗旧会话/历史消息中的残留块，不参与新链路。
 */

export function stripStreamMetaBlocks(text: string): string {
  if (!text) return text;
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