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
import { createFileRoute } from '@tanstack/react-router'
import { ExternalLink, LayoutGrid } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import {
  Item,
  ItemActions,
  ItemContent,
  ItemGroup,
  ItemTitle,
} from '@/components/ui/item'
import { listSSOApps } from '@/features/sso/api'

export const Route = createFileRoute('/_authenticated/apps')({
  component: AppsPage,
})

function AppsPage() {
  const { t } = useTranslation()
  const query = useQuery({ queryKey: ['sso-apps'], queryFn: listSSOApps })
  const apps = query.data ?? []

  let content = (
    <ItemGroup className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
      {apps.map((app) => (
        <Item
          key={app.client_id}
          variant='outline'
          render={<a href={app.launch_url} />}
          className='hover:bg-accent'
        >
          <ItemContent>
            <ItemTitle>{app.name || app.client_id}</ItemTitle>
          </ItemContent>
          <ItemActions>
            <ExternalLink className='size-4' aria-hidden='true' />
          </ItemActions>
        </Item>
      ))}
    </ItemGroup>
  )
  if (query.isLoading) {
    content = <LoadingState />
  } else if (query.isError) {
    content = <ErrorState onRetry={() => query.refetch()} />
  } else if (apps.length === 0) {
    content = (
      <EmptyState icon={LayoutGrid} title={t('No applications available')} />
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Applications')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>{content}</SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
