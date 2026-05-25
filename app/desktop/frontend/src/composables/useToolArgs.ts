import { computed, reactive, ref, watch, type Ref } from 'vue'

import type { ToolManifest } from '@/types/workbench'

export function useToolArgs(selectedTool: Ref<ToolManifest | null>) {
  const parameterMode = ref<'structured' | 'raw'>('structured')
  const rawArgs = ref('')
  const pythonEnv = ref('python')
  const formModel = reactive<Record<string, string | number | boolean | null>>({})

  const buildRawArgs = (tool: ToolManifest | null) => {
    if (!tool) {
      return ''
    }

    const parts: string[] = []
    for (const param of tool.params) {
      const value = formModel[param.key]
      const argKey = param.argKey || param.key

      if (param.type === 'boolean') {
        if (value === true) {
          parts.push(`-${argKey}`)
        }
        continue
      }

      if (value === undefined || value === null || value === '') {
        continue
      }

      const escapedValue =
        typeof value === 'string' && /\s/.test(value) ? `"${value}"` : String(value)
      parts.push(`-${argKey}`, escapedValue)
    }

    return parts.join(' ')
  }

  const seedForm = (tool: ToolManifest | null) => {
    for (const key of Object.keys(formModel)) {
      delete formModel[key]
    }

    if (!tool) {
      rawArgs.value = ''
      return
    }

    for (const param of tool.params) {
      if (param.default !== undefined) {
        formModel[param.key] = param.default as string | number | boolean | null
        continue
      }

      switch (param.type) {
        case 'number':
          formModel[param.key] = null
          break
        case 'boolean':
          formModel[param.key] = false
          break
        default:
          formModel[param.key] = ''
      }
    }

    rawArgs.value = buildRawArgs(tool)
    if (tool.kind !== 'python') {
      pythonEnv.value = 'python'
    }
  }

  watch(
    selectedTool,
    (tool) => {
      seedForm(tool)
    },
    { immediate: true },
  )

  watch(
    formModel,
    () => {
      if (parameterMode.value === 'structured') {
        rawArgs.value = buildRawArgs(selectedTool.value)
      }
    },
    { deep: true },
  )

  const canSubmit = computed(() => {
    if (!selectedTool.value) {
      return false
    }
    if (parameterMode.value === 'raw') {
      return true
    }
    return selectedTool.value.params.every((param) => {
      if (!param.required) {
        return true
      }
      const value = formModel[param.key]
      return value !== undefined && value !== null && value !== ''
    })
  })

  return {
    parameterMode,
    rawArgs,
    pythonEnv,
    formModel,
    canSubmit,
  }
}
