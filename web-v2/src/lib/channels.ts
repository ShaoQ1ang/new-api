import { api } from './api';
import { isUnauthorizedError } from './preview';

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
  try {
    const response = await api.get<ChannelResponse>(
      '/api/channel/?p=0&page_size=20&id_sort=false&tag_mode=false',
    );
    const payload = response.data;

    if (!payload.success) {
      throw new Error(payload.message || 'Failed to load channels');
    }

    return payload.data?.items || [];
  } catch (error) {
    if (!isUnauthorizedError(error)) {
      throw error;
    }

    return [
      {
        id: 1,
        name: 'Primary OpenAI',
        type: 1,
        status: 1,
        models: 'gpt-4.1,gpt-4.1-mini',
        response_time: 820,
        group: 'default',
      },
      {
        id: 2,
        name: 'Claude Fallback',
        type: 14,
        status: 1,
        models: 'claude-3.7-sonnet',
        response_time: 910,
        group: 'default',
      },
      {
        id: 3,
        name: 'Gemini Batch',
        type: 24,
        status: 0,
        models: 'gemini-2.5-pro',
        response_time: 1040,
        group: 'batch',
      },
    ];
  }
}
