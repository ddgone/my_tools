import type { Component } from 'vue'
import {
  CodeSlash,
  ConstructOutline,
  FingerPrintOutline,
  KeyOutline,
  LinkOutline,
  SwapHorizontalOutline,
  TimeOutline,
} from '@vicons/ionicons5'
import type { BuiltinToolDefinition, BuiltinToolId } from '@/types/builtin'

export const builtinTools: BuiltinToolDefinition[] = [
  {
    id: 'time_toolkit',
    name: '时间处理',
    description: '支持任意时间表示之间互转，覆盖秒/毫秒时间戳、ISO、带时区时间和本地时间格式。',
    keywords: ['时间戳', '毫秒', '秒', 'utc', '时区', 'datetime', 'timezone', 'beijing', 'iso', 'rfc3339'],
    group: '时间与日期',
    badge: '聚合工具',
    icon: 'time',
    accent: 'rgb(var(--color-brand-primary) / 1)',
  },
  {
    id: 'json_toolkit',
    name: 'JSON 工具',
    description: '提供 JSON 格式化、压缩、校验和稳定排序，适合接口调试和数据整理。',
    keywords: ['json', '格式化', '压缩', '校验', '排序', 'pretty', 'minify'],
    group: '文本处理',
    badge: '结构化数据',
    icon: 'json',
    accent: 'rgb(var(--color-mode-remote) / 1)',
  },
  {
    id: 'base64_toolkit',
    name: 'Base64 工具',
    description: '在文本与 Base64 之间双向转换，内置 Unicode 支持，适合快速编解码。',
    keywords: ['base64', '编码', '解码', 'unicode', '文本'],
    group: '文本处理',
    badge: '编解码',
    icon: 'base64',
    accent: 'rgb(var(--color-kind-rust) / 1)',
  },
  {
    id: 'url_toolkit',
    name: 'URL 工具',
    description: '提供 URL 与 URL Component 编解码、查询串拆解与重组，适合接口联调和参数排查。',
    keywords: ['url', 'encode', 'decode', 'query', '参数', 'uri'],
    group: '网络与协议',
    badge: '编解码',
    icon: 'url',
    accent: 'rgb(var(--color-success) / 1)',
  },
  {
    id: 'hash_toolkit',
    name: 'Hash 摘要',
    description: '快速计算文本的 SHA 系列摘要，适合校验内容一致性和生成对比指纹。',
    keywords: ['hash', 'sha', 'sha256', 'sha512', '摘要', '指纹'],
    group: '安全与校验',
    badge: '摘要',
    icon: 'hash',
    accent: 'rgb(var(--color-mode-remote) / 1)',
  },
  {
    id: 'jwt_toolkit',
    name: 'JWT 查看',
    description: '解析 JWT 的 Header、Payload 与时间字段，适合调试鉴权链路，不做签名校验。',
    keywords: ['jwt', 'token', 'payload', 'header', 'exp', 'auth'],
    group: '安全与校验',
    badge: '鉴权调试',
    icon: 'jwt',
    accent: 'rgb(var(--color-warning) / 1)',
  },
]

const builtinToolIconMap: Record<BuiltinToolId, Component> = {
  time_toolkit: TimeOutline,
  json_toolkit: CodeSlash,
  base64_toolkit: SwapHorizontalOutline,
  url_toolkit: LinkOutline,
  hash_toolkit: FingerPrintOutline,
  jwt_toolkit: KeyOutline,
}

export function getBuiltinToolById(id: string) {
  return builtinTools.find((tool) => tool.id === id)
}

export function getBuiltinToolIcon(id: string): Component {
  return builtinToolIconMap[id as BuiltinToolId] ?? ConstructOutline
}

export function matchBuiltinTools(query: string) {
  const normalized = query.trim().toLowerCase()
  if (!normalized) {
    return builtinTools
  }

  return builtinTools.filter((tool) => {
    return tool.name.toLowerCase().includes(normalized)
      || tool.description.toLowerCase().includes(normalized)
      || tool.group.toLowerCase().includes(normalized)
      || tool.keywords.some((keyword) => keyword.toLowerCase().includes(normalized))
  })
}
