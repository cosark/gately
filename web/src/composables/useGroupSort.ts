// 列表分组、排序模式、多选与拖动的状态，Web 服务站点、DDNS 任务、端口转发规则共用。
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api'
import { MAX_GROUP_LEN, buildSections, flatten, moveInto, moveSection, moveStep, type MoveTarget, type Section } from '../groupSort'
import { useCollapsed } from './useCollapsed'

// DropOver 为拖动时的悬停位置：id 为条目或分组标题，after 为 true 表示插到条目之后
export interface DropOver {
  id: string
  after: boolean
}

// useSortSelect 管理排序模式、选中项与拖动状态。选中多项后拖动其中任一项，所有选中项一起移动。
export function useSortSelect() {
  const sortMode = ref(false)
  const selected = ref(new Set<string>())
  // dragIds 为正在拖动的条目（按显示顺序），over 为当前悬停的放置位置
  const dragIds = ref<string[]>([])
  const over = ref<DropOver | null>(null)

  function isSelected(id: string) {
    return selected.value.has(id)
  }

  function setSelected(ids: string[], v: boolean) {
    const next = new Set(selected.value)
    for (const id of ids) {
      if (v) {
        next.add(id)
      } else {
        next.delete(id)
      }
    }
    selected.value = next
  }

  function clearSelection() {
    selected.value = new Set()
  }

  function resetDrag() {
    dragIds.value = []
    over.value = null
  }

  function toggleSortMode() {
    sortMode.value = !sortMode.value
    clearSelection()
    resetDrag()
  }

  // dragStart 开始拖动：拖动的是选中项时带上同一范围内全部选中项，ordered 为该范围内按显示顺序排列的条目
  function dragStart(id: string, ordered: string[], e: DragEvent) {
    dragIds.value = isSelected(id) ? ordered.filter((x) => isSelected(x)) : [id]
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = 'move'
      e.dataTransfer.setData('text/plain', id)
      if (dragIds.value.length > 1) {
        setCountImage(e.dataTransfer, dragIds.value.length)
      }
    }
  }

  // dragOver 记录悬停位置：鼠标在条目下半部分时插到它之后；allowAfter 为 false 时（分组标题）只记录目标
  function dragOver(id: string, e: DragEvent, allowAfter = true) {
    if (!dragIds.value.length) {
      return
    }
    let after = false
    if (allowAfter) {
      const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
      after = e.clientY > rect.top + rect.height / 2
    }
    if (over.value?.id !== id || over.value.after !== after) {
      over.value = { id, after }
    }
  }

  function dragLeave(id: string) {
    if (over.value?.id === id) {
      over.value = null
    }
  }

  // takeDrop 取出本次拖动的条目与放置位置并清空拖动状态；放到被拖动的条目自身上时返回 null
  function takeDrop(): { ids: Set<string>; over: DropOver } | null {
    const ids = new Set(dragIds.value)
    const o = over.value
    resetDrag()
    if (!ids.size || !o || ids.has(o.id)) {
      return null
    }
    return { ids, over: o }
  }

  // dropClass 返回条目在拖动中的样式：被拖动、插入线在上方或下方
  function dropClass(id: string) {
    return {
      dragging: dragIds.value.includes(id),
      'drop-before': over.value?.id === id && !over.value.after && !dragIds.value.includes(id),
      'drop-after': over.value?.id === id && over.value.after && !dragIds.value.includes(id),
    }
  }

  return { sortMode, selected, dragIds, over, isSelected, setSelected, clearSelection, toggleSortMode, dragStart, dragOver, dragLeave, takeDrop, resetDrag, dropClass }
}

// setCountImage 拖动多项时用“N 项”小标签作为拖动图像，提示正在一起移动
function setCountImage(dt: DataTransfer, count: number) {
  const el = document.createElement('div')
  el.textContent = `× ${count}`
  el.style.cssText =
    'position:fixed;top:-100px;left:-100px;padding:4px 10px;border-radius:12px;background:var(--el-color-primary);color:#fff;font:600 13px/1.4 sans-serif'
  document.body.appendChild(el)
  dt.setDragImage(el, 10, 10)
  window.setTimeout(() => el.remove(), 0)
}

