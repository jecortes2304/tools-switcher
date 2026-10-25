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
  const [major, setMajor] = useState(21)
  const [versions, setVersions] = useState([25, 21, 17, 11, 8])
  const [packages, setPackages] = useState<Package[]>([])
  const [selected, setSelected] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [working, setWorking] = useState(false)
  const [error, setError] = useState('')
  useEffect(() => {
    let active = true
    api
      .Versions()
      .then((v) => {
        if (active && v?.length) setVersions(v)
      })
      .catch(() => {})
    return () => {
      active = false
    }
  }, [])
  useEffect(() => {
    let active = true
    setPackages([])
    setError('')
    if (!repository) return
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
  const source = enabled.find((r) => r.id === repository)
  return (
    <Modal title="Instalar JDKs" onClose={onClose}>
      {!enabled.length ? (
        <Empty title="Añade un repositorio">
          Conecta Temurin o una carpeta offline desde Repositorios.
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
          {source?.kind === 'adoptium' && (
            <label className="field">
              Versión de Java
              <select value={major} onChange={(e) => setMajor(Number(e.target.value))}>
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
          <div className="catalog-list" aria-busy={loading}>
            {loading && <p className="hint">Consultando catálogo…</p>}
            {error && (
              <p className="error-message" role="alert">
                {error}
              </p>
            )}
            {!loading && !error && !packages.length && (
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
                  <small>
                    {p.os} · {p.arch} · {bytes(p.size)}
                  </small>
                </span>
              </label>
            ))}
          </div>
          <button
            className="primary full"
            disabled={!selected.length || working || loading}
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
