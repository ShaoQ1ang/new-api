import { api } from './api';
import { isUnauthorizedError } from './preview';

export type UsageLogRecord = {
  id: number;
  created_at: number;
  type: number;
  content?: string;
  token_name?: string;
  model_name?: string;
  group?: string;
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
  days?: number;
  startTimestamp?: number;
  endTimestamp?: number;
  tokenName?: string;
  modelName?: string;
  group?: string;
  requestId?: string;
  logType?: number;
  page?: number;
  pageSize?: number;
};

function getRange(days: number) {
  const end = Math.floor(Date.now() / 1000);
  const start = end - days * 24 * 60 * 60;
  return { start, end };
}

export async function fetchUsageLogs(query: UsageQuery) {
  const range =
    typeof query.startTimestamp === 'number' && typeof query.endTimestamp === 'number'
      ? { start: query.startTimestamp, end: query.endTimestamp }
      : getRange(query.days || 7);
  const { start, end } = range;
  const tokenName = query.tokenName || '';
  const modelName = query.modelName || '';
  const group = query.group || '';
  const requestId = query.requestId || '';
  const logType = query.logType || 0;
  const page = query.page || 1;
  const pageSize = query.pageSize || 20;

  const logsUrl = `/api/log/self/?p=${page}&page_size=${pageSize}&type=${logType}&token_name=${encodeURIComponent(
    tokenName,
  )}&model_name=${encodeURIComponent(modelName)}&start_timestamp=${start}&end_timestamp=${end}&group=${encodeURIComponent(
    group,
  )}&request_id=${encodeURIComponent(requestId)}`;
  const statUrl = `/api/log/self/stat?type=${logType}&token_name=${encodeURIComponent(
    tokenName,
  )}&model_name=${encodeURIComponent(modelName)}&start_timestamp=${start}&end_timestamp=${end}&group=${encodeURIComponent(
    group,
  )}`;

  try {
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
  } catch (error) {
    if (!isUnauthorizedError(error)) {
      throw error;
    }

    const previewItems: UsageLogRecord[] = [
      {
        id: 1,
        created_at: end - 3600,
        type: 2,
        content: '操作 generate',
        token_name: 'Production App',
        model_name: 'gpt-4.1',
        group: 'default',
        quota: 11952,
        prompt_tokens: 8754,
        completion_tokens: 1320,
        use_time: 2145,
        is_stream: false,
        request_id: 'preview_req_001',
        other: '{"request_path":"/responses"}',
      },
      {
        id: 2,
        created_at: end - 2400,
        type: 2,
        content: '操作 generate',
        token_name: 'Production App',
        model_name: 'gpt-4.1-mini',
        group: 'default',
        quota: 4932,
        prompt_tokens: 2210,
        completion_tokens: 814,
        use_time: 980,
        is_stream: true,
        request_id: 'preview_req_002',
        other: '{"request_path":"/responses"}',
      },
      {
        id: 3,
        created_at: end - 900,
        type: 6,
        content: 'token重算：tokens=2810',
        token_name: 'Team Sandbox',
        model_name: 'claude-3.7-sonnet',
        group: 'default',
        quota: 2812,
        prompt_tokens: 0,
        completion_tokens: 0,
        use_time: 0,
        is_stream: false,
        request_id: 'preview_req_003',
        other:
          '{"request_path":"/v1/messages","billing_source":"wallet","actual_quota":14032,"pre_consumed_quota":16844}',
      },
    ];

    const filteredItems = previewItems.filter((item) => {
      if (logType && item.type !== logType) return false;
      if (tokenName && !(item.token_name || '').toLowerCase().includes(tokenName.toLowerCase())) return false;
      if (modelName && !(item.model_name || '').toLowerCase().includes(modelName.toLowerCase())) return false;
      if (group && !(item.group || '').toLowerCase().includes(group.toLowerCase())) return false;
      if (requestId && !(item.request_id || '').toLowerCase().includes(requestId.toLowerCase())) return false;
      return true;
    });

    return {
      items: filteredItems.slice((page - 1) * pageSize, page * pageSize),
      total: filteredItems.length,
      stat: {
        quota: filteredItems.reduce((sum, item) => sum + (item.quota || 0), 0),
        rpm: 0,
        tpm: 0,
      },
    };
  }
}
