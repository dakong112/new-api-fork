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
import type {
  AdminPermissionMatrix,
  PermissionResourceDef,
  PermissionRoleDef,
} from '@/lib/admin-permissions'
import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

// A catalog resource tagged with the application that enforces it; an empty
// app is this gateway.
export type AppPermissionResource = PermissionResourceDef & { app: string }

export type AuthzRole = PermissionRoleDef & { description: string }

export interface AuthzCatalog {
  resources: AppPermissionResource[]
  roles: AuthzRole[]
}

export interface AuthzRoleInput {
  key: string
  name: string
  description: string
}

export const AUTHZ_CATALOG_QUERY_KEY = ['authz-catalog', 'all-apps']

// getAuthzCatalog returns the resources of every application with all roles.
export async function getAuthzCatalog(): Promise<AuthzCatalog> {
  const res = await api.get('/api/authz/catalog', { params: { app: '*' } })
  requireServerSuccess(res.data)
  return {
    resources: res.data?.data?.resources ?? [],
    roles: res.data?.data?.roles ?? [],
  }
}

export async function createAuthzRole(input: AuthzRoleInput): Promise<void> {
  requireServerSuccess((await api.post('/api/authz/roles', input)).data)
}

export async function updateAuthzRole(input: AuthzRoleInput): Promise<void> {
  requireServerSuccess(
    (
      await api.put(`/api/authz/roles/${encodeURIComponent(input.key)}`, {
        name: input.name,
        description: input.description,
      })
    ).data
  )
}

export async function deleteAuthzRole(key: string): Promise<void> {
  requireServerSuccess(
    (await api.delete(`/api/authz/roles/${encodeURIComponent(key)}`)).data
  )
}

// setAuthzRolePermissions replaces a custom role's grants with the granted
// cells of the matrix, as "resource.action" codes.
export async function setAuthzRolePermissions(
  key: string,
  matrix: AdminPermissionMatrix
): Promise<void> {
  const permissions = Object.entries(matrix).flatMap(([resource, actions]) =>
    Object.entries(actions)
      .filter(([, granted]) => granted)
      .map(([action]) => `${resource}.${action}`)
  )
  requireServerSuccess(
    (
      await api.put(`/api/authz/roles/${encodeURIComponent(key)}/permissions`, {
        permissions,
      })
    ).data
  )
}
