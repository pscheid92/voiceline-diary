import { today } from './copy.js'

export default function Entry({ entry, copy, caption, note, children }) {
  return (
    <section className="entry">
      <p className="caption">{caption}</p>
      {note && <p className="warn">{note}</p>}

      <h2 className="entry-date">{today()}</h2>
      <Scale value={entry.day_rating} copy={copy} />
      {entry.emotion && (
        <p className="emotion">
          {copy.mostly} <em>{entry.emotion}</em>
        </p>
      )}

      <div className="halves">
        <List title={copy.well} items={entry.went_well} tone="good" hint={copy.upToThree} />
        <List title={copy.badly} items={entry.went_badly} tone="hard" hint={copy.upToThree} />
      </div>
      <List title={copy.todos} items={entry.todos} tone="todo" />

      {entry.transcript && (
        <details className="transcript">
          <summary>{copy.transcript}</summary>
          <p>{entry.transcript}</p>
        </details>
      )}

      <div className="row row-actions">{children}</div>
    </section>
  )
}

function Scale({ value, copy }) {
  if (value == null) return <p className="scale-empty">{copy.noRating}</p>
  return (
    <div className="scale" role="img" aria-label={`${value}/10`}>
      {Array.from({ length: 10 }, (_, i) => {
        const n = i + 1
        return (
          <span
            key={n}
            className={`step${n === value ? ' here' : ''}${n < value ? ' passed' : ''}`}
          >
            <span className="mark" />
            <span className="num">{n}</span>
          </span>
        )
      })}
    </div>
  )
}

function List({ title, items, tone, hint }) {
  if (!items || items.length === 0) return null
  return (
    <div className={`list list-${tone}`}>
      <h3>
        {title}
        {hint && <span className="cap">{hint}</span>}
      </h3>
      <ul>
        {items.map((item, i) => (
          <li key={i}>{item}</li>
        ))}
      </ul>
    </div>
  )
}
