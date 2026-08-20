import { formatISO } from 'date-fns'
import { useRef, useState } from 'react'
import { createPlayer, startCapture } from './audio.js'
import { COPY as t } from './copy.js'
import { CMD, CODE, MSG, ROLE } from './protocol.js'

export const STATUS = {
  idle: 'idle',
  connecting: 'connecting',
  talking: 'talking',
  saving: 'saving',
  review: 'review',
  saved: 'saved',
  error: 'error',
}

const CLIPPING = 0.99
const CLIP_SHOWN_FOR = 3000

function socketURL() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${location.host}/api/v1/voice?now=${encodeURIComponent(formatISO(new Date()))}`
}

export function useVoiceSession() {
  const [status, setStatus] = useState(STATUS.idle)
  const [turns, setTurns] = useState([])
  const [entry, setEntry] = useState(null)
  const [draft, setDraft] = useState(null)
  const [complete, setComplete] = useState(false)
  const [entryNote, setEntryNote] = useState('')
  const [asking, setAsking] = useState('')
  const [incomplete, setIncomplete] = useState(false)
  const [result, setResult] = useState(null)
  const [error, setError] = useState('')
  const [speaking, setSpeaking] = useState(false)
  const [level, setLevel] = useState(0)
  const [clipping, setClipping] = useState(false)
  const [micLabel, setMicLabel] = useState('')
  const [paused, setPaused] = useState(false)

  const wsRef = useRef(null)
  const stopCaptureRef = useRef(null)
  const playerRef = useRef(null)
  const doneRef = useRef(false)
  const reviewRef = useRef(false)
  const clipTimer = useRef(null)
  const pausedRef = useRef(false)

  const said = turns.filter(Boolean)
  const midSentence = said.length > 0 && said[said.length - 1].role === ROLE.user

  const cleanup = (letFinish = false) => {
    if (stopCaptureRef.current) {
      stopCaptureRef.current()
      stopCaptureRef.current = null
    }
    if (playerRef.current) {
      if (letFinish) playerRef.current.finish()
      else playerRef.current.close()
      playerRef.current = null
    }
  }

  const fail = (message) => {
    doneRef.current = true
    cleanup()
    setError(message)
    setStatus(STATUS.error)
  }

  const reset = () => {
    setError('')
    setResult(null)
    setEntry(null)
    setEntryNote('')
    setTurns([])
    setDraft(null)
    setComplete(false)
    setAsking('')
    setIncomplete(false)
    setSpeaking(false)
    setLevel(0)
    setPaused(false)
    pausedRef.current = false
  }

  const goIdle = () => {
    doneRef.current = true
    reviewRef.current = false
    reset()
    setStatus(STATUS.idle)
  }

  const openMicrophone = () =>
    startCapture(
      (b64) => {
        if (!pausedRef.current) send(CMD.audio, { data: b64 })
      },
      (rms, peak) => {
        setLevel(rms)
        if (peak < CLIPPING) return

        setClipping(true)
        clearTimeout(clipTimer.current)
        clipTimer.current = setTimeout(() => setClipping(false), CLIP_SHOWN_FOR)
      },
    )

  const send = (type, fields = {}) => {
    const ws = wsRef.current
    if (ws?.readyState === WebSocket.OPEN && !doneRef.current) {
      ws.send(JSON.stringify({ type, ...fields }))
    }
  }

  const arriving = {
    [MSG.ready]: () => setStatus(STATUS.talking),

    [MSG.audio]: (msg) => {
      if (!pausedRef.current) playerRef.current?.play(msg.data)
    },

    [MSG.interrupted]: () => playerRef.current?.flush(),

    [MSG.turn]: (msg) =>
      setTurns((prev) => {
        const next = prev.slice()
        next[msg.turn.index] = msg.turn
        return next
      }),

    [MSG.draft]: (msg) => {
      setDraft(msg.draft)
      setComplete(!!msg.complete)
    },

    [MSG.asking]: () => {
      setStatus(STATUS.talking)
      setAsking(t.stillAsking)
    },

    [MSG.concluded]: (msg) => {
      cleanup(true)
      reviewRef.current = true
      setEntry(msg.draft)
      setEntryNote(
        { [CODE.salvaged]: t.salvagedNote, [CODE.incomplete]: t.incompleteNote }[msg.code] || '',
      )
      setIncomplete(msg.code === CODE.incomplete)
      setStatus(STATUS.review)
    },

    [MSG.discarded]: () => goIdle(),

    [MSG.filed]: (msg) => {
      doneRef.current = true
      reviewRef.current = false
      cleanup()
      setResult(msg.filed)
      setStatus(STATUS.saved)
    },

    [MSG.error]: (msg) => fail(t.errors[msg.code] || t.errors.generic),
  }

  const lost = (why) => {
    if (doneRef.current) return
    if (reviewRef.current) {
      doneRef.current = true
      setEntryNote(t.lostReview)
      return
    }
    fail(why)
  }

  const listen = (ws) => {
    ws.onmessage = (e) => {
      const msg = JSON.parse(e.data)
      arriving[msg.type]?.(msg)
    }
    ws.onclose = () => lost(t.connectionLost)
    ws.onerror = () => lost(t.connectionFailed)
  }

  const start = async () => {
    reset()
    doneRef.current = false
    reviewRef.current = false
    setStatus(STATUS.connecting)

    playerRef.current = createPlayer(setSpeaking)
    try {
      stopCaptureRef.current = await openMicrophone()
      setMicLabel(stopCaptureRef.current.label)
    } catch {
      fail(t.micDenied)
      return
    }

    wsRef.current = new WebSocket(socketURL())
    listen(wsRef.current)
  }

  const home = () => {
    cleanup()
    goIdle()
  }

  const togglePause = () => {
    const next = !pausedRef.current
    pausedRef.current = next
    setPaused(next)
    if (next) playerRef.current?.flush()
  }

  const endTurn = () => send(CMD.endTurn)

  const conclude = (type) => {
    setAsking('')
    setStatus(STATUS.saving)
    send(type)
  }

  const finish = () => conclude(CMD.finish)

  const abort = () => conclude(CMD.leave)

  const save = () => {
    setStatus(STATUS.saving)
    send(CMD.save, { at: formatISO(new Date()) })
  }

  const discard = () => send(CMD.discard)

  return {
    status,
    turns,
    entry,
    entryNote,
    asking,
    incomplete,
    draft,
    complete,
    result,
    error,
    speaking,
    level,
    clipping,
    micLabel,
    paused,
    midSentence,
    togglePause,
    home,
    start,
    endTurn,
    finish,
    abort,
    save,
    discard,
  }
}
