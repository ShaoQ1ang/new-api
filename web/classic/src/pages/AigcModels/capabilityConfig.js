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

export const IMAGE_MODES = ['text_to_image', 'image_edit'];

export const MUSIC_PROTOCOL_LEGACY = 'legacy-suno';
export const MUSIC_PROTOCOL_SUNOAPI_V1 = 'sunoapi-v1';

export const VIDEO_MODES = [
  'text_to_video',
  'first_frame',
  'first_last_frame',
  'reference',
  'video_extension',
  'video_edit',
];

export const IMAGE_SIZE_OPTIONS = [
  // Legacy and compact square sizes.
  '256x256',
  '512x512',
  '768x768',

  // OpenAI-compatible 1K presets: 21:9 through 9:16.
  '1008x432',
  '1024x576',
  '1008x672',
  '1024x768',
  '1024x1024',
  '768x1024',
  '672x1008',
  '576x1024',

  // OpenAI-compatible 2K presets.
  '2016x864',
  '2560x1440',
  '2496x1664',
  '2304x1728',
  '2048x2048',
  '1728x2304',
  '1664x2496',
  '1440x2560',

  // OpenAI-compatible 4K presets.
  '5376x3024',
  '4992x3328',
  '4672x3504',
  '4096x4096',
  '3504x4672',
  '3328x4992',
  '3024x5376',

  // Provider-specific sizes used by GPT Image, DALL-E and Qwen Image.
  '1024x1536',
  '1536x1024',
  '1024x1792',
  '1792x1024',
  '1328x1328',
  '1664x928',
  '928x1664',
  '1472x1140',
  '1140x1472',
];

// The visual image-size picker uses these canonical dimensions while the
// persisted capability format continues to store plain `WIDTHxHEIGHT` sizes.
export const IMAGE_SIZE_PRESETS = {
  '16:9': { '1K': '1024x576', '2K': '2560x1440', '4K': '5376x3024' },
  '3:2': { '1K': '1008x672', '2K': '2496x1664', '4K': '4992x3328' },
  '4:3': { '1K': '1024x768', '2K': '2304x1728', '4K': '4672x3504' },
  '1:1': { '1K': '1024x1024', '2K': '2048x2048', '4K': '4096x4096' },
  '3:4': { '1K': '768x1024', '2K': '1728x2304', '4K': '3504x4672' },
  '2:3': { '1K': '672x1008', '2K': '1664x2496', '4K': '3328x4992' },
  '9:16': { '1K': '576x1024', '2K': '1440x2560', '4K': '3024x5376' },
};
export const IMAGE_ASPECT_RATIOS = Object.keys(IMAGE_SIZE_PRESETS);
export const IMAGE_RESOLUTIONS = ['1K', '2K', '4K'];
export const DEFAULT_IMAGE_SIZES = Object.values(IMAGE_SIZE_PRESETS).flatMap(
  (tiers) => IMAGE_RESOLUTIONS.map((resolution) => tiers[resolution]),
);

export const VIDEO_RESOLUTION_OPTIONS = ['480p', '720p', '1080p', '2k', '4k'];
export const VIDEO_RATIO_OPTIONS = [
  '16:9',
  '9:16',
  '1:1',
  '4:3',
  '3:4',
  '21:9',
];
export const VIDEO_DURATION_OPTIONS = [2, 3, 4, 5, 6, 7, 8, 10, 12, 15];
export const IMAGE_COUNT_OPTIONS = [1, 2, 3, 4, 5, 6, 7, 8];

export const VIDEO_INPUT_ROLES = [
  'general_reference_image',
  'general_reference_video',
  'general_reference_audio',
  'first_frame',
  'last_frame',
  'driving_audio',
  'first_clip',
  'source_video',
];

export const VIDEO_COMBINATION_OPTIONS = [
  ['image'],
  ['video'],
  ['audio'],
  ['image', 'video'],
  ['image', 'audio'],
  ['video', 'audio'],
  ['image', 'video', 'audio'],
];

export function configTemplate(type) {
  switch (type) {
    case 'image':
      return {
        image: {
          adapter: 'openai-image',
          modes: {
            text_to_image: imageModeTemplate('text_to_image'),
          },
        },
      };
    case 'video':
      return {
        video: {
          adapter: 'openai-video',
          task_protocol: 'newapi-video',
          modes: { text_to_video: videoModeTemplate() },
          output_specs: [videoOutputTemplate(['text_to_video'])],
        },
      };
    case 'music':
      return musicConfigTemplate(MUSIC_PROTOCOL_SUNOAPI_V1);
    default:
      return { text: { upstream_model_id: '', max_output_tokens: 0 } };
  }
}

