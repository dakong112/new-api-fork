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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ArrowLeft, ArrowRightLeft, Plus, Wallet } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable, StaticRowActions } from '@/components/data-table'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { RedemptionCard } from '@/features/wallet/components/redemption-card'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatQuota } from '@/lib/format'

import {
  getTeam,
  getTeamUsage,
  redeemToTeam,
  removeTeamMember,
  type TeamMember,
  type TeamUsageRow,
} from '../api'
import { MemberDialog } from './member-dialog'
import { TeamRoleBadge } from './team-role-badge'
import { TeamTransferDialog } from './transfer-dialog'

export function TeamDetail(props: { teamId: number }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const queryClient = useQueryClient()
  const [memberDialog, setMemberDialog] = useState<{
    open: boolean
    member?: TeamMember
  }>({ open: false })
  const [removing, setRemoving] = useState<TeamMember | null>(null)
  const [transferOpen, setTransferOpen] = useState(false)
  const [code, setCode] = useState('')

  const teamQuery = useQuery({
    queryKey: ['team', props.teamId],
    queryFn: () => getTeam(props.teamId),
  })
  const detail = teamQuery.data
  const callerIsOwner = detail?.role === 'owner'
  const canManage = callerIsOwner || detail?.role === 'admin'
  const usageQuery = useQuery({
    queryKey: ['team', props.teamId, 'usage'],
    queryFn: () => getTeamUsage(props.teamId),
    enabled: canManage,
  })

  const removeMutation = useMutation({
    mutationFn: (member: TeamMember) =>
      removeTeamMember(props.teamId, member.user_id),
    onSuccess: () => {
      setRemoving(null)
      void queryClient.invalidateQueries({ queryKey: ['team', props.teamId] })
    },
  })
  const redeemMutation = useMutation({
    mutationFn: () => redeemToTeam(props.teamId, code.trim()),
    onSuccess: (quota) => {
      toast.success(
        t('Redemption successful! Added: {{quota}}', {
          quota: formatQuota(quota),
        })
      )
      setCode('')
      void queryClient.invalidateQueries({ queryKey: ['team', props.teamId] })
      void queryClient.invalidateQueries({ queryKey: ['teams'] })
    },
  })

  // Admins manage plain members; only the owner manages admins.
  const canChange = (member: TeamMember) =>
    canManage && (member.role === 'member' || callerIsOwner)

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          <span className='flex items-center gap-2'>
            <Button
              variant='ghost'
              size='icon-sm'
              aria-label={t('Back to teams')}
              render={<Link to='/team' />}
            >
              <ArrowLeft />
            </Button>
            {detail?.team.name ?? t('Team')}
          </span>
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button variant='outline' onClick={() => setTransferOpen(true)}>
            <ArrowRightLeft />
            {t('Transfer to team')}
          </Button>
          {canManage && (
            <Button
              render={<Link to='/wallet' search={{ team_id: props.teamId }} />}
            >
              <Wallet />
              {t('Top up team')}
            </Button>
          )}
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='mx-auto flex w-full max-w-7xl flex-col gap-4 sm:gap-5'>
            <div className='grid grid-cols-2 divide-x rounded-lg border'>
              <div className='px-3 py-3 sm:px-5 sm:py-4'>
                <div className='text-muted-foreground text-xs'>
                  {t('Team balance')}
                </div>
                <div className='mt-1 text-xl font-semibold sm:text-2xl'>
                  {formatQuota(detail?.balance ?? 0)}
                </div>
              </div>
              <div className='px-3 py-3 sm:px-5 sm:py-4'>
                <div className='text-muted-foreground text-xs'>
                  {t('My role')}
                </div>
                <div className='mt-2'>
                  {detail && <TeamRoleBadge role={detail.role} />}
                </div>
              </div>
            </div>

            <RedemptionCard
              enabled
              code={code}
              onCodeChange={setCode}
              onRedeem={() => {
                if (code.trim()) redeemMutation.mutate()
              }}
              redeeming={redeemMutation.isPending}
            />

            <section className='space-y-3'>
              <div className='flex items-center justify-between gap-2'>
                <h2 className='text-base font-semibold'>{t('Members')}</h2>
                {canManage && (
                  <Button
                    size='sm'
                    onClick={() => setMemberDialog({ open: true })}
                  >
                    <Plus />
                    {t('Add member')}
                  </Button>
                )}
              </div>
              <StaticDataTable<TeamMember>
                data={detail?.members ?? []}
                getRowKey={(member) => member.user_id}
                columns={[
                  {
                    id: 'user',
                    header: t('User'),
                    cell: (member) => member.display_name || member.username,
                  },
                  {
                    id: 'role',
                    header: t('Role'),
                    cell: (member) => <TeamRoleBadge role={member.role} />,
                  },
                  {
                    id: 'limit',
                    header: t('Spending limit'),
                    cell: (member) =>
                      member.limit_quota > 0
                        ? formatQuota(member.limit_quota)
                        : t('Unlimited'),
                  },
                  {
                    id: 'used',
                    header: t('Used'),
                    cell: (member) => formatQuota(member.used_quota),
                  },
                  {
                    id: 'remaining',
                    header: t('Remaining'),
                    cell: (member) =>
                      member.limit_quota > 0
                        ? formatQuota(member.remain_quota)
                        : t('Unlimited'),
                  },
                  ...(canManage
                    ? [
                        {
                          id: 'actions',
                          header: '',
                          className: 'w-20',
                          cell: (member: TeamMember) => (
                            <StaticRowActions
                              editLabel={t('Edit member')}
                              deleteLabel={t('Remove member')}
                              menuLabel={t('More actions')}
                              editDisabled={!canChange(member)}
                              deleteDisabled={
                                member.role === 'owner' || !canChange(member)
                              }
                              onEdit={() =>
                                setMemberDialog({ open: true, member })
                              }
                              onDelete={() => setRemoving(member)}
                            />
                          ),
                        },
                      ]
                    : []),
                ]}
              />
            </section>

            {canManage && (
              <section className='space-y-3'>
                <h2 className='text-base font-semibold'>
                  {t('Usage by member')}
                </h2>
                <StaticDataTable<TeamUsageRow>
                  data={usageQuery.data ?? []}
                  getRowKey={(row) => row.token_id}
                  emptyContent={t('No usage yet')}
                  columns={[
                    {
                      id: 'user',
                      header: t('User'),
                      cell: (row) => row.username || t('Removed member'),
                    },
                    {
                      id: 'requests',
                      header: t('Requests'),
                      cell: (row) => formatNumber(row.requests, locale),
                    },
                    {
                      id: 'quota',
                      header: t('Used'),
                      cell: (row) => formatQuota(row.quota),
                    },
                  ]}
                />
              </section>
            )}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <MemberDialog
        teamId={props.teamId}
        callerIsOwner={callerIsOwner}
        open={memberDialog.open}
        member={memberDialog.member}
        onOpenChange={(open) =>
          setMemberDialog((state) => ({ ...state, open }))
        }
      />
      <TeamTransferDialog
        teamId={props.teamId}
        open={transferOpen}
        onOpenChange={setTransferOpen}
      />
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => {
          if (!open) setRemoving(null)
        }}
        title={t('Remove member')}
        desc={t(
          'Remove {{name}} from the team? Their team API key is deleted.',
          { name: removing?.display_name || removing?.username }
        )}
        destructive
        isLoading={removeMutation.isPending}
        handleConfirm={() => {
          if (removing) removeMutation.mutate(removing)
        }}
      />
    </>
  )
}
