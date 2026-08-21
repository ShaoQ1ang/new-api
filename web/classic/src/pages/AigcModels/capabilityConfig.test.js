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

import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import {
  IMAGE_SIZE_OPTIONS,
  MUSIC_PROTOCOL_LEGACY,
  MUSIC_PROTOCOL_SUNOAPI_V1,
  combinationValue,
  configTemplate,
  normalizeIntegerValues,
  parseCombinationValue,
  setImageModes,
  setMusicProtocol,
  syncMusicModelCapabilities,
  setVideoModes,
} from './capabilityConfig.js';

test('offers common image sizes across compact, 1K, 2K, 4K and provider presets', () => {
  for (const size of [
    '256x256',
    '1024x576',
    '2048x1152',
    '3840x2160',
    '1024x1792',
    '1328x1328',
  ]) {
    assert.ok(IMAGE_SIZE_OPTIONS.includes(size), `missing ${size}`);
  }
  assert.equal(
    new Set(IMAGE_SIZE_OPTIONS).size,
    IMAGE_SIZE_OPTIONS.length,
    'image size presets must be unique',
  );
});

describe('AIGC capability config helpers', () => {
  test('adds image edit with a valid source image input', () => {
    const config = setImageModes(configTemplate('image'), [
      'text_to_image',
      'image_edit',
    ]);

    assert.deepEqual(config.image.modes.image_edit.input, {
      role: 'source_image',
      min: 1,
      max: 1,
      accept: ['image/png', 'image/jpeg', 'image/webp'],
    });
  });

  test('removes disabled video modes from output specs', () => {
    const config = configTemplate('video');
    config.video.modes.first_frame = { upstream_model_id: 'video-i2v' };
    config.video.output_specs[0].modes.push('first_frame');

    const next = setVideoModes(config, ['first_frame']);

    assert.equal(next.video.modes.text_to_video, undefined);
    assert.deepEqual(next.video.output_specs[0].modes, ['first_frame']);
  });

  test('creates an output spec when enabling the first video mode', () => {
    const config = configTemplate('video');
    config.video.output_specs = [];

    const next = setVideoModes(config, ['reference']);

    assert.deepEqual(next.video.output_specs[0].modes, ['reference']);
  });

  test('adds a default output spec for a newly enabled video mode', () => {
    const next = setVideoModes(configTemplate('video'), [
      'text_to_video',
      'first_frame',
    ]);

    assert.deepEqual(next.video.output_specs[1].modes, ['first_frame']);
    assert.equal(next.video.output_specs[1].id, 'first_frame-default');
  });

  test('normalizes integer multi-select values', () => {
    assert.deepEqual(normalizeIntegerValues(['10', 5, '5', 'bad']), [5, 10]);
  });

  test('creates a complete SunoAPI v1 music profile by default', () => {
    const config = configTemplate('music');
    const music = config.music;
    const mode = music.modes.text_to_music;

    assert.equal(music.adapter, 'sunoapi-music');
    assert.equal(music.task_protocol, MUSIC_PROTOCOL_SUNOAPI_V1);
    assert.equal(mode.output.min_tracks, 2);
    assert.equal(mode.output.max_tracks, 2);
    assert.equal(mode.parameters.exact_lyrics.max_length, 5000);
    assert.equal(mode.parameters.duration.supported, false);
    assert.equal(mode.parameters.advanced_weights.supported, true);
  });

  test('adapts version-specific music capabilities to the assigned model', () => {
    const base = configTemplate('music');
    const v55 = syncMusicModelCapabilities(base, 'V5_5');
    assert.deepEqual(v55.music.modes.text_to_music.parameters.duration, {
      supported: true,
      min: 10,
      max: 360,
    });
    assert.equal(
      v55.music.modes.text_to_music.parameters.persona.voice_persona_supported,
      true,
    );

    const v4 = syncMusicModelCapabilities(v55, 'V4');
    assert.deepEqual(v4.music.modes.text_to_music.parameters.duration, {
      supported: false,
    });
    assert.equal(
      v4.music.modes.text_to_music.parameters.persona.voice_persona_supported,
      false,
    );
  });

  test('switches music protocols without losing the upstream assignment', () => {
    const config = configTemplate('music');
    config.music.modes.text_to_music.upstream_model_id = 'V5_5';

    const legacy = setMusicProtocol(config, MUSIC_PROTOCOL_LEGACY);
    assert.equal(legacy.music.adapter, 'music-task');
    assert.equal(legacy.music.task_protocol, undefined);
    assert.equal(legacy.music.modes.text_to_music.upstream_model_id, 'V5_5');

    const restored = setMusicProtocol(legacy, MUSIC_PROTOCOL_SUNOAPI_V1);
    assert.equal(restored.music.task_protocol, MUSIC_PROTOCOL_SUNOAPI_V1);
    assert.equal(restored.music.modes.text_to_music.upstream_model_id, 'V5_5');
    assert.equal(
      restored.music.modes.text_to_music.parameters.duration.supported,
      true,
    );
    assert.equal(
      restored.music.modes.text_to_music.parameters.persona
        .voice_persona_supported,
      true,
    );
  });

  test('round trips media combinations', () => {
    assert.deepEqual(
      parseCombinationValue(combinationValue(['video', 'image'])),
      ['image', 'video'],
    );
  });
});
