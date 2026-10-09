import { useQuery } from '@tanstack/react-query'
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
import {
  AlertTriangle,
  ArrowDown,
  ArrowUp,
  ChevronDown,
  Search,
  X,
  Info,
  Pencil,
  Plus,
  Trash2,
} from 'lucide-react'
import { Reorder } from 'motion/react'
import {
  useState,
  useMemo,
  useEffect,
  useCallback,
  memo,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'

import { AutoGroupOrderItem } from '@/components/auto-group-order-item'
import { StaticDataTable } from '@/components/data-table/static/static-data-table'
import { StaticRowActions } from '@/components/data-table/static/static-row-actions'
import { Dialog } from '@/components/dialog'
import {
  sideDrawerContentClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { EmptyState } from '@/components/empty-state'
import { StatusBadge } from '@/components/status-badge'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Combobox } from '@/components/ui/combobox'
import { Input } from '@/components/ui/input'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from '@/components/ui/input-group'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { sortGroupNames } from '@/lib/group-order'
import { requireServerSuccess } from '@/lib/server-error-message'

import { getGroupUsage } from '../api'
import { safeJsonParse } from '../utils/json-parser'
import { GroupRenameDialog } from './group-rename-dialog'
import { GroupSpecialUsableRulesEditor } from './group-special-usable-editor'

export type GroupSettingsSection =
  | 'pricing'
  | 'overrides'
  | 'visibility'
  | 'auto'
  | 'fallback'

type GroupRatioVisualEditorProps = {
  section: GroupSettingsSection
  onSectionChange: (section: GroupSettingsSection) => void
  defaultUseAutoGroupField: ReactNode
  groupRatio: string
  topupGroupRatio: string
  userUsableGroups: string
  groupGroupRatio: string
  autoGroups: string
  groupDisplayOrder: string
  /** JSON object: group name -> ordered fallback group names. */
  groupFallbackGroups: string
  /** JSON array of groups that retry 502/504/524 on another channel. */
  groupNetworkRetryGroups: string
  /** Renaming reloads settings from the server, so it waits for a save. */
  hasUnsavedChanges: boolean
  maxTokenAutoGroupsField: ReactNode
  groupSpecialUsableGroup: string
  onChange: (field: string, value: string) => void
}

type GroupPricingRow = {
  _id: string
  /** Added in this session; only these names are edited inline. */
  isNew?: boolean
  name: string
  ratio: string
  topupRatio: string
  selectable: boolean
  description: string
}

type RegistryEntry = {
  name: string
  ratio: number
}

const sectionCardClassName = 'min-w-0 shadow-none'
const sectionHeaderClassName = 'gap-2 border-b'

let groupPricingIdCounter = 0
function createGroupPricingId() {
  groupPricingIdCounter += 1
  return `gpr_${groupPricingIdCounter}`
}

function normalizeRatio(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 1
}

function parseRatioMap(value: string): Record<string, number> {
  return safeJsonParse<Record<string, number>>(value, {
    fallback: {},
    silent: true,
  })
}

function parseUsableMap(value: string): Record<string, string> {
  return safeJsonParse<Record<string, string>>(value, {
    fallback: {},
    silent: true,
  })
}

function parseNestedRatioMap(
  value: string
): Record<string, Record<string, number>> {
  return safeJsonParse<Record<string, Record<string, number>>>(value, {
    fallback: {},
    silent: true,
  })
}

function buildGroupPricingRows(
  groupRatio: string,
  userUsableGroups: string,
  topupGroupRatio: string
): GroupPricingRow[] {
  const ratioMap = parseRatioMap(groupRatio)
  const usableMap = parseUsableMap(userUsableGroups)
  const topupMap = parseRatioMap(topupGroupRatio)
  const names = new Set([
    ...Object.keys(ratioMap),
    ...Object.keys(usableMap),
    ...Object.keys(topupMap),
  ])

  return [...names].map((name) => ({
    _id: createGroupPricingId(),
    name,
    ratio: String(normalizeRatio(ratioMap[name])),
    topupRatio: Object.hasOwn(topupMap, name) ? String(topupMap[name]) : '',
    selectable: Object.hasOwn(usableMap, name),
    description: String(usableMap[name] ?? ''),
  }))
}

function normalizeOrderJson(groupDisplayOrder: string): string {
  try {
    const parsed: unknown = JSON.parse(groupDisplayOrder || '[]')
    return JSON.stringify(Array.isArray(parsed) ? parsed : [])
  } catch {
    return '[]'
  }
}

function orderGroupPricingRows(
  rows: GroupPricingRow[],
  groupDisplayOrder: string
): GroupPricingRow[] {
  let order: string[] = []
  try {
    const parsed: unknown = JSON.parse(groupDisplayOrder || '[]')
    if (Array.isArray(parsed)) {
      order = parsed.filter((g) => typeof g === 'string')
    }
  } catch {
    // An unparsable draft (JSON mode) keeps the current order.
  }
  const names = [...new Set(rows.map((row) => row.name.trim()))]
  const rank = new Map(
    sortGroupNames(names.filter(Boolean), order).map((name, i) => [name, i])
  )
  // Stable sort keeps every row, including duplicate names mid-edit; rows
  // without a name yet (just added) stay at the end.
  const rankOf = (row: GroupPricingRow) =>
    rank.get(row.name.trim()) ?? Number.POSITIVE_INFINITY
  return [...rows].sort((a, b) => rankOf(a) - rankOf(b))
}

function serializeGroupPricingRows(rows: GroupPricingRow[]) {
  const groupRatio: Record<string, number> = {}
  const userUsableGroups: Record<string, string> = {}
  const topupGroupRatio: Record<string, number> = {}

  for (const row of rows) {
    const name = row.name.trim()
    if (!name) continue
    groupRatio[name] = normalizeRatio(row.ratio)
    if (row.selectable) {
      userUsableGroups[name] = row.description
    }
    const topup = row.topupRatio.trim()
    if (topup !== '' && Number.isFinite(Number(topup))) {
      topupGroupRatio[name] = Number(topup)
    }
  }

  return {
    GroupRatio: JSON.stringify(groupRatio, null, 2),
    UserUsableGroups: JSON.stringify(userUsableGroups, null, 2),
    TopupGroupRatio: JSON.stringify(topupGroupRatio, null, 2),
  }
}

function groupPricingSignature(rows: GroupPricingRow[]): string {
  const serialized = serializeGroupPricingRows(rows)
  return JSON.stringify({
    groupRatio: parseRatioMap(serialized.GroupRatio),
    userUsableGroups: parseUsableMap(serialized.UserUsableGroups),
    topupGroupRatio: parseRatioMap(serialized.TopupGroupRatio),
  })
}

function sourceGroupPricingSignature(
  groupRatio: string,
  userUsableGroups: string,
  topupGroupRatio: string
): string {
  return JSON.stringify({
    groupRatio: parseRatioMap(groupRatio),
    userUsableGroups: parseUsableMap(userUsableGroups),
    topupGroupRatio: parseRatioMap(topupGroupRatio),
  })
}

function UnknownGroupBadge() {
  const { t } = useTranslation()
  return (
    <StatusBadge variant='danger' copyable={false}>
      <AlertTriangle className='mr-1 h-3 w-3' />
      {t('Not in pricing table')}
    </StatusBadge>
  )
}

type GroupNameSelectProps = {
  options: string[]
  value: string | null
  placeholder: string
  onValueChange: (value: string) => void
  className?: string
  /** Accessible name when the placeholder is not unique on the page. */
  ariaLabel?: string
}

function GroupNameSelect(props: GroupNameSelectProps) {
  const options = useMemo(() => {
    if (props.value && !props.options.includes(props.value)) {
      return [props.value, ...props.options]
    }
    return props.options
  }, [props.options, props.value])

  return (
    <Combobox
      options={options.map((name) => ({ value: name, label: name }))}
      value={props.value}
      onValueChange={(value) => {
        if (value) props.onValueChange(value)
      }}
      className={props.className ?? 'w-48'}
      placeholder={props.placeholder}
      aria-label={props.ariaLabel ?? props.placeholder}
    />
  )
}

export const GroupRatioVisualEditor = memo(function GroupRatioVisualEditor({
  section,
  onSectionChange,
  defaultUseAutoGroupField,
  groupRatio,
  topupGroupRatio,
  userUsableGroups,
  groupGroupRatio,
  autoGroups,
  groupDisplayOrder,
  groupFallbackGroups,
  groupNetworkRetryGroups,
  hasUnsavedChanges,
  maxTokenAutoGroupsField,
  groupSpecialUsableGroup,
  onChange,
}: GroupRatioVisualEditorProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [detailGroup, setDetailGroup] = useState<string | null>(null)

  const registry = useMemo<RegistryEntry[]>(() => {
    const ratioMap = parseRatioMap(groupRatio)
    const usableMap = parseUsableMap(userUsableGroups)
    const topupMap = parseRatioMap(topupGroupRatio)
    const names = new Set([
      ...Object.keys(ratioMap),
      ...Object.keys(usableMap),
      ...Object.keys(topupMap),
    ])
    return [...names].map((name) => ({
      name,
      ratio: normalizeRatio(ratioMap[name]),
    }))
  }, [groupRatio, userUsableGroups, topupGroupRatio])

  const registryNames = useMemo(
    () => registry.map((entry) => entry.name),
    [registry]
  )

  // Auto groups
  const autoGroupsList = useMemo(() => {
    return safeJsonParse<string[]>(autoGroups, {
      fallback: [],
      context: 'auto groups',
    })
  }, [autoGroups])

  const handleAutoGroupAdd = useCallback(
    (name: string) => {
      if (autoGroupsList.includes(name)) return
      onChange('AutoGroups', JSON.stringify([...autoGroupsList, name], null, 2))
    },
    [autoGroupsList, onChange]
  )

  const handleAutoGroupDelete = useCallback(
    (index: number) => {
      const list = autoGroupsList.filter((_, i) => i !== index)
      onChange('AutoGroups', JSON.stringify(list, null, 2))
    },
    [autoGroupsList, onChange]
  )

  const handleAutoGroupMove = useCallback(
    (index: number, direction: 'up' | 'down') => {
      const list = [...autoGroupsList]
      const newIndex = direction === 'up' ? index - 1 : index + 1
      if (newIndex < 0 || newIndex >= list.length) return
      ;[list[index], list[newIndex]] = [list[newIndex], list[index]]
      onChange('AutoGroups', JSON.stringify(list, null, 2))
    },
    [autoGroupsList, onChange]
  )

  const autoGroupCandidates = useMemo(
    () => registryNames.filter((name) => !autoGroupsList.includes(name)),
    [registryNames, autoGroupsList]
  )

  return (
    <Tabs
      value={section}
      onValueChange={(value) => onSectionChange(value as GroupSettingsSection)}
      className='min-w-0 gap-5'
    >
      <div className='min-w-0 overflow-x-auto pb-1'>
        <TabsList aria-label={t('Group settings')} className='w-full min-w-max'>
          <TabsTrigger value='pricing' className='px-3'>
            {t('Pricing groups')}
          </TabsTrigger>
          <TabsTrigger value='overrides' className='px-3'>
            {t('Special ratio rules')}
          </TabsTrigger>
          <TabsTrigger value='visibility' className='px-3'>
            {t('Group visibility')}
          </TabsTrigger>
          <TabsTrigger value='auto' className='px-3'>
            {t('Auto group order')}
          </TabsTrigger>
          <TabsTrigger value='fallback' className='px-3'>
            {t('Fallback groups')}
          </TabsTrigger>
        </TabsList>
      </div>
      <TabsContent value='pricing' keepMounted>
        <GroupPricingTable
          groupRatio={groupRatio}
          userUsableGroups={userUsableGroups}
          topupGroupRatio={topupGroupRatio}
          groupDisplayOrder={groupDisplayOrder}
          networkRetryGroups={groupNetworkRetryGroups}
          hasUnsavedChanges={hasUnsavedChanges}
          onChange={onChange}
          onShowDetail={setDetailGroup}
        />
      </TabsContent>
      <TabsContent value='overrides' keepMounted>
        <GroupOverrideRules
          registry={registry}
          groupGroupRatio={groupGroupRatio}
          onChange={onChange}
        />
      </TabsContent>
      <TabsContent value='visibility' keepMounted>
        <GroupSpecialUsableRulesEditor
          value={groupSpecialUsableGroup}
          groupOptions={registryNames}
          onChange={(value) => onChange('GroupSpecialUsableGroup', value)}
        />
      </TabsContent>
      <TabsContent value='auto' keepMounted>
        <Card className={sectionCardClassName}>
          <CardHeader className={sectionHeaderClassName}>
            <CardTitle>{t('Auto group order')}</CardTitle>
            <CardDescription>
              {t(
                'Priority order for tokens in the auto group. The system tries groups from top to bottom.'
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className='grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(16rem,0.7fr)]'>
            <div className='flex min-w-0 flex-col gap-3'>
              <GroupNameSelect
                options={autoGroupCandidates}
                value={null}
                className='w-full'
                placeholder={t('Add group')}
                onValueChange={handleAutoGroupAdd}
              />
              {autoGroupsList.length === 0 ? (
                <EmptyState
                  className='min-h-40'
                  title={t('No auto groups configured')}
                  description={t(
                    'Add groups in the order they should be tried.'
                  )}
                />
              ) : (
                <Reorder.Group
                  as='ol'
                  axis='y'
                  values={autoGroupsList}
                  onReorder={(groups) =>
                    onChange('AutoGroups', JSON.stringify(groups, null, 2))
                  }
                  aria-label={t('Auto group order')}
                  className='flex flex-col gap-2'
                >
                  {autoGroupsList.map((group, index) => (
                    <AutoGroupOrderItem
                      key={group}
                      group={group}
                      index={index}
                      count={autoGroupsList.length}
                      onMove={handleAutoGroupMove}
                      onRemove={() => handleAutoGroupDelete(index)}
                      leading={
                        <span className='text-muted-foreground w-5 shrink-0 text-center text-sm tabular-nums'>
                          {formatNumber(index + 1, locale)}
                        </span>
                      }
                    >
                      {!registryNames.includes(group) && <UnknownGroupBadge />}
                    </AutoGroupOrderItem>
                  ))}
                </Reorder.Group>
              )}
            </div>
            <div className='bg-muted/20 flex min-w-0 flex-col gap-4 self-start rounded-lg border p-4'>
              {defaultUseAutoGroupField}
              <Separator />
              {maxTokenAutoGroupsField}
            </div>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value='fallback' keepMounted>
        <GroupFallbackEditor
          groupNames={registryNames}
          value={groupFallbackGroups}
          onChange={(value) => onChange('GroupFallbackGroups', value)}
        />
      </TabsContent>

      <GroupDetailSheet
        groupName={detailGroup}
        onOpenChange={(open) => {
          if (!open) setDetailGroup(null)
        }}
        registry={registry}
        topupGroupRatio={topupGroupRatio}
        userUsableGroups={userUsableGroups}
        groupGroupRatio={groupGroupRatio}
        autoGroups={autoGroupsList}
        groupSpecialUsableGroup={groupSpecialUsableGroup}
      />
    </Tabs>
  )
})

type GroupFallbackEditorProps = {
  groupNames: string[]
  value: string
  onChange: (value: string) => void
}

/**
 * One row per group: the backup groups a request moves to, in order, when
 * every channel of the group fails. Groups a user cannot select are skipped
 * at request time, and billing follows the group that served the request.
 */
function GroupFallbackEditor(props: GroupFallbackEditorProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const fallbacks = useMemo(
    () =>
      safeJsonParse<Record<string, string[]>>(props.value, {
        fallback: {},
        context: 'fallback groups',
      }),
    [props.value]
  )
  const groups = props.groupNames.filter((name) => name !== 'auto')

  const update = (group: string, list: string[]) => {
    const next = { ...fallbacks, [group]: list }
    if (list.length === 0) delete next[group]
    props.onChange(JSON.stringify(next, null, 2))
  }

  return (
    <Card className={sectionCardClassName}>
      <CardHeader className={sectionHeaderClassName}>
        <CardTitle>{t('Fallback groups')}</CardTitle>
        <CardDescription>
          {t(
            'When every channel of a group fails for a model, requests move to its fallback groups from top to bottom. Drag or use the arrows to change the order. Fallback groups the user cannot select are skipped, and the request is billed at the ratio of the group that served it.'
          )}
          <span className='mt-1 block'>
            {t(
              'Fallback groups apply only to tokens with fallback group retry enabled. Upstream 502, 504 and 524 errors move on to fallback groups only when Network retry is on for the group in Pricing groups.'
            )}
          </span>
        </CardDescription>
      </CardHeader>
      <CardContent className='flex flex-col divide-y'>
        {groups.length === 0 ? (
          <EmptyState className='min-h-40' title={t('No groups configured')} />
        ) : (
          groups.map((group) => {
            const list = fallbacks[group] ?? []
            const move = (index: number, direction: 'up' | 'down') => {
              const target = direction === 'up' ? index - 1 : index + 1
              if (target < 0 || target >= list.length) return
              const next = [...list]
              ;[next[index], next[target]] = [next[target], next[index]]
              update(group, next)
            }
            return (
              <div
                key={group}
                className='grid gap-2 py-3 first:pt-0 last:pb-0 sm:grid-cols-[10rem_minmax(0,1fr)]'
              >
                <span
                  className='truncate pt-2 text-sm font-medium'
                  title={group}
                >
                  {group}
                </span>
                <div className='flex min-w-0 flex-col gap-2'>
                  <GroupNameSelect
                    options={groups.filter(
                      (name) => name !== group && !list.includes(name)
                    )}
                    value={null}
                    className='w-full sm:w-64'
                    placeholder={
                      list.length === 0 ? t('No fallback') : t('Add group')
                    }
                    ariaLabel={t('Add fallback group for {{group}}', {
                      group,
                    })}
                    onValueChange={(name) => update(group, [...list, name])}
                  />
                  {list.length > 0 && (
                    <Reorder.Group
                      as='ol'
                      axis='y'
                      values={list}
                      onReorder={(next) => update(group, next)}
                      aria-label={t('Fallback groups for {{group}}', {
                        group,
                      })}
                      className='flex flex-col gap-2'
                    >
                      {list.map((name, index) => (
                        <AutoGroupOrderItem
                          key={name}
                          group={name}
                          index={index}
                          count={list.length}
                          onMove={move}
                          onRemove={(removed) =>
                            update(
                              group,
                              list.filter((item) => item !== removed)
                            )
                          }
                          leading={
                            <span className='text-muted-foreground w-5 shrink-0 text-center text-sm tabular-nums'>
                              {formatNumber(index + 1, locale)}
                            </span>
                          }
                        >
                          {!groups.includes(name) && <UnknownGroupBadge />}
                        </AutoGroupOrderItem>
                      ))}
                    </Reorder.Group>
                  )}
                </div>
              </div>
            )
          })
        )}
      </CardContent>
    </Card>
  )
}

type GroupPricingTableProps = {
  groupRatio: string
  userUsableGroups: string
  topupGroupRatio: string
  /** JSON array of group names; the row order users see groups in. */
  groupDisplayOrder: string
  /** JSON array of groups that retry 502/504/524 on another channel. */
  networkRetryGroups: string
  hasUnsavedChanges: boolean
  onChange: (field: string, value: string) => void
  onShowDetail: (name: string) => void
}

function GroupPricingTable({
  groupRatio,
  userUsableGroups,
  topupGroupRatio,
  groupDisplayOrder,
  networkRetryGroups,
  hasUnsavedChanges,
  onChange,
  onShowDetail,
}: GroupPricingTableProps) {
  const { t } = useTranslation()
  const networkRetryList = useMemo(
    () =>
      safeJsonParse<string[]>(networkRetryGroups, {
        fallback: [],
        context: 'network retry groups',
      }),
    [networkRetryGroups]
  )
  const emitNetworkRetry = useCallback(
    (next: string[]) =>
      onChange('GroupNetworkRetryGroups', JSON.stringify(next, null, 2)),
    [onChange]
  )
  const [search, setSearch] = useState('')
  const [renamingGroup, setRenamingGroup] = useState<string | null>(null)
  const [rows, setRows] = useState<GroupPricingRow[]>(() =>
    orderGroupPricingRows(
      buildGroupPricingRows(groupRatio, userUsableGroups, topupGroupRatio),
      groupDisplayOrder
    )
  )

  useEffect(() => {
    const incomingSignature = sourceGroupPricingSignature(
      groupRatio,
      userUsableGroups,
      topupGroupRatio
    )
    setRows((currentRows) => {
      if (groupPricingSignature(currentRows) === incomingSignature) {
        // Our own edits echo back unchanged; only re-sort when the order
        // changed elsewhere (form reset, JSON mode), so rows never jump
        // while a name is being retyped.
        const currentOrder = JSON.stringify(
          currentRows.map((row) => row.name.trim()).filter(Boolean)
        )
        return currentOrder === normalizeOrderJson(groupDisplayOrder)
          ? currentRows
          : orderGroupPricingRows(currentRows, groupDisplayOrder)
      }
      return orderGroupPricingRows(
        buildGroupPricingRows(groupRatio, userUsableGroups, topupGroupRatio),
        groupDisplayOrder
      )
    })
  }, [groupRatio, userUsableGroups, topupGroupRatio, groupDisplayOrder])

  const emitRows = useCallback(
    (nextRows: GroupPricingRow[]) => {
      setRows(nextRows)
      const serialized = serializeGroupPricingRows(nextRows)
      onChange('GroupRatio', serialized.GroupRatio)
      onChange('UserUsableGroups', serialized.UserUsableGroups)
      onChange('TopupGroupRatio', serialized.TopupGroupRatio)
      // Row order is the display order, so renames and deletes stay in sync.
      onChange(
        'GroupDisplayOrder',
        JSON.stringify(
          nextRows.map((row) => row.name.trim()).filter(Boolean),
          null,
          2
        )
      )
    },
    [onChange]
  )

  const moveRow = useCallback(
    (id: string, offset: -1 | 1) => {
      const from = rows.findIndex((row) => row._id === id)
      const to = from + offset
      if (from < 0 || to < 0 || to >= rows.length) return
      const nextRows = [...rows]
      ;[nextRows[from], nextRows[to]] = [nextRows[to], nextRows[from]]
      emitRows(nextRows)
    },
    [emitRows, rows]
  )

  const updateRow = useCallback(
    (
      id: string,
      field: Exclude<keyof GroupPricingRow, '_id'>,
      value: string | number | boolean
    ) => {
      const previousName = rows.find((row) => row._id === id)?.name.trim()
      emitRows(
        rows.map((row) => (row._id === id ? { ...row, [field]: value } : row))
      )
      // The network retry switch follows a renamed group, including through
      // an empty name while the old one is deleted and a new one typed.
      if (
        field === 'name' &&
        previousName !== undefined &&
        networkRetryList.includes(previousName)
      ) {
        emitNetworkRetry(
          networkRetryList.map((name) =>
            name === previousName ? String(value).trim() : name
          )
        )
      }
    },
    [emitRows, rows, networkRetryList, emitNetworkRetry]
  )

  const addRow = useCallback(() => {
    setSearch('')
    const existingNames = new Set(rows.map((row) => row.name))
    let index = 1
    let name = `group_${index}`
    while (existingNames.has(name)) {
      index += 1
      name = `group_${index}`
    }
    emitRows([
      ...rows,
      {
        _id: createGroupPricingId(),
        isNew: true,
        name,
        ratio: '1',
        topupRatio: '',
        selectable: true,
        description: '',
      },
    ])
  }, [emitRows, rows])

  const removeRow = useCallback(
    (id: string) => {
      const removedName = rows.find((row) => row._id === id)?.name.trim()
      emitRows(rows.filter((row) => row._id !== id))
      if (removedName !== undefined && networkRetryList.includes(removedName)) {
        emitNetworkRetry(
          networkRetryList.filter((name) => name !== removedName)
        )
      }
    },
    [emitRows, rows, networkRetryList, emitNetworkRetry]
  )

  const duplicateNames = useMemo(() => {
    const counts = new Map<string, number>()
    for (const row of rows) {
      const name = row.name.trim()
      if (!name) continue
      counts.set(name, (counts.get(name) ?? 0) + 1)
    }
    return [...counts.entries()]
      .filter(([, count]) => count > 1)
      .map(([name]) => name)
  }, [rows])

  // Groups are bound by name, so a renamed or removed group strands every
  // channel, token and user still on the old name. Compare against the
  // draft rows so the warning appears before the change is saved.
  const usageQuery = useQuery({
    queryKey: ['group-usage'],
    queryFn: async () => requireServerSuccess(await getGroupUsage()).data ?? [],
  })
  const usageByName = useMemo(
    () => new Map((usageQuery.data ?? []).map((u) => [u.name, u])),
    [usageQuery.data]
  )
  const strandedGroups = useMemo(() => {
    const names = new Set(rows.map((row) => row.name.trim()))
    return (usageQuery.data ?? [])
      .filter((u) => u.name !== 'auto' && !names.has(u.name))
      .sort((a, b) => a.name.localeCompare(b.name))
  }, [rows, usageQuery.data])

  const query = search.trim().toLowerCase()
  const visibleRows = rows.filter(
    (row) =>
      !query ||
      row.name.toLowerCase().includes(query) ||
      row.description.toLowerCase().includes(query)
  )

  return (
    <Card className={sectionCardClassName}>
      <CardHeader className={sectionHeaderClassName}>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
          <div>
            <CardTitle>{t('Pricing groups')}</CardTitle>
            <CardDescription>
              {t(
                'All group names live here. Ratio applies when calls are billed as this group; top-up ratio applies to users whose account is in this group.'
              )}
              <span className='mt-1 block'>
                {t(
                  'Network retry: when an upstream returns 502, 504 or 524, the request moves to an untried channel of the same group. It works without fallback groups; if the group has fallback groups and the token enables fallback group retry, the request moves on to them after every channel of the group fails. A timed-out upstream may already have run the request, so upstream costs can repeat; users are billed once.'
                )}
              </span>
            </CardDescription>
          </div>
          <Button onClick={addRow} size='sm' className='sm:self-start'>
            <Plus className='mr-2 h-4 w-4' />
            {t('Add group')}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className='flex min-w-0 flex-col gap-4'>
          <InputGroup className='max-w-sm'>
            <InputGroupAddon>
              <Search aria-hidden='true' />
            </InputGroupAddon>
            <InputGroupInput
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              aria-label={t('Search groups by name or description')}
              placeholder={t('Search groups by name or description')}
            />
            {search && (
              <InputGroupAddon align='inline-end'>
                <InputGroupButton
                  size='icon-xs'
                  aria-label={t('Clear search')}
                  onClick={() => setSearch('')}
                >
                  <X aria-hidden='true' />
                </InputGroupButton>
              </InputGroupAddon>
            )}
          </InputGroup>
          {strandedGroups.length > 0 && (
            <Alert variant='destructive'>
              <AlertTriangle className='h-4 w-4' />
              <AlertTitle>
                {t('Groups in use but missing from this table')}
              </AlertTitle>
              <AlertDescription>
                <p>
                  {t(
                    'These group names are still set on channels, tokens or users. Requests using them fail. Rename them back here, or update the channels, tokens and users.'
                  )}
                </p>
                <ul className='mt-2 list-disc space-y-0.5 pl-5'>
                  {strandedGroups.map((u) => (
                    <li key={u.name}>
                      <span className='font-mono font-medium'>{u.name}</span>
                      {' — '}
                      {t(
                        '{{channels}} channels ({{enabled}} enabled), {{tokens}} tokens, {{users}} users',
                        {
                          channels: u.channels,
                          enabled: u.enabled_channels,
                          tokens: u.tokens,
                          users: u.users,
                        }
                      )}
                    </li>
                  ))}
                </ul>
              </AlertDescription>
            </Alert>
          )}
          <StaticDataTable
            tableClassName='min-w-[860px]'
            tableProps={{ 'aria-label': t('Pricing groups') }}
            data={visibleRows}
            getRowKey={(row) => row._id}
            emptyClassName='text-muted-foreground h-20 text-sm'
            emptyContent={
              <EmptyState
                className='min-h-40'
                title={
                  query
                    ? t('No results found')
                    : t('No groups yet. Add a group to get started.')
                }
                action={
                  query ? (
                    <Button
                      variant='outline'
                      size='sm'
                      onClick={() => setSearch('')}
                    >
                      {t('Clear search')}
                    </Button>
                  ) : undefined
                }
              />
            }
            columns={[
              {
                id: 'group',
                header: t('Group name'),
                className: 'min-w-40',
                cell: (row) => (
                  // Existing names are read-only: groups are bound by name, so
                  // they change through Rename, which updates every reference.
                  <Input
                    value={row.name}
                    aria-label={t('Group name')}
                    readOnly={!row.isNew}
                    title={
                      row.isNew
                        ? undefined
                        : t(
                            'Use the rename button to change an existing group name'
                          )
                    }
                    className={row.isNew ? undefined : 'bg-muted/40'}
                    onChange={(event) =>
                      updateRow(row._id, 'name', event.target.value)
                    }
                    aria-invalid={duplicateNames.includes(row.name.trim())}
                  />
                ),
              },
              {
                id: 'ratio',
                header: t('Ratio'),
                className: 'w-28',
                cell: (row) => (
                  <Input
                    type='number'
                    min={0}
                    step={0.0001}
                    value={row.ratio}
                    aria-label={t('Ratio')}
                    onChange={(event) =>
                      updateRow(row._id, 'ratio', event.target.value)
                    }
                  />
                ),
              },
              {
                id: 'topup-ratio',
                header: t('Top-up ratio'),
                className: 'w-28',
                cell: (row) => (
                  <Input
                    type='number'
                    min={0}
                    step={0.0001}
                    value={row.topupRatio}
                    aria-label={t('Top-up ratio')}
                    placeholder={t('Not set')}
                    onChange={(event) =>
                      updateRow(row._id, 'topupRatio', event.target.value)
                    }
                  />
                ),
              },
              {
                id: 'selectable',
                header: t('User selectable'),
                className: 'w-28 text-center',
                cell: (row) => (
                  <div className='flex justify-center'>
                    <Checkbox
                      checked={row.selectable}
                      onCheckedChange={(checked) =>
                        updateRow(row._id, 'selectable', checked === true)
                      }
                      aria-label={t('User selectable')}
                    />
                  </div>
                ),
              },
              {
                id: 'network-retry',
                header: (
                  <span
                    title={t(
                      'Retry upstream 502, 504 and 524 on an untried channel of this group, then on its fallback groups when the token allows it.'
                    )}
                  >
                    {t('Network retry')}
                  </span>
                ),
                className: 'w-28 text-center',
                cell: (row) => {
                  const name = row.name.trim()
                  return (
                    <div className='flex justify-center'>
                      <Switch
                        size='sm'
                        disabled={!name}
                        checked={!!name && networkRetryList.includes(name)}
                        onCheckedChange={(checked) =>
                          emitNetworkRetry(
                            checked
                              ? [
                                  ...networkRetryList.filter((g) => g !== name),
                                  name,
                                ]
                              : networkRetryList.filter((g) => g !== name)
                          )
                        }
                        aria-label={t('Network retry for {{group}}', {
                          group: name,
                        })}
                      />
                    </div>
                  )
                },
              },
              {
                id: 'channels',
                header: t('Usable channels'),
                className: 'w-28 text-center',
                cellClassName: 'text-center',
                cell: (row) => {
                  if (!usageQuery.data) {
                    return <span className='text-muted-foreground'>-</span>
                  }
                  const usage = usageByName.get(row.name.trim())
                  const enabled = usage?.enabled_channels ?? 0
                  const total = usage?.channels ?? 0
                  if (enabled === 0) {
                    return (
                      <span
                        title={t(
                          'No enabled channel serves this group; requests using it fail.'
                        )}
                      >
                        <StatusBadge
                          label={t('None ({{enabled}}/{{total}})', {
                            enabled,
                            total,
                          })}
                          variant='danger'
                          copyable={false}
                        />
                      </span>
                    )
                  }
                  return (
                    <span className='tabular-nums'>
                      {enabled}/{total}
                    </span>
                  )
                },
              },
              {
                id: 'description',
                header: t('Description'),
                className: 'min-w-56',
                cell: (row) =>
                  row.selectable ? (
                    <Input
                      value={row.description}
                      aria-label={t('Group description')}
                      placeholder={t('Group description')}
                      onChange={(event) =>
                        updateRow(row._id, 'description', event.target.value)
                      }
                    />
                  ) : (
                    <span className='text-muted-foreground px-3 text-sm'>
                      -
                    </span>
                  ),
              },
              {
                id: 'actions',
                header: t('Actions'),
                className: 'text-right',
                cellClassName: 'text-right',
                cell: (row) => (
                  <div className='flex justify-end gap-1'>
                    <Button
                      variant='ghost'
                      size='sm'
                      onClick={() => moveRow(row._id, -1)}
                      disabled={Boolean(query) || rows[0]?._id === row._id}
                      aria-label={t('Move {{group}} up', {
                        group: row.name.trim(),
                      })}
                    >
                      <ArrowUp className='h-4 w-4' />
                    </Button>
                    <Button
                      variant='ghost'
                      size='sm'
                      onClick={() => moveRow(row._id, 1)}
                      disabled={Boolean(query) || rows.at(-1)?._id === row._id}
                      aria-label={t('Move {{group}} down', {
                        group: row.name.trim(),
                      })}
                    >
                      <ArrowDown className='h-4 w-4' />
                    </Button>
                    {!row.isNew && (
                      <Button
                        variant='ghost'
                        size='sm'
                        onClick={() => setRenamingGroup(row.name.trim())}
                        disabled={hasUnsavedChanges}
                        title={
                          hasUnsavedChanges
                            ? t('Save or discard your changes before renaming')
                            : undefined
                        }
                        aria-label={t('Rename {{group}}', {
                          group: row.name.trim(),
                        })}
                      >
                        <Pencil className='h-4 w-4' />
                      </Button>
                    )}
                    <Button
                      variant='ghost'
                      size='sm'
                      onClick={() => onShowDetail(row.name.trim())}
                      disabled={!row.name.trim()}
                      aria-label={t('Details')}
                    >
                      <Info className='h-4 w-4' />
                    </Button>
                    <Button
                      variant='ghost'
                      size='sm'
                      onClick={() => removeRow(row._id)}
                      aria-label={t('Delete')}
                    >
                      <Trash2 className='h-4 w-4' />
                    </Button>
                  </div>
                ),
              },
            ]}
          />

          <GroupRenameDialog
            groupName={renamingGroup}
            usage={renamingGroup ? usageByName.get(renamingGroup) : undefined}
            onOpenChange={(open) => {
              if (!open) setRenamingGroup(null)
            }}
          />

          {duplicateNames.length > 0 && (
            <p className='text-destructive text-sm'>
              {t('Duplicate group names: {{names}}', {
                names: duplicateNames.join(', '),
              })}
            </p>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

type GroupOverride = {
  targetGroup: string
  ratio: number
}

type GroupOverrideRulesProps = {
  registry: RegistryEntry[]
  groupGroupRatio: string
  onChange: (field: string, value: string) => void
}

function GroupOverrideRules({
  registry,
  groupGroupRatio,
  onChange,
}: GroupOverrideRulesProps) {
  const { t } = useTranslation()
  const [userGroupDialogOpen, setUserGroupDialogOpen] = useState(false)
  const [userGroupInput, setUserGroupInput] = useState<string | null>(null)
  const [overrideDialogOpen, setOverrideDialogOpen] = useState(false)
  const [overrideUserGroup, setOverrideUserGroup] = useState<string | null>(
    null
  )
  const [overrideEditData, setOverrideEditData] =
    useState<GroupOverride | null>(null)

  const registryNames = useMemo(
    () => registry.map((entry) => entry.name),
    [registry]
  )

  const baseRatioByName = useMemo(() => {
    const map = new Map<string, number>()
    for (const entry of registry) map.set(entry.name, entry.ratio)
    return map
  }, [registry])

  const groupGroupRatioList = useMemo(() => {
    const map = parseNestedRatioMap(groupGroupRatio)
    return Object.entries(map).map(([userGroup, overrides]) => ({
      userGroup,
      overrides: Object.entries(overrides).map(([targetGroup, ratio]) => ({
        targetGroup,
        ratio,
      })),
    }))
  }, [groupGroupRatio])

  const emitMap = useCallback(
    (map: Record<string, Record<string, number>>) => {
      onChange('GroupGroupRatio', JSON.stringify(map, null, 2))
    },
    [onChange]
  )

  const handleUserGroupSave = useCallback(() => {
    if (!userGroupInput) return
    const map = parseNestedRatioMap(groupGroupRatio)
    if (!map[userGroupInput]) {
      map[userGroupInput] = {}
    }
    emitMap(map)
    setUserGroupDialogOpen(false)
    setUserGroupInput(null)
  }, [userGroupInput, groupGroupRatio, emitMap])

  const handleUserGroupDelete = useCallback(
    (userGroup: string) => {
      const map = parseNestedRatioMap(groupGroupRatio)
      delete map[userGroup]
      emitMap(map)
    },
    [groupGroupRatio, emitMap]
  )

  const handleOverrideAdd = useCallback((userGroup: string) => {
    setOverrideUserGroup(userGroup)
    setOverrideEditData(null)
    setOverrideDialogOpen(true)
  }, [])

  const handleOverrideEdit = useCallback(
    (userGroup: string, override: GroupOverride) => {
      setOverrideUserGroup(userGroup)
      setOverrideEditData(override)
      setOverrideDialogOpen(true)
    },
    []
  )

  const handleOverrideSave = useCallback(
    (targetGroup: string, ratio: number, oldTargetGroup?: string) => {
      if (!overrideUserGroup) return
      const map = parseNestedRatioMap(groupGroupRatio)
      if (!map[overrideUserGroup]) {
        map[overrideUserGroup] = {}
      }
      if (oldTargetGroup && oldTargetGroup !== targetGroup) {
        delete map[overrideUserGroup][oldTargetGroup]
      }
      map[overrideUserGroup][targetGroup] = ratio
      emitMap(map)
      setOverrideDialogOpen(false)
    },
    [overrideUserGroup, groupGroupRatio, emitMap]
  )

  const handleOverrideDelete = useCallback(
    (userGroup: string, targetGroup: string) => {
      const map = parseNestedRatioMap(groupGroupRatio)
      if (map[userGroup]) {
        delete map[userGroup][targetGroup]
        if (Object.keys(map[userGroup]).length === 0) {
          delete map[userGroup]
        }
      }
      emitMap(map)
    },
    [groupGroupRatio, emitMap]
  )

  return (
    <Card className={sectionCardClassName}>
      <CardHeader className={sectionHeaderClassName}>
        <CardTitle>{t('Special ratio rules')}</CardTitle>
        <CardDescription>
          {t(
            'Each rule reads as a sentence: users of one group pay a special ratio when billed as another group. Without a rule, the billing group base ratio applies.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className='space-y-4'>
          <Button
            onClick={() => {
              setUserGroupInput(null)
              setUserGroupDialogOpen(true)
            }}
            size='sm'
          >
            <Plus className='mr-2 h-4 w-4' />
            {t('Add user group')}
          </Button>
          {groupGroupRatioList.length === 0 && (
            <EmptyState
              className='min-h-40'
              title={t('No special ratios configured')}
              description={t(
                'Base group ratios apply until you add an override.'
              )}
            />
          )}
          {groupGroupRatioList.length > 0 && (
            <div className='space-y-3'>
              {groupGroupRatioList.map((userGroupData) => (
                <Collapsible key={userGroupData.userGroup} defaultOpen>
                  <div className='rounded-lg border'>
                    <div className='flex items-center justify-between gap-2 p-3'>
                      <CollapsibleTrigger
                        render={
                          <Button
                            variant='ghost'
                            className='h-auto min-w-0 justify-start whitespace-normal'
                          />
                        }
                        aria-label={t('Rules for {{group}}', {
                          group: userGroupData.userGroup,
                        })}
                      >
                        <ChevronDown
                          className='size-4 shrink-0'
                          aria-hidden='true'
                        />
                        <span className='min-w-0 truncate font-semibold'>
                          {userGroupData.userGroup}
                        </span>
                        {!registryNames.includes(userGroupData.userGroup) && (
                          <AlertTriangle
                            className='text-destructive h-4 w-4'
                            aria-label={t('Not in pricing table')}
                          />
                        )}
                        <span className='text-muted-foreground text-sm'>
                          {t('{{count}} override', {
                            count: userGroupData.overrides.length,
                          })}
                        </span>
                      </CollapsibleTrigger>
                      <div className='flex shrink-0 gap-1'>
                        <Button
                          variant='ghost'
                          size='sm'
                          aria-label={t('Add ratio override')}
                          onClick={() =>
                            handleOverrideAdd(userGroupData.userGroup)
                          }
                        >
                          <Plus className='h-4 w-4' />
                        </Button>
                        <Button
                          variant='ghost'
                          size='sm'
                          aria-label={t('Remove {{group}}', {
                            group: userGroupData.userGroup,
                          })}
                          onClick={() =>
                            handleUserGroupDelete(userGroupData.userGroup)
                          }
                        >
                          <Trash2 className='h-4 w-4' />
                        </Button>
                      </div>
                    </div>
                    <CollapsibleContent>
                      {userGroupData.overrides.length > 0 && (
                        <div className='border-t'>
                          <StaticDataTable
                            className='rounded-none border-0'
                            data={userGroupData.overrides}
                            getRowKey={(override) => override.targetGroup}
                            columns={[
                              {
                                id: 'target-group',
                                header: t('Billing group'),
                                cellClassName: 'font-medium',
                                cell: (override) => (
                                  <span className='inline-flex max-w-48 items-center gap-1.5'>
                                    <span
                                      className='truncate'
                                      title={override.targetGroup}
                                    >
                                      {override.targetGroup}
                                    </span>
                                    {!registryNames.includes(
                                      override.targetGroup
                                    ) && (
                                      <AlertTriangle
                                        className='text-destructive h-3.5 w-3.5'
                                        aria-label={t('Not in pricing table')}
                                      />
                                    )}
                                  </span>
                                ),
                              },
                              {
                                id: 'ratio',
                                header: t('Ratio'),
                                cell: (override) => {
                                  const baseRatio = baseRatioByName.get(
                                    override.targetGroup
                                  )
                                  return (
                                    <span className='inline-flex items-center gap-1.5'>
                                      {override.ratio}
                                      {baseRatio !== undefined &&
                                        baseRatio !== override.ratio && (
                                          <span className='text-muted-foreground text-xs'>
                                            {t('(instead of {{ratio}})', {
                                              ratio: baseRatio,
                                            })}
                                          </span>
                                        )}
                                    </span>
                                  )
                                },
                              },
                              {
                                id: 'actions',
                                header: t('Actions'),
                                className: 'text-right',
                                cellClassName: 'text-right',
                                cell: (override) => (
                                  <StaticRowActions
                                    editLabel={t('Edit')}
                                    deleteLabel={t('Delete')}
                                    menuLabel={t('Open menu')}
                                    onEdit={() =>
                                      handleOverrideEdit(
                                        userGroupData.userGroup,
                                        override
                                      )
                                    }
                                    onDelete={() =>
                                      handleOverrideDelete(
                                        userGroupData.userGroup,
                                        override.targetGroup
                                      )
                                    }
                                  />
                                ),
                              },
                            ]}
                          />
                        </div>
                      )}
                    </CollapsibleContent>
                  </div>
                </Collapsible>
              ))}
            </div>
          )}
        </div>
      </CardContent>

      {/* Add user group dialog */}
      <Dialog
        open={userGroupDialogOpen}
        onOpenChange={setUserGroupDialogOpen}
        title={t('Add user group')}
        description={t(
          'Create a new user group to configure ratio overrides for.'
        )}
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              variant='outline'
              onClick={() => setUserGroupDialogOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={handleUserGroupSave} disabled={!userGroupInput}>
              {t('Add')}
            </Button>
          </>
        }
      >
        <div className='space-y-4 py-4'>
          <div className='space-y-2'>
            <Label>{t('User group name')}</Label>
            <GroupNameSelect
              className='w-full'
              options={registryNames}
              value={userGroupInput}
              placeholder={t('Select a group')}
              onValueChange={setUserGroupInput}
            />
          </div>
        </div>
      </Dialog>

      <GroupOverrideDialog
        open={overrideDialogOpen}
        onOpenChange={setOverrideDialogOpen}
        onSave={handleOverrideSave}
        editData={overrideEditData}
        userGroup={overrideUserGroup}
        groupOptions={registryNames}
        baseRatioByName={baseRatioByName}
      />
    </Card>
  )
}

type GroupOverrideDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (targetGroup: string, ratio: number, oldTargetGroup?: string) => void
  editData: GroupOverride | null
  userGroup: string | null
  groupOptions: string[]
  baseRatioByName: Map<string, number>
}

function GroupOverrideDialog({
  open,
  onOpenChange,
  onSave,
  editData,
  userGroup,
  groupOptions,
  baseRatioByName,
}: GroupOverrideDialogProps) {
  const { t } = useTranslation()
  const [targetGroup, setTargetGroup] = useState<string | null>(null)
  const [ratio, setRatio] = useState('')

  useEffect(() => {
    if (!open) {
      setTargetGroup(null)
      setRatio('')
      return
    }

    setTargetGroup(editData?.targetGroup ?? null)
    setRatio(editData ? String(editData.ratio) : '')
  }, [editData, open])

  const baseRatio = targetGroup ? baseRatioByName.get(targetGroup) : undefined

  const handleSave = () => {
    if (!targetGroup || !ratio.trim()) return
    const parsedRatio = Number.parseFloat(ratio)
    if (Number.isNaN(parsedRatio)) return

    onSave(targetGroup, parsedRatio, editData?.targetGroup)
    setTargetGroup(null)
    setRatio('')
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={editData ? t('Edit ratio override') : t('Add ratio override')}
      description={
        userGroup
          ? t(
              'Configure a custom ratio for "{{userGroup}}" users when using a specific token group.',
              { userGroup }
            )
          : t(
              'Configure a custom ratio for when users use a specific token group.'
            )
      }
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleSave}>
            {editData ? t('Update') : t('Add')}
          </Button>
        </>
      }
    >
      <div className='space-y-4 py-4'>
        <div className='space-y-2'>
          <Label>{t('Billing group')}</Label>
          <GroupNameSelect
            className='w-full'
            options={groupOptions}
            value={targetGroup}
            placeholder={t('Select a group')}
            onValueChange={setTargetGroup}
          />
          <p className='text-muted-foreground text-xs'>
            {t('The token group that will have a custom ratio')}
          </p>
        </div>
        <div className='space-y-2'>
          <Label>{t('Ratio')}</Label>
          <Input
            value={ratio}
            onChange={(e) => {
              const val = e.target.value
              if (val === '' || !Number.isNaN(Number.parseFloat(val))) {
                setRatio(val)
              }
            }}
            placeholder={baseRatio === undefined ? '0.9' : String(baseRatio)}
          />
          <p className='text-muted-foreground text-xs'>
            {baseRatio !== undefined
              ? t('(instead of {{ratio}})', { ratio: baseRatio })
              : t(
                  'Multiplier applied when {{userGroup}} uses {{targetGroup}}',
                  {
                    userGroup: userGroup || t('this user group'),
                    targetGroup: targetGroup || t('this token group'),
                  }
                )}
          </p>
        </div>
      </div>
    </Dialog>
  )
}

type GroupDetailSheetProps = {
  groupName: string | null
  onOpenChange: (open: boolean) => void
  registry: RegistryEntry[]
  topupGroupRatio: string
  userUsableGroups: string
  groupGroupRatio: string
  autoGroups: string[]
  groupSpecialUsableGroup: string
}

type VisibilityRule = {
  userGroup: string
  visible: boolean
  description: string
}

function parseSpecialGroupKey(rawKey: string): {
  visible: boolean
  groupName: string
} {
  if (rawKey.startsWith('-:')) {
    return { visible: false, groupName: rawKey.slice(2) }
  }
  if (rawKey.startsWith('+:')) {
    return { visible: true, groupName: rawKey.slice(2) }
  }
  return { visible: true, groupName: rawKey }
}

function GroupDetailSheet(props: GroupDetailSheetProps) {
  const { t } = useTranslation()
  const name = props.groupName

  const detail = useMemo(() => {
    if (!name) return null

    const entry = props.registry.find((item) => item.name === name)
    const topupMap = parseRatioMap(props.topupGroupRatio)
    const usableMap = parseUsableMap(props.userUsableGroups)
    const overrideMap = parseNestedRatioMap(props.groupGroupRatio)
    const specialMap = safeJsonParse<Record<string, Record<string, string>>>(
      props.groupSpecialUsableGroup,
      { fallback: {}, silent: true }
    )

    // Overrides that apply when other user groups bill as this group
    const incomingOverrides: { userGroup: string; ratio: number }[] = []
    for (const [userGroup, overrides] of Object.entries(overrideMap)) {
      if (Object.hasOwn(overrides, name)) {
        incomingOverrides.push({ userGroup, ratio: overrides[name] })
      }
    }

    // Overrides that apply when users of this group bill as other groups
    const outgoingOverrides = Object.entries(overrideMap[name] ?? {}).map(
      ([targetGroup, ratio]) => ({ targetGroup, ratio })
    )

    // Visibility rules targeting this group
    const visibilityRules: VisibilityRule[] = []
    for (const [userGroup, inner] of Object.entries(specialMap)) {
      if (typeof inner !== 'object' || inner === null) continue
      for (const [rawKey, desc] of Object.entries(inner)) {
        const parsed = parseSpecialGroupKey(rawKey)
        if (parsed.groupName !== name) continue
        visibilityRules.push({
          userGroup,
          visible: parsed.visible,
          description: typeof desc === 'string' ? desc : '',
        })
      }
    }

    const autoIndex = props.autoGroups.indexOf(name)

    return {
      ratio: entry?.ratio,
      topupRatio: Object.hasOwn(topupMap, name) ? String(topupMap[name]) : null,
      selectable: Object.hasOwn(usableMap, name),
      description: String(usableMap[name] ?? ''),
      incomingOverrides,
      outgoingOverrides,
      visibilityRules,
      autoIndex,
    }
  }, [
    name,
    props.registry,
    props.topupGroupRatio,
    props.userUsableGroups,
    props.groupGroupRatio,
    props.autoGroups,
    props.groupSpecialUsableGroup,
  ])

  return (
    <Sheet open={name !== null} onOpenChange={props.onOpenChange}>
      <SheetContent
        side='right'
        className={sideDrawerContentClassName('sm:max-w-lg')}
      >
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {t('Group details')}
            {name ? `: ${name}` : ''}
          </SheetTitle>
          <SheetDescription>
            {t('Everything configured for this group, in one place.')}
          </SheetDescription>
        </SheetHeader>

        {detail && (
          <div className={sideDrawerFormClassName('gap-5')}>
            <section className='space-y-2'>
              <h3 className='text-sm font-semibold'>{t('Overview')}</h3>
              <dl className='space-y-1.5 text-sm'>
                <div className='flex justify-between'>
                  <dt className='text-muted-foreground'>{t('Ratio')}</dt>
                  <dd className='font-medium'>{detail.ratio ?? '-'}</dd>
                </div>
                <div className='flex justify-between'>
                  <dt className='text-muted-foreground'>{t('Top-up ratio')}</dt>
                  <dd className='font-medium'>
                    {detail.topupRatio ?? t('Not set')}
                  </dd>
                </div>
                <div className='flex justify-between'>
                  <dt className='text-muted-foreground'>
                    {t('User selectable')}
                  </dt>
                  <dd className='font-medium'>
                    {detail.selectable ? t('Yes') : t('No')}
                  </dd>
                </div>
                {detail.selectable && detail.description && (
                  <div className='flex justify-between gap-4'>
                    <dt className='text-muted-foreground'>
                      {t('Description')}
                    </dt>
                    <dd className='text-right font-medium'>
                      {detail.description}
                    </dd>
                  </div>
                )}
                <div className='flex justify-between'>
                  <dt className='text-muted-foreground'>
                    {t('Auto group order')}
                  </dt>
                  <dd className='font-medium'>
                    {detail.autoIndex >= 0
                      ? t('Position {{position}}', {
                          position: detail.autoIndex + 1,
                        })
                      : t('Not included')}
                  </dd>
                </div>
              </dl>
            </section>

            <section className='space-y-2'>
              <h3 className='text-sm font-semibold'>
                {t('Ratio overrides when billed as this group')}
              </h3>
              {detail.incomingOverrides.length === 0 ? (
                <p className='text-muted-foreground text-sm'>{t('None')}</p>
              ) : (
                <ul className='space-y-1 text-sm'>
                  {detail.incomingOverrides.map((item) => (
                    <li
                      key={item.userGroup}
                      className='flex justify-between rounded-md border px-3 py-1.5'
                    >
                      <span>
                        {t('Users in {{group}}', { group: item.userGroup })}
                      </span>
                      <span className='font-medium'>{item.ratio}</span>
                    </li>
                  ))}
                </ul>
              )}
            </section>

            <section className='space-y-2'>
              <h3 className='text-sm font-semibold'>
                {t('Ratio overrides for users of this group')}
              </h3>
              {detail.outgoingOverrides.length === 0 ? (
                <p className='text-muted-foreground text-sm'>{t('None')}</p>
              ) : (
                <ul className='space-y-1 text-sm'>
                  {detail.outgoingOverrides.map((item) => (
                    <li
                      key={item.targetGroup}
                      className='flex justify-between rounded-md border px-3 py-1.5'
                    >
                      <span>
                        {t('When billed as {{group}}', {
                          group: item.targetGroup,
                        })}
                      </span>
                      <span className='font-medium'>{item.ratio}</span>
                    </li>
                  ))}
                </ul>
              )}
            </section>

            <section className='space-y-2'>
              <h3 className='text-sm font-semibold'>
                {t('Special visibility rules')}
              </h3>
              {detail.visibilityRules.length === 0 ? (
                <p className='text-muted-foreground text-sm'>{t('None')}</p>
              ) : (
                <ul className='space-y-1 text-sm'>
                  {detail.visibilityRules.map((rule) => (
                    <li
                      key={`${rule.userGroup}-${rule.visible}`}
                      className='flex items-center justify-between rounded-md border px-3 py-1.5'
                    >
                      <span>
                        {rule.visible
                          ? t('Extra visible to {{group}}', {
                              group: rule.userGroup,
                            })
                          : t('Hidden from {{group}}', {
                              group: rule.userGroup,
                            })}
                      </span>
                      <StatusBadge
                        variant={rule.visible ? 'info' : 'danger'}
                        copyable={false}
                      >
                        {rule.visible ? t('Visible') : t('Hidden')}
                      </StatusBadge>
                    </li>
                  ))}
                </ul>
              )}
            </section>
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}
