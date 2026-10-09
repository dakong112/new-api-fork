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

import { buildQueryParams } from '../filters'

const range = { start_timestamp: 100, end_timestamp: 200 }

describe('buildQueryParams', () => {
  it('sends only the conditions that are set', () => {
    const params = buildQueryParams(range, {
      time_granularity: 'day',
      model_name: 'gpt-4o',
      group: 'vip',
      token_id: 7,
      token_name: 'my key (#7)',
      channel_id: 3,
    })

    expect(params).toEqual({
      ...range,
      default_time: 'day',
      model_name: 'gpt-4o',
      group: 'vip',
      token_id: 7,
      channel_id: 3,
    })
  })

  it('omits empty conditions and never sends the key display name', () => {
    const params = buildQueryParams(range, {
      time_granularity: 'hour',
      model_name: '',
      group: '',
      token_id: undefined,
      token_name: 'stale label',
    })

    expect(params).toEqual({ ...range, default_time: 'hour' })
  })
})
