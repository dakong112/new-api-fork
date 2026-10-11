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

import {
  buildGroupVendorIndex,
  filterGroupsByVendor,
  isGroupAvailable,
  OTHER_VENDOR,
} from '../group-vendors'

const vendors = [
  { id: 1, name: 'DeepSeek', icon: 'DeepSeek.Color' },
  { id: 2, name: '智谱', icon: 'Zhipu.Color' },
  { id: 3, name: 'Unused', icon: 'X' },
]
const models = [
  { vendor_id: 1, enable_groups: ['mix', 'ds'] },
  { vendor_id: 2, enable_groups: ['mix'] },
  { enable_groups: ['misc'] },
]

describe('buildGroupVendorIndex', () => {
  it('lists only vendors with groups, in vendor order, with Other last', () => {
    const index = buildGroupVendorIndex(models, vendors)
    expect(index.vendors.map((v) => v.name)).toEqual([
      'DeepSeek',
      '智谱',
      OTHER_VENDOR,
    ])
    expect(index.vendors[0].icon).toBe('DeepSeek.Color')
  })

  it('maps a group to every vendor of its models', () => {
    const index = buildGroupVendorIndex(models, vendors)
    expect(index.vendorsByGroup.get('mix')).toEqual(['DeepSeek', '智谱'])
    expect(index.vendorsByGroup.get('misc')).toEqual([OTHER_VENDOR])
  })
})

describe('filterGroupsByVendor', () => {
  const index = buildGroupVendorIndex(models, vendors)
  const groups = ['auto', 'ds', 'mix', 'misc']

  it('keeps every group when no vendor is selected', () => {
    expect(filterGroupsByVendor(groups, (g) => g, null, index)).toEqual(groups)
  })

  it('keeps the vendor groups, auto, and the pinned value', () => {
    expect(
      filterGroupsByVendor(groups, (g) => g, '智谱', index, 'misc')
    ).toEqual(['auto', 'mix', 'misc'])
  })

  it('selects groups whose models have no vendor under Other', () => {
    expect(filterGroupsByVendor(groups, (g) => g, OTHER_VENDOR, index)).toEqual(
      ['auto', 'misc']
    )
  })
})

describe('isGroupAvailable', () => {
  const index = buildGroupVendorIndex(models, vendors)

  it('treats a group with an enabled model and auto as available', () => {
    expect(isGroupAvailable('mix', index)).toBe(true)
    expect(isGroupAvailable('auto', index)).toBe(true)
  })

  it('treats a group without enabled models as unavailable', () => {
    expect(isGroupAvailable('empty', index)).toBe(false)
  })
})
