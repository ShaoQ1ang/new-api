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

import { afterEach, describe, expect, test } from 'bun:test';

import { getStoredUser } from './managementPermissions';

const originalLocalStorage = globalThis.localStorage;

afterEach(() => {
  globalThis.localStorage = originalLocalStorage;
});

describe('getStoredUser', () => {
  test('returns the stored user when the value is valid JSON', () => {
    globalThis.localStorage = {
      getItem: () => JSON.stringify({ id: 1, role: 100 }),
    };

    expect(getStoredUser()).toEqual({ id: 1, role: 100 });
  });

  test('returns null instead of throwing for a malformed stored user', () => {
    globalThis.localStorage = {
      getItem: () => '{invalid-json',
    };

    expect(getStoredUser()).toBeNull();
  });
});
