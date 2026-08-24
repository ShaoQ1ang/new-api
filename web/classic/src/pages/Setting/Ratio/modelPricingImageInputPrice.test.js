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
  buildImageInputPriceValueFromModelMap,
  extractImageInputPriceMap,
} from './modelPricingImageInputPrice.js';

test('extractImageInputPriceMap returns supported prices and free count', () => {
  assert.deepEqual(
    extractImageInputPriceMap(`{
      "image-edit-model": {
        "default": 0.01,
        "1k": 0.02,
        "2k": 0.03,
        "4k": 0.04,
        "8k": 0.05,
        "free_count": 1
      }
    }`),
    {
      'image-edit-model': {
        default: 0.01,
        '1k': 0.02,
        '2k': 0.03,
        '4k': 0.04,
        '8k': 0.05,
        free_count: 1,
      },
    },
  );
});

test('buildImageInputPriceValueFromModelMap preserves unrelated models', () => {
  const result = buildImageInputPriceValueFromModelMap(
    `{
      "other-model": { "default": 0.5 },
      "image-edit-model": { "default": 0.01, "free_count": 1 }
    }`,
    {
      'image-edit-model': {
        default: 0.02,
        '1k': 0.03,
        '2k': null,
        '4k': null,
        '8k': null,
        free_count: 2,
      },
    },
  );

  assert.deepEqual(JSON.parse(result), {
    'other-model': { default: 0.5 },
    'image-edit-model': { default: 0.02, '1k': 0.03, free_count: 2 },
  });
});

test('buildImageInputPriceValueFromModelMap removes a cleared model', () => {
  const result = buildImageInputPriceValueFromModelMap(
    `{"image-edit-model":{"default":0.01,"free_count":1}}`,
    {
      'image-edit-model': {
        default: null,
        '1k': null,
        '2k': null,
        '4k': null,
        '8k': null,
        free_count: null,
      },
    },
  );

  assert.deepEqual(JSON.parse(result), {});
});
