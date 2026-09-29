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
import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { MultiSelect } from '@/components/multi-select'
import { Button } from '@/components/ui/button'
import { Combobox } from '@/components/ui/combobox'
import type { Channel } from '@/features/channels/types'

export type ProbeTask = {
  key: string
  channelId: number | null
  models: string[]
}

type ProbeTaskListProps = {
  tasks: ProbeTask[]
  channels: Channel[]
  disabled: boolean
  onChange: (tasks: ProbeTask[]) => void
}

export function ProbeTaskList(props: ProbeTaskListProps) {
  const { t } = useTranslation()
  const channelOptions = props.channels.map((channel) => ({
    value: String(channel.id),
    label: `#${channel.id} ${channel.name}`,
  }))

  const update = (key: string, patch: Partial<ProbeTask>) =>
    props.onChange(
      props.tasks.map((task) =>
        task.key === key ? { ...task, ...patch } : task
      )
    )

  return (
    <div className='flex flex-col gap-3'>
      {props.tasks.map((task) => {
        const channel = props.channels.find((c) => c.id === task.channelId)
        const modelOptions = (channel?.models ?? '')
          .split(',')
          .map((m) => m.trim())
          .filter(Boolean)
          .map((m) => ({ value: m, label: m }))
        return (
          <div
            key={task.key}
            className='grid gap-2 rounded-lg border p-3 md:grid-cols-[minmax(0,14rem)_minmax(0,1fr)_auto] md:items-center'
          >
            <Combobox
              aria-label={t('Supplier channel')}
              options={channelOptions}
              value={task.channelId === null ? '' : String(task.channelId)}
              onValueChange={(value) =>
                update(task.key, {
                  channelId: value ? Number(value) : null,
                  models: [],
                })
              }
              placeholder={t('Select channel')}
              emptyText={t('No channels found')}
              disabled={props.disabled}
            />
            <MultiSelect
              options={modelOptions}
              selected={task.models}
              onChange={(models) => update(task.key, { models })}
              placeholder={t('Select models')}
              disabled={props.disabled || !channel}
              allowCreate
            />
            <Button
              variant='ghost'
              size='icon'
              aria-label={t('Remove')}
              disabled={props.disabled || props.tasks.length === 1}
              onClick={() =>
                props.onChange(props.tasks.filter((x) => x.key !== task.key))
              }
            >
              <Trash2 className='h-4 w-4' aria-hidden='true' />
            </Button>
          </div>
        )
      })}
      <Button
        variant='outline'
        className='self-start'
        disabled={props.disabled}
        onClick={() =>
          props.onChange([
            ...props.tasks,
            { key: crypto.randomUUID(), channelId: null, models: [] },
          ])
        }
      >
        <Plus className='mr-1 h-4 w-4' aria-hidden='true' />
        {t('Add supplier')}
      </Button>
    </div>
  )
}
