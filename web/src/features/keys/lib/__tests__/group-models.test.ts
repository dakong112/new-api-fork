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
import { describe, expect, test } from 'vitest'

import { getApiKeyModels } from '../group-models'

const groupModels = {
  user_group: 'default',
  groups: {
    default: ['glm-5'],
    vip: ['deepseek-v4', 'gpt-5'],
    pro: ['claude-sonnet', 'gpt-5'],
    auto: ['deepseek-v4', 'glm-5', 'gpt-5'],
  },
}

const unrestricted = {
  auto_groups: [],
  model_limits_enabled: false,
  model_limits: '',
}

describe('getApiKeyModels', () => {
  test('a key gets the models of its group', () => {
    expect(
      getApiKeyModels({ ...unrestricted, group: 'vip' }, groupModels)
    ).toEqual(['deepseek-v4', 'gpt-5'])
  })

  test('a key without a group gets the models of the user group', () => {
    expect(
      getApiKeyModels({ ...unrestricted, group: '' }, groupModels)
    ).toEqual(['glm-5'])
  })

  test('an Auto key with its own Auto groups gets their combined models', () => {
    expect(
      getApiKeyModels(
        { ...unrestricted, group: 'auto', auto_groups: ['pro', 'vip'] },
        groupModels
      )
    ).toEqual(['claude-sonnet', 'deepseek-v4', 'gpt-5'])
  })

  test('an Auto key without its own Auto groups gets the global Auto models', () => {
    expect(
      getApiKeyModels({ ...unrestricted, group: 'auto' }, groupModels)
    ).toEqual(['deepseek-v4', 'glm-5', 'gpt-5'])
  })

  test('model limits keep only the allowed models the group serves', () => {
    expect(
      getApiKeyModels(
        {
          ...unrestricted,
          group: 'vip',
          model_limits_enabled: true,
          model_limits: 'gpt-5,claude-sonnet',
        },
        groupModels
      )
    ).toEqual(['gpt-5'])
  })

  test('a group the user cannot select gets no models', () => {
    expect(
      getApiKeyModels({ ...unrestricted, group: 'retired' }, groupModels)
    ).toEqual([])
  })
})
