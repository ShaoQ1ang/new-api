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

import test from 'node:test';
import assert from 'node:assert/strict';
import {
  getPricingSyncCategory,
  parseCurrentPricingOptions,
  shouldDiscardSelectedPricingField,
} from './modelPricingSyncFields.js';

test('current pricing options include image input prices for every sync path', () => {
  const current = parseCurrentPricingOptions({
    ModelPrice: '{"request-model":0.2}',
    VideoSecondsPrice: '{"video-model":{"720p":{"default":0.1}}}',
    ImageInputPrice: '{"other-model":{"default":0.01}}',
  });

  assert.deepEqual(current.ModelPrice, { 'request-model': 0.2 });
  assert.deepEqual(current.VideoSecondsPrice, {
    'video-model': { '720p': { default: 0.1 } },
  });
  assert.deepEqual(current.ImageInputPrice, {
    'other-model': { default: 0.01 },
  });
});

test('image input price is an ancillary synchronization field', () => {
  assert.equal(getPricingSyncCategory('image_input_price'), 'ancillary');
});

test('ancillary image prices coexist with per-request and video selections', () => {
  assert.equal(
    shouldDiscardSelectedPricingField('image_input_price', 'model_price'),
    false,
  );
  assert.equal(
    shouldDiscardSelectedPricingField(
      'image_input_price',
      'video_seconds_price',
    ),
    false,
  );
  assert.equal(
    shouldDiscardSelectedPricingField('model_price', 'image_input_price'),
    false,
  );
  assert.equal(
    shouldDiscardSelectedPricingField(
      'video_seconds_price',
      'image_input_price',
    ),
    false,
  );
});

test('exclusive base pricing fields still replace each other', () => {
  assert.equal(
    shouldDiscardSelectedPricingField('model_price', 'video_seconds_price'),
    true,
  );
});
