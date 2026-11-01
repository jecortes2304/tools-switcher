import { useEffect, useRef, useState } from 'react'
import { api, errorMessage, type UpdateStatus } from '../app/api'
import { Modal } from '../components/ui'

export function useAppUpdate() {
  const [update, setUpdate] = useState<UpdateStatus | null>(null)
  const [open, setOpen] = useState(false)
  const [working, setWorking] = useState(false)
  const [error, setError] = useState('')
  const announced = useRef('')
  useEffect(() => {
    let active = true
    let timer: ReturnType<typeof setTimeout>
    const refresh = async () => {
      try {
        const next = await api.UpdateStatus()
        if (active) {
          setUpdate(next)
          if (next.status === 'available' && announced.current !== next.version) {
            announced.current = next.version
            setOpen(true)
          }
        }
      } catch {
        if (active) setError('No se puede consultar el estado. Vuelve a intentarlo.')
      } finally {
        if (active) timer = setTimeout(refresh, 700)
      }
    }
    void refresh()
    return () => {
      active = false
      clearTimeout(timer)
    }
  }, [])
  const act = async () => {
    setWorking(true)
    setError('')
    try {
      if (update?.status === 'ready') await api.Restart()
      else if (
        update?.status === 'available' ||
        (update?.status === 'error' && update.errorStage !== 'check' && update.version)
      )
        await api.InstallUpdate()
      else await api.CheckUpdates()
      setUpdate(await api.UpdateStatus())
    } catch (failure) {
      setError(errorMessage(failure))
    } finally {
      setWorking(false)
    }
  }
  return {
    update,
    open,
    working,
    error,
    act,
    show: () => {
      setError('')
      setOpen(true)
    },
    hide: () => setOpen(false),
  }
}

export type AppUpdate = ReturnType<typeof useAppUpdate>

export function updateDescription(update: UpdateStatus | null) {
  if (!update?.configured) return 'Las actualizaciones aún no están configuradas.'
  return (
    {
      'up-to-date': 'Ya tienes la última versión.',
      available: `Nueva versión disponible: ${update.version}`,
      checking: 'Buscando actualizaciones…',
      downloading: 'Descargando la nueva versión…',
      verifying: 'Verificando integridad y firma…',
      installing: 'Preparando la actualización…',
      ready: 'Todo listo. Reinicia cuando quieras.',
      error: 'No se ha podido completar la actualización.',
    }[update.status] ?? 'Busca una nueva versión de tools-switcher.'
  )
}

function size(value: number) {
  if (value <= 0) return '0 MB'
  return `${(value / 1024 / 1024).toLocaleString('es', { maximumFractionDigits: 1 })} MB`
}

function ReleaseNotes({ notes }: { notes: string }) {
  return (
    <div className="release-notes">
      {notes.split('\n').map((line, index) => {
        const text = line
          .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
          .replace(/\*\*/g, '')
          .trim()
        if (!text) return null
        if (text.startsWith('#')) return <h4 key={index}>{text.replace(/^#+\s*/, '')}</h4>
        return <p key={index}>{text.replace(/^[-*]\s+/, '• ')}</p>
      })}
    </div>
  )
}

export function UpdateWizard({ controller, version }: { controller: AppUpdate; version: string }) {
  const { update, open, working, error, act, hide } = controller
  if (!open) return null
  const state = update?.status ?? 'idle'
  const active = ['checking', 'downloading', 'verifying', 'installing'].includes(state)
  const total = update?.total ?? 0
  const written = update?.written ?? 0
  const percent = total > 0 ? Math.min(100, Math.max(0, Math.round((written / total) * 100))) : null
  const ready = state === 'ready'
  const retryDownload = state === 'error' && !!update?.version && update.errorStage !== 'check'
  const action = ready
    ? 'Reiniciar ahora'
    : state === 'available'
      ? 'Descargar e instalar'
      : retryDownload
        ? 'Reintentar descarga'
        : state === 'error'
          ? 'Volver a comprobar'
          : 'Buscar actualizaciones'
  const symbol =
    ready || state === 'up-to-date' ? '✓' : state === 'error' ? '!' : active ? '↓' : '↗'
  return (
    <Modal title="Actualizar tools-switcher" onClose={hide}>
      <div className={`update-hero ${ready ? 'is-ready' : ''}`}>
        <span className={`update-symbol ${active ? 'is-active' : ''}`} aria-hidden="true">
          {symbol}
        </span>
        <h3>
          {ready
            ? 'Tu próxima versión está lista'
            : state === 'available'
              ? 'Hay novedades para ti'
              : state === 'up-to-date'
                ? 'Estás al día'
                : state === 'error'
                  ? 'Vamos a intentarlo de nuevo'
                  : 'Tu espacio, siempre al día'}
        </h3>
        <p role="status" aria-live="polite">
          {updateDescription(update)}
        </p>
        <div className="update-versions mono">
          <span>v{version}</span>
          {update?.version && update.version !== version && (
            <>
              <span aria-hidden="true">→</span>
              <strong>v{update.version}</strong>
            </>
          )}
        </div>
      </div>
      {(state === 'downloading' || state === 'verifying' || state === 'installing' || ready) && (
        <section className="update-progress" aria-label="Progreso de actualización">
          <div className="row spread">
            <strong>
              {ready
                ? 'Preparada'
                : state === 'downloading'
                  ? 'Descarga'
                  : state === 'verifying'
                    ? 'Verificación de firma'
                    : 'Preparación'}
            </strong>
            <span className="mono">
              {ready
                ? '100 %'
                : state === 'downloading' && percent !== null
                  ? `${percent} %`
                  : 'En curso'}
            </span>
          </div>
          <progress
            aria-label="Descarga de actualización"
            max={100}
            value={ready ? 100 : state === 'downloading' ? (percent ?? undefined) : undefined}
          />
          <p className="hint mono">
            {size(written)}
            {total > 0 ? ` de ${size(total)}` : ''}
            {state === 'downloading' && (update?.rate ?? 0) > 0 ? ` · ${size(update!.rate)}/s` : ''}
          </p>
        </section>
      )}
      {update?.notes && (
        <section className="update-notes">
          <h3>Novedades de esta versión</h3>
          <ReleaseNotes notes={update.notes} />
        </section>
      )}
      {(error || update?.error) && (
        <p className="error-message" role="alert">
          {error || update?.error}
        </p>
      )}
      <p className="hint update-assurance">
        {ready
          ? 'Al reiniciar se aplicará la actualización. Tus preferencias y JDKs se conservarán.'
          : 'La descarga se verifica con la firma de tools-switcher antes de preparar la instalación.'}
      </p>
      <div className="dialog-actions update-actions">
        <button onClick={hide}>
          {active ? 'Ocultar' : ready ? 'Reiniciar más tarde' : 'Más tarde'}
        </button>
        <button
          className="primary grow"
          disabled={working || active || !update?.configured}
          onClick={() => void act()}
        >
          {active ? (state === 'checking' ? 'Buscando…' : 'Actualizando…') : action}
        </button>
      </div>
      {active && (
        <p className="hint">Puedes ocultar esta ventana; el proceso continuará en segundo plano.</p>
      )}
    </Modal>
  )
}
