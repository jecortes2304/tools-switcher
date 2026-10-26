import { useState } from 'react'
import { Clipboard } from '@wailsio/runtime'
import {
  api,
  bytes,
  equalPath,
  type Installation,
  type Run,
  type Snapshot,
  type Variable,
} from '../app/api'
import { Empty, Modal } from '../components/ui'
import { Catalog } from './Catalog'

export function JDKs({ snapshot, run }: { snapshot: Snapshot; run: Run }) {
  const [query, setQuery] = useState('')
  const [catalog, setCatalog] = useState(false)
  const [context, setContext] = useState<Installation | null>(null)
  const [remove, setRemove] = useState<Installation | null>(null)
  const [variable, setVariable] = useState<Installation | null>(null)
  const [variables, setVariables] = useState<Variable[]>([])
  const [variableName, setVariableName] = useState('JAVA_HOME')
  const [environmentMenu, setEnvironmentMenu] = useState(false)
  const [discovered, setDiscovered] = useState<Installation[] | null>(null)
  const [working, setWorking] = useState(false)
  const installations = snapshot.installations ?? []
  const repositories = snapshot.repositories ?? []
  const active = installations.find((i) =>
    equalPath(i.path, snapshot.activePath, snapshot.platform),
  )
  const jobs = snapshot.downloads ?? []
  const visible = installations.filter((i) =>
    `${i.name} ${i.version} ${i.path} ${i.vendor}`.toLowerCase().includes(query.toLowerCase()),
  )
  const importFolder = () =>
    run(async () => {
      const path = await api.ChooseDirectory()
      if (path) await api.Import(path)
    })
  const assign = (i: Installation, name: string) =>
    run(
      () => api.Assign(i.id, name),
      name.toUpperCase() === 'PATH'
        ? 'PATH actualizado. Abre una terminal nueva.'
        : `${name} actualizado. Abre una terminal nueva.`,
    )
  return (
    <>
      <div className="section-heading">
        <h1>Java Development Kits</h1>
        <span className="badge">{installations.length} locales</span>
      </div>
      <section className="panel active-panel">
        <div className="eyebrow">
          <span className={`dot ${snapshot.activePath ? 'live' : ''}`} /> ENTORNO ·{' '}
          {{ user: 'USUARIO', system: 'SISTEMA', session: 'SESIÓN' }[snapshot.activeSource] ??
            'SIN CONFIGURAR'}
        </div>
        <h2>{active?.name ?? (snapshot.activeName || 'Elige tu entorno Java')}</h2>
        <p className="mono truncate" title={snapshot.activePath}>
          {snapshot.activePath || 'JAVA_HOME todavía no está configurado'}
        </p>
        {snapshot.activePath && (
          <button
            className="text-button"
            onClick={() => run(() => Clipboard.SetText(snapshot.activePath), 'Ruta copiada')}
          >
            ⧉ Copiar JAVA_HOME
          </button>
        )}
      </section>
      <div className="toolbar">
        <input
          aria-label="Filtrar JDKs"
          placeholder="⌕ Buscar versión o proveedor…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <button className="primary" onClick={() => setCatalog(true)}>
          + Instalar
        </button>
      </div>
      <div className="row spread minor">
        <span className="eyebrow">TUS INSTALACIONES</span>
        <div className="row">
          <button className="text-button" onClick={importFolder}>
            Importar
          </button>
          <button
            className="text-button"
            disabled={working}
            onClick={async () => {
              setWorking(true)
              await run(async () => setDiscovered((await api.Discover()) ?? []))
              setWorking(false)
            }}
          >
            {working ? 'Buscando…' : 'Detectar'}
          </button>
        </div>
      </div>
      {!installations.length && (
        <Empty title="Cada proyecto, su Java">
          Instala una JDK desde un repositorio o importa una que ya tengas.
        </Empty>
      )}
      {!!installations.length && !visible.length && (
        <Empty title="Sin coincidencias">Prueba otro nombre o versión.</Empty>
      )}
      <div className="stack">
        {visible.map((i) => {
          const isActive = equalPath(i.path, snapshot.activePath, snapshot.platform)
          const repository = repositories.find((r) => r.id === i.repositoryId)
          return (
            <article
              key={i.id}
              className={`jdk-card ${isActive ? 'active' : ''}`}
              onContextMenu={(e) => {
                e.preventDefault()
                setEnvironmentMenu(false)
                setContext(i)
              }}
            >
              <div className="row">
                <span className={`jdk-icon ${isActive ? 'success' : ''}`}>♨</span>
                <div className="grow">
                  <h3>{i.name}</h3>
                  <small>
                    {repository?.name ??
                      (i.repositoryId ? 'Repositorio eliminado' : 'Instalación local')}{' '}
                    · <span className="mono">{i.version}</span>
                  </small>
                </div>
                {isActive ? (
                  <span className="badge success">Activa</span>
                ) : (
                  <button
                    className="compact"
                    onClick={() =>
                      run(
                        () => api.Activate(i.id),
                        'JAVA_HOME y PATH actualizados. Abre una terminal nueva.',
                      )
                    }
                  >
                    Activar
                  </button>
                )}
                <button
                  className="icon-button"
                  aria-label={`Opciones de ${i.name}`}
                  onClick={() => {
                    setEnvironmentMenu(false)
                    setContext(i)
                  }}
                >
                  ⋮
                </button>
              </div>
              <p className="mono truncate path" title={i.path}>
                {i.path}
              </p>
            </article>
          )
        })}
      </div>
      {!!jobs.length && (
        <section className="downloads">
          <h2>Actividad</h2>
          {jobs.map((j) => (
            <article className="panel download" key={j.id}>
              <div className="row spread">
                <strong>{j.name}</strong>
                {['queued', 'downloading', 'extracting'].includes(j.status) && (
                  <button
                    className="text-button"
                    onClick={() => run(() => api.CancelDownload(j.id))}
                  >
                    Cancelar
                  </button>
                )}
              </div>
              <small>
                {{
                  queued: 'En cola',
                  downloading: 'Descargando',
                  extracting: 'Verificando e instalando',
                  completed: 'Instalada',
                  error: 'Error',
                  cancelled: 'Cancelada',
                }[j.status] ?? j.status}{' '}
                {j.status === 'downloading' && `· ${bytes(j.written)} / ${bytes(j.total)}`}
              </small>
              {['downloading', 'extracting'].includes(j.status) && (
                <progress
                  max={j.total || 1}
                  value={j.status === 'extracting' ? undefined : j.written}
                />
              )}
              {j.error && <p className="error-message">{j.error}</p>}
            </article>
          ))}
        </section>
      )}
      {catalog && (
        <Catalog repositories={repositories} run={run} onClose={() => setCatalog(false)} />
      )}
      {context && (
        <Modal title={context.name} onClose={() => setContext(null)}>
          <p className="mono wrap">{context.path}</p>
          <div className="action-list">
            <button
              onClick={async () => {
                if (
                  await run(
                    () => api.Activate(context.id),
                    'JAVA_HOME y PATH actualizados. Abre una terminal nueva.',
                  )
                )
                  setContext(null)
              }}
            >
              ◉ Activar JDK (JAVA_HOME y PATH)
            </button>
            <button
              onClick={() => setEnvironmentMenu((open) => !open)}
              aria-expanded={environmentMenu}
            >
              ⌘ Variables de entorno {environmentMenu ? '▾' : '▸'}
            </button>
            {environmentMenu && (
              <div className="environment-menu">
                {['JAVA_HOME', 'JDK_HOME', 'GRAALVM_HOME', 'JRE_HOME'].map((name) => {
                  const assigned = snapshot.environment?.find((v) => v.name === name)?.value ?? ''
                  const isAssigned =
                    !!assigned && equalPath(assigned, context.path, snapshot.platform)
                  return (
                    <button
                      key={name}
                      disabled={name === 'GRAALVM_HOME' && !/graal/i.test(context.name)}
                      onClick={async () => {
                        if (await assign(context, name)) setContext(null)
                      }}
                    >
                      <span>{name}</span>
                      <small>
                        {isAssigned ? '✓ Asignada a esta JDK' : assigned || 'Sin configurar'}
                      </small>
                    </button>
                  )
                })}
                <button
                  onClick={async () => {
                    const i = context
                    if (await run(async () => setVariables((await api.Variables()) ?? []))) {
                      setContext(null)
                      setVariableName('')
                      setVariable(i)
                    }
                  }}
                >
                  Otra variable…
                </button>
              </div>
            )}
            <button
              onClick={async () => {
                if (await assign(context, 'PATH')) setContext(null)
              }}
            >
              ↗ Añadir a PATH
            </button>
            <button onClick={() => run(() => api.OpenInstallation(context.id))}>
              ▱ Abrir ubicación
            </button>
            <button
              onClick={async () => {
                await run(() => api.Restore(context.id), 'Variables anteriores restauradas')
                setContext(null)
              }}
            >
              ↶ Restaurar variables anteriores
            </button>
            <button
              className="danger"
              onClick={() => {
                setRemove(context)
                setContext(null)
              }}
            >
              × {context.managed ? 'Eliminar JDK…' : 'Desvincular JDK…'}
            </button>
          </div>
        </Modal>
      )}
      {variable && (
        <Modal title="Variable de entorno" onClose={() => setVariable(null)}>
          <p>
            Asigna la carpeta de {variable.name} a una variable existente o escribe un nombre nuevo.
          </p>
          <label className="field">
            Nombre
            <input
              list="environment-variables"
              value={variableName}
              onChange={(e) => setVariableName(e.target.value)}
              pattern="[A-Za-z_][A-Za-z0-9_]*"
              autoFocus
            />
          </label>
          <datalist id="environment-variables">
            {variables.map((v) => (
              <option value={v.name} key={v.name} />
            ))}
          </datalist>
          <p className="hint">El valor anterior se guardará para poder restaurarlo.</p>
          <button
            className="primary full"
            disabled={!/^[A-Za-z_][A-Za-z0-9_]*$/.test(variableName)}
            onClick={async () => {
              if (await assign(variable, variableName)) setVariable(null)
            }}
          >
            Asignar variable
          </button>
        </Modal>
      )}
      {remove && (
        <Modal
          title={remove.managed ? 'Eliminar JDK' : 'Desvincular JDK'}
          onClose={() => setRemove(null)}
        >
          <p>
            {remove.managed
              ? `Se borrarán los archivos de ${remove.name} del vault.`
              : `Se quitará ${remove.name} de la lista y se conservarán sus archivos.`}{' '}
            Sus variables gestionadas se restaurarán.
          </p>
          <div className="dialog-actions">
            <button onClick={() => setRemove(null)}>Cancelar</button>
            <button
              className="danger-button"
              onClick={async () => {
                if (
                  await run(() => api.RemoveInstallation(remove.id, remove.managed), 'JDK retirada')
                )
                  setRemove(null)
              }}
            >
              Confirmar
            </button>
          </div>
        </Modal>
      )}
      {discovered && (
        <Modal title="JDKs detectadas" onClose={() => setDiscovered(null)}>
          {!discovered.length && (
            <Empty title="Sin instalaciones detectadas">
              Puedes seleccionar la carpeta raíz de una JDK con Importar.
            </Empty>
          )}
          <div className="catalog-list">
            {discovered.map((i) => (
              <div className="panel" key={i.id}>
                <h3>{i.name}</h3>
                <p className="mono wrap">{i.path}</p>
                <button
                  disabled={installations.some((existing) =>
                    equalPath(existing.path, i.path, snapshot.platform),
                  )}
                  onClick={() => run(() => api.Import(i.path), 'JDK importada')}
                >
                  Importar
                </button>
              </div>
            ))}
          </div>
        </Modal>
      )}
    </>
  )
}
