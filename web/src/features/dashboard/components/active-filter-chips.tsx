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
import { X } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import type { DashboardFilters } from '@/features/dashboard/types'

type ActiveFilterChipsProps = {
  filters: DashboardFilters
  onChange: (filters: DashboardFilters) => void
}

/**
 * Shows the applied conditions (not the time range) as removable chips, so
 * the current view is obvious and each condition clears in one click.
 */
export function ActiveFilterChips(props: ActiveFilterChipsProps) {
  const { t } = useTranslation()
  const f = props.filters
  const chips: { key: string; label: string; clear: DashboardFilters }[] = []
  if (f.model_name) {
    chips.push({
      key: 'model',
      label: `${t('Model')}: ${f.model_name}`,
      clear: { model_name: '' },
    })
  }
  if (f.group) {
    chips.push({
      key: 'group',
      label: `${t('Group')}: ${f.group}`,
      clear: { group: '' },
    })
  }
  if (f.token_id) {
    chips.push({
      key: 'token',
      label: `${t('API Key')}: ${f.token_name || `#${f.token_id}`}`,
      clear: { token_id: undefined, token_name: '' },
    })
  }
  if (f.channel_id) {
    chips.push({
      key: 'channel',
      label: `${t('Channel ID')}: ${f.channel_id}`,
      clear: { channel_id: undefined },
    })
  }
  if (f.username) {
    chips.push({
      key: 'username',
      label: `${t('Username')}: ${f.username}`,
      clear: { username: '' },
    })
  }
  if (chips.length === 0) return null

  return (
    <div className='flex flex-wrap items-center gap-1.5'>
      {chips.map((chip) => (
        <span
          key={chip.key}
          className='bg-muted inline-flex max-w-full items-center gap-1 rounded-full py-0.5 pr-1 pl-2.5 text-xs'
        >
          <span className='truncate'>{chip.label}</span>
          <button
            type='button'
            className='hover:bg-background rounded-full p-0.5'
            aria-label={t('Remove filter {{name}}', { name: chip.label })}
            onClick={() => props.onChange({ ...f, ...chip.clear })}
          >
            <X className='h-3 w-3' aria-hidden='true' />
          </button>
        </span>
      ))}
      <Button
        variant='ghost'
        size='sm'
        className='h-6 px-2 text-xs'
        onClick={() =>
          props.onChange({
            ...f,
            model_name: '',
            group: '',
            token_id: undefined,
            token_name: '',
            channel_id: undefined,
            username: '',
          })
        }
      >
        {t('Clear all')}
      </Button>
    </div>
  )
}
