/*
Copyright (C) 2023-2026 QuantumNous

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
import type { AuthUser } from '@/stores/auth-store'
import { ROLE } from './roles'

export const MANAGEMENT_PERMISSION = {
  SKILL_HUB_CONTENT: 'skill_hub.content.manage',
  SKILL_HUB_REPORTS: 'skill_hub.reports.manage',
  CHAT_MODELS: 'chat_models.manage',
  CLIENT_RELEASES: 'client_releases.manage',
  CLIENT_RELEASES_PUBLISH: 'client_releases.publish',
} as const

export type ManagementPermission =
  (typeof MANAGEMENT_PERMISSION)[keyof typeof MANAGEMENT_PERMISSION]

type ManagementUser = Pick<AuthUser, 'role' | 'management_permissions'>

export function hasManagementPermission(
  user: ManagementUser | null | undefined,
  permission: ManagementPermission
): boolean {
  if (!user) return false
  if (user.role >= ROLE.ADMIN) return true
  return user.management_permissions?.includes(permission) === true
}

export function hasAnyManagementPermission(
  user: ManagementUser | null | undefined,
  permissions: readonly ManagementPermission[]
): boolean {
  return permissions.some((permission) =>
    hasManagementPermission(user, permission)
  )
}
