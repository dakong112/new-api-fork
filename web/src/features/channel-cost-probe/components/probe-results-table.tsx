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
import { Loader2 } from 'lucide-react'
import { Fragment } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import type { ProbeResult } from '../types'

export type ProbeRow = {
  key: string
  channelName: string
  model: string
  status: 'running' | 'done' | 'error'
  error?: string
  result?: ProbeResult
}

type ProbeResultsTableProps = {
  rows: ProbeRow[]
}

export function ProbeResultsTable(props: ProbeResultsTableProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('Supplier')}</TableHead>
          <TableHead>{t('Model')}</TableHead>
          <TableHead>{t('Round')}</TableHead>
          <TableHead>{t('Status')}</TableHead>
          <TableHead>{t('Uncached input')}</TableHead>
          <TableHead>{t('Cache Read')}</TableHead>
          <TableHead>{t('Cache Write')}</TableHead>
          <TableHead>{t('Output')}</TableHead>
          <TableHead>{t('Reasoning tokens')}</TableHead>
          <TableHead>{t('Answer')}</TableHead>
          <TableHead>{t('Time')}</TableHead>
          <TableHead>{t('Raw usage / error')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.rows.map((row) => {
          const label = (
            <>
              <TableCell className='max-w-40 truncate' title={row.channelName}>
                {row.channelName}
              </TableCell>
              <TableCell className='font-mono text-xs'>
                {row.model}
                {row.result && row.result.upstream_model !== row.model && (
                  <div className='text-muted-foreground'>
                    → {row.result.upstream_model}
                  </div>
                )}
              </TableCell>
            </>
          )
          const rounds = row.result?.rounds ?? []
          // Trailing row for the round in flight, or the error that stopped the run.
          const statusRow =
            row.status === 'done' ? null : (
              <TableRow>
                {label}
                <TableCell className='whitespace-nowrap'>
                  {row.status === 'running' &&
                    t('Round {{n}}', { n: rounds.length + 1 })}
                </TableCell>
                <TableCell colSpan={9}>
                  {row.status === 'running' ? (
                    <Loader2
                      className='h-4 w-4 animate-spin'
                      aria-label={t('Testing...')}
                    />
                  ) : (
                    <span className='text-destructive text-xs'>
                      {row.error}
                    </span>
                  )}
                </TableCell>
              </TableRow>
            )
          return (
            <Fragment key={row.key}>
              {rounds.map((round, i) => (
                // oxlint-disable-next-line react/no-array-index-key -- rounds are an ordered, immutable list
                <TableRow key={i}>
                  {label}
                  <TableCell className='whitespace-nowrap'>
                    {t('Round {{n}}', { n: i + 1 })}
                  </TableCell>
                  <TableCell>
                    <StatusBadge
                      label={round.error ? t('Failed') : t('Success')}
                      variant={round.error ? 'danger' : 'success'}
                      copyable={false}
                    />
                  </TableCell>
                  <TableCell>
                    {formatNumber(round.input_tokens, locale)}
                  </TableCell>
                  <TableCell className='font-semibold'>
                    {formatNumber(round.cache_read_tokens, locale)}
                  </TableCell>
                  <TableCell>
                    {formatNumber(round.cache_write_tokens, locale)}
                  </TableCell>
                  <TableCell>
                    {formatNumber(round.output_tokens, locale)}
                  </TableCell>
                  <TableCell className='font-semibold'>
                    {formatNumber(round.reasoning_tokens, locale)}
                  </TableCell>
                  <TableCell className='max-w-60 text-xs whitespace-normal'>
                    {round.correct !== undefined && (
                      <StatusBadge
                        label={round.correct ? t('Correct (21)') : t('Wrong')}
                        variant={round.correct ? 'success' : 'danger'}
                        copyable={false}
                      />
                    )}
                    {round.answer && (
                      <p
                        className='text-muted-foreground mt-1 line-clamp-3'
                        title={round.answer}
                      >
                        {round.answer}
                      </p>
                    )}
                  </TableCell>
                  <TableCell className='whitespace-nowrap'>
                    {formatNumber(round.latency_ms / 1000, locale)}s
                  </TableCell>
                  <TableCell
                    className={
                      round.error
                        ? 'text-destructive max-w-96 text-xs break-words whitespace-normal'
                        : 'max-w-96 font-mono text-xs break-all whitespace-normal'
                    }
                  >
                    {round.error || round.raw_usage}
                  </TableCell>
                </TableRow>
              ))}
              {statusRow}
            </Fragment>
          )
        })}
      </TableBody>
    </Table>
  )
}
