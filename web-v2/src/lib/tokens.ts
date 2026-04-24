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

export async function fetchTokens() {
  try {
    const response = await api.get<TokenResponse>('/api/token/?p=1&size=20');
    const payload = response.data;

    if (!payload.success) {
      throw new Error(payload.message || 'Failed to load tokens');
    }

    return payload.data?.items || [];
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
        expired_time: now + 86400 * 30,
        group: 'default',
      },
      {
        id: 2,
        key: 'sk-...C3D4',
        name: 'Team Sandbox',
        status: 1,
        unlimited_quota: true,
        model_limits_enabled: true,
        created_time: now - 86400 * 7,
        expired_time: -1,
        group: 'internal',
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
