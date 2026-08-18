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
  '2048x1152',
  '2016x1344',
  '2048x1536',
  '2048x2048',
  '1536x2048',
  '1344x2016',
  '1152x2048',

  // OpenAI-compatible 4K presets.
  '3808x1632',
  '3840x2160',
  '3504x2336',
  '3264x2448',
  '2880x2880',
  '2448x3264',
  '2336x3504',
  '2160x3840',

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
      return {
        music: {
          adapter: 'suno',
          modes: {
            text_to_music: {
              upstream_model_id: '',
              parameters: {
                instrumental: { supported: true, default: false },
              },
              output: { min_tracks: 1, max_tracks: 2 },
            },
          },
        },
      };
    default:
      return { text: { upstream_model_id: '', max_output_tokens: 0 } };
  }
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
      sizes: ['1024x1024'],
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
