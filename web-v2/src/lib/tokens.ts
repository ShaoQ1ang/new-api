import { api } from './api';

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

type TokenResponse = {
  success: boolean;
  message?: string;
  data?: {
    items?: TokenRecord[];
    total?: number;
  };
};

export async function fetchTokens() {
  const response = await api.get<TokenResponse>('/api/token/?p=1&size=20');
  const payload = response.data;

  if (!payload.success) {
    throw new Error(payload.message || 'Failed to load tokens');
  }

  return payload.data?.items || [];
}
