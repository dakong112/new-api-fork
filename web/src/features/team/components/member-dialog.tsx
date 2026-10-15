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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { DraftNumberInput } from '@/components/draft-number-input'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  getEditableQuotaStep,
  parseQuotaFromDollars,
  quotaUnitsToEditableAmount,
} from '@/lib/format'

import {
  addTeamMember,
  updateTeamMember,
  type TeamMember,
  type TeamRole,
} from '../api'

type MemberDialogProps = {
  teamId: number
  /** Only the owner may grant or revoke the admin role. */
  callerIsOwner: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The member to edit; absent when adding one. */
  member?: TeamMember
}

export function MemberDialog(props: MemberDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [account, setAccount] = useState('')
  const [role, setRole] = useState<TeamRole>('member')
  const [limit, setLimit] = useState(0)

  useEffect(() => {
    if (!props.open) return
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setAccount('')
    setRole(props.member?.role ?? 'member')
    setLimit(quotaUnitsToEditableAmount(props.member?.limit_quota ?? 0))
  }, [props.open, props.member])

  const mutation = useMutation({
    mutationFn: () => {
      const limitQuota = parseQuotaFromDollars(limit)
      if (props.member) {
        return updateTeamMember(props.teamId, props.member.user_id, {
          role: props.member.role === 'owner' ? undefined : role,
          limit_quota: limitQuota,
        })
      }
      const value = account.trim()
      return addTeamMember(props.teamId, {
        ...(value.includes('@') ? { email: value } : { username: value }),
        role,
        limit_quota: limitQuota,
      })
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['team', props.teamId] })
      void queryClient.invalidateQueries({ queryKey: ['teams'] })
      props.onOpenChange(false)
    },
  })

  const editingOwner = props.member?.role === 'owner'
  const canSubmit =
    (props.member !== undefined || account.trim() !== '') &&
    Number.isFinite(limit) &&
    limit >= 0

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.member ? t('Edit member') : t('Add member')}
      description={
        props.member
          ? props.member.display_name || props.member.username
          : undefined
      }
      contentClassName='sm:max-w-md'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            disabled={!canSubmit || mutation.isPending}
            onClick={() => mutation.mutate()}
          >
            {t('Save')}
          </Button>
        </>
      }
    >
      {!props.member && (
        <div className='space-y-2'>
          <Label htmlFor='team-member-account'>{t('Username or email')}</Label>
          <Input
            id='team-member-account'
            value={account}
            onChange={(event) => setAccount(event.target.value)}
          />
        </div>
      )}
      {!editingOwner && (
        <div className='space-y-2'>
          <Label>{t('Role')}</Label>
          <Select
            items={[
              { value: 'member', label: t('Member') },
              { value: 'admin', label: t('Admin') },
            ]}
            value={role}
            onValueChange={(value) => value && setRole(value as TeamRole)}
            disabled={!props.callerIsOwner}
          >
            <SelectTrigger className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value='member'>{t('Member')}</SelectItem>
                <SelectItem value='admin'>{t('Admin')}</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>
      )}
      <div className='space-y-2'>
        <Label htmlFor='team-member-limit'>{t('Spending limit')}</Label>
        <DraftNumberInput
          id='team-member-limit'
          value={limit}
          onValueChange={setLimit}
          min={0}
          step={getEditableQuotaStep()}
        />
        <p className='text-muted-foreground text-xs'>
          {t('0 means unlimited; the member can spend up to the team balance.')}
        </p>
      </div>
    </Dialog>
  )
}
