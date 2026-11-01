import { Clipboard } from '@wailsio/runtime'
import { api, type Run, type Settings as Preferences, type Snapshot } from '../app/api'
import { Toggle } from '../components/ui'
import { updateDescription, type AppUpdate } from './UpdateWizard'

export function Settings({
  snapshot,
  run,
  version,
  updater,
}: {
  snapshot: Snapshot
  run: Run
  version: string
  updater: AppUpdate
}) {
  const settings = snapshot.settings
  const save = (patch: Partial<Preferences>) =>
    run(() => api.SaveSettings({ ...settings, ...patch }), 'Preferencias guardadas')
  return (
    <>
      <div className="section-heading">
        <h1>Ajustes</h1>
        <span className="badge">Personaliza tu espacio</span>
      </div>
      <section className="panel">
        <h2>Apariencia</h2>
        <p className="hint">A tu manera, también de día.</p>
        <div className="theme-options">
          {[
            { id: 'dark', name: 'Obsidian', icon: '☾' },
            { id: 'light', name: 'Frost', icon: '☀' },
            { id: 'system', name: 'Sistema', icon: '◐' },
          ].map((t) => (
            <button
              key={t.id}
              className={`theme-tile ${settings.theme === t.id ? 'selected' : ''}`}
              aria-pressed={settings.theme === t.id}
              onClick={() => save({ theme: t.id })}
            >
              <span className={`theme-preview ${t.id}`}>{t.icon}</span>
              {t.name}
            </button>
          ))}
        </div>
      </section>
      <section className="panel">
        <h2>Vault de herramientas</h2>
        <p className="hint">
          Las nuevas instalaciones se organizarán aquí. Las existentes conservarán su ubicación.
        </p>
        <p className="mono wrap inset">{settings.vault}</p>
        <button
          className="full"
          onClick={() =>
            run(async () => {
              const vault = await api.ChooseDirectory()
              if (vault) await api.SaveSettings({ ...settings, vault })
            }, 'Carpeta actualizada')
          }
        >
          ▱ Cambiar carpeta
        </button>
      </section>
      <section className="panel">
        <h2>Archivo de preferencias</h2>
        <p className="hint">
          Localiza state.json para guardar una copia de tus preferencias, fuentes e instalaciones.
        </p>
        <p className="mono wrap inset">{snapshot.preferencesFile}</p>
        <div className="row">
          <button className="grow" onClick={() => run(() => api.ShowPreferencesFile())}>
            ▱ Mostrar archivo de preferencias
          </button>
          <button
            aria-label="Copiar ruta de preferencias"
            onClick={() => run(() => Clipboard.SetText(snapshot.preferencesFile), 'Ruta copiada')}
          >
            ⧉
          </button>
        </div>
        <p className="hint">
          Para sustituirlo por una copia anterior, sal completamente desde la bandeja antes de
          reemplazarlo y vuelve a abrir la aplicación.
        </p>
      </section>
      <section className="panel">
        <h2>Comportamiento</h2>
        <Toggle
          label="Iniciar con el sistema"
          detail="Se abrirá en segundo plano al iniciar sesión."
          checked={settings.startAtLogin}
          onChange={(startAtLogin) => save({ startAtLogin })}
        />
        <Toggle
          label="Cerrar a la bandeja"
          detail="Mantén tools-switcher disponible junto al reloj."
          checked={settings.closeToTray}
          onChange={(closeToTray) => save({ closeToTray })}
        />
        <Toggle
          label="Buscar actualizaciones"
          detail="Comprueba nuevas versiones al iniciar y cada seis horas."
          checked={settings.autoCheckUpdates}
          onChange={(autoCheckUpdates) => save({ autoCheckUpdates })}
        />
      </section>
      <section className="panel">
        <h2>Diagnóstico del entorno</h2>
        <dl>
          <dt>Plataforma</dt>
          <dd>
            {snapshot.platform} · {snapshot.architecture}
          </dd>
          <dt>
            JAVA_HOME detectado ·{' '}
            {{ user: 'Usuario', system: 'Sistema', session: 'Sesión' }[snapshot.activeSource] ??
              'Sin configurar'}
          </dt>
          <dd className="mono wrap">{snapshot.activePath || 'Sin configurar'}</dd>
          <dt>Java resuelto por PATH</dt>
          <dd className="mono wrap">{snapshot.resolvedJava || 'No encontrado'}</dd>
        </dl>
        {snapshot.environment?.map((variable) => (
          <div className="environment-detail" key={variable.name}>
            <strong className="mono">{variable.name}</strong>
            <p className="mono wrap">{variable.value || 'Sin configurar'}</p>
            {variable.error && <p className="error-message">{variable.error}</p>}
            {variable.systemValue && variable.systemValue !== variable.value && (
              <small className="wrap">Sistema: {variable.systemValue}</small>
            )}
            {variable.processValue && variable.processValue !== variable.value && (
              <small className="wrap">La sesión de la app conserva: {variable.processValue}</small>
            )}
          </div>
        ))}
        <p className="hint">
          Abre una terminal nueva después de cambiar variables. En Windows, otras entradas del PATH
          del sistema pueden tener prioridad.
        </p>
      </section>
      <section className="panel">
        <div className="row spread">
          <h2>tools-switcher</h2>
          <span className="badge mono">v{version}</span>
        </div>
        <p className="hint">{updateDescription(updater.update)}</p>
        <button className="full" onClick={updater.show}>
          Abrir asistente de actualización
        </button>
      </section>
      <p className="footnote">Tu entorno, bajo control.</p>
    </>
  )
}
