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

type TokenStatResponse = {
  success: boolean;
  message?: string;
  data?: {
    quota?: number;
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
};

function getRange(days: number) {
  const end = Math.floor(Date.now() / 1000);
  const start = end - days * 24 * 60 * 60;
  return { start, end };
}

async function fetchTokenStatByName(tokenName: string, days: number) {
  const { start, end } = getRange(days);
  const response = await api.get<TokenStatResponse>(
    `/api/log/self/stat?type=2&token_name=${encodeURIComponent(
      tokenName,
    )}&start_timestamp=${start}&end_timestamp=${end}`,
  );
  const payload = response.data;

  if (!payload.success) {
    throw new Error(payload.message || 'Failed to load token stats');
  }

  return payload.data?.quota || 0;
}

export async function fetchTokens() {
  try {
    const response = await api.get<TokenResponse>('/api/token/?p=1&size=20');
    const payload = response.data;

    if (!payload.success) {
      throw new Error(payload.message || 'Failed to load tokens');
    }

    const items = payload.data?.items || [];
    const usagePairs = await Promise.all(
      items.map(async (token) => {
        const tokenName = token.name || '';
        if (!tokenName) {
          return { today_quota: 0, total_quota: Math.max(0, token.used_quota || 0) };
        }

        const [todayQuota, totalQuota] = await Promise.all([
          fetchTokenStatByName(tokenName, 1),
          fetchTokenStatByName(tokenName, 30),
        ]);

        return {
          today_quota: todayQuota,
          total_quota: totalQuota,
        };
      }),
    );

    return items.map((token, index) => ({
      ...token,
      ...usagePairs[index],
    }));
  } catch (error) {
    if (!isUnauthorizedError(error)) {
      throw error;
    }

    const now = Math.floor(Date.now() / 1000);
    return [
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
      },
    ];
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
