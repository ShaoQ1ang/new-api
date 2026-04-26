import { api } from './api';

export type TaskLogRecord = {
  id: number;
  created_at: number;
  updated_at: number;
  task_id: string;
  platform: string;
  user_id: number;
  group: string;
  channel_id: number;
  quota: number;
  action: string;
  status: string;
  fail_reason: string;
  result_url?: string;
  submit_time: number;
  start_time: number;
  finish_time: number;
  progress: string;
  properties?: {
    input?: string;
    upstream_model_name?: string;
    origin_model_name?: string;
  } | null;
  data?: unknown;
};

type TaskLogsResponse = {
  success: boolean;
  message?: string;
  data?: {
    items?: TaskLogRecord[];
    total?: number;
    page?: number;
    page_size?: number;
  };
};

export async function fetchTaskLogs(params: {
  page: number;
  pageSize: number;
  taskId?: string;
  status?: string;
  platform?: string;
  startTimestamp?: number;
  endTimestamp?: number;
}) {
  const search = new URLSearchParams({
    p: String(params.page),
    page_size: String(params.pageSize),
  });

  if (params.taskId) search.set('task_id', params.taskId);
  if (params.status) search.set('status', params.status);
  if (params.platform) search.set('platform', params.platform);
  if (params.startTimestamp) search.set('start_timestamp', String(params.startTimestamp));
  if (params.endTimestamp) search.set('end_timestamp', String(params.endTimestamp));

  const response = await api.get<TaskLogsResponse>(`/api/task/self?${search.toString()}`);
  if (response.data.success === false) {
    throw new Error(response.data.message || 'Failed to load task logs');
  }

  return {
    items: response.data.data?.items || [],
    total: response.data.data?.total || 0,
    page: response.data.data?.page || params.page,
    pageSize: response.data.data?.page_size || params.pageSize,
  };
}
