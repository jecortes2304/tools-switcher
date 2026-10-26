import { useState } from 'react'
import { api, type Repository, type Run } from '../app/api'
import { Empty, Modal, Toggle } from '../components/ui'

type Props = { repositories: Repository[]; run: Run }

export function Repositories({ repositories, run }: Props) {
  const [query, setQuery] = useState('')
  const [providerId, setProviderId] = useState('')
  const [kind, setKind] = useState('local')
  const [source, setSource] = useState('')
  const [name, setName] = useState('')
  const [working, setWorking] = useState(false)
  const [remove, setRemove] = useState<Repository | null>(null)
  const providers = repositories.filter(
    (r) => r.providerId && r.name.toLowerCase().includes(query.toLowerCase()),
  )
  const selected =
    providers.find((r) => r.id === providerId) ?? providers.find((r) => !r.enabled) ?? providers[0]
  const visible = repositories.filter((r) => !r.providerId || r.enabled || r.error)
  const perform = async (task: () => Promise<unknown>, success?: string) => {
    setWorking(true)
    try {
      return await run(task, success)
    } finally {
      setWorking(false)
    }
  }
  return (
    <>
      <div className="section-heading">
        <h1>Repositorios</h1>
        <span className="badge">{repositories.filter((r) => r.enabled).length} activos</span>
      </div>
      <section className="panel">
        <h2>Proveedores incluidos</h2>
        <p className="hint">
          Elige una distribución. Solo los proveedores activos aparecerán al instalar.
        </p>
        <input
          aria-label="Buscar proveedor"
          placeholder="Buscar Oracle, GraalVM, Amazon…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        {selected ? (
          <>
            <label className="field">
              Proveedor
              <select
                aria-label="Proveedor"
                value={selected.id}
                onChange={(e) => setProviderId(e.target.value)}
              >
                {providers.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.name}
                    {r.enabled ? ' · Activo' : ''}
                  </option>
                ))}
              </select>
            </label>
            <button
              className={selected.enabled ? 'full' : 'primary full'}
              disabled={working}
              onClick={() =>
                perform(
                  () => api.ToggleRepository(selected.id, !selected.enabled),
                  selected.enabled ? 'Proveedor desactivado' : 'Proveedor validado y activado',
                )
              }
            >
              {working
                ? 'Comprobando proveedor…'
                : selected.enabled
                  ? 'Desactivar proveedor'
                  : 'Validar y activar proveedor'}
            </button>
          </>
        ) : (
          <Empty title="Sin coincidencias">Prueba otro nombre de proveedor.</Empty>
        )}
      </section>
      <details className="panel advanced-sources">
        <summary>Añadir carpeta o catálogo personalizado</summary>
        <form
          onSubmit={async (e) => {
            e.preventDefault()
            if (
              await perform(
                () => api.AddRepository(name, source, kind),
                'Repositorio validado y añadido',
              )
            ) {
              setName('')
              setSource('')
            }
          }}
        >
          <div className="segmented">
            <button
              type="button"
              className={kind === 'local' ? 'selected' : ''}
              onClick={() => {
                setKind('local')
                setSource('')
              }}
            >
              ▱ Carpeta offline
            </button>
            <button
              type="button"
              className={kind === 'remote' ? 'selected' : ''}
              onClick={() => {
                setKind('remote')
                setSource('')
              }}
            >
              ↗ Manifiesto HTTPS
            </button>
          </div>
          <label className="field">
            {kind === 'remote' ? 'URL del manifiesto' : 'Carpeta de JDKs'}
            <div className="input-action">
              <input
                required
                type={kind === 'remote' ? 'url' : 'text'}
                value={source}
                onChange={(e) => setSource(e.target.value)}
                placeholder={
                  kind === 'remote' ? 'https://servidor/catalogo.json' : 'Selecciona una carpeta'
                }
              />
              {kind === 'local' && (
                <button
                  type="button"
                  aria-label="Buscar carpeta"
                  onClick={() =>
                    run(async () => {
                      const path = await api.ChooseDirectory()
                      if (path) setSource(path)
                    })
                  }
                >
                  …
                </button>
              )}
            </div>
          </label>
          <label className="field">
            Nombre opcional
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Mi repositorio"
              maxLength={80}
            />
          </label>
          <p className="hint">
            {kind === 'remote'
              ? 'Para catálogos JSON de tools-switcher. Las páginas web y los enlaces directos a ZIP no son catálogos.'
              : 'Carpeta con JDKs o archivos ZIP/TAR.GZ compatibles. Funciona sin conexión.'}
          </p>
          <button className="primary full" disabled={working}>
            {working ? 'Validando origen…' : '+ Validar y añadir'}
          </button>
        </form>
      </details>
      <div className="section-heading minor">
        <h2>Fuentes configuradas</h2>
        <button
          className="text-button"
          disabled={working || !repositories.some((r) => r.enabled)}
          onClick={() =>
            perform(async () => {
              const results = await Promise.allSettled(
                repositories.filter((r) => r.enabled).map((r) => api.SyncRepository(r.id)),
              )
              if (results.some((r) => r.status === 'rejected'))
                throw new Error(
                  'Algún proveedor no se pudo comprobar. Revisa su estado en la lista.',
                )
            }, 'Fuentes comprobadas')
          }
        >
          ↻ Sincronizar
        </button>
      </div>
      {!visible.length && (
        <Empty title="Activa un proveedor">
          Elige uno de la lista o añade una carpeta offline.
        </Empty>
      )}
      <div className="stack">
        {visible.map((r) => (
          <article className={`panel repository ${!r.enabled ? 'muted' : ''}`} key={r.id}>
            <div className="row">
              <span className="repo-icon">{r.kind === 'local' ? '▱' : '◎'}</span>
              <div className="grow">
                <h3>{r.name}</h3>
                <p className="mono truncate" title={r.source}>
                  {r.providerId
                    ? r.kind === 'adoptium'
                      ? 'Catálogo de Eclipse Adoptium'
                      : r.kind === 'corretto'
                        ? 'Catálogo de Amazon'
                        : 'Catálogo de Foojay'
                    : r.source}
                </p>
              </div>
              {!r.providerId && (
                <button
                  className="icon-button danger"
                  aria-label={`Eliminar ${r.name}`}
                  onClick={() => setRemove(r)}
                >
                  ×
                </button>
              )}
            </div>
            <div className="row spread">
              <span className={r.error ? 'error-message' : 'success'}>
                {r.error
                  ? 'No se pudo comprobar'
                  : r.lastSync
                    ? `${r.count} paquetes en última consulta`
                    : 'Pendiente de comprobar'}
              </span>
              <button
                className="text-button"
                disabled={!r.enabled || working}
                onClick={() => perform(() => api.SyncRepository(r.id), 'Fuente comprobada')}
              >
                ↻ Comprobar
              </button>
            </div>
            {r.error && <p className="error-message">{r.error}</p>}
            <Toggle
              label={r.enabled ? 'Activo' : 'Desactivado'}
              detail={
                r.lastSync ? `Última consulta: ${new Date(r.lastSync).toLocaleString()}` : undefined
              }
              checked={r.enabled}
              disabled={working}
              onChange={(enabled) => perform(() => api.ToggleRepository(r.id, enabled))}
            />
          </article>
        ))}
      </div>
      {remove && (
        <Modal title="Eliminar repositorio" onClose={() => setRemove(null)}>
          <p>Se eliminará «{remove.name}» del catálogo. Las JDKs instaladas se conservarán.</p>
          <div className="dialog-actions">
            <button onClick={() => setRemove(null)}>Cancelar</button>
            <button
              className="danger-button"
              disabled={working}
              onClick={async () => {
                if (await perform(() => api.RemoveRepository(remove.id), 'Repositorio eliminado'))
                  setRemove(null)
              }}
            >
              Eliminar
            </button>
          </div>
        </Modal>
      )}
    </>
  )
}
