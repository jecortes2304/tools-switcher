import { useState } from 'react'
import { api, type Repository, type Run } from '../app/api'
import { Empty, Modal, Toggle } from '../components/ui'

type Props = { repositories: Repository[]; run: Run }
export function Repositories({ repositories, run }: Props) {
  const [kind, setKind] = useState('remote')
  const [source, setSource] = useState('https://api.adoptium.net/v3')
  const [name, setName] = useState('')
  const [working, setWorking] = useState(false)
  const [remove, setRemove] = useState<Repository | null>(null)
  const choose = async () => {
    await run(async () => {
      const path = await api.ChooseDirectory()
      if (path) setSource(path)
    })
  }
  return (
    <>
      <div className="section-heading">
        <h1>Repositorios</h1>
        <span className="badge">{repositories.filter((r) => r.enabled).length} activos</span>
      </div>
      <form
        className="panel"
        onSubmit={async (e) => {
          e.preventDefault()
          setWorking(true)
          const ok = await run(
            () => api.AddRepository(name, source, kind),
            'Repositorio validado y añadido',
          )
          if (ok) {
            setName('')
            setSource('')
          }
          setWorking(false)
        }}
      >
        <h2>Añadir un catálogo</h2>
        <div className="segmented">
          <button
            type="button"
            className={kind === 'remote' ? 'selected' : ''}
            onClick={() => {
              setKind('remote')
              setSource('https://api.adoptium.net/v3')
            }}
          >
            ↗ URL remota
          </button>
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
        </div>
        <label className="field">
          {kind === 'remote' ? 'URL del catálogo' : 'Carpeta de JDKs'}
          <div className="input-action">
            <input
              required
              type={kind === 'remote' ? 'url' : 'text'}
              value={source}
              onChange={(e) => setSource(e.target.value)}
              placeholder={
                kind === 'remote' ? 'https://api.adoptium.net/v3' : 'Selecciona una carpeta'
              }
            />
            {kind === 'local' && (
              <button type="button" onClick={choose} aria-label="Buscar carpeta">
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
            ? 'API de Adoptium o manifiesto compatible de tools-switcher. Validaremos el catálogo y un enlace de descarga.'
            : 'Busca JDKs descomprimidas y archivos ZIP o TAR.GZ compatibles. No necesita conexión.'}
        </p>
        <button className="primary full" disabled={working}>
          {working ? 'Validando origen…' : '+ Validar y añadir'}
        </button>
      </form>
      <div className="section-heading minor">
        <h2>Catálogos guardados</h2>
        <button
          className="text-button"
          disabled={working || !repositories.some((r) => r.enabled)}
          onClick={async () => {
            setWorking(true)
            await run(async () => {
              const results = await Promise.allSettled(
                repositories.filter((r) => r.enabled).map((r) => api.Catalog(r.id, 21)),
              )
              const failures = results.filter((r) => r.status === 'rejected')
              if (failures.length)
                throw new Error(
                  `${failures.length} repositorios no se pudieron sincronizar. Revisa el estado de cada uno.`,
                )
            }, 'Catálogos sincronizados')
            setWorking(false)
          }}
        >
          ↻ Sincronizar
        </button>
      </div>
      {!repositories.length && (
        <Empty title="Tu catálogo empieza aquí">Añade Temurin para descargar tu primera JDK.</Empty>
      )}
      <div className="stack">
        {repositories.map((r) => (
          <article className={`panel repository ${!r.enabled ? 'muted' : ''}`} key={r.id}>
            <div className="row">
              <span className="repo-icon">{r.kind === 'local' ? '▱' : '◎'}</span>
              <div className="grow">
                <h3>{r.name}</h3>
                <p className="mono truncate" title={r.source}>
                  {r.source}
                </p>
              </div>
              <button
                className="icon-button danger"
                aria-label={`Eliminar ${r.name}`}
                onClick={() => setRemove(r)}
              >
                ×
              </button>
            </div>
            <div className="row spread">
              <span className={r.error ? 'error-message' : 'success'}>
                {r.error ? 'Error de sincronización' : `${r.count} paquetes compatibles`}
              </span>
              <button
                className="text-button"
                disabled={!r.enabled || working}
                onClick={() => run(() => api.Catalog(r.id, 21), 'Catálogo actualizado')}
              >
                ↻ Actualizar
              </button>
            </div>
            {r.error && <p className="error-message">{r.error}</p>}
            <Toggle
              label={r.enabled ? 'Activo' : 'Desactivado'}
              detail={
                r.lastSync ? `Última consulta: ${new Date(r.lastSync).toLocaleString()}` : undefined
              }
              checked={r.enabled}
              onChange={(enabled) => run(() => api.ToggleRepository(r.id, enabled))}
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
              onClick={async () => {
                if (await run(() => api.RemoveRepository(remove.id), 'Repositorio eliminado'))
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
