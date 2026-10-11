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
import type { TFunction } from 'i18next'
import { ChevronDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import type { PolicyEvent } from '@/features/system-settings/request-policies/api'
import { PolicyDecisionRecord } from '@/features/system-settings/request-policies/decision-record'
import { formatLogQuota, formatUseTime } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { RetryAttempt, RetryProcess } from '../../lib/retry-attempts'

type RetryProcessProps = {
  process: RetryProcess
  events: PolicyEvent[]
  onViewLog: (logId: number) => void
}

const DASH = '—'

function failureLabel(t: TFunction, status: number | undefined): string {
  const code = status ?? 0
  if (code === 524) return t('Upstream timeout')
  if (code === 504) return t('Gateway timeout')
  if (code >= 500) return t('Upstream error')
  if (code >= 400) return t('Request rejected')
  return t('Failed')
}

function outcome(
  t: TFunction,
  succeeded: boolean | null
): { label: string; variant: StatusVariant } {
  if (succeeded === null) {
    return { label: t('Final result unknown'), variant: 'neutral' }
  }
  if (succeeded) return { label: t('Succeeded in the end'), variant: 'success' }
  return { label: t('Failed in the end'), variant: 'danger' }
}

function summaryCharge(t: TFunction, process: RetryProcess): string | null {
  const quota = process.summary.totalQuota
  if (process.summary.succeeded === null || quota === null) return null
  if (process.summary.succeeded) {
    return t('Charged {{amount}}', { amount: formatLogQuota(quota) })
  }
  return t('Not charged (fully refunded)')
}

function Banner(props: RetryProcessProps) {
  const { t } = useTranslation()
  const current = props.process.currentAttempt
  const finalLogId = props.process.finalLogId
  if (current === null) return null
  if (finalLogId === null) {
    return (
      <p className='text-muted-foreground rounded-md border px-3 py-2 text-sm'>
        {t('The final record of this request was not found')}
      </p>
    )
  }
  const winner = props.process.attempts.find((a) => a.succeeded)
  const text = props.process.summary.succeeded
    ? t(
        'This is attempt {{attempt}} of the request. It then succeeded on #{{channel}}.',
        { attempt: current, channel: winner?.channelId }
      )
    : t(
        'This is attempt {{attempt}} of the request. It failed after all retries.',
        { attempt: current }
      )
  return (
    <div className='bg-muted/40 flex flex-wrap items-center justify-between gap-2 rounded-md border px-3 py-2 text-sm'>
      <span>{text}</span>
      <Button
        size='sm'
        variant='outline'
        onClick={() => props.onViewLog(finalLogId)}
      >
        {t('View final record')}
      </Button>
    </div>
  )
}

function AttemptItem(props: { attempt: RetryAttempt; current: boolean }) {
  const { t } = useTranslation()
  const a = props.attempt
  const duration =
    a.durationMs === null ? DASH : formatUseTime(a.durationMs / 1000)
  const charge = a.succeeded
    ? t('Charged {{amount}}', { amount: formatLogQuota(a.quota) })
    : t('No charge for failed attempts')
  const raw = [a.status, a.errorCode].filter(Boolean).join(' ')
  return (
    <li
      aria-current={props.current ? 'step' : undefined}
      className={cn(
        'rounded-md border px-3 py-2 text-sm',
        props.current && 'border-primary bg-primary/5'
      )}
    >
      <div className='flex flex-wrap items-center gap-x-2 gap-y-1'>
        <span className='text-muted-foreground tabular-nums'>{a.attempt}.</span>
        <span className='font-medium'>
          #{a.channelId} {a.channelName}
        </span>
        <StatusBadge
          copyable={false}
          variant={a.succeeded ? 'success' : 'danger'}
          label={a.succeeded ? t('Success') : failureLabel(t, a.status)}
        />
        {raw ? (
          <span className='text-muted-foreground font-mono text-xs'>{raw}</span>
        ) : null}
      </div>
      <p className='text-muted-foreground mt-1 text-xs'>
        {t('Duration')} {duration} · {charge}
      </p>
      {a.message ? (
        <p className='mt-1 truncate text-xs' title={a.message}>
          {a.message}
        </p>
      ) : null}
      {a.upstreamMayCharge ? (
        <p className='text-warning mt-1 text-xs'>
          {t('Timed out after reaching the upstream, which may have billed it')}
        </p>
      ) : null}
      {a.streamInterrupted ? (
        <p className='text-warning mt-1 text-xs'>
          {t('Stream interrupted midway; charged for the output delivered')}
        </p>
      ) : null}
    </li>
  )
}

/** Admin view of every attempt of one request: outcome, banner, timeline. */
export function RetryProcessView(props: RetryProcessProps) {
  const { t } = useTranslation()
  const summary = props.process.summary
  const result = outcome(t, summary.succeeded)
  const charge = summaryCharge(t, props.process)
  const total =
    summary.totalSeconds === null ? DASH : formatUseTime(summary.totalSeconds)
  return (
    <div className='flex flex-col gap-3'>
      <div
        role='status'
        className='flex flex-wrap items-center gap-x-3 gap-y-1 text-sm'
      >
        <StatusBadge
          copyable={false}
          variant={result.variant}
          label={result.label}
        />
        <span>{t('{{count}} attempts', { count: summary.attemptCount })}</span>
        <span>{t('Total time {{time}}', { time: total })}</span>
        {charge ? <span>{charge}</span> : null}
      </div>
      <Banner {...props} />
      <ol aria-label={t('Retry process')} className='flex flex-col gap-2'>
        {props.process.attempts.map((attempt) => (
          <AttemptItem
            key={attempt.attempt}
            attempt={attempt}
            current={attempt.attempt === props.process.currentAttempt}
          />
        ))}
      </ol>
      <Collapsible>
        <CollapsibleTrigger
          render={
            <Button variant='ghost' size='sm' className='group w-fit'>
              {t('Routing decision details')}
              <ChevronDown
                aria-hidden='true'
                className='size-4 transition-transform group-data-[panel-open]:rotate-180'
              />
            </Button>
          }
        />
        <CollapsibleContent className='pt-2'>
          <PolicyDecisionRecord events={props.events} />
        </CollapsibleContent>
      </Collapsible>
    </div>
  )
}