// useGroupRename 返回询问分组新名称的函数：新名称与现有的另一个分组同名时先确认合并。
// 返回 null 表示取消或名称未变
export function useGroupRename() {
  const { t } = useI18n()
  return async function askName(current: string, existing: string[]): Promise<string | null> {
    let name: string
    try {
      const r = await ElMessageBox.prompt(t('groupSort.renamePrompt'), t('groupSort.renameTitle', { name: current }), {
        inputValue: current,
        confirmButtonText: t('common.save'),
        cancelButtonText: t('common.cancel'),
        inputValidator: (v: string) => {
          const n = (v ?? '').trim()
          if (!n) {
            return t('groupSort.nameRequired')
          }
          if ([...n].length > MAX_GROUP_LEN) {
            return t('groupSort.nameTooLong', { max: MAX_GROUP_LEN })
          }
          return true
        },
      })
      name = r.value.trim()
    } catch {
      return null
    }
    if (name === current) {
      return null
    }
    if (existing.includes(name)) {
      try {
        await ElMessageBox.confirm(t('groupSort.mergeConfirm', { from: current, to: name }), t('groupSort.renameTitle', { name: current }), { type: 'warning' })
      } catch {
        return null
      }
    }
    return name
  }
}

// GroupItem 是分组列表中依次显示的一项：分组标题或条目
export type GroupItem<T> =
  | {
      kind: 'group'
      id: string
      name: string
      count: number
      collapsed: boolean
      index: number
      canUp: boolean
      canDown: boolean
      // 排序模式下分组标题的勾选状态：全部选中或部分选中
      checked: boolean
      indeterminate: boolean
      ids: string[]
    }
  | { kind: 'item'; id: string; item: T; grouped: boolean; canUp: boolean; canDown: boolean }

// displayItems 生成依次显示的分组标题与条目。没有任何命名分组时不显示标题；筛选与排序时展开全部分组
export function displayItems<T>(
  sections: Section<T>[],
  idOf: (it: T) => string,
  opts: {
    query: string
    sorting: boolean
    matches: (it: T, q: string) => boolean
    isCollapsed: (name: string) => boolean
    isSelected: (id: string) => boolean
    // groupIdPrefix 加在分组标题 id 前，同一页面有多个分组列表时区分各列表的分组
    groupIdPrefix?: string
  },
): GroupItem<T>[] {
  const q = opts.sorting ? '' : opts.query.trim().toLowerCase()
  const grouped = sections.some((s) => s.name)
  const namedCount = sections.filter((s) => s.name).length
  const out: GroupItem<T>[] = []
  sections.forEach((sec, si) => {
    const rows = q ? sec.items.filter((it) => opts.matches(it, q)) : sec.items
    if (!rows.length) {
      return
    }
    const collapsed = grouped && !q && !opts.sorting && opts.isCollapsed(sec.name)
    if (grouped) {
      const ids = rows.map(idOf)
      const n = ids.filter((id) => opts.isSelected(id)).length
      out.push({
        kind: 'group',
        id: `${opts.groupIdPrefix ?? ''}g:${sec.name}`,
        name: sec.name,
        count: rows.length,
        collapsed,
        index: si,
        canUp: !!sec.name && si > 0,
        canDown: !!sec.name && si < namedCount - 1,
        checked: n > 0 && n === ids.length,
        indeterminate: n > 0 && n < ids.length,
        ids,
      })
    }
    if (collapsed) {
      return
    }
    rows.forEach((it, ri) => out.push({ kind: 'item', id: idOf(it), item: it, grouped, canUp: ri > 0, canDown: ri < rows.length - 1 }))
  })
  return out
}

// dropTarget 把悬停位置换算为移动目标：放在分组标题上时移到该分组末尾
export function dropTarget<T>(sections: Section<T>[], idOf: (it: T) => string, over: DropOver, groupIdPrefix = ''): MoveTarget | null {
  const g = `${groupIdPrefix}g:`
  if (over.id.startsWith(g)) {
    return { group: over.id.slice(g.length) }
  }
  const sec = sections.find((s) => s.items.some((it) => idOf(it) === over.id))
  if (!sec) {
    return null
  }
  return over.after ? { group: sec.name, afterId: over.id } : { group: sec.name, beforeId: over.id }
}

