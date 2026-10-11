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
/** Vendor name used for models that have no vendor ("Other"). */
export const OTHER_VENDOR = ''

export type GroupVendor = { name: string; icon?: string }

export type GroupVendorIndex = {
  /** Vendors that own at least one group, in vendor order, Other last. */
  vendors: GroupVendor[]
  vendorsByGroup: Map<string, string[]>
}

type VendorSource = { id: number; name: string; icon?: string }
type ModelSource = { vendor_id?: number; enable_groups?: string[] }

/**
 * A group belongs to every vendor of its enabled models, so a group can sit
 * under several vendors and follows new models without manual upkeep.
 */
export function buildGroupVendorIndex(
  models: ModelSource[],
  vendors: VendorSource[]
): GroupVendorIndex {
  const nameById = new Map(vendors.map((v) => [v.id, v.name]))
  const byGroup = new Map<string, Set<string>>()
  for (const model of models) {
    const vendor =
      (model.vendor_id && nameById.get(model.vendor_id)) || OTHER_VENDOR
    for (const group of model.enable_groups ?? []) {
      if (!byGroup.has(group)) byGroup.set(group, new Set())
      byGroup.get(group)?.add(vendor)
    }
  }
  const used = new Set([...byGroup.values()].flatMap((set) => [...set]))
  const ordered: GroupVendor[] = vendors
    .filter((v) => used.has(v.name))
    .map((v) => ({ name: v.name, icon: v.icon }))
  if (used.has(OTHER_VENDOR)) ordered.push({ name: OTHER_VENDOR })
  const rank = new Map(ordered.map((v, i) => [v.name, i]))
  const vendorsByGroup = new Map(
    [...byGroup].map(([group, set]) => [
      group,
      [...set].sort((a, b) => (rank.get(a) ?? 0) - (rank.get(b) ?? 0)),
    ])
  )
  return { vendors: ordered, vendorsByGroup }
}

/**
 * Narrows groups to one vendor. `auto` spans every vendor, and `pinned` (the
 * current value) stays so a selected group never disappears from its picker.
 */
export function filterGroupsByVendor<T>(
  groups: T[],
  getName: (group: T) => string,
  vendor: string | null,
  index: GroupVendorIndex,
  pinned?: string
): T[] {
  if (vendor === null) return groups
  return groups.filter((group) => {
    const name = getName(group)
    return (
      name === 'auto' ||
      name === pinned ||
      (index.vendorsByGroup.get(name) ?? []).includes(vendor)
    )
  })
}

/** A group is usable once it has an enabled model; auto always is. */
export function isGroupAvailable(name: string, index: GroupVendorIndex) {
  return name === 'auto' || index.vendorsByGroup.has(name)
}
