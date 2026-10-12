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
import { api } from '@/lib/api'

export interface SSOApp {
  client_id: string
  name: string
  launch_url: string
}

interface ApiEnvelope<T> {
  success: boolean
  message?: string
  data?: T
}

export async function createSSOCode(params: {
  client_id: string
  redirect_uri: string
  state: string
}): Promise<string> {
  const res = await api.post<ApiEnvelope<{ redirect_url: string }>>(
    '/api/sso/code',
    params
  )
  if (!res.data.success || !res.data.data?.redirect_url) {
    throw new Error(res.data.message || 'SSO authorization failed')
  }
  return res.data.data.redirect_url
}

export async function listSSOApps(): Promise<SSOApp[]> {
  const res = await api.get<ApiEnvelope<SSOApp[]>>('/api/sso/apps')
  if (!res.data.success) {
    throw new Error(res.data.message || 'Failed to load applications')
  }
  return res.data.data ?? []
}
