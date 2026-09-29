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
import { useQuery } from '@tanstack/react-query'
import { FlaskConical, Loader2, Play } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { TitledCard } from '@/components/ui/titled-card'
import { getServerErrorMessage } from '@/lib/server-error-message'

import { getAllChannels, probeChannelCost } from './api'
import {
  type ProbeRow,
  ProbeResultsTable,
} from './components/probe-results-table'
import { type ProbeTask, ProbeTaskList } from './components/probe-task-list'

const PREFIX_SIZES = [
  { value: 6000, label: '6k' },
  { value: 30000, label: '30k' },
  { value: 100000, label: '100k' },
]
const REASONING_EFFORTS = ['low', 'medium', 'high', 'xhigh']

export function ChannelCostProbe() {
  const { t } = useTranslation()
  const channelsQuery = useQuery({
    queryKey: ['channel-cost-probe', 'channels'],
    queryFn: getAllChannels,
  })
  const channels = channelsQuery.data ?? []

  const [tasks, setTasks] = useState<ProbeTask[]>([
    { key: crypto.randomUUID(), channelId: null, models: [] },
  ])
  const [rounds, setRounds] = useState('3')
  const [mode, setMode] = useState<'cache' | 'candy'>('candy')
  const [prefixTokens, setPrefixTokens] = useState(100000)
  const [reasoningEffort, setReasoningEffort] = useState('')
  const [rows, setRows] = useState<ProbeRow[]>([])
  const [running, setRunning] = useState(false)

  const handleRun = async () => {
    const parsed = Math.trunc(Number(rounds))
    const roundCount = Number.isFinite(parsed)
      ? Math.min(Math.max(parsed, 1), 10)
      : 3
    const jobs = tasks
      .filter((task) => task.channelId !== null)
      .flatMap((task) =>
        task.models.map((model) => ({
          key: `${task.key}:${model}`,
          channelId: task.channelId as number,
          channelName:
            channels.find((c) => c.id === task.channelId)?.name ??
            `#${task.channelId}`,
          model,
        }))
      )
    if (jobs.length === 0) return

    setRows(
      jobs.map((job) => ({
        key: job.key,
        channelName: job.channelName,
        model: job.model,
        status: 'running',
      }))
    )
    setRunning(true)

    const patchRow = (key: string, patch: (row: ProbeRow) => ProbeRow) =>
      setRows((prev) => prev.map((row) => (row.key === key ? patch(row) : row)))

    // Channels run in parallel; models within a channel run one after another
    // so their requests do not interleave on the same key.
    const byChannel = new Map<number, typeof jobs>()
    for (const job of jobs) {
      byChannel.set(job.channelId, [
        ...(byChannel.get(job.channelId) ?? []),
        job,
      ])
    }
    await Promise.all(
      [...byChannel.values()].map(async (channelJobs) => {
        for (const job of channelJobs) {
          // One call per round so each result shows up as soon as it lands;
          // the shared run id keeps the prefix identical across calls.
          const runId = crypto.randomUUID()
          try {
            for (let round = 1; round <= roundCount; round++) {
              const result = await probeChannelCost({
                channel_id: job.channelId,
                model: job.model,
                rounds: 1,
                mode,
                prefix_tokens: prefixTokens,
                reasoning_effort: reasoningEffort,
                run_id: runId,
                start_round: round,
              })
              patchRow(job.key, (row) => ({
                ...row,
                result: {
                  ...result,
                  rounds: [...(row.result?.rounds ?? []), ...result.rounds],
                },
              }))
            }
            patchRow(job.key, (row) => ({ ...row, status: 'done' }))
          } catch (error) {
            patchRow(job.key, (row) => ({
              ...row,
              status: 'error',
              error: getServerErrorMessage(error, t('Test failed')),
            }))
          }
        }
      })
    )
    setRunning(false)
  }

  const canRun =
    !running &&
    tasks.some((task) => task.channelId !== null && task.models.length > 0)

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Supplier Cache Test')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-7xl min-w-0 flex-col gap-4'>
          <TitledCard
            title={t('Suppliers and models')}
            description={t(
              'Sends a prompt with the same long prefix several times in a row. From round 2 on, a working cache shows most input under Cache Read. The candy question also checks the answer (21) and reasoning tokens to spot downgraded models. Tests spend real balance; a 100k prefix costs noticeably more.'
            )}
            icon={<FlaskConical className='h-4 w-4' />}
            disableHoverEffect
          >
            <div className='flex flex-col gap-4'>
              <ProbeTaskList
                tasks={tasks}
                channels={channels}
                disabled={running}
                onChange={setTasks}
              />
              <div className='flex flex-wrap items-end gap-4'>
                <div className='flex flex-col gap-1.5'>
                  <Label htmlFor='probe-rounds'>{t('Rounds per model')}</Label>
                  <Input
                    id='probe-rounds'
                    inputMode='numeric'
                    className='w-28'
                    value={rounds}
                    disabled={running}
                    onChange={(e) => setRounds(e.target.value)}
                  />
                </div>
                <div className='flex flex-col gap-1.5'>
                  <Label htmlFor='probe-mode'>{t('Test question')}</Label>
                  <NativeSelect
                    id='probe-mode'
                    value={mode}
                    disabled={running}
                    onChange={(e) =>
                      setMode(e.target.value as 'cache' | 'candy')
                    }
                  >
                    <NativeSelectOption value='candy'>
                      {t('Candy question (quality + cache)')}
                    </NativeSelectOption>
                    <NativeSelectOption value='cache'>
                      {t('Short reply (cache only)')}
                    </NativeSelectOption>
                  </NativeSelect>
                </div>
                <div className='flex flex-col gap-1.5'>
                  <Label htmlFor='probe-prefix'>{t('Context length')}</Label>
                  <NativeSelect
                    id='probe-prefix'
                    value={String(prefixTokens)}
                    disabled={running}
                    onChange={(e) => setPrefixTokens(Number(e.target.value))}
                  >
                    {PREFIX_SIZES.map((size) => (
                      <NativeSelectOption key={size.value} value={size.value}>
                        {size.label}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                </div>
                <div className='flex flex-col gap-1.5'>
                  <Label htmlFor='probe-effort'>{t('Reasoning Effort')}</Label>
                  <NativeSelect
                    id='probe-effort'
                    value={reasoningEffort}
                    disabled={running}
                    onChange={(e) => setReasoningEffort(e.target.value)}
                  >
                    <NativeSelectOption value=''>
                      {t('Model default')}
                    </NativeSelectOption>
                    {REASONING_EFFORTS.map((effort) => (
                      <NativeSelectOption key={effort} value={effort}>
                        {effort}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                </div>
                <Button disabled={!canRun} onClick={handleRun}>
                  {running ? (
                    <Loader2
                      className='mr-1 h-4 w-4 animate-spin'
                      aria-hidden='true'
                    />
                  ) : (
                    <Play className='mr-1 h-4 w-4' aria-hidden='true' />
                  )}
                  {t('Start test')}
                </Button>
              </div>
            </div>
          </TitledCard>

          {rows.length > 0 && (
            <TitledCard
              title={t('Result')}
              disableHoverEffect
              className='min-w-0'
            >
              <ProbeResultsTable rows={rows} />
            </TitledCard>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
