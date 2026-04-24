import { api } from './api';

type ChannelRecord = {
  id: number;
  name?: string;
  type?: number | string;
  status?: number;
  models?: string;
  model_mapping?: string;
  response_time?: number;
  group?: string;
  priority?: number;
  tag?: string;
};

type ChannelResponse = {
  success: boolean;
  message?: string;
  data?: {
    items?: ChannelRecord[];
    total?: number;
  };
};

export async function fetchChannels() {
  const response = await api.get<ChannelResponse>(
    '/api/channel/?p=0&page_size=20&id_sort=false&tag_mode=false',
  );
  const payload = response.data;

  if (!payload.success) {
    throw new Error(payload.message || 'Failed to load channels');
  }

  return payload.data?.items || [];
}
