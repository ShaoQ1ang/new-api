/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

export const CHANNEL_TYPE_ADVANCED_CUSTOM = 58;

export const ADVANCED_CUSTOM_INCOMING_PATH_OPTIONS = [
  { value: '/v1/chat/completions', label: 'OpenAI Chat' },
  { value: '/v1/responses', label: 'OpenAI Responses' },
  { value: '/v1/responses/compact', label: 'OpenAI Responses Compact' },
  { value: '/v1/models', label: 'OpenAI Models' },
  { value: '/v1/embeddings', label: 'OpenAI Embeddings' },
  { value: '/v1/images/generations', label: 'OpenAI Image Generations' },
  { value: '/v1/images/edits', label: 'OpenAI Image Edits' },
  { value: '/v1/completions', label: 'OpenAI Completions' },
  { value: '/v1/audio/speech', label: 'OpenAI Audio Speech' },
  { value: '/v1/audio/transcriptions', label: 'OpenAI Audio Transcriptions' },
  { value: '/v1/audio/translations', label: 'OpenAI Audio Translations' },
  { value: '/v1/rerank', label: 'OpenAI Rerank' },
  { value: '/v1/realtime', label: 'OpenAI Realtime' },
  { value: '/v1/messages', label: 'Claude Messages' },
  {
    value: '/v1beta/models/{model}:generateContent',
    label: 'Gemini Generate Content',
  },
  {
    value: '/v1beta/models/{model}:embedContent',
    label: 'Gemini Embed Content',
  },
  {
    value: '/v1beta/models/{model}:batchEmbedContents',
    label: 'Gemini Batch Embed Contents',
  },
];

export const ADVANCED_CUSTOM_CONVERTER_OPTIONS = [
  { value: 'none', label: 'Native forwarding' },
  {
    value: 'anthropic_messages_to_openai_chat_completions',
    label: 'Anthropic Messages to OpenAI Chat',
  },
  {
    value: 'openai_chat_completions_to_anthropic_messages',
    label: 'OpenAI Chat to Anthropic Messages',
  },
  {
    value: 'openai_chat_completions_to_openai_responses',
    label: 'OpenAI Chat to OpenAI Responses',
  },
  {
    value: 'openai_responses_to_openai_chat_completions',
    label: 'OpenAI Responses to OpenAI Chat',
  },
  {
    value: 'openai_responses_to_gemini_generate_content',
    label: 'OpenAI Responses to Gemini Generate Content',
  },
  {
    value: 'gemini_generate_content_to_openai_chat_completions',
    label: 'Gemini Generate Content to OpenAI Chat',
  },
  {
    value: 'openai_chat_completions_to_gemini_generate_content',
    label: 'OpenAI Chat to Gemini Generate Content',
  },
];

export const ADVANCED_CUSTOM_AUTH_OPTIONS = [
  { value: 'default', label: 'Default Bearer' },
  { value: 'none', label: 'No Auth' },
  { value: 'header', label: 'Header' },
  { value: 'query', label: 'Query' },
];

const bearerAuth = () => ({
  type: 'header',
  name: 'Authorization',
  value: 'Bearer {api_key}',
});
const apiKeyAuth = () => ({
  type: 'header',
  name: 'x-api-key',
  value: '{api_key}',
});
const geminiAuth = () => ({ type: 'query', name: 'key', value: '{api_key}' });

export const ADVANCED_CUSTOM_TEMPLATES = [
  {
    value: 'official_openai_chat',
    label: 'Official OpenAI Chat',
    routes: [
      route(
        '/v1/chat/completions',
        '/v1/chat/completions',
        'none',
        bearerAuth(),
      ),
    ],
  },
  {
    value: 'official_openai_responses',
    label: 'Official OpenAI Responses',
    routes: [route('/v1/responses', '/v1/responses', 'none', bearerAuth())],
  },
  {
    value: 'responses_to_openai_chat',
    label: 'OpenAI Responses to OpenAI Chat',
    routes: [
      route(
        '/v1/responses',
        '/v1/chat/completions',
        'openai_responses_to_openai_chat_completions',
        bearerAuth(),
      ),
    ],
  },
  {
    value: 'official_openai_embeddings',
    label: 'Official OpenAI Embeddings',
    routes: [route('/v1/embeddings', '/v1/embeddings', 'none', bearerAuth())],
  },
  {
    value: 'official_claude_messages',
    label: 'Official Claude Messages',
    routes: [route('/v1/messages', '/v1/messages', 'none', apiKeyAuth())],
  },
  {
    value: 'official_gemini_native',
    label: 'Official Gemini Native',
    routes: [
      route(
        '/v1beta/models/{model}:generateContent',
        '/v1beta/models/{model}:generateContent',
        'none',
        geminiAuth(),
      ),
      route(
        '/v1beta/models/{model}:embedContent',
        '/v1beta/models/{model}:embedContent',
        'none',
        geminiAuth(),
      ),
    ],
  },
  {
    value: 'gemini_from_openai_chat',
    label: 'Official Gemini from OpenAI Chat',
    routes: [
      route(
        '/v1/chat/completions',
        '/v1beta/models/{model}:generateContent',
        'openai_chat_completions_to_gemini_generate_content',
        geminiAuth(),
      ),
    ],
  },
];

