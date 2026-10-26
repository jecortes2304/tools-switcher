import { useEffect, useState } from 'react'
import { api, bytes, errorMessage, type Package, type Repository, type Run } from '../app/api'
import { Empty, Modal } from '../components/ui'

export function Catalog({
  repositories,
  run,
  onClose,
}: {
  repositories: Repository[]
  run: Run
  onClose: () => void
}) {
  const enabled = repositories.filter((r) => r.enabled)
  const [repository, setRepository] = useState(enabled[0]?.id ?? '')
  const [selection, setSelection] = useState<{
    repository: string
    versions: number[]
    major: number
  }>({ repository: '', versions: [], major: 0 })
  const [versionError, setVersionError] = useState('')
  const source = enabled.find((r) => r.id === repository)
  const major = selection.repository === repository ? selection.major : 0
  const versions = selection.versions
  const [packages, setPackages] = useState<Package[]>([])
  const [selected, setSelected] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [working, setWorking] = useState(false)
  const [error, setError] = useState('')
  useEffect(() => {
    let active = true
    setVersionError('')
    setSelection({ repository, versions: [], major: 0 })
    if (!repository) return
    if (!['adoptium', 'foojay', 'corretto'].includes(source?.kind ?? '')) {
      setSelection({ repository, versions: [], major: 21 })
      return
    }
    api
      .RepositoryVersions(repository)
      .then((values) => {
        if (!active) return
        const list = values ?? []
        setSelection({ repository, versions: list, major: list.includes(21) ? 21 : (list[0] ?? 0) })
        if (!list.length)
          setVersionError('Este proveedor no anuncia versiones estables disponibles.')
      })
      .catch((error) => {
        if (active) setVersionError(errorMessage(error))
      })
    return () => {
      active = false
    }
  }, [repository, source?.kind])
  useEffect(() => {
    let active = true
    setPackages([])
    setError('')
    setLoading(false)
    if (!repository || !major) return
    setLoading(true)
    api
      .Catalog(repository, major)
      .then((result) => {
        if (active) setPackages(result ?? [])
      })
      .catch((e) => {
        if (active) setError(errorMessage(e))
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [repository, major])
  useEffect(() => {
    setSelected([])
  }, [repository])
  return (
    <Modal title="Instalar JDKs" onClose={onClose}>
      {!enabled.length ? (
        <Empty title="Añade un repositorio">
          Activa un proveedor o añade una carpeta offline desde Repositorios.
        </Empty>
      ) : (
        <>
          <label className="field">
            Repositorio
            <select value={repository} onChange={(e) => setRepository(e.target.value)}>
              {enabled.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}
                </option>
              ))}
            </select>
          </label>
          {['adoptium', 'foojay', 'corretto'].includes(source?.kind ?? '') && (
            <label className="field">
              Versión de Java
              <select
                value={major}
                onChange={(e) =>
                  setSelection((current) => ({ ...current, major: Number(e.target.value) }))
                }
              >
                {versions.map((v) => (
                  <option key={v} value={v}>
                    Java {v}
                  </option>
                ))}
              </select>
            </label>
          )}
          <p className="hint">
            Paquetes compatibles con este equipo. La selección se conserva al cambiar de versión de
            Java.
          </p>
          {versionError && (
            <p className="error-message" role="alert">
              {versionError}
            </p>
          )}
          <div className="catalog-list" aria-busy={loading}>
            {loading && <p className="hint">Consultando catálogo…</p>}
            {error && (
              <p className="error-message" role="alert">
                {error}
              </p>
            )}
            {!!major && !loading && !error && !packages.length && (
              <Empty title="Sin paquetes">Prueba otra versión o sincroniza el repositorio.</Empty>
            )}
            {packages.map((p) => (
              <label className="package-row" key={p.id}>
                <input
                  type="checkbox"
                  checked={selected.includes(p.id)}
                  onChange={(e) =>
                    setSelected((ids) =>
                      e.target.checked ? [...ids, p.id] : ids.filter((id) => id !== p.id),
                    )
                  }
                />
                <span>
                  <strong>{p.name}</strong>
                  <small className="mono wrap">{p.filename}</small>
                  <small>
                    {p.os} · {p.arch} · {bytes(p.size)}
                  </small>
                </span>
              </label>
            ))}
          </div>
          <button
            className="primary full"
            disabled={!selected.length || working || loading || !major}
            onClick={async () => {
              setWorking(true)
              if (await run(() => api.Queue(selected), 'Instalaciones añadidas a la cola'))
                onClose()
              setWorking(false)
            }}
          >
            {working ? 'Preparando…' : `Instalar selección (${selected.length})`}
          </button>
        </>
      )}
    </Modal>
  )
}
