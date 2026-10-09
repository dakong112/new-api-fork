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
import { useQuery } from '@tanstack/react-query'

import { requireServerSuccess } from '@/lib/server-error-message'

import { getGroupRatios } from '../api'

const EMPTY_GROUP_RATIOS: Record<string, number> = {}

/** Configured ratio per group name; empty until loaded. */
export function useChannelGroupRatios(): Record<string, number> {
  const { data } = useQuery({
    queryKey: ['group-ratios'],
    queryFn: async () => requireServerSuccess(await getGroupRatios()),
    select: (res) => res.data ?? EMPTY_GROUP_RATIOS,
    staleTime: 60 * 1000,
  })
  return data ?? EMPTY_GROUP_RATIOS
}
