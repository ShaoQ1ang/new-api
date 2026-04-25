import { api } from './api';
import { isUnauthorizedError } from './preview';

type TokenRecord = {
  id: number;
  key?: string;
  name?: string;
  status?: number;
  remain_quota?: number;
  unlimited_quota?: boolean;
  model_limits_enabled?: boolean;
  created_time?: number;
  accessed_time?: number;
  expired_time?: number;
  group?: string;
  used_quota?: number;
};

type TokenLogRecord = {
  token_name?: string;
  quota?: number;
};

type TokenLogResponse = {
  success: boolean;
  message?: string;
  data?: {
    items?: TokenLogRecord[];
    total?: number;
  };
};

export type TokenInput = {
  id?: number;
  name: string;
  group: string;
  remain_quota: number;
  unlimited_quota: boolean;
  model_limits_enabled: boolean;
  expired_time: number;
};

type TokenStatusInput = {
  id: number;
  status: number;
};

type TokenResponse = {
  success: boolean;
  message?: string;
  data?: {
    items?: TokenRecord[];
    total?: number;
  };
};

export type TokenWorkspaceRecord = TokenRecord & {
  today_quota?: number;
  total_quota?: number;
  preview?: boolean;
};

export type TokenWorkspaceResponse = {
  items: TokenWorkspaceRecord[];
  total: number;
};

function getRange(days: number) {
  const end = Math.floor(Date.now() / 1000);
  const start = end - days * 24 * 60 * 60;
  return { start, end };
}

async function fetchTokenUsageByWindow(days: number) {
  const { start, end } = getRange(days);
  const response = await api.get<TokenLogResponse>(
    `/api/log/self/?p=0&page_size=1000&type=2&start_timestamp=${start}&end_timestamp=${end}`,
  );
  const payload = response.data;

  if (!payload.success) {
    throw new Error(payload.message || 'Failed to load token logs');
  }

  const usageMap = new Map<string, number>();
  for (const item of payload.data?.items || []) {
    const tokenName = item.token_name || '';
    if (!tokenName) continue;
    usageMap.set(tokenName, (usageMap.get(tokenName) || 0) + (item.quota || 0));
  }

  return usageMap;
}

export async function fetchTokens(page = 1, pageSize = 20): Promise<TokenWorkspaceResponse> {
  try {
    const response = await api.get<TokenResponse>(`/api/token/?p=${page}&size=${pageSize}`);
    const payload = response.data;

    if (!payload.success) {
      throw new Error(payload.message || 'Failed to load tokens');
    }

    const items = payload.data?.items || [];
    const [todayUsageMap, last30UsageMap] = await Promise.all([
      fetchTokenUsageByWindow(1),
      fetchTokenUsageByWindow(30),
    ]);

    return {
      items: items.map((token) => ({
        ...token,
        today_quota: todayUsageMap.get(token.name || '') || 0,
        total_quota: last30UsageMap.get(token.name || '') || 0,
        preview: false,
      })),
      total: payload.data?.total || items.length,
    };
  } catch (error) {
    if (!isUnauthorizedError(error)) {
      throw error;
    }

    const now = Math.floor(Date.now() / 1000);
    const items = [
      {
        id: 1,
        key: 'sk-...A1B2',
        name: 'Production App',
        status: 1,
        remain_quota: 1200000,
        unlimited_quota: false,
        model_limits_enabled: false,
        created_time: now - 86400 * 15,
        accessed_time: now - 4000,
        expired_time: now + 86400 * 30,
        group: 'default',
        today_quota: 164300,
        total_quota: 2400000,
        preview: true,
      },
      {
        id: 2,
        key: 'sk-...C3D4',
        name: 'Team Sandbox',
        status: 1,
        unlimited_quota: true,
        model_limits_enabled: true,
        created_time: now - 86400 * 7,
        accessed_time: now - 8200,
        expired_time: -1,
        group: 'internal',
        today_quota: 0,
        total_quota: 448500,
        preview: true,
      },
    ];

    return {
      items: items.slice((page - 1) * pageSize, page * pageSize),
      total: items.length,
    };
  }
}

export async function createToken(input: TokenInput) {
  const response = await api.post('/api/token/', input);
  const payload = response.data;

  if (!payload.success) {
    throw new Error(payload.message || 'Failed to create token');
  }
}

export async function updateToken(input: TokenInput) {
  const response = await api.put('/api/token/', input);
  const payload = response.data;

  if (!payload.success) {
    throw new Error(payload.message || 'Failed to update token');
  }

  return payload.data;
}

export async function updateTokenStatus(input: TokenStatusInput) {
  const response = await api.put('/api/token/?status_only=1', input);
  const payload = response.data;

  if (!payload.success) {
    throw new Error(payload.message || 'Failed to update token status');
  }

  return payload.data;
}

export async function deleteToken(id: number) {
  const response = await api.delete(`/api/token/${id}/`);
  const payload = response.data;

  if (!payload.success) {
    throw new Error(payload.message || 'Failed to delete token');
  }
}

export async function fetchTokenKey(id: number) {
  const response = await api.post(`/api/token/${id}/key`);
  const payload = response.data;

  if (!payload.success || !payload.data?.key) {
    throw new Error(payload.message || 'Failed to fetch token key');
  }

  return payload.data.key as string;
}
