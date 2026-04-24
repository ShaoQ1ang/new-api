import { api } from './api';

export type UsageLogRecord = {
  id: number;
  created_at: number;
  token_name?: string;
  model_name?: string;
  quota?: number;
  prompt_tokens?: number;
  completion_tokens?: number;
  use_time?: number;
  is_stream?: boolean;
  request_id?: string;
  other?: string;
};

type UsageLogResponse = {
  success: boolean;
  message?: string;
  data?: {
    items?: UsageLogRecord[];
    total?: number;
  };
};

type UsageStatResponse = {
  success: boolean;
  message?: string;
  data?: {
    quota?: number;
    rpm?: number;
    tpm?: number;
  };
};

export type UsageQuery = {
  days: number;
  tokenName?: string;
};

function getRange(days: number) {
  const end = Math.floor(Date.now() / 1000);
  const start = end - days * 24 * 60 * 60;
  return { start, end };
}

export async function fetchUsageLogs(query: UsageQuery) {
  const { start, end } = getRange(query.days);
  const tokenName = query.tokenName || '';

  const logsUrl = `/api/log/self/?p=0&page_size=500&type=2&token_name=${encodeURIComponent(
    tokenName,
  )}&start_timestamp=${start}&end_timestamp=${end}`;
  const statUrl = `/api/log/self/stat?type=2&token_name=${encodeURIComponent(
    tokenName,
  )}&start_timestamp=${start}&end_timestamp=${end}`;

  const [logsResponse, statResponse] = await Promise.all([
    api.get<UsageLogResponse>(logsUrl),
    api.get<UsageStatResponse>(statUrl),
  ]);

  if (!logsResponse.data.success) {
    throw new Error(logsResponse.data.message || 'Failed to load usage logs');
  }

  if (!statResponse.data.success) {
    throw new Error(statResponse.data.message || 'Failed to load usage stats');
  }

  return {
    items: logsResponse.data.data?.items || [],
    total: logsResponse.data.data?.total || 0,
    stat: statResponse.data.data || {},
  };
}
