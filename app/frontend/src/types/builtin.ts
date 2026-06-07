export type BuiltinToolId =
  | 'time_toolkit'
  | 'json_toolkit'
  | 'base64_toolkit'
  | 'url_toolkit'
  | 'hash_toolkit'
  | 'jwt_toolkit'

export type BuiltinToolIconKey = 'time' | 'json' | 'base64' | 'url' | 'hash' | 'jwt'

export interface BuiltinToolDefinition {
  id: BuiltinToolId
  name: string
  description: string
  keywords: string[]
  group: string
  badge: string
  icon: BuiltinToolIconKey
  accent: string
}

export interface BuiltinToolTabState {
  tabId: string
  builtinToolId: BuiltinToolId
  openedAt: number
}
