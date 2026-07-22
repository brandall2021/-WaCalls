import { apiGet } from "@/lib/api";

export interface Stats {
  sessions: { total: number; connected: number };
  calls: { active: number };
  recordings: { total: number; duration_ms: number; size_bytes: number };
  webhooks: { configs: number; delivered: number; failed: number };
}

export const fetchStats = () => apiGet<Stats>("/api/stats");
