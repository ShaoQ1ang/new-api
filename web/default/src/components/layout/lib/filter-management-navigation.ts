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
import {
  MANAGEMENT_PERMISSION,
  hasAnyManagementPermission,
  hasManagementPermission,
} from '@/lib/management-permissions'
import { ROLE } from '@/lib/roles'
import type { NavGroup, NavItem } from '../types'

function filterAdminItem(item: NavItem, user: AuthUser): NavItem | null {
  if (item.url === '/models/metadata') {
    return hasManagementPermission(user, MANAGEMENT_PERMISSION.CHAT_MODELS)
      ? { ...item, url: '/models/chat' }
      : null
  }

  if (item.url === '/client-releases') {
    return hasAnyManagementPermission(user, [
      MANAGEMENT_PERMISSION.CLIENT_RELEASES,
      MANAGEMENT_PERMISSION.CLIENT_RELEASES_PUBLISH,
    ])
      ? item
      : null
  }

  if ('items' in item && item.items) {
    const items = item.items.filter((subItem) => {
      if (subItem.url === '/skill-hub/reports') {
        return hasManagementPermission(
          user,
          MANAGEMENT_PERMISSION.SKILL_HUB_REPORTS
        )
      }
      if (subItem.url === '/skill-hub' || subItem.url === '/skill-hub/tags') {
        return hasManagementPermission(
          user,
          MANAGEMENT_PERMISSION.SKILL_HUB_CONTENT
        )
      }
      return false
    })
    return items.length > 0 ? { ...item, items } : null
  }

  return null
}

export function filterNavigationByManagementPermissions(
  navGroups: NavGroup[],
  user: AuthUser | null | undefined
): NavGroup[] {
  if (!user) return navGroups.filter((group) => group.id !== 'admin')
  if (user.role >= ROLE.ADMIN) return navGroups

  return navGroups
    .map((group) => {
      if (group.id !== 'admin') return group
      return {
        ...group,
        items: group.items
          .map((item) => filterAdminItem(item, user))
          .filter((item): item is NavItem => item !== null),
      }
    })
    .filter((group) => group.items.length > 0)
}
