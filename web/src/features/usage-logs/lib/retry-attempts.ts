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
import type { PolicyEvent } from '@/features/system-settings/request-policies/api'

/** Fields of a usage log row this module reads. */
export type RetryLogSource = {
  id: number
  type: number
  channel: number
  channel_name?: string | null
  quota: number
  use_time: number
  other: string
}

export type RetryAttempt = {
  attempt: number
  channelId: number
  channelName: string
  group: string
  succeeded: boolean
  /** The stream broke mid-way but the request was still billed. */
  streamInterrupted?: boolean
  status?: number
  errorCode?: string
  message?: string
  /** null when the attempt has no end event, so the UI shows "—". */
  durationMs: number | null
  /** Charge of the final record for the succeeded attempt, otherwise 0. */
  quota: number
  /** Timeouts reach the upstream before failing, so it may have billed. */
  upstreamMayCharge: boolean
}

export type RetryProcess = {
  attempts: RetryAttempt[]
  /**
   * Outcome of the whole request. While no final record is known (still
   * loading, paginated out or missing) `succeeded`, `totalSeconds` and
   * `totalQuota` are null instead of guessed.
   */
  summary: {
    succeeded: boolean | null
    attemptCount: number
    totalSeconds: number | null
    totalQuota: number | null
  }
  /** Attempt number of the opened record; null when it is the final one. */
  currentAttempt: number | null
  /** Log id holding the final outcome, or null when it is not known. */
  finalLogId: number | null
}

const TIMEOUT_STATUSES = new Set([504, 524])
const LOG_TYPE_CONSUME = 2

function policyOf(log: RetryLogSource): PolicyEvent[] {
  try {
    const other = JSON.parse(log.other || '{}') as {
      admin_info?: { request_policy?: PolicyEvent[] }
    }
    return other.admin_info?.request_policy ?? []
  } catch {
    return []
  }
}

type Row = { log: RetryLogSource; events: PolicyEvent[] }

function longest(rows: Row[]): Row | undefined {
  return rows.reduce<Row | undefined>(
    (best, row) =>
      !best || row.events.length > best.events.length ? row : best,
    undefined
  )
}

/**
 * Rebuilds every attempt of one request from the routing events. A record is
 * final when it is a consume log or its events end with a "stop" decision;
 * only then are the outcome, total time and charge known. Otherwise the
 * attempts are listed from the longest event list available and the summary
 * outcome stays null. Returns null for a request that never retried, so its
 * details stay as they are.
 */
export function buildRetryProcess(
  opened: RetryLogSource,
  siblings: RetryLogSource[]
): RetryProcess | null {
  const logs = siblings.some((row) => row.id === opened.id)
    ? siblings
    : [opened, ...siblings]
  const rows = logs.map((log): Row => ({ log, events: policyOf(log) }))
  const isFinal = (row: Row) =>
    row.log.type === LOG_TYPE_CONSUME ||
    row.events.at(-1)?.decision.action === 'stop'
  const finals = rows.filter(isFinal)
  const final =
    longest(finals.filter((row) => row.log.type === LOG_TYPE_CONSUME)) ??
    longest(finals)
  const events = (final ?? longest(rows))?.events ?? []
  const attemptNumbers = [...new Set(events.map((e) => e.attempt))].filter(
    (n) => n > 0
  )
  if (attemptNumbers.length < 2) return null

  const names = new Map(
    rows.map((row) => [row.log.channel, row.log.channel_name ?? ''])
  )
  const finalConsumed = final?.log.type === LOG_TYPE_CONSUME
  const lastAttempt = attemptNumbers.at(-1)
  const attempts = attemptNumbers.map((attempt): RetryAttempt => {
    const own = events.filter((e) => e.attempt === attempt)
    const start = own.find((e) => e.decision.action === 'attempt')
    const failure = own.find((e) => e.decision.action === 'failure')
    const success = own.find((e) => e.decision.action === 'success')
    const streamStop = own.find(
      (e) =>
        e.decision.action === 'stop' &&
        e.decision.reason === 'stream_not_successful'
    )
    const interrupted =
      success === undefined &&
      failure === undefined &&
      streamStop !== undefined &&
      finalConsumed &&
      attempt === lastAttempt
    const isSuccess = success !== undefined || interrupted
    const end = failure ?? success ?? streamStop
    const channelId = start?.channel_id ?? end?.channel_id ?? 0
    return {
      attempt,
      channelId,
      channelName: names.get(channelId) ?? '',
      group: start?.group ?? '',
      succeeded: isSuccess,
      ...(interrupted ? { streamInterrupted: true } : {}),
      status: failure?.status,
      errorCode: failure?.error_code,
      message: failure?.message,
      durationMs:
        end && start ? Math.max(0, end.elapsed_ms - start.elapsed_ms) : null,
      quota: isSuccess && final ? final.log.quota : 0,
      upstreamMayCharge:
        failure !== undefined && TIMEOUT_STATUSES.has(failure.status ?? 0),
    }
  })

  let totalQuota: number | null = null
  if (final) totalQuota = finalConsumed ? final.log.quota : 0
  const openedEvents = rows.find((r) => r.log.id === opened.id)?.events ?? []
  return {
    attempts,
    summary: {
      succeeded: final ? finalConsumed : null,
      attemptCount: attempts.length,
      totalSeconds: final ? final.log.use_time : null,
      totalQuota,
    },
    currentAttempt:
      final?.log.id === opened.id
        ? null
        : (openedEvents.at(-1)?.attempt ?? null),
    finalLogId: final?.log.id ?? null,
  }
}
