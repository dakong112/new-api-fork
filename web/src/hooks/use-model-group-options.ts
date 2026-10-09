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
import { useMemo } from 'react'

import { getEnabledModels } from '@/features/channels/api'
import { getGroups } from '@/features/users/api'
import { getUserGroups, getUserModels } from '@/lib/api'
import { sortGroupNames, useGroupDisplayOrder } from '@/lib/group-order'
import { requireServerSuccess } from '@/lib/server-error-message'

type Option = { label: string; value: string }

/**
 * Model and group choices for filter dropdowns. Admins see every enabled
 * model and group; users only the models and groups they can use. Groups
 * follow the admin-defined display order and exclude "auto".
 */
export function useModelGroupOptions(isAdmin: boolean): {
  modelOptions: Option[]
  groupOptions: Option[]
} {
  const { data: adminGroups } = useQuery({
    queryKey: ['groups'],
    queryFn: async () => requireServerSuccess(await getGroups()),
    enabled: isAdmin,
  })
  const { data: userGroups } = useQuery({
    queryKey: ['user-groups'],
    queryFn: async () => requireServerSuccess(await getUserGroups()),
    enabled: !isAdmin,
  })
  const { data: modelList } = useQuery({
    queryKey: ['log-filter-models', isAdmin],
    queryFn: async () =>
      requireServerSuccess(
        await (isAdmin ? getEnabledModels() : getUserModels())
      ),
  })
  const groupOrder = useGroupDisplayOrder()

  const modelOptions = useMemo(
    () =>
      [...new Set(Array.isArray(modelList?.data) ? modelList.data : [])]
        .sort()
        .map((model) => ({ label: model, value: model })),
    [modelList]
  )
  const groupOptions = useMemo(() => {
    const groups = isAdmin
      ? (adminGroups?.data ?? [])
      : Object.keys(userGroups?.data ?? {})
    return sortGroupNames(
      groups.filter((group) => group !== 'auto'),
      groupOrder
    ).map((group) => ({ label: group, value: group }))
  }, [isAdmin, adminGroups, userGroups, groupOrder])

  return { modelOptions, groupOptions }
}
