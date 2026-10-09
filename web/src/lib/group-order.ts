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
import { useStatus } from '@/hooks/use-status'

/**
 * Sort group names for display: "auto" first, then the admin-defined order
 * (System Settings > Group Pricing), then any remaining groups alphabetically.
 */
export function sortGroupNames(names: string[], order: string[]): string[] {
  const rank = new Map(order.map((name, index) => [name, index]))
  return [...names].sort((a, b) => {
    if (a === 'auto' || b === 'auto') return a === 'auto' ? -1 : 1
    const ra = rank.get(a)
    const rb = rank.get(b)
    if (ra !== undefined && rb !== undefined) return ra - rb
    if (ra !== undefined) return -1
    if (rb !== undefined) return 1
    return a.localeCompare(b)
  })
}

/** The admin-defined group display order, from the public status. */
export function useGroupDisplayOrder(): string[] {
  const { status } = useStatus()
  const order = status?.group_display_order
  return Array.isArray(order) ? order.filter((g) => typeof g === 'string') : []
}
