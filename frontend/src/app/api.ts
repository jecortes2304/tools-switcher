export * as api from '../../bindings/github.com/jecortes2304/tools-switcher/internal/desktop/service'
export type {
  Snapshot,
  Settings,
  Repository,
  Installation,
  Package,
  Download,
} from '../../bindings/github.com/jecortes2304/tools-switcher/internal/manager/models'
export type { UpdateStatus } from '../../bindings/github.com/jecortes2304/tools-switcher/internal/desktop/models'
export type { Variable } from '../../bindings/github.com/jecortes2304/tools-switcher/internal/platform/models'
export type Run = (task: () => Promise<unknown>, success?: string) => Promise<boolean>
export const errorMessage = (error: unknown) =>
  error instanceof Error ? error.message : String(error)
export const bytes = (value: number) => (value ? `${(value / 1024 / 1024).toFixed(0)} MB` : 'Local')
export const equalPath = (a: string, b: string, platform = 'windows') => {
  const normalize = (path: string) => {
    const normalized = path.replace(/\\/g, '/').replace(/\/$/, '')
    return platform === 'windows' ? normalized.toLowerCase() : normalized
  }
  return normalize(a) === normalize(b)
}
