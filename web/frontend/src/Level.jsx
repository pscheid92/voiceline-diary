export default function Level({ level }) {
  const lit = level < 0.002 ? 0 : Math.max(1, Math.min(7, Math.round(Math.sqrt(level) * 20)))
  return (
    <span className="level" aria-hidden="true">
      {Array.from({ length: 7 }, (_, i) => (
        <span key={i} className={`tick${i < lit ? ' lit' : ''}`} />
      ))}
    </span>
  )
}
