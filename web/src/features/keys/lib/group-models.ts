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
import type { ApiKey, UserGroupModels } from '../types'

/** The models a group serves; a key without a group uses the user's group. */
export function getGroupModels(
  group: string | null | undefined,
  groupModels: UserGroupModels
): string[] {
  return groupModels.groups[group || groupModels.user_group] ?? []
}

/**
 * The models a key gets from /v1/models: its group's models, the union of
 * its own Auto groups when it picked them, narrowed by its model limits.
 */
export function getApiKeyModels(
  apiKey: Pick<
    ApiKey,
    'group' | 'auto_groups' | 'model_limits_enabled' | 'model_limits'
  >,
  groupModels: UserGroupModels
): string[] {
  let models = getGroupModels(apiKey.group, groupModels)
  if (apiKey.group === 'auto' && apiKey.auto_groups?.length) {
    models = [
      ...new Set(
        apiKey.auto_groups.flatMap((group) => groupModels.groups[group] ?? [])
      ),
    ].sort()
  }
  if (!apiKey.model_limits_enabled) return models
  const limits = new Set(
    (apiKey.model_limits || '').split(',').map((model) => model.trim())
  )
  return models.filter((model) => limits.has(model))
}
