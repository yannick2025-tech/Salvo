import { post } from './client'
import type { GeneratorCategoryInfo } from '@/types'

export function listGenerators() {
  return post<{ categories: GeneratorCategoryInfo[] }>('/generators/list', {})
}
