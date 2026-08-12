import test from 'node:test';
import assert from 'node:assert/strict';
import {
  buildImageResolutionPriceValueFromModelMap,
  extractImageResolutionPriceMap,
} from './modelPricingImageResolutionPrice.js';

test('extractImageResolutionPriceMap returns controlled image tiers', () => {
  assert.deepEqual(
    extractImageResolutionPriceMap(`{
      "gpt-image-2": { "1k": 0.1, "2k": 0.2, "4k": 0.4 }
    }`),
    {
      'gpt-image-2': { '1k': 0.1, '2k': 0.2, '4k': 0.4 },
    },
  );
});

test('buildImageResolutionPriceValueFromModelMap preserves unrelated models', () => {
  const result = buildImageResolutionPriceValueFromModelMap(
    `{
      "other-image": { "2k": 0.3 },
      "gpt-image-2": { "1k": 0.1 }
    }`,
    {
      'gpt-image-2': { '1k': 0.12, '2k': 0.24, '4k': null },
    },
  );

  assert.deepEqual(JSON.parse(result), {
    'other-image': { '2k': 0.3 },
    'gpt-image-2': { '1k': 0.12, '2k': 0.24 },
  });
});

test('buildImageResolutionPriceValueFromModelMap removes cleared model', () => {
  const result = buildImageResolutionPriceValueFromModelMap(
    `{"gpt-image-2":{"1k":0.1,"2k":0.2}}`,
    {
      'gpt-image-2': { '1k': null, '2k': null, '4k': null },
    },
  );

  assert.deepEqual(JSON.parse(result), {});
});
