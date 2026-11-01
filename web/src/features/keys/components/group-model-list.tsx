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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { cn } from '@/lib/utils'

/** Lists longer than this get a search box. */
const SEARCH_THRESHOLD = 12

type GroupModelListProps = {
  models: string[]
  /** Shown above the list, e.g. which group the models belong to. */
  title?: string
  /** Classes for the scrolling model area, e.g. a max height. */
  listClassName?: string
  className?: string
}

/**
 * The model names of a group as buttons that copy the name, so users paste
 * exact names into their clients instead of typing them.
 */
export function GroupModelList(props: GroupModelListProps) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const [search, setSearch] = useState('')
  const visibleModels = useMemo(() => {
    const query = search.trim().toLowerCase()
    if (!query) return props.models
    return props.models.filter((model) => model.toLowerCase().includes(query))
  }, [props.models, search])

  return (
    <div className={cn('flex min-w-0 flex-col gap-2', props.className)}>
      <div className='flex items-center justify-between gap-2'>
        <div className='min-w-0'>
          <p className='truncate text-xs font-medium'>
            {props.title ??
              t('Available models ({{count}})', {
                count: props.models.length,
              })}
          </p>
          {props.models.length > 0 && (
            <p className='text-muted-foreground text-[11px]'>
              {t('Click a model name to copy it')}
            </p>
          )}
        </div>
        {props.models.length > 1 && (
          <CopyButton
            value={props.models.join(',')}
            variant='outline'
            size='sm'
            className='h-7 text-xs'
            iconClassName='size-3.5'
            aria-label={t('Copy all model names')}
          >
            {t('Copy all')}
          </CopyButton>
        )}
      </div>
      {props.models.length > SEARCH_THRESHOLD && (
        <Input
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={t('Search models')}
          aria-label={t('Search models')}
          className='h-8 text-xs'
        />
      )}
      {props.models.length === 0 ? (
        <p className='text-muted-foreground text-xs'>
          {t('No models are available in this group yet')}
        </p>
      ) : (
        <ul
          aria-label={t('Models')}
          className={cn(
            'flex flex-wrap content-start gap-1.5 overflow-y-auto',
            props.listClassName
          )}
        >
          {visibleModels.map((model) => (
            <li key={model} className='max-w-full'>
              <Button
                variant='outline'
                size='xs'
                className='max-w-full cursor-copy font-mono font-normal'
                title={model}
                aria-label={t('Copy model name {{model}}', { model })}
                onClick={() => void copyToClipboard(model)}
              >
                <span className='truncate'>{model}</span>
              </Button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
