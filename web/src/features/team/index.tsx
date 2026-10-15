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
import { Link, useNavigate } from '@tanstack/react-router'
import { Plus, UsersRound } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { EmptyState } from '@/components/empty-state'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { formatQuota } from '@/lib/format'

import { createTeam, getSelfTeams, type UserTeam } from './api'
import { TeamRoleBadge } from './components/team-role-badge'

export function Teams() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [name, setName] = useState('')
  const teamsQuery = useQuery({ queryKey: ['teams'], queryFn: getSelfTeams })
  const createMutation = useMutation({
    mutationFn: () => createTeam(name.trim()),
    onSuccess: (team) => {
      void queryClient.invalidateQueries({ queryKey: ['teams'] })
      setCreateOpen(false)
      setName('')
      void navigate({
        to: '/team/$teamId',
        params: { teamId: String(team.id) },
      })
    },
  })
  const teams = teamsQuery.data ?? []

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Teams')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button onClick={() => setCreateOpen(true)}>
            <Plus />
            {t('Create team')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          {!teamsQuery.isLoading && teams.length === 0 ? (
            <EmptyState
              icon={UsersRound}
              title={t('No teams yet')}
              description={t(
                'Create a team to share a wallet with members, each with their own spending limit.'
              )}
            />
          ) : (
            <StaticDataTable<UserTeam>
              data={teams}
              getRowKey={(team) => team.id}
              columns={[
                {
                  id: 'name',
                  header: t('Name'),
                  cell: (team) => (
                    <Link
                      to='/team/$teamId'
                      params={{ teamId: String(team.id) }}
                      className='font-medium hover:underline'
                    >
                      {team.name}
                    </Link>
                  ),
                },
                {
                  id: 'role',
                  header: t('My role'),
                  cell: (team) => <TeamRoleBadge role={team.role} />,
                },
                {
                  id: 'balance',
                  header: t('Team balance'),
                  cell: (team) => formatQuota(team.balance),
                },
                {
                  id: 'limit',
                  header: t('My limit'),
                  cell: (team) =>
                    team.limit_quota > 0
                      ? formatQuota(team.limit_quota)
                      : t('Unlimited'),
                },
                {
                  id: 'remaining',
                  header: t('My remaining limit'),
                  cell: (team) =>
                    team.limit_quota > 0
                      ? formatQuota(team.key_remain_quota)
                      : t('Unlimited'),
                },
                {
                  id: 'used',
                  header: t('Used'),
                  cell: (team) => formatQuota(team.key_used_quota),
                },
              ]}
            />
          )}
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <Dialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        title={t('Create team')}
        contentClassName='sm:max-w-md'
        contentHeight='auto'
        footer={
          <>
            <Button variant='outline' onClick={() => setCreateOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button
              disabled={!name.trim() || createMutation.isPending}
              onClick={() => createMutation.mutate()}
            >
              {t('Create')}
            </Button>
          </>
        }
      >
        <div className='space-y-2'>
          <Label htmlFor='team-name'>{t('Team name')}</Label>
          <Input
            id='team-name'
            value={name}
            maxLength={64}
            onChange={(event) => setName(event.target.value)}
          />
        </div>
      </Dialog>
    </>
  )
}
