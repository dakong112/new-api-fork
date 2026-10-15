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
import { describe, expect, it } from 'vitest'

import type { NavGroup } from '@/components/layout/types'
import { ROLE } from '@/lib/roles'

import { filterNavGroupsForUser } from '../use-sidebar-view'

const groups: NavGroup[] = [
  {
    id: 'personal',
    title: 'Personal',
    items: [{ title: 'Teams', url: '/team' }],
  },
  {
    id: 'admin',
    title: 'Admin',
    items: [
      {
        title: 'Users',
        url: '/users',
        requiredPermission: { resource: 'user', action: 'read' },
      },
      {
        title: 'Channels',
        url: '/channels',
        requiredPermission: { resource: 'channel', action: 'read' },
      },
      { title: 'Roles', url: '/roles', requiredRole: ROLE.SUPER_ADMIN },
    ],
  },
]

function titlesFor(role: number, admin_permissions = {}) {
  return filterNavGroupsForUser(groups, {
    id: 1,
    username: 'staff',
    role,
    permissions: { admin_permissions },
  }).flatMap((group) => group.items.map((item) => item.title))
}

describe('sidebar permission filtering', () => {
  it('an admin without user.read loses the Users entry but keeps granted ones', () => {
    expect(
      titlesFor(ROLE.ADMIN, { channel: { read: true }, user: { read: false } })
    ).toEqual(['Teams', 'Channels'])
  })

  it('root sees every admin entry without explicit permissions', () => {
    expect(titlesFor(ROLE.SUPER_ADMIN)).toEqual([
      'Teams',
      'Users',
      'Channels',
      'Roles',
    ])
  })

  it('an admin with no admin permissions gets no empty Admin group', () => {
    const result = filterNavGroupsForUser(groups, {
      id: 1,
      username: 'staff',
      role: ROLE.ADMIN,
    })
    expect(result.map((group) => group.id)).toEqual(['personal'])
  })
})
