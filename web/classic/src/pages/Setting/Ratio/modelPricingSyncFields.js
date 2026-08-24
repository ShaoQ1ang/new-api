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

const RATIO_SYNC_FIELDS = new Set([
  'model_ratio',
  'completion_ratio',
  'cache_ratio',
  'create_cache_ratio',
  'image_ratio',
  'audio_ratio',
  'audio_completion_ratio',
]);

export function getPricingSyncCategory(ratioType) {
  if (ratioType === 'image_input_price') return 'ancillary';
  if (ratioType === 'model_price') return 'price';
  if (ratioType === 'video_seconds_price') return 'video';
  if (ratioType === 'billing_mode' || ratioType === 'billing_expr') {
    return 'tiered';
  }
  if (RATIO_SYNC_FIELDS.has(ratioType)) return 'ratio';
  return 'ratio';
}

export function shouldDiscardSelectedPricingField(
  incomingRatioType,
  existingRatioType,
) {
  const incomingCategory = getPricingSyncCategory(incomingRatioType);
  const existingCategory = getPricingSyncCategory(existingRatioType);
  if (
    incomingCategory === 'ancillary' ||
    existingCategory === 'ancillary' ||
    incomingCategory === 'tiered' ||
    existingCategory === 'tiered'
  ) {
    return false;
  }
  return incomingCategory !== existingCategory;
}
