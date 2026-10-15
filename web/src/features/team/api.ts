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
import { requireServerSuccess } from '@/lib/server-error-message'

export type TeamRole = 'owner' | 'admin' | 'member'

export interface Team {
  id: number
  name: string
  owner_id: number
  wallet_user_id: number
  status: number
  created_at: number
}

// A team as one of its members sees it. A limit of 0 means unlimited.
export interface UserTeam extends Team {
  role: TeamRole
  limit_quota: number
  balance: number
  key_remain_quota: number
  key_used_quota: number
}

export interface TeamMember {
  user_id: number
  username: string
  display_name: string
  role: TeamRole
  limit_quota: number
  remain_quota: number
  used_quota: number
}

export interface TeamDetail {
  team: Team
  role: TeamRole
  balance: number
  members: TeamMember[]
}

export interface TeamUsageRow {
  token_id: number
  user_id: number
  username: string
  quota: number
  requests: number
}

export interface TeamMemberPayload {
  username?: string
  role?: TeamRole
  limit_quota?: number
}

type ApiResponse<T> = { success: boolean; message?: string; data: T }

async function unwrap<T>(request: Promise<{ data: ApiResponse<T> }>) {
  return requireServerSuccess((await request).data).data
}

export const getSelfTeams = () => unwrap<UserTeam[]>(api.get('/api/team/self'))

export const createTeam = (name: string) =>
  unwrap<Team>(api.post('/api/team', { name, limit_quota: 0 }))

export const getTeam = (id: number) =>
  unwrap<TeamDetail>(api.get(`/api/team/${id}`))

export const addTeamMember = (id: number, payload: TeamMemberPayload) =>
  unwrap<unknown>(api.post(`/api/team/${id}/members`, payload))

export const updateTeamMember = (
  id: number,
  userId: number,
  payload: TeamMemberPayload
) => unwrap<unknown>(api.put(`/api/team/${id}/members/${userId}`, payload))

export const removeTeamMember = (id: number, userId: number) =>
  unwrap<unknown>(api.delete(`/api/team/${id}/members/${userId}`))

export const transferToTeam = (id: number, quota: number) =>
  unwrap<unknown>(api.post(`/api/team/${id}/transfer`, { quota }))

export const redeemToTeam = (id: number, key: string) =>
  unwrap<number>(api.post(`/api/team/${id}/redeem`, { key }))

export const getTeamUsage = (id: number) =>
  unwrap<TeamUsageRow[]>(api.get(`/api/team/${id}/usage`))
