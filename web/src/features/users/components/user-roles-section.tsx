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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SideDrawerSection } from '@/components/drawer-layout'
import { MultiSelect } from '@/components/multi-select'
import { Button } from '@/components/ui/button'
import type { PermissionRoleDef } from '@/lib/admin-permissions'
import { handleServerError } from '@/lib/handle-server-error'
import { AuthOperationError } from '@/lib/secure-verification'

import { getUserAuthzRoles, setUserAuthzRoles } from '../api'
import type { User } from '../types'
import { useUsers } from './users-provider'

type UserRolesSectionProps = {
  user: User
  roles: PermissionRoleDef[]
}

// UserRolesSection assigns custom roles to a user. It saves on its own,
// because the change needs its own single-use step-up proof.
export function UserRolesSection(props: UserRolesSectionProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { requestVerification } = useUsers()
  const queryKey = ['user-authz-roles', props.user.id]
  const rolesQuery = useQuery({
    queryKey,
    queryFn: () => getUserAuthzRoles(props.user.id),
  })
  const [selected, setSelected] = useState<string[]>([])
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (rolesQuery.data) setSelected(rolesQuery.data)
  }, [rolesQuery.data])

  const changed =
    rolesQuery.data !== undefined &&
    [...selected].sort().join() !== [...rolesQuery.data].sort().join()

  const save = async () => {
    setSaving(true)
    try {
      const proof = await requestVerification({
        scope: 'admin.user.update',
        context: { user_id: props.user.id },
        title: t('Verify to update user roles'),
        description: t(
          'Confirm your identity before changing the account {{username}}.',
          { username: props.user.username }
        ),
      })
      if (!proof) return
      const roles = await setUserAuthzRoles(
        props.user.id,
        selected,
        proof.proof_token
      )
      queryClient.setQueryData(queryKey, roles)
      toast.success(t('Roles updated'))
    } catch (error) {
      handleServerError(AuthOperationError.from(error), t('Failed to save'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <SideDrawerSection>
      <h3 className='text-sm font-medium'>{t('Roles')}</h3>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Custom roles replace the default administrator permissions and grant access in connected applications.'
        )}
      </p>
      <MultiSelect
        options={props.roles
          .filter((role) => !role.built_in)
          .map((role) => ({ value: role.key, label: role.name }))}
        selected={selected}
        onChange={setSelected}
        placeholder={t('Select roles')}
        ariaLabel={t('Roles')}
        disabled={rolesQuery.isLoading || saving}
      />
      <div className='flex justify-end'>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={!changed || saving}
          onClick={save}
        >
          {t('Save roles')}
        </Button>
      </div>
    </SideDrawerSection>
  )
}
