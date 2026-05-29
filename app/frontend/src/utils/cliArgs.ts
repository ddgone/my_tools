function isWhitespace(char: string): boolean {
  return /\s/u.test(char)
}

export function parseCliArgs(input: string): string[] {
  const args: string[] = []
  let current = ''
  let quote: '"' | "'" | '' = ''
  let tokenStarted = false

  const chars = Array.from(input)
  for (let i = 0; i < chars.length; i++) {
    const char = chars[i]

    if (!quote && isWhitespace(char)) {
      if (tokenStarted) {
        args.push(current)
        current = ''
        tokenStarted = false
      }
      continue
    }

    if (char === '\\') {
      const next = chars[i + 1]
      if (next === undefined) {
        if (quote) {
          throw new Error('参数解析失败：尾部转义符不完整')
        }
        current += char
        tokenStarted = true
        continue
      }

      if (!quote) {
        if (isWhitespace(next) || next === '"' || next === '\'' || next === '\\') {
          current += next
          i++
        } else {
          current += char
        }
      } else if (quote === '"') {
        if (next === '"' || next === '\\') {
          current += next
          i++
        } else {
          current += char
        }
      } else {
        if (next === '\'' || next === '\\') {
          current += next
          i++
        } else {
          current += char
        }
      }

      tokenStarted = true
      continue
    }

    if (char === '"' || char === '\'') {
      if (!quote) {
        quote = char
        tokenStarted = true
        continue
      }

      if (quote === char) {
        quote = ''
        continue
      }

      current += char
      tokenStarted = true
      continue
    }

    current += char
    tokenStarted = true
  }

  if (quote === '"') {
    throw new Error('参数解析失败：双引号未闭合')
  }
  if (quote === '\'') {
    throw new Error('参数解析失败：单引号未闭合')
  }

  if (tokenStarted) {
    args.push(current)
  }

  return args
}

export function validateCliArgs(input: string): string | null {
  try {
    parseCliArgs(input)
    return null
  } catch (error) {
    return error instanceof Error ? error.message : '参数解析失败'
  }
}
