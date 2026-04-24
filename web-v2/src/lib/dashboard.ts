import { api } from './api';

type DashboardDataPoint = {
  created_at: number;
  count?: number;
  quota?: number;
  model_name?: string;
};

type DashboardResponse = {
  success: boolean;
  message?: string;
  data?: DashboardDataPoint[];
};

export async function fetchDashboardOverview() {
  const now = new Date();
  const end = Math.floor(now.getTime() / 1000);
  const start = end - 7 * 24 * 60 * 60;

  const response = await api.get<DashboardResponse>(
    `/api/data/self/?start_timestamp=${start}&end_timestamp=${end}&default_time=7`,
  );
  const payload = response.data;

  if (!payload.success) {
    throw new Error(payload.message || 'Failed to load dashboard overview');
  }

  const items = payload.data || [];
  const totalRequests = items.reduce((sum, item) => sum + (item.count || 0), 0);
  const totalQuota = items.reduce((sum, item) => sum + (item.quota || 0), 0);
  const providerCount = new Set(
    items.map((item) => item.model_name).filter(Boolean),
  ).size;

  return {
    items,
    totalRequests,
    totalQuota,
    providerCount,
  };
}
