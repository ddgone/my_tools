import type { ParameterCondition, ParameterSpec, ParameterVisibility, ToolManifest } from '@/types/workbench'

export type FormValue = string | number | boolean | null
export type FormModel = Record<string, FormValue>

function primitiveEquals(left: unknown, right: unknown): boolean {
  if (left === right) {
    return true
  }
  if (left === null || left === undefined || right === null || right === undefined) {
    return false
  }
  return String(left) === String(right)
}

function matchesCondition(condition: ParameterCondition, formModel: FormModel): boolean {
  return primitiveEquals(formModel[condition.key], condition.equals)
}

function matchesVisibility(visibility: ParameterVisibility | undefined, formModel: FormModel): boolean {
  if (!visibility) {
    return true
  }

  const allMatched =
    !visibility.all || visibility.all.length === 0 || visibility.all.every((condition) => matchesCondition(condition, formModel))
  const anyMatched =
    !visibility.any || visibility.any.length === 0 || visibility.any.some((condition) => matchesCondition(condition, formModel))

  return allMatched && anyMatched
}

export function isParamVisible(param: ParameterSpec, formModel: FormModel): boolean {
  return matchesVisibility(param.visibleWhen, formModel)
}

export function getVisibleParams(tool: ToolManifest, formModel: FormModel): ParameterSpec[] {
  return tool.params.filter((param) => isParamVisible(param, formModel))
}

export function shouldEmitParam(param: ParameterSpec): boolean {
  return param.emit !== false
}

export function findMissingRequiredParam(tool: ToolManifest, formModel: FormModel): ParameterSpec | null {
  for (const param of getVisibleParams(tool, formModel)) {
    if (!param.required) {
      continue
    }

    const value = formModel[param.key]
    if (value === null || value === undefined || value === '') {
      return param
    }
  }
  return null
}