function route(incomingPath, upstreamPath, converter, auth) {
  return {
    incoming_path: incomingPath,
    upstream_path: upstreamPath,
    converter,
    auth,
  };
}

export function createAdvancedCustomConfig() {
  return {
    advanced_routes: [
      route('/v1/chat/completions', '/v1/chat/completions', 'none'),
    ],
  };
}

export function cloneAdvancedCustomConfig(config) {
  return JSON.parse(JSON.stringify(config));
}

export function parseAdvancedCustomConfig(value) {
  if (!value || !String(value).trim()) return null;
  try {
    const parsed = JSON.parse(value);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return null;
    }
    return normalizeAdvancedCustomConfig(parsed);
  } catch {
    return null;
  }
}

export function normalizeAdvancedCustomConfig(config) {
  const routes = Array.isArray(config?.advanced_routes)
    ? config.advanced_routes.map((item) => {
        const normalized = {
          incoming_path: String(item?.incoming_path || '').trim(),
          upstream_path: String(item?.upstream_path || '').trim(),
          converter: item?.converter || 'none',
        };
        const models = Array.isArray(item?.models)
          ? [
              ...new Set(
                item.models
                  .map((model) => String(model).trim())
                  .filter(Boolean),
              ),
            ]
          : [];
        if (models.length) normalized.models = models;
        if (item?.auth) {
          normalized.auth = { type: item.auth.type };
          if (item.auth.type !== 'none') {
            normalized.auth.name = String(item.auth.name || '').trim();
            normalized.auth.value = String(item.auth.value || '').trim();
          }
        }
        return normalized;
      })
    : [];
  return { advanced_routes: routes };
}

export function stringifyAdvancedCustomConfig(config) {
  return JSON.stringify(normalizeAdvancedCustomConfig(config), null, 2);
}

export function getAdvancedCustomConverterOptions(incomingPath) {
  return ADVANCED_CUSTOM_CONVERTER_OPTIONS.filter(
    (option) =>
      option.value === 'none' ||
      converterMatchesPath(incomingPath, option.value),
  );
}

export function getAdvancedCustomDefaults(converter, incomingPath) {
  if (converter === 'none') {
    if (incomingPath === '/v1/messages') {
      return { upstream_path: incomingPath, auth: apiKeyAuth() };
    }
    if (incomingPath.includes(':')) {
      return { upstream_path: incomingPath, auth: geminiAuth() };
    }
    return { upstream_path: incomingPath, auth: bearerAuth() };
  }
  if (
    converter === 'anthropic_messages_to_openai_chat_completions' ||
    converter === 'gemini_generate_content_to_openai_chat_completions' ||
    converter === 'openai_responses_to_openai_chat_completions'
  ) {
    return { upstream_path: '/v1/chat/completions', auth: bearerAuth() };
  }
  if (converter === 'openai_chat_completions_to_openai_responses') {
    return { upstream_path: '/v1/responses', auth: bearerAuth() };
  }
  if (converter === 'openai_chat_completions_to_anthropic_messages') {
    return { upstream_path: '/v1/messages', auth: apiKeyAuth() };
  }
  return {
    upstream_path: '/v1beta/models/{model}:generateContent',
    auth: geminiAuth(),
  };
}

export function buildAdvancedCustomAuth(mode, previousAuth) {
  if (mode === 'default') return undefined;
  if (mode === 'none') return { type: 'none' };
  if (mode === 'query') {
    return {
      type: 'query',
      name: previousAuth?.name || 'api_key',
      value: previousAuth?.value || '{api_key}',
    };
  }
  return {
    type: 'header',
    name: previousAuth?.name || 'Authorization',
    value: previousAuth?.value || 'Bearer {api_key}',
  };
}

