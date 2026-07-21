import type { ResourceRow } from '@/api/management'

export type ResourceTreeRow = ResourceRow & { children?: ResourceTreeRow[] }

function compareResourceRows(left: ResourceRow, right: ResourceRow) {
  const leftOrder = Number(left.sort_order ?? 0)
  const rightOrder = Number(right.sort_order ?? 0)
  if (leftOrder !== rightOrder) return leftOrder - rightOrder
  return Number(left.id ?? 0) - Number(right.id ?? 0)
}

/** 将平铺资源记录组装为按 sort_order 排序的树。 */
export function buildResourceTree(rows: ResourceRow[]): ResourceTreeRow[] {
  const children = new Map<string, ResourceRow[]>()
  for (const row of rows) {
    const parent = String(row.parent_id ?? '0')
    children.set(parent, [...(children.get(parent) ?? []), row])
  }
  const build = (parent: string): ResourceTreeRow[] =>
    [...(children.get(parent) ?? [])]
      .sort(compareResourceRows)
      .map((row) => {
        const nested = build(String(row.id))
        return nested.length > 0 ? { ...row, children: nested } : { ...row }
      })
  return build('0')
}

/** 按平台/租户视角过滤资源，并保留匹配节点的祖先链路。 */
export function filterResourceRowsByScopeSide(
  rows: ResourceRow[],
  scopeSide?: 'platform' | 'tenant'
): ResourceRow[] {
  if (!scopeSide) return rows
  const rowById = new Map(rows.map((row) => [String(row.id), row]))
  const matched = new Set<string>()
  for (const row of rows) {
    const mask = Number(row.scope_mask ?? 0)
    const fits = scopeSide === 'platform' ? (mask & 1) !== 0 : (mask & 2) !== 0
    if (fits) matched.add(String(row.id))
  }
  const visible = new Set<string>()
  for (const id of matched) {
    let current: string | undefined = id
    while (current && current !== '0') {
      visible.add(current)
      current = String(rowById.get(current)?.parent_id ?? '0')
      if (current === '0') break
    }
  }
  return rows.filter((row) => visible.has(String(row.id)))
}

/** 按关键词过滤资源，并保留匹配节点的祖先链路。 */
export function filterResourceRows(rows: ResourceRow[], keyword: string): ResourceRow[] {
  const term = keyword.trim().toLowerCase()
  if (!term) return rows
  const rowById = new Map(rows.map((row) => [String(row.id), row]))
  const matched = new Set<string>()
  for (const row of rows) {
    if (matchesResourceKeyword(row, term)) matched.add(String(row.id))
  }
  const visible = new Set<string>()
  for (const id of matched) {
    let current: string | undefined = id
    while (current && current !== '0') {
      visible.add(current)
      current = String(rowById.get(current)?.parent_id ?? '0')
      if (current === '0') break
    }
  }
  return rows.filter((row) => visible.has(String(row.id)))
}

/** 将树形资源拍平，供移动端列表展示层级缩进。 */
export function flattenResourceTree(
  rows: ResourceTreeRow[],
  depth = 0
): Array<ResourceTreeRow & { depth: number }> {
  const result: Array<ResourceTreeRow & { depth: number }> = []
  for (const row of rows) {
    result.push({ ...row, depth })
    if (row.children?.length) result.push(...flattenResourceTree(row.children, depth + 1))
  }
  return result
}

export function matchesResourceKeyword(row: ResourceRow, term: string): boolean {
  const values = [row.name, row.code, row.route_path, row.api_path, row.component_key]
  return values.some((value) => String(value ?? '').toLowerCase().includes(term))
}

/** 目录与菜单节点允许继续挂载子资源。 */
export function canAddResourceChild(row: ResourceRow) {
  const type = Number(row.type)
  return type === 1 || type === 2
}
