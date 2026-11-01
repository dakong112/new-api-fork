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

import { getUserGroupModels } from '../api'

/**
 * The models of every group the user may select. Model hints are optional,
 * so a failed load shows no list instead of an error toast.
 */
export function useUserGroupModels(enabled = true) {
  return useQuery({
    queryKey: ['user-group-models'],
    queryFn: async () => requireServerSuccess(await getUserGroupModels()),
    select: (res) => res.data,
    enabled,
    staleTime: 60_000,
    meta: { errorToast: false },
  })
}
