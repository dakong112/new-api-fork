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

import { OTHER_VENDOR, type GroupVendor } from '@/lib/group-vendors'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

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
  const options = [
    { key: 'all', value: null, label: t('All'), icon: undefined },
    ...props.vendors.map((v) => ({
      key: v.name || 'other',
      value: v.name,
      label: v.name === OTHER_VENDOR ? t('Other') : v.name,
      icon: v.icon,
    })),
  ]
  return (
    <div
      role='radiogroup'
      aria-label={t('Vendor')}
      className={cn(
        cards
          ? 'grid grid-cols-3 gap-2 sm:grid-cols-4'
          : 'flex flex-wrap gap-1.5',
        props.className
      )}
    >
      {options.map((option) => {
        const checked = props.value === option.value
        return (
          <button
            key={option.key}
            type='button'
            role='radio'
            aria-checked={checked}
            onClick={() => props.onChange(option.value)}
            className={cn(
              'border transition-colors',
              cards
                ? 'flex flex-col items-center gap-1.5 rounded-lg p-3 text-sm font-medium'
                : 'flex items-center gap-1 rounded-md px-2 py-1 text-xs',
              checked
                ? 'border-primary bg-primary/5 text-foreground'
                : 'text-muted-foreground hover:bg-accent hover:text-foreground'
            )}
          >
            {option.icon ? (
              <span aria-hidden='true'>
                {getLobeIcon(option.icon, cards ? 24 : 14)}
              </span>
            ) : null}
            <span className='truncate'>{option.label}</span>
          </button>
        )
      })}
    </div>
  )
}
