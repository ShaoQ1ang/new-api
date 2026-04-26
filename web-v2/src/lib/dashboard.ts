import { api } from './api';
import { isUnauthorizedError } from './preview';

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

  let items = [] as DashboardDataPoint[];

  try {
    const response = await api.get<DashboardResponse>(
      `/api/data/self/?start_timestamp=${start}&end_timestamp=${end}&default_time=7`,
    );
    const payload = response.data;

    if (!payload.success) {
      throw new Error(payload.message || 'Failed to load dashboard overview');
    }

    items = payload.data || [];
  } catch (error) {
    if (!isUnauthorizedError(error)) {
      throw error;
    }

    items = [
      { created_at: end - 86400 * 3, count: 42, quota: 123400, model_name: 'gpt-4.1' },
      { created_at: end - 86400 * 2, count: 31, quota: 82200, model_name: 'gpt-4.1-mini' },
      { created_at: end - 86400 * 2, count: 18, quota: 54100, model_name: 'claude-3.7-sonnet' },
      { created_at: end - 86400, count: 27, quota: 68100, model_name: 'gemini-2.5-pro' },
      { created_at: end, count: 21, quota: 47400, model_name: 'gpt-4.1' },
    ];
  }
  const totalRequests = items.reduce((sum, item) => sum + (item.count || 0), 0);
  const totalQuota = items.reduce((sum, item) => sum + (item.quota || 0), 0);
  const providerCount = new Set(
    items.map((item) => item.model_name).filter(Boolean),
  ).size;
  const activeDays = new Set(
    items
      .map((item) => item.created_at)
      .filter(Boolean)
      .map((timestamp) => new Date(timestamp * 1000).toISOString().slice(0, 10)),
  ).size;

  const modelMap = new Map<
    string,
    {
      requests: number;
      quota: number;
    }
  >();

  for (const item of items) {
    const modelName = item.model_name || 'unknown';
    const current = modelMap.get(modelName) || { requests: 0, quota: 0 };
    current.requests += item.count || 0;
    current.quota += item.quota || 0;
    modelMap.set(modelName, current);
  }

  const topModels = Array.from(modelMap.entries())
    .map(([name, value]) => ({
      name,
      requests: value.requests,
      quota: value.quota,
      share: totalRequests > 0 ? value.requests / totalRequests : 0,
    }))
    .sort((left, right) => right.requests - left.requests)
    .slice(0, 4);

  const recentItems = [...items]
    .sort((left, right) => right.created_at - left.created_at)
    .slice(0, 5);

  return {
    items,
    totalRequests,
    totalQuota,
    providerCount,
    activeDays,
    topModels,
    recentItems,
  };
}
