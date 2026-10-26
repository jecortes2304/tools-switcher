import { useCallback, useEffect, useState } from 'react'
import { api, errorMessage, type Run, type Snapshot } from './app/api'
import { JDKs } from './features/JDKs'
import { Repositories } from './features/Repositories'
import { Settings } from './features/Settings'
import './styles.css'

const tabs = [
  { id: 'jdks', label: 'JDKs', icon: '◇' },
  { id: 'repositories', label: 'Repositorios', icon: '▱' },
  { id: 'settings', label: 'Ajustes', icon: '☷' },
]

export default function App() {
  const [tab, setTab] = useState('jdks')
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null)
  const [version, setVersion] = useState('0.1.0')
  const [message, setMessage] = useState<{ text: string; error: boolean } | null>(null)
  const [startupError, setStartupError] = useState('')
  const refresh = useCallback(async () => setSnapshot(await api.Snapshot()), [])
  useEffect(() => {
    let active = true
    let timer: ReturnType<typeof setTimeout>
    const poll = async () => {
      try {
        const next = await api.Snapshot()
        if (active) {
          setSnapshot(next)
          setStartupError('')
        }
      } catch (error) {
        if (active) setStartupError(errorMessage(error))
      } finally {
        if (active) timer = setTimeout(poll, 1500)
      }
    }
    void poll()
    api
      .Version()
      .then((v) => {
        if (active) setVersion(v)
      })
      .catch(() => {})
    return () => {
      active = false
      clearTimeout(timer)
    }
  }, [])
  useEffect(() => {
    if (!message) return
    const timer = setTimeout(() => setMessage(null), message.error ? 10000 : 4500)
    return () => clearTimeout(timer)
  }, [message])
  useEffect(() => {
    const theme = snapshot?.settings.theme ?? 'dark'
    const media = matchMedia('(prefers-color-scheme: dark)')
    const apply = () => {
      document.documentElement.dataset.theme =
        theme === 'system' ? (media.matches ? 'dark' : 'light') : theme
    }
    apply()
    media.addEventListener('change', apply)
    return () => media.removeEventListener('change', apply)
  }, [snapshot?.settings.theme])
  const run: Run = async (task, success) => {
    try {
      await task()
      await refresh()
      if (success) setMessage({ text: success, error: false })
      return true
    } catch (error) {
      setMessage({ text: errorMessage(error), error: true })
      await refresh().catch(() => {})
      return false
    }
  }
  const activeCount = snapshot?.installations?.length ?? 0
  const running =
    snapshot?.downloads?.filter((j) => ['queued', 'downloading', 'extracting'].includes(j.status))
      .length ?? 0
  return (
    <div className="app-shell">
      <header className="app-header">
        <img className="brand-icon" src="/appicon.png" alt="" />
        <div>
          <strong>
            tools<span className="brand-accent">-switcher</span>
          </strong>
          <small>
            <span className="dot live" /> Tu entorno de desarrollo
          </small>
        </div>
        <span className="version mono">v{version}</span>
      </header>
      <main id="main-content">
        {startupError && (
          <div className="error-banner" role="alert">
            No se ha podido conectar con la aplicación: {startupError}
            <button onClick={() => run(refresh)}>Reintentar</button>
          </div>
        )}
        {!snapshot && !startupError && (
          <div className="empty">
            <span className="empty-icon">⌘</span>
            <h2>Preparando tu espacio…</h2>
          </div>
        )}
        {snapshot && (
          <>
            {tab === 'jdks' && <JDKs snapshot={snapshot} run={run} />}
            {tab === 'repositories' && (
              <Repositories repositories={snapshot.repositories ?? []} run={run} />
            )}
            {tab === 'settings' && <Settings snapshot={snapshot} run={run} version={version} />}
          </>
        )}
      </main>
      <footer className="status-bar">
        <span>
          <span className="dot live" />{' '}
          {running ? `${running} instalaciones en curso` : `${activeCount} JDKs registradas`}
        </span>
        <span className="mono">{snapshot?.architecture ?? 'LOCAL'}</span>
      </footer>
      <nav className="navigation" aria-label="Principal">
        {tabs.map((t) => (
          <button
            key={t.id}
            className={tab === t.id ? 'selected' : ''}
            aria-current={tab === t.id ? 'page' : undefined}
            onClick={() => setTab(t.id)}
          >
            <span aria-hidden="true">{t.icon}</span>
            {t.label}
          </button>
        ))}
      </nav>
      {message && (
        <div
          className={`toast ${message.error ? 'toast-error' : ''}`}
          role={message.error ? 'alert' : 'status'}
        >
          <span>{message.text}</span>
          <button aria-label="Cerrar aviso" onClick={() => setMessage(null)}>
            ×
          </button>
        </div>
      )}
    </div>
  )
}
