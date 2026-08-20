export default function Preview({ entry, copy }) {
  const rows = [
    [copy.previewRating, entry.day_rating == null ? null : `${entry.day_rating}/10`],
    [copy.previewEmotion, entry.emotion || null],
    [`${copy.well} (${copy.upToThree})`, (entry.went_well || []).length ? entry.went_well : null],
    [
      `${copy.badly} (${copy.upToThree})`,
      (entry.went_badly || []).length ? entry.went_badly : null,
    ],
    [copy.todos, (entry.todos || []).length ? entry.todos : null],
  ]
  return (
    <aside className="preview" aria-label={copy.previewTitle}>
      <p className="caption">{copy.previewTitle}</p>
      <dl>
        {rows.map(([label, value]) => (
          <div key={label} className={`field${value == null ? ' empty' : ''}`}>
            <dt>{label}</dt>
            <dd>
              {value == null ? (
                <span className="gap">{copy.previewGap}</span>
              ) : Array.isArray(value) ? (
                value.map((v, i) => (
                  <span key={i} className="item">
                    {v}
                  </span>
                ))
              ) : (
                value
              )}
            </dd>
          </div>
        ))}
      </dl>
    </aside>
  )
}
