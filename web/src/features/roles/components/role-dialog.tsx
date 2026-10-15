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
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import {
  PermissionMatrix,
  type PermissionMatrixGroup,
} from '@/components/permission-matrix'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import type { AdminPermissionMatrix } from '@/lib/admin-permissions'

import {
  AUTHZ_CATALOG_QUERY_KEY,
  createAuthzRole,
  setAuthzRolePermissions,
  updateAuthzRole,
  type AuthzCatalog,
  type AuthzRole,
} from '../api'

const ROLE_KEY_PATTERN = /^[a-z][a-z0-9_]{1,31}$/

// Applications whose permission codes live in this catalog, in display order.
const APP_GROUPS = [
  { app: '', labelKey: 'API gateway' },
  { app: 'canvas', labelKey: 'Infinite canvas' },
]

type RoleDialogProps = {
  catalog: AuthzCatalog
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The role to edit or, when built in, to view; absent when creating. */
  role?: AuthzRole
}

export function RoleDialog(props: RoleDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [key, setKey] = useState('')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [grants, setGrants] = useState<AdminPermissionMatrix>({})
  const readOnly = props.role?.built_in === true

  useEffect(() => {
    if (!props.open) return
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setKey(props.role?.key ?? '')
    setName(props.role?.name ?? '')
    setDescription(props.role?.description ?? '')
    setGrants(props.role?.grants ?? {})
  }, [props.open, props.role])

  const groups = useMemo<PermissionMatrixGroup[]>(
    () =>
      APP_GROUPS.map((group) => ({
        key: group.app || 'gateway',
        labelKey: group.labelKey,
        resources: props.catalog.resources.filter(
          (resource) => resource.app === group.app
        ),
      })).filter((group) => group.resources.length > 0),
    [props.catalog.resources]
  )

  const mutation = useMutation({
    mutationFn: async () => {
      const input = { key, name: name.trim(), description: description.trim() }
      if (props.role) await updateAuthzRole(input)
      else await createAuthzRole(input)
      await setAuthzRolePermissions(key, grants)
    },
    onSuccess: () => {
      toast.success(t('Role saved'))
      void queryClient.invalidateQueries({ queryKey: AUTHZ_CATALOG_QUERY_KEY })
      void queryClient.invalidateQueries({
        queryKey: ['admin-permission-catalog'],
      })
      props.onOpenChange(false)
    },
    // A new role that failed only at the grant step still exists; refresh
    // so it shows up and can be edited.
    onSettled: () =>
      void queryClient.invalidateQueries({ queryKey: AUTHZ_CATALOG_QUERY_KEY }),
  })

  const keyValid = ROLE_KEY_PATTERN.test(key)
  let title = t('Create role')
  if (readOnly) title = t('View role')
  else if (props.role) title = t('Edit role')

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={title}
      contentClassName='sm:max-w-3xl'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {readOnly ? t('Close') : t('Cancel')}
          </Button>
          {!readOnly && (
            <Button
              disabled={!keyValid || !name.trim() || mutation.isPending}
              onClick={() => mutation.mutate()}
            >
              {t('Save')}
            </Button>
          )}
        </>
      }
    >
      <div className='grid gap-4 sm:grid-cols-2'>
        <div className='space-y-2'>
          <Label htmlFor='role-name'>{t('Name')}</Label>
          <Input
            id='role-name'
            value={name}
            maxLength={100}
            disabled={readOnly}
            onChange={(event) => setName(event.target.value)}
          />
        </div>
        <div className='space-y-2'>
          <Label htmlFor='role-key'>{t('Key')}</Label>
          <Input
            id='role-key'
            value={key}
            disabled={props.role !== undefined}
            aria-invalid={key !== '' && !keyValid}
            aria-describedby='role-key-hint'
            onChange={(event) => setKey(event.target.value)}
          />
          <p id='role-key-hint' className='text-muted-foreground text-xs'>
            {t(
              'Lowercase letters, digits and underscores, starting with a letter (2-32 characters).'
            )}
          </p>
        </div>
      </div>
      <div className='space-y-2'>
        <Label htmlFor='role-description'>{t('Description')}</Label>
        <Textarea
          id='role-description'
          value={description}
          maxLength={500}
          rows={2}
          disabled={readOnly}
          onChange={(event) => setDescription(event.target.value)}
        />
      </div>
      <div className='space-y-2'>
        <Label>{t('Permissions')}</Label>
        <PermissionMatrix
          groups={groups}
          value={grants}
          onChange={setGrants}
          disabled={readOnly}
        />
      </div>
    </Dialog>
  )
}
