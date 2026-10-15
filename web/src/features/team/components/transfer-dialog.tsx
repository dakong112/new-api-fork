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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { DraftNumberInput } from '@/components/draft-number-input'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { getSelf } from '@/lib/api'
import {
  formatQuota,
  getEditableQuotaStep,
  parseQuotaFromDollars,
} from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'

import { transferToTeam } from '../api'

type TeamTransferDialogProps = {
  teamId: number
  open: boolean
  onOpenChange: (open: boolean) => void
}

// TeamTransferDialog moves quota from the caller's personal balance into the
// team wallet.
export function TeamTransferDialog(props: TeamTransferDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [amount, setAmount] = useState(0)
  const selfQuery = useQuery({
    queryKey: ['user-self'],
    queryFn: async () => requireServerSuccess(await getSelf()),
    enabled: props.open,
  })
  const personalQuota: number = selfQuery.data?.data?.quota ?? 0
  const quota = parseQuotaFromDollars(amount)

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (props.open) setAmount(0)
  }, [props.open])

  const mutation = useMutation({
    mutationFn: () => transferToTeam(props.teamId, quota),
    onSuccess: () => {
      toast.success(
        t('Transferred {{quota}} to the team', { quota: formatQuota(quota) })
      )
      void queryClient.invalidateQueries({ queryKey: ['team', props.teamId] })
      void queryClient.invalidateQueries({ queryKey: ['teams'] })
      void queryClient.invalidateQueries({ queryKey: ['user-self'] })
      props.onOpenChange(false)
    },
  })

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Transfer to team')}
      description={t(
        'Move quota from your personal balance to the team wallet.'
      )}
      contentClassName='sm:max-w-md'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            disabled={quota <= 0 || quota > personalQuota || mutation.isPending}
            onClick={() => mutation.mutate()}
          >
            {t('Transfer')}
          </Button>
        </>
      }
    >
      <div className='space-y-1'>
        <Label className='text-muted-foreground text-xs'>
          {t('Personal balance')}
        </Label>
        <div className='text-2xl font-semibold'>
          {formatQuota(personalQuota)}
        </div>
      </div>
      <div className='space-y-2'>
        <Label htmlFor='team-transfer-amount'>{t('Transfer Amount')}</Label>
        <DraftNumberInput
          id='team-transfer-amount'
          value={amount}
          onValueChange={setAmount}
          min={0}
          step={getEditableQuotaStep()}
        />
        <p className='text-muted-foreground text-xs'>{formatQuota(quota)}</p>
      </div>
    </Dialog>
  )
}
