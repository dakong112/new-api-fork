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
import { Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { handleServerError } from '@/lib/handle-server-error'

import { type GroupUsage, renameGroup } from '../api'

type GroupRenameDialogProps = {
  /** Group being renamed; the dialog is closed when null. */
  groupName: string | null
  usage?: GroupUsage
  onOpenChange: (open: boolean) => void
}

/**
 * Renames a group on the server, rewriting channels, tokens, users,
 * subscriptions and group settings together, then reloads the settings.
 */
export function GroupRenameDialog(props: GroupRenameDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [newName, setNewName] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const oldName = props.groupName ?? ''
  const trimmed = newName.trim()

  const close = () => {
    setNewName('')
    props.onOpenChange(false)
  }

  const submit = async () => {
    setSubmitting(true)
    try {
      const res = await renameGroup(oldName, trimmed)
      if (!res.success || !res.data) {
        handleServerError(res)
        return
      }
      toast.success(
        t(
          'Renamed {{old}} to {{new}}: {{channels}} channels, {{tokens}} tokens, {{users}} users updated',
          {
            old: oldName,
            new: trimmed,
            channels: res.data.channels,
            tokens: res.data.tokens,
            users: res.data.users,
          }
        )
      )
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['system-options'] }),
        queryClient.invalidateQueries({ queryKey: ['group-usage'] }),
        queryClient.invalidateQueries({ queryKey: ['status'] }),
      ])
      close()
    } catch (error) {
      handleServerError(error)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={props.groupName !== null}
      onOpenChange={(open) => {
        if (!open) close()
      }}
      title={t('Rename group {{name}}', { name: oldName })}
      description={t(
        'Channels, tokens, users, subscription plans and all group settings using this name are updated together. Usage logs keep the old name.'
      )}
      contentClassName='sm:max-w-md'
      footer={
        <>
          <Button variant='outline' onClick={close} disabled={submitting}>
            {t('Cancel')}
          </Button>
          <Button
            onClick={submit}
            disabled={submitting || !trimmed || trimmed === oldName}
          >
            {submitting && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {t('Rename')}
          </Button>
        </>
      }
    >
      <div className='flex flex-col gap-3'>
        <div className='flex flex-col gap-1.5'>
          <Label htmlFor='group-rename-input'>{t('New group name')}</Label>
          <Input
            id='group-rename-input'
            value={newName}
            placeholder={oldName}
            autoFocus
            onChange={(e) => setNewName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && trimmed && trimmed !== oldName) {
                void submit()
              }
            }}
          />
        </div>
        {props.usage && (
          <p className='text-muted-foreground text-sm'>
            {t(
              'This will update {{channels}} channels, {{tokens}} tokens and {{users}} users.',
              {
                channels: props.usage.channels,
                tokens: props.usage.tokens,
                users: props.usage.users,
              }
            )}
          </p>
        )}
      </div>
    </Dialog>
  )
}