export function musicConfigTemplate(protocol, upstreamModelID = '') {
  const sunoAPIV1 = protocol === MUSIC_PROTOCOL_SUNOAPI_V1;
  return {
    music: {
      adapter: sunoAPIV1 ? 'sunoapi-music' : 'music-task',
      ...(sunoAPIV1 ? { task_protocol: MUSIC_PROTOCOL_SUNOAPI_V1 } : {}),
      modes: {
        text_to_music: {
          upstream_model_id: upstreamModelID,
          parameters: sunoAPIV1
            ? {
                instrumental: {
                  supported: true,
                  default: false,
                  configurable: true,
                },
                exact_lyrics: { supported: true, max_length: 5000 },
                style: { supported: true, max_length: 1000 },
                title: { supported: true, max_length: 100 },
                persona: {
                  supported: true,
                  voice_persona_supported: false,
                },
                duration: { supported: false },
                negative_tags: { supported: true, max_length: 1000 },
                vocal_gender: { supported: true },
                advanced_weights: { supported: true },
              }
            : {
                instrumental: { supported: true, default: false },
              },
          output: sunoAPIV1
            ? { min_tracks: 2, max_tracks: 2 }
            : { min_tracks: 1, max_tracks: 2 },
        },
      },
    },
  };
}

export function musicProtocol(config) {
  return config?.music?.task_protocol === MUSIC_PROTOCOL_SUNOAPI_V1
    ? MUSIC_PROTOCOL_SUNOAPI_V1
    : MUSIC_PROTOCOL_LEGACY;
}

export function setMusicProtocol(config, protocol) {
  const upstreamModelID =
    config?.music?.modes?.text_to_music?.upstream_model_id || '';
  return syncMusicModelCapabilities(
    musicConfigTemplate(protocol, upstreamModelID),
    upstreamModelID,
  );
}

export function syncMusicModelCapabilities(config, upstreamModelID) {
  const next = structuredClone(config);
  const mode = next?.music?.modes?.text_to_music;
  if (!mode) return next;
  mode.upstream_model_id = upstreamModelID;
  if (musicProtocol(next) !== MUSIC_PROTOCOL_SUNOAPI_V1) return next;

  const parameters = mode.parameters;
  parameters.persona ||= { supported: true };
  parameters.persona.voice_persona_supported =
    upstreamModelID === 'V5' || upstreamModelID === 'V5_5';
  parameters.duration =
    upstreamModelID === 'V5_5'
      ? { supported: true, min: 10, max: 360 }
      : { supported: false };
  return next;
}

export function imageModeTemplate(mode, upstreamModelID = '') {
  return {
    upstream_model_id: upstreamModelID,
    ...(mode === 'image_edit'
      ? {
          input: {
            role: 'source_image',
            min: 1,
            max: 1,
            accept: ['image/png', 'image/jpeg', 'image/webp'],
          },
        }
      : {}),
    output: {
      sizes: [...DEFAULT_IMAGE_SIZES],
      counts: [1],
      default_size: '1024x1024',
      default_count: 1,
    },
  };
}

export function videoModeTemplate(upstreamModelID = '') {
  return { upstream_model_id: upstreamModelID, inputs: {} };
}

export function videoOutputTemplate(modes = []) {
  return {
    id: 'default',
    modes,
    resolutions: ['720p'],
    aspect_ratios: ['16:9'],
    durations: [5],
    generate_audio: { supported: false, default: false },
  };
}

export function profileConfigAssignments(type, config) {
  if (type === 'text') {
    return config?.text
      ? [{ mode: 'default', upstream: config.text.upstream_model_id || '' }]
      : [];
  }
  const modes = config?.[type]?.modes;
  if (!modes || typeof modes !== 'object') return [];
  return Object.entries(modes)
    .map(([mode, value]) => ({
      mode,
      upstream: value?.upstream_model_id || '',
    }))
    .sort((a, b) => a.mode.localeCompare(b.mode));
}

export function setImageModes(config, selectedModes) {
  const next = structuredClone(config);
  const previous = next.image?.modes || {};
  next.image.modes = Object.fromEntries(
    selectedModes.map((mode) => [
      mode,
      previous[mode] || imageModeTemplate(mode),
    ]),
  );
  return next;
}

export function setVideoModes(config, selectedModes) {
  const next = structuredClone(config);
  const previous = next.video?.modes || {};
  next.video.modes = Object.fromEntries(
    selectedModes.map((mode) => [mode, previous[mode] || videoModeTemplate()]),
  );
  next.video.output_specs = (next.video.output_specs || [])
    .map((spec) => ({
      ...spec,
      modes: (spec.modes || []).filter((mode) => selectedModes.includes(mode)),
    }))
    .filter((spec) => spec.modes.length > 0);
  const coveredModes = new Set(
    next.video.output_specs.flatMap((spec) => spec.modes || []),
  );
  for (const mode of selectedModes) {
    if (coveredModes.has(mode)) continue;
    const spec = videoOutputTemplate([mode]);
    spec.id =
      next.video.output_specs.length === 0 ? 'default' : `${mode}-default`;
    next.video.output_specs.push(spec);
  }
  return next;
}

export function normalizeIntegerValues(values) {
  return [...new Set((values || []).map(Number).filter(Number.isInteger))].sort(
    (left, right) => left - right,
  );
}

export function combinationValue(combination) {
  return [...combination].sort().join('+');
}

export function parseCombinationValue(value) {
  return String(value)
    .split('+')
    .map((item) => item.trim())
    .filter(Boolean);
}
