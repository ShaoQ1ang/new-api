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

export const MANAGEMENT_PERMISSION = Object.freeze({
  SKILL_HUB_CONTENT: 'skill_hub.content.manage',
  SKILL_HUB_REPORTS: 'skill_hub.reports.manage',
  CHAT_MODELS: 'chat_models.manage',
  CLIENT_RELEASES: 'client_releases.manage',
  CLIENT_RELEASES_PUBLISH: 'client_releases.publish',
});

export function getStoredUser() {
  try {
    const raw = localStorage.getItem('user');
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

export function hasManagementPermission(permission, user = getStoredUser()) {
  if (!user) return false;
  if (Number(user.role) >= 10) return true;
  return (
    Array.isArray(user.management_permissions) &&
    user.management_permissions.includes(permission)
  );
}

export function hasAnyManagementPermission(
  permissions,
  user = getStoredUser(),
) {
  return permissions.some((permission) =>
    hasManagementPermission(permission, user),
  );
}
