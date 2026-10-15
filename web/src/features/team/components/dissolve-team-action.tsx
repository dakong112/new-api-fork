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
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import { formatQuota } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'
import { AuthOperationError } from '@/lib/secure-verification'

import { dissolveTeam } from '../api'

type DissolveTeamActionProps = {
  teamId: number
  teamName: string
  balance: number
}

// DissolveTeamAction lets the owner end the team after a security check; the
// team balance goes back to the owner's personal balance.
export function DissolveTeamAction(props: DissolveTeamActionProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const verification = useSecureVerification()
  const [open, setOpen] = useState(false)
  const [pending, setPending] = useState(false)

  const handleDissolve = async () => {
    setPending(true)
    try {
      const proof = await verification.requestVerification({
        scope: 'team.dissolve',
        context: { team_id: props.teamId },
        title: t('Verify to dissolve the team'),
      })
      if (!proof) return
      const result = await dissolveTeam(props.teamId, proof.proof_token)
      if (!result.success) {
        handleServerError(result, t('Failed to dissolve the team'))
        return
      }
      toast.success(
        t('Team dissolved. {{quota}} returned to your balance.', {
          quota: formatQuota(result.data.quota),
        })
      )
      setOpen(false)
      queryClient.removeQueries({ queryKey: ['team', props.teamId] })
      void queryClient.invalidateQueries({ queryKey: ['teams'] })
      void navigate({ to: '/team' })
    } catch (error) {
      handleServerError(AuthOperationError.from(error))
    } finally {
      setPending(false)
    }
  }

  return (
    <>
      <Button variant='destructive' onClick={() => setOpen(true)}>
        <Trash2 />
        {t('Dissolve team')}
      </Button>
      <ConfirmDialog
        open={open && !verification.isActive}
        onOpenChange={setOpen}
        title={t('Dissolve team')}
        desc={t(
          'Dissolve {{name}}? The remaining team balance of {{quota}} returns to your personal balance, and every member API key of this team is deleted. This cannot be undone.',
          { name: props.teamName, quota: formatQuota(props.balance) }
        )}
        confirmText={t('Dissolve team')}
        destructive
        isLoading={pending}
        handleConfirm={handleDissolve}
      />
      <SecureVerificationDialog {...verification.dialogProps} />
    </>
  )
}
