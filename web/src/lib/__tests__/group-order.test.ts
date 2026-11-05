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
import { renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { sortGroupNames, useGroupDisplayOrder } from '../group-order'

const statusMock = vi.hoisted(() => ({
  current: { status: null as Record<string, unknown> | null },
}))
vi.mock('@/hooks/use-status', () => ({
  useStatus: () => statusMock.current,
}))

describe('sortGroupNames', () => {
  it('puts auto first, then the configured order, then the rest alphabetically', () => {
    expect(
      sortGroupNames(
        ['default', 'svip', 'auto', 'vip', 'beta'],
        ['vip', 'default']
      )
    ).toEqual(['auto', 'vip', 'default', 'beta', 'svip'])
  })

  it('falls back to alphabetical order when nothing is configured', () => {
    expect(sortGroupNames(['vip', 'default', 'svip'], [])).toEqual([
      'default',
      'svip',
      'vip',
    ])
  })

  it('ignores configured names that are not present', () => {
    expect(sortGroupNames(['default', 'vip'], ['gone', 'vip'])).toEqual([
      'vip',
      'default',
    ])
  })

  it('does not mutate the input', () => {
    const names = ['vip', 'default']
    sortGroupNames(names, ['default'])
    expect(names).toEqual(['vip', 'default'])
  })
})

describe('useGroupDisplayOrder', () => {
  it.each([
    ['configured', { group_display_order: ['vip', 1, 'default'] }],
    ['unset', {}],
    ['missing status', null],
  ])(
    'returns the same array across re-renders when the status is %s',
    (_label, status) => {
      statusMock.current = { status }
      const { result, rerender } = renderHook(() => useGroupDisplayOrder())
      const first = result.current
      rerender()
      expect(result.current).toBe(first)
    }
  )

  it('keeps only string group names from the status order', () => {
    statusMock.current = {
      status: { group_display_order: ['vip', 1, 'default'] },
    }
    const { result } = renderHook(() => useGroupDisplayOrder())
    expect(result.current).toEqual(['vip', 'default'])
  })
})
