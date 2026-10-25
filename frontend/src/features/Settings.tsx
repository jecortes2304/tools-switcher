import { useEffect, useState } from 'react'
import {
  api,
  type Run,
  type Settings as Preferences,
  type Snapshot,
  type UpdateStatus,
} from '../app/api'
import { Toggle } from '../components/ui'

export function Settings({
  snapshot,
  run,
  version,
}: {
  snapshot: Snapshot
  run: Run
  version: string
}) {
  const [update, setUpdate] = useState<UpdateStatus | null>(null)
  const [working, setWorking] = useState(false)
  useEffect(() => {
    let active = true
    const refresh = () =>
      api
        .UpdateStatus()
        .then((v) => {
          if (active) setUpdate(v)
        })
        .catch(() => {})
    void refresh()
    const timer = setInterval(refresh, 1500)
    return () => {
      active = false
      clearInterval(timer)
    }
  }, [])
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
          <dt>JAVA_HOME del usuario</dt>
          <dd className="mono wrap">{snapshot.activePath || 'Sin configurar'}</dd>
          <dt>Java resuelto por PATH</dt>
          <dd className="mono wrap">{snapshot.resolvedJava || 'No encontrado'}</dd>
        </dl>
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
        <p className="hint">
          {!update?.configured
            ? 'Las actualizaciones firmadas estarán disponibles cuando se configure la clave de publicación.'
            : ({
                'up-to-date': 'Ya tienes la última versión.',
                available: `Nueva versión disponible: ${update.version}`,
                checking: 'Buscando actualizaciones…',
                downloading: 'Descargando actualización…',
                verifying: 'Verificando firma…',
                installing: 'Instalando…',
                ready: 'Actualización preparada. Reinicia para terminar.',
              }[update.status] ?? 'Actualizaciones verificadas mediante firma digital.')}
        </p>
        {update?.error && <p className="error-message">{update.error}</p>}
        <button
          className="full"
          disabled={
            working ||
            !update?.configured ||
            ['checking', 'downloading', 'verifying', 'installing'].includes(update?.status ?? '')
          }
          onClick={async () => {
            setWorking(true)
            await run(async () => {
              if (update?.status === 'ready') await api.Restart()
              else if (update?.status === 'available') await api.InstallUpdate()
              else setUpdate(await api.CheckUpdates())
            })
            setWorking(false)
          }}
        >
          {update?.status === 'ready'
            ? 'Reiniciar tools-switcher'
            : update?.status === 'available'
              ? 'Descargar e instalar'
              : 'Comprobar actualizaciones'}
        </button>
      </section>
      <p className="footnote">Tu entorno, bajo control.</p>
    </>
  )
}
