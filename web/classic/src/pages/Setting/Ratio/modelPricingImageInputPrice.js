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

export const IMAGE_INPUT_PRICE_KEYS = [
  'default',
  '1k',
  '2k',
  '4k',
  '8k',
  'free_count',
];

const parseImageInputPrice = (rawImageInputPrice) => {
  if (!rawImageInputPrice) {
    return {};
  }
  try {
    const parsed = JSON.parse(rawImageInputPrice);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch (error) {
    console.error('Failed to parse ImageInputPrice:', error);
    return {};
  }
};

const cloneImageInputPriceMap = (value) =>
  JSON.parse(JSON.stringify(value || {}));

export function extractImageInputPriceMap(rawImageInputPrice) {
  const parsed = parseImageInputPrice(rawImageInputPrice);
  const result = {};

  Object.entries(parsed).forEach(([modelName, prices]) => {
    if (!prices || typeof prices !== 'object' || Array.isArray(prices)) {
      return;
    }
    result[modelName] = IMAGE_INPUT_PRICE_KEYS.reduce((acc, key) => {
      if (prices[key] !== undefined) {
        acc[key] = prices[key];
      }
      return acc;
    }, {});
  });

  return result;
}

export function buildImageInputPriceValueFromModelMap(
  rawImageInputPrice,
  modelMap,
) {
  const nextImageInputPrice = cloneImageInputPriceMap(
    parseImageInputPrice(rawImageInputPrice),
  );

  Object.entries(modelMap || {}).forEach(([modelName, prices]) => {
    const nextModelPrices = cloneImageInputPriceMap(
      nextImageInputPrice[modelName],
    );

    IMAGE_INPUT_PRICE_KEYS.forEach((key) => {
      const value = prices?.[key];
      if (value !== null && value !== undefined && value !== '') {
        nextModelPrices[key] = value;
      } else {
        delete nextModelPrices[key];
      }
    });

    if (Object.keys(nextModelPrices).length > 0) {
      nextImageInputPrice[modelName] = nextModelPrices;
    } else {
      delete nextImageInputPrice[modelName];
    }
  });

  return JSON.stringify(nextImageInputPrice, null, 2);
}
