export interface AIConfig {
  baseURL: string;
  apiKey: string;
  model: string;
  maxTokens: number;
  temperature: number;
}

export interface NL2SQLResult {
  sql: string;
  explanation: string;
  confidence: number;
}

export interface SchemaIndexStatus {
  schemaKey?: string;
  databaseName?: string;
  exists: boolean;
  refreshing: boolean;
  dirty: boolean;
  stale: boolean;
  lastRefreshedAt?: string;
  lastError?: string;
  tableCount: number;
  source?: "memory" | "persisted" | "rebuilt";
}

export interface SQLExplanation {
  summary: string;
  steps: ExplanationStep[];
  optimizations: string[];
  estimatedCost: string;
}

export interface ExplanationStep {
  step: number;
  operation: string;
  description: string;
}

export interface DataInsight {
  summary: string;
  anomalies: AnomalyItem[];
  trends: TrendItem[];
  statistics: Record<string, string>;
}

export interface AnomalyItem {
  column: string;
  description: string;
  severity: "low" | "medium" | "high";
}

export interface TrendItem {
  column: string;
  description: string;
  direction: "up" | "down" | "stable";
}

export interface Diagnosis {
  errorType: string;
  cause: string;
  suggestion: string;
  fixedSQL: string;
}