export function validateAdvancedCustomConfig(config) {
  if (!config) return { message: 'Advanced custom configuration is required' };
  const routes = normalizeAdvancedCustomConfig(config).advanced_routes;
  if (!routes.length) {
    return {
      message: 'Advanced custom configuration requires at least one route',
    };
  }

  const pathState = new Map();
  let modelListSeen = false;
  for (let index = 0; index < routes.length; index += 1) {
    const current = routes[index];
    const incomingPath = current.incoming_path;
    if (!incomingPath) return error(index, 'Incoming path is required');
    if (!incomingPath.startsWith('/')) {
      return error(index, 'Incoming path must start with /');
    }
    if (incomingPath.includes('?')) {
      return error(index, 'Incoming path must not include query');
    }
    if (!current.upstream_path)
      return error(index, 'Upstream path is required');
    if (!isFullUrlOrPath(current.upstream_path)) {
      return error(
        index,
        'Upstream path must be a full URL or a path starting with /',
      );
    }
    if (
      !ADVANCED_CUSTOM_CONVERTER_OPTIONS.some(
        ({ value }) => value === current.converter,
      )
    ) {
      return error(index, 'Converter is not registered');
    }
    if (!converterMatchesPath(incomingPath, current.converter)) {
      return error(index, 'Converter does not match incoming path');
    }

    if (incomingPath === '/v1/models') {
      if (modelListSeen)
        return error(index, 'Only one OpenAI Models route is allowed');
      modelListSeen = true;
      if (current.models?.length) {
        return error(
          index,
          'OpenAI Models route does not support client model rules',
        );
      }
      if (current.converter !== 'none') {
        return error(index, 'OpenAI Models route must use native forwarding');
      }
      if (current.upstream_path.includes('{model}')) {
        return error(
          index,
          'OpenAI Models upstream path must not contain {model}',
        );
      }
    }

    const state = pathState.get(incomingPath) || {
      catchAll: false,
      models: new Set(),
    };
    const models = current.models || [];
    if (!models.length) {
      if (state.catchAll) {
        return error(
          index,
          'Only one catch-all route is allowed for the same incoming path',
        );
      }
      state.catchAll = true;
    } else {
      if (state.catchAll) {
        return error(
          index,
          'Catch-all route must be last for the same incoming path',
        );
      }
      const routeModels = new Set();
      for (const model of models) {
        if (model === 're:') return error(index, 'Model regex cannot be empty');
        if (routeModels.has(model))
          return error(index, 'Duplicate model in route models');
        if (state.models.has(model)) {
          return error(
            index,
            'Route models must be unique for the same incoming path',
          );
        }
        routeModels.add(model);
        state.models.add(model);
      }
    }
    pathState.set(incomingPath, state);

    if (current.auth && current.auth.type !== 'none') {
      if (!['header', 'query'].includes(current.auth.type)) {
        return error(index, 'Auth type is invalid');
      }
      if (!current.auth.name) return error(index, 'Auth name is required');
      if (!current.auth.value) return error(index, 'Auth value is required');
    }
  }
  return null;
}

function error(routeIndex, message) {
  return { routeIndex, message };
}

function converterMatchesPath(incomingPath, converter) {
  if (converter === 'none') return true;
  if (converter === 'anthropic_messages_to_openai_chat_completions') {
    return incomingPath === '/v1/messages';
  }
  if (
    converter === 'openai_chat_completions_to_anthropic_messages' ||
    converter === 'openai_chat_completions_to_openai_responses' ||
    converter === 'openai_chat_completions_to_gemini_generate_content'
  ) {
    return incomingPath === '/v1/chat/completions';
  }
  if (
    converter === 'openai_responses_to_openai_chat_completions' ||
    converter === 'openai_responses_to_gemini_generate_content'
  ) {
    return incomingPath === '/v1/responses';
  }
  return (
    incomingPath.includes(':generateContent') ||
    incomingPath.includes(':streamGenerateContent')
  );
}

function isFullUrlOrPath(value) {
  if (value.startsWith('/')) return !value.startsWith('//');
  try {
    const parsed = new URL(value);
    return (
      ['http:', 'https:'].includes(parsed.protocol) && Boolean(parsed.host)
    );
  } catch {
    return false;
  }
}
