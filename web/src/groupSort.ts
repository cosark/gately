// 列表分组与排序的纯函数：Web 服务站点、DDNS 任务、端口转发规则共用。
// 分组只影响管理界面的展示：分组按首次出现的顺序排列，未分组的条目放在最后。

// MAX_GROUP_LEN 为分组名称的最大字符数，与后端 crud.MaxGroupLen 一致
export const MAX_GROUP_LEN = 64

export interface Section<T> {
  // name 为分组名称，空字符串为未分组
  name: string
  items: T[]
}

// MoveTarget 描述移动的目的地：beforeId / afterId 指定插入位置，都不提供时放到分组末尾
export interface MoveTarget {
  group: string
  beforeId?: string
  afterId?: string
}

// buildSections 把条目按分组归类，保持各分组内的原有顺序
export function buildSections<T>(items: T[], groupOf: (it: T) => string): Section<T>[] {
  const map = new Map<string, T[]>()
  for (const it of items) {
    const g = groupOf(it)
    if (!map.has(g)) {
      map.set(g, [])
    }
    map.get(g)!.push(it)
  }
  const named = [...map.entries()].filter(([g]) => g).map(([name, list]) => ({ name, items: list }))
  const ungrouped = map.get('') ?? []
  return ungrouped.length ? [...named, { name: '', items: ungrouped }] : named
}

// flatten 按分组展示顺序展开为保存用的顺序
export function flatten<T>(sections: Section<T>[]): T[] {
  return sections.flatMap((s) => s.items)
}

// moveInto 把 moving 中的条目按当前显示顺序整体移到目标分组的指定位置，返回新的分组列表。
// 目标分组不存在时新建，放在未分组之前；移空的分组被去掉。插入位置本身是被移动的条目时不做任何改变。
export function moveInto<T>(sections: Section<T>[], idOf: (it: T) => string, moving: Set<string>, target: MoveTarget): Section<T>[] | null {
  if (!moving.size || (target.beforeId && moving.has(target.beforeId)) || (target.afterId && moving.has(target.afterId))) {
    return null
  }
  const picked = flatten(sections).filter((it) => moving.has(idOf(it)))
  if (!picked.length) {
    return null
  }
  const next = sections.map((s) => ({ name: s.name, items: s.items.filter((it) => !moving.has(idOf(it))) }))
  let sec = next.find((s) => s.name === target.group)
  if (!sec) {
    sec = { name: target.group, items: [] }
    const ui = next.findIndex((s) => !s.name)
    if (target.group && ui >= 0) {
      next.splice(ui, 0, sec)
    } else {
      next.push(sec)
    }
  }
  let at = sec.items.length
  if (target.beforeId) {
    const i = sec.items.findIndex((it) => idOf(it) === target.beforeId)
    at = i < 0 ? at : i
  } else if (target.afterId) {
    const i = sec.items.findIndex((it) => idOf(it) === target.afterId)
    at = i < 0 ? at : i + 1
  }
  sec.items.splice(at, 0, ...picked)
  return next.filter((s) => s.items.length)
}

// moveStep 把条目在所在分组内上移（d = -1）或下移（d = 1）一位
export function moveStep<T>(sections: Section<T>[], idOf: (it: T) => string, id: string, d: number): Section<T>[] | null {
  const next = sections.map((s) => ({ name: s.name, items: [...s.items] }))
  for (const s of next) {
    const i = s.items.findIndex((it) => idOf(it) === id)
    if (i < 0) {
      continue
    }
    const j = i + d
    if (j < 0 || j >= s.items.length) {
      return null
    }
    ;[s.items[i], s.items[j]] = [s.items[j], s.items[i]]
    return next
  }
  return null
}

// moveSection 整体移动一个命名分组；未分组始终在最后
export function moveSection<T>(sections: Section<T>[], index: number, d: number): Section<T>[] | null {
  const j = index + d
  if (j < 0 || j >= sections.length || !sections[index]?.name || !sections[j].name) {
    return null
  }
  const next = [...sections]
  ;[next[index], next[j]] = [next[j], next[index]]
  return next
}

// groupSuggestions 生成分组输入框的候选项：已有分组名称中包含输入内容的
export function groupSuggestions(names: Iterable<string>, query: string) {
  const q = query.trim().toLowerCase()
  const set = new Set<string>()
  for (const n of names) {
    if (n && (!q || n.toLowerCase().includes(q))) {
      set.add(n)
    }
  }
  return [...set].map((value) => ({ value }))
}