// useGroupedList 为通过通用 CRUD 接口管理的条目列表（DDNS 任务、端口转发规则）提供分组、排序与批量操作。
// 顺序用 <base>-order 保存，分组用 <base>-group 批量保存，都不会重启运行中的实例。
export function useGroupedList<T extends { key?: string; group?: string }>(opts: {
  // storageKey 为分组收起状态的存储键
  storageKey: string
  items: () => T[]
  base: string
  reload: () => Promise<void>
  matches: (it: T, q: string) => boolean
}) {
  const sel = useSortSelect()
  const filterText = ref('')
  const fold = useCollapsed(opts.storageKey)
  const idOf = (it: T) => it.key ?? ''
  const sections = computed(() => buildSections(opts.items(), (it) => it.group ?? ''))
  const groupNames = computed(() => sections.value.map((s) => s.name).filter(Boolean))
  const items = computed(() =>
    displayItems(sections.value, idOf, {
      query: filterText.value,
      sorting: sel.sortMode.value,
      matches: opts.matches,
      isCollapsed: (name) => fold.isCollapsed(name),
      isSelected: sel.isSelected,
    }),
  )
  const orderedIds = computed(() => flatten(sections.value).map(idOf))
  const busy = ref(false)

  // persist 保存新的分组与顺序：先按目标分组批量修改分组，再保存整体顺序
  async function persist(next: Section<T>[] | null) {
    if (!next || busy.value) {
      return
    }
    const changes = new Map<string, string[]>()
    for (const s of next) {
      for (const it of s.items) {
        if ((it.group ?? '') !== s.name) {
          changes.set(s.name, [...(changes.get(s.name) ?? []), idOf(it)])
        }
      }
    }
    const order = flatten(next).map(idOf)
    busy.value = true
    try {
      for (const [group, keys] of changes) {
        await api.put(`${opts.base}-group`, { keys, group })
      }
      if (order.join('\n') !== orderedIds.value.join('\n')) {
        await api.put(`${opts.base}-order`, { keys: order })
      }
    } catch (e) {
      ElMessage.error((e as Error).message)
    } finally {
      busy.value = false
    }
    await opts.reload()
  }

  function toggleGroup(name: string) {
    fold.setCollapsed(name, !fold.isCollapsed(name))
  }

  function moveItem(id: string, d: number) {
    persist(moveStep(sections.value, idOf, id, d))
  }

  function moveGroup(index: number, d: number) {
    persist(moveSection(sections.value, index, d))
  }

  // setGroup 把选中项移到指定分组末尾，留空为移出分组
  async function setGroup(name: string) {
    await persist(moveInto(sections.value, idOf, new Set(sel.selected.value), { group: name.trim() }))
  }

  function dragStart(id: string, e: DragEvent) {
    sel.dragStart(id, orderedIds.value, e)
  }

  function drop() {
    const d = sel.takeDrop()
    if (!d) {
      return
    }
    const target = dropTarget(sections.value, idOf, d.over)
    if (target) {
      persist(moveInto(sections.value, idOf, d.ids, target))
    }
  }

  function selectAll() {
    sel.setSelected(orderedIds.value, true)
  }

  // renameGroup 重命名分组：组内条目一起改为新名称，位置与顺序不变；新名称已存在时并入该分组，收起状态随名称保留
  const askName = useGroupRename()
  async function renameGroup(name: string) {
    const sec = sections.value.find((s) => s.name === name)
    if (!sec) {
      return
    }
    const to = await askName(name, groupNames.value)
    if (to === null) {
      return
    }
    busy.value = true
    try {
      await api.put(`${opts.base}-group`, { keys: sec.items.map(idOf), group: to })
      fold.setCollapsed(to, fold.isCollapsed(name) || fold.isCollapsed(to))
      fold.setCollapsed(name, false)
    } catch (e) {
      ElMessage.error((e as Error).message)
    } finally {
      busy.value = false
    }
    await opts.reload()
  }

  return { ...sel, filterText, sections, groupNames, items, orderedIds, busy, toggleGroup, moveItem, moveGroup, setGroup, dragStart, drop, selectAll, renameGroup }
}
