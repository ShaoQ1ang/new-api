export const IMAGE_RESOLUTION_PRICE_TIERS = ['1k', '2k', '4k'];

const parseImageResolutionPrice = (rawImageResolutionPrice) => {
  if (!rawImageResolutionPrice) {
    return {};
  }
  try {
    const parsed = JSON.parse(rawImageResolutionPrice);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch (error) {
    console.error('Failed to parse ImageResolutionPrice:', error);
    return {};
  }
};

const cloneImageResolutionPriceMap = (value) =>
  JSON.parse(JSON.stringify(value || {}));

export function extractImageResolutionPriceMap(rawImageResolutionPrice) {
  const parsed = parseImageResolutionPrice(rawImageResolutionPrice);
  const result = {};

  Object.entries(parsed).forEach(([modelName, prices]) => {
    if (!prices || typeof prices !== 'object' || Array.isArray(prices)) {
      return;
    }
    result[modelName] = IMAGE_RESOLUTION_PRICE_TIERS.reduce((acc, tier) => {
      if (prices[tier] !== undefined) {
        acc[tier] = prices[tier];
      }
      return acc;
    }, {});
  });

  return result;
}

export function buildImageResolutionPriceValueFromModelMap(
  rawImageResolutionPrice,
  modelMap,
) {
  const nextImageResolutionPrice = cloneImageResolutionPriceMap(
    parseImageResolutionPrice(rawImageResolutionPrice),
  );

  Object.entries(modelMap || {}).forEach(([modelName, prices]) => {
    const nextModelPrices = cloneImageResolutionPriceMap(
      nextImageResolutionPrice[modelName],
    );

    IMAGE_RESOLUTION_PRICE_TIERS.forEach((tier) => {
      const value = prices?.[tier];
      if (value !== null && value !== undefined && value !== '') {
        nextModelPrices[tier] = value;
      } else {
        delete nextModelPrices[tier];
      }
    });

    if (Object.keys(nextModelPrices).length > 0) {
      nextImageResolutionPrice[modelName] = nextModelPrices;
    } else {
      delete nextImageResolutionPrice[modelName];
    }
  });

  return JSON.stringify(nextImageResolutionPrice, null, 2);
}
