import { useEffect, useRef, useState } from 'react'
import { COPY as t, today } from './copy.js'
import { ROLE } from './protocol.js'
import { STATUS, useVoiceSession } from './useVoiceSession.js'
import Entry from './Entry.jsx'
import Preview from './Preview.jsx'
import Level from './Level.jsx'

export default function App() {
  const [diaryURL, setDiaryURL] = useState('')
  const s = useVoiceSession()
  const lastLine = useRef(null)

  useEffect(() => {
    fetch('/api/v1/config')
      .then((r) => r.json())
      .then((c) => setDiaryURL(c.diary_url || ''))
      .catch(() => {})
  }, [])

  useEffect(() => {
    lastLine.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  }, [s.turns.length])

  return (
    <main className="page">
      <header className="masthead">
        {s.status === STATUS.saved || s.status === STATUS.error ? (
          <button className="wordmark wordmark-home" onClick={s.home}>
            {t.wordmark}
          </button>
        ) : (
          <span className="wordmark">{t.wordmark}</span>
        )}
        {s.status !== STATUS.review && s.status !== STATUS.saved && (
          <span className="today">{today()}</span>
        )}
      </header>

      {s.status === STATUS.idle && (
        <section className="stage">
          <h1 className="ask">{t.ask}</h1>
          <p className="lede">{t.lede}</p>
          <div className="row">
            <button className="btn btn-primary" onClick={s.start}>
              {t.start}
            </button>
          </div>
          <p className="aside">{t.micNeeded}</p>
          {diaryURL && (
            <a className="quiet" href={diaryURL} target="_blank" rel="noreferrer">
              {t.diary}
            </a>
          )}
        </section>
      )}

      {s.status === STATUS.connecting && <p className="waiting">{t.connecting}</p>}
      {s.status === STATUS.saving && <p className="waiting">{t.saving}</p>}

      {s.turns.length > 0 && s.status !== STATUS.saved && s.status !== STATUS.review && (
        <div className="talking">
          <section className="script">
            {s.turns.filter(Boolean).map((turn, i) => (
              <p
                key={turn.index}
                ref={i === s.turns.length - 1 ? lastLine : null}
                className={`line line-${turn.role}`}
              >
                <span className="who">{turn.role === ROLE.user ? t.you : t.companion}</span>
                <span className="said">{turn.text}</span>
              </p>
            ))}
          </section>
          {s.draft && <Preview entry={s.draft} copy={t} />}
        </div>
      )}

      {s.status === STATUS.talking && (
        <div className="row controls">
          <span
            className={`state state-${s.paused ? 'paused' : s.speaking ? 'speaking' : 'listening'}`}
          >
            <span className="pip" aria-hidden="true" />
            {s.paused ? t.paused : s.speaking ? t.speaking : t.listening}
            {!s.paused && !s.speaking && (
              <>
                <Level level={s.level} />
                <span className="reading">{s.level < 0.002 ? t.quiet : t.hearingYou}</span>
              </>
            )}
          </span>
          <span className="spacer" />
          <button className="btn btn-ghost" onClick={s.togglePause}>
            {s.paused ? t.resume : t.pause}
          </button>
          <button
            className="btn btn-ghost"
            onClick={s.endTurn}
            disabled={s.paused || !s.midSentence}
          >
            {t.done}
          </button>
          <button
            className={`btn btn-primary${s.complete ? ' offered' : ''}`}
            onClick={s.finish}
            disabled={s.paused}
          >
            {t.finish}
          </button>
          {s.asking && <p className="aside offer">{s.asking}</p>}
          {s.complete && !s.asking && <p className="aside offer">{t.offer}</p>}
          {s.clipping && <p className="warn">{t.clipping}</p>}
          {s.micLabel && <p className="aside">{s.micLabel}</p>}
          <p className="leaving">
            <button className="btn btn-quiet" onClick={s.abort}>
              {t.abort}
            </button>
          </p>
        </div>
      )}

      {s.status === STATUS.error && (
        <section className="stage">
          <p className="warn">{s.error}</p>
          <button className="btn btn-primary" onClick={s.start}>
            {t.retry}
          </button>
        </section>
      )}

      {s.status === STATUS.review && s.entry && (
        <Entry entry={s.entry} copy={t} caption={t.review} note={s.entryNote}>
          {!s.incomplete && (
            <button className="btn btn-primary" onClick={s.save}>
              {t.save}
            </button>
          )}
          <button className={`btn btn-${s.incomplete ? 'primary' : 'ghost'}`} onClick={s.discard}>
            {t.discard}
          </button>
        </Entry>
      )}

      {s.status === STATUS.saved && s.result && (
        <Entry entry={s.result} copy={t} caption={t.saved}>
          <a className="btn btn-primary" href={s.result.url} target="_blank" rel="noreferrer">
            {t.open}
          </a>
          <button className="btn btn-ghost" onClick={s.start}>
            {t.again}
          </button>
        </Entry>
      )}
    </main>
  )
}
