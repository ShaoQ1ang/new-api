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
  combinationValue,
  configTemplate,
  normalizeIntegerValues,
  parseCombinationValue,
  setImageModes,
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

  test('round trips media combinations', () => {
    assert.deepEqual(
      parseCombinationValue(combinationValue(['video', 'image'])),
      ['image', 'video'],
    );
  });
});
