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
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable, StaticRowActions } from '@/components/data-table'
import { SectionPageLayout } from '@/components/layout'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'

import {
  AUTHZ_CATALOG_QUERY_KEY,
  deleteAuthzRole,
  getAuthzCatalog,
  type AuthzRole,
} from './api'
import { RoleDialog } from './components/role-dialog'

export function Roles() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialog, setDialog] = useState<{ open: boolean; role?: AuthzRole }>({
    open: false,
  })
  const [deleting, setDeleting] = useState<AuthzRole | null>(null)
  const catalogQuery = useQuery({
    queryKey: AUTHZ_CATALOG_QUERY_KEY,
    queryFn: getAuthzCatalog,
  })
  const catalog = catalogQuery.data ?? { resources: [], roles: [] }
  const deleteMutation = useMutation({
    mutationFn: (role: AuthzRole) => deleteAuthzRole(role.key),
    onSuccess: () => {
      setDeleting(null)
      void queryClient.invalidateQueries({ queryKey: AUTHZ_CATALOG_QUERY_KEY })
      void queryClient.invalidateQueries({
        queryKey: ['admin-permission-catalog'],
      })
    },
  })

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Roles & Permissions')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button onClick={() => setDialog({ open: true })}>
            <Plus />
            {t('Create role')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <StaticDataTable<AuthzRole>
            data={catalog.roles}
            getRowKey={(role) => role.key}
            columns={[
              {
                id: 'name',
                header: t('Name'),
                cell: (role) => (
                  <div className='min-w-0'>
                    <div className='font-medium'>
                      {role.built_in ? t(role.name) : role.name}
                    </div>
                    {role.description && (
                      <div className='text-muted-foreground truncate text-xs'>
                        {role.built_in ? t(role.description) : role.description}
                      </div>
                    )}
                  </div>
                ),
              },
              {
                id: 'key',
                header: t('Role key'),
                cell: (role) => <code className='text-xs'>{role.key}</code>,
              },
              {
                id: 'type',
                header: t('Type'),
                cell: (role) =>
                  role.built_in ? (
                    <StatusBadge
                      label={t('Built-in')}
                      variant='neutral'
                      size='sm'
                    />
                  ) : (
                    <StatusBadge label={t('Custom')} variant='info' size='sm' />
                  ),
              },
              {
                id: 'actions',
                header: '',
                className: 'w-20',
                cell: (role) => (
                  <StaticRowActions
                    editLabel={role.built_in ? t('View role') : t('Edit role')}
                    deleteLabel={t('Delete')}
                    menuLabel={t('More actions')}
                    deleteDisabled={role.built_in}
                    onEdit={() => setDialog({ open: true, role })}
                    onDelete={() => setDeleting(role)}
                  />
                ),
              },
            ]}
          />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <RoleDialog
        catalog={catalog}
        open={dialog.open}
        role={dialog.role}
        onOpenChange={(open) => setDialog((state) => ({ ...state, open }))}
      />
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
        title={t('Delete role')}
        desc={t(
          'Delete the role {{name}}? Users holding it lose its permissions.',
          { name: deleting?.name }
        )}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() => {
          if (deleting) deleteMutation.mutate(deleting)
        }}
      />
    </>
  )
}
