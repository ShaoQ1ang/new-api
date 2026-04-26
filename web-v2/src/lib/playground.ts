import { api } from './api';

type TokenListResponse = {
  success?: boolean;
  message?: string;
  data?: {
    items?: PlaygroundTokenRecord[];
  };
};

type TokenKeyResponse = {
  success?: boolean;
  message?: string;
  data?: {
    key?: string;
  };
};

export type PlaygroundTokenRecord = {
  id: number;
  name?: string;
  status?: number;
  key?: string;
};

type UserModelsResponse = {
  success?: boolean;
  message?: string;
  data?: string[];
};

type ChatCompletionResponse = {
  choices?: Array<{
    message?: {
      content?: string | ChatContentPart[];
    };
  }>;
  error?: {
    message?: string;
  };
};

type ChatContentPart = {
  type?: string;
  text?: string;
};

type ImageGenerationResponse = {
  data?: Array<{
    url?: string;
    b64_json?: string;
  }>;
  error?: {
    message?: string;
  };
  message?: string;
};

type VideoGenerationResponse = {
  task_id?: string;
  status?: string;
  error?: {
    message?: string;
  };
  message?: string;
};

type VideoTaskResponse = {
  task_id?: string;
  status?: string;
  url?: string;
  error?: {
    message?: string;
  };
  message?: string;
};

function normalizeChatContent(content: string | ChatContentPart[] | undefined) {
  if (typeof content === 'string') return content;
  if (Array.isArray(content)) {
    return content
      .map((item) => item?.text || '')
      .filter(Boolean)
      .join('\n');
  }
  return '';
}

async function parsePayload<T>(response: Response) {
  const contentType = response.headers.get('content-type') || '';
  if (contentType.includes('application/json')) {
    return (await response.json()) as T;
  }

  const text = await response.text();
  return { message: text } as T;
}

async function tokenFetch<T>(input: string, init: RequestInit, apiKey: string) {
  const response = await fetch(input, {
    ...init,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${apiKey}`,
      ...(init.headers || {}),
    },
  });

  const payload = await parsePayload<T>(response);
  if (!response.ok) {
    const message =
      (payload as any)?.error?.message ||
      (payload as any)?.message ||
      `Request failed with status ${response.status}`;
    throw new Error(message);
  }

  return payload;
}

function createImageCompatiblePayload(params: {
  model: string;
  prompt: string;
  referenceImages: string[];
}) {
  const firstReference = params.referenceImages[0];
  return {
    model: params.model,
    prompt: params.prompt,
    n: 1,
    size: '1024x1024',
    response_format: 'url',
    ...(firstReference
      ? {
          image: firstReference,
          images: [firstReference],
          input_reference: firstReference,
        }
      : {}),
  };
}

function createVideoCompatiblePayload(params: {
  model: string;
  prompt: string;
  referenceImages: string[];
}) {
  const firstReference = params.referenceImages[0];
  return {
    model: params.model,
    prompt: params.prompt,
    duration: 5,
    n: 1,
    response_format: 'url',
    ...(firstReference
      ? {
          image: firstReference,
          images: [firstReference],
          input_reference: firstReference,
        }
      : {}),
  };
}

export async function fetchPlaygroundModels() {
  const response = await api.get<UserModelsResponse>('/api/user/models');
  if (response.data.success === false) {
    throw new Error(response.data.message || 'Failed to load models');
  }
  return response.data.data || [];
}

export async function fetchPlaygroundTokens() {
  const response = await api.get<TokenListResponse>('/api/token/?p=1&size=100');
  if (response.data.success === false) {
    throw new Error(response.data.message || 'Failed to load tokens');
  }

  return (response.data.data?.items || []).filter((item) => item.status === 1);
}

export async function fetchPlaygroundTokenKey(id: number) {
  const response = await api.post<TokenKeyResponse>(`/api/token/${id}/key`);
  if (response.data.success === false || !response.data.data?.key) {
    throw new Error(response.data.message || 'Failed to fetch token key');
  }

  return response.data.data.key;
}

export async function sendPlaygroundChat(params: {
  model: string;
  prompt: string;
  referenceImages: string[];
}) {
  const response = await api.post<ChatCompletionResponse>('/pg/chat/completions', {
    model: params.model,
    stream: false,
    messages: [
      {
        role: 'user',
        content: [
          { type: 'text', text: params.prompt },
          ...params.referenceImages.map((imageUrl) => ({
            type: 'image_url',
            image_url: { url: imageUrl },
          })),
        ],
      },
    ],
  });

  if (response.data.error?.message) {
    throw new Error(response.data.error.message);
  }

  return normalizeChatContent(response.data.choices?.[0]?.message?.content) || 'No response';
}

export async function sendPlaygroundImage(params: {
  model: string;
  prompt: string;
  referenceImages: string[];
  apiKey: string;
}) {
  const payload = await tokenFetch<ImageGenerationResponse>(
    '/v1/images/generations',
    {
      method: 'POST',
      body: JSON.stringify(createImageCompatiblePayload(params)),
    },
    params.apiKey,
  );

  if (payload.error?.message) {
    throw new Error(payload.error.message);
  }

  const item = payload.data?.[0];
  if (item?.url) return item.url;
  if (item?.b64_json) return `data:image/png;base64,${item.b64_json}`;
  throw new Error(payload.message || 'Image response did not include a result');
}

export async function createPlaygroundVideo(params: {
  model: string;
  prompt: string;
  referenceImages: string[];
  apiKey: string;
}) {
  const payload = await tokenFetch<VideoGenerationResponse>(
    '/v1/video/generations',
    {
      method: 'POST',
      body: JSON.stringify(createVideoCompatiblePayload(params)),
    },
    params.apiKey,
  );

  if (payload.error?.message) {
    throw new Error(payload.error.message);
  }
  if (!payload.task_id) {
    throw new Error(payload.message || 'Video task did not return task_id');
  }
  return payload.task_id;
}

export async function fetchPlaygroundVideoTask(taskId: string, apiKey: string) {
  const payload = await tokenFetch<VideoTaskResponse>(`/v1/video/generations/${taskId}`, {
    method: 'GET',
  }, apiKey);

  if (payload.error?.message) {
    throw new Error(payload.error.message);
  }

  return payload;
}
