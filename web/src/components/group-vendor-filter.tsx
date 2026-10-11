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
import { useTranslation } from 'react-i18next'

import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { OTHER_VENDOR, type GroupVendor } from '@/lib/group-vendors'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

// '' means Other, so "All" needs its own item value. The toggle also treats an
// empty item value as unset, so Other gets a sentinel too.
const ALL = '__all__'
const OTHER = '__other__'
const toItem = (name: string) => (name === OTHER_VENDOR ? OTHER : name)

type GroupVendorFilterProps = {
  vendors: GroupVendor[]
  /** Selected vendor name; null means all vendors. */
  value: string | null
  onChange: (value: string | null) => void
  variant?: 'cards' | 'chips'
  className?: string
}

export function GroupVendorFilter(props: GroupVendorFilterProps) {
  const { t } = useTranslation()
  // One vendor (or none) filters nothing.
  if (props.vendors.length < 2) return null
  const cards = props.variant === 'cards'
  const current = props.value === null ? ALL : toItem(props.value)
  const options = [
    { key: 'all', value: ALL, label: t('All'), icon: undefined },
    ...props.vendors.map((v) => ({
      key: `v:${v.name}`,
      value: toItem(v.name),
      label: v.name === OTHER_VENDOR ? t('Other') : v.name,
      icon: v.icon,
    })),
  ]
  return (
    <ToggleGroup
      value={[current]}
      onValueChange={(values) => {
        // Clicking the selected item empties the list; keep the selection.
        const next = values.find((value) => value !== current)
        if (next === undefined) return
        if (next === ALL) props.onChange(null)
        else props.onChange(next === OTHER ? OTHER_VENDOR : next)
      }}
      aria-label={t('Vendor')}
      variant='outline'
      size={cards ? 'default' : 'sm'}
      spacing={cards ? 2 : 1}
      className={cn(
        cards
          ? 'grid w-full grid-cols-3 gap-2 sm:grid-cols-4'
          : 'h-auto flex-wrap',
        props.className
      )}
    >
      {options.map((option) => (
        <ToggleGroupItem
          key={option.key}
          value={option.value}
          className={cn(cards && 'h-auto w-full flex-col gap-1.5 p-3')}
        >
          {option.icon ? (
            <span aria-hidden='true'>
              {getLobeIcon(option.icon, cards ? 24 : 14)}
            </span>
          ) : null}
          <span className='truncate'>{option.label}</span>
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  )
}
