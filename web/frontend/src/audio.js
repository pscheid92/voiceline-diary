const workletSource = `
class Capture extends AudioWorkletProcessor {
  process(inputs) {
    const input = inputs[0][0]
    if (input) this.port.postMessage(new Float32Array(input))
    return true
  }
}
registerProcessor('capture', Capture)
`

export async function startCapture(onChunk, onLevel) {
  const stream = await navigator.mediaDevices.getUserMedia({
    audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true },
  })

  const ctx = new AudioContext({ sampleRate: 16000 })
  await ctx.resume()

  const url = URL.createObjectURL(new Blob([workletSource], { type: 'application/javascript' }))
  await ctx.audioWorklet.addModule(url)
  URL.revokeObjectURL(url)

  const source = ctx.createMediaStreamSource(stream)
  const node = new AudioWorkletNode(ctx, 'capture')

  let frames = []
  let samples = 0
  let acc = 0
  let accCount = 0
  let peak = 0
  let lastLevel = 0

  node.port.onmessage = (e) => {
    const frame = e.data
    frames.push(frame)
    samples += frame.length

    for (let i = 0; i < frame.length; i++) {
      acc += frame[i] * frame[i]
      const a = Math.abs(frame[i])
      if (a > peak) peak = a
    }
    accCount += frame.length

    const now = performance.now()
    if (onLevel && now - lastLevel > 100 && accCount > 0) {
      onLevel(Math.sqrt(acc / accCount), peak)
      acc = 0
      accCount = 0
      peak = 0
      lastLevel = now
    }

    if (samples >= 4000) {
      onChunk(encodePCM(frames, samples))
      frames = []
      samples = 0
    }
  }

  source.connect(node)

  const mute = ctx.createGain()
  mute.gain.value = 0
  node.connect(mute).connect(ctx.destination)

  const track = stream.getAudioTracks()[0]
  const stop = () => {
    node.port.onmessage = null
    stream.getTracks().forEach((t) => t.stop())
    ctx.close()
  }
  stop.label = track?.label || 'microphone'
  return stop
}

function encodePCM(frames, total) {
  const pcm = new Int16Array(total)
  let off = 0
  for (const frame of frames) {
    for (let i = 0; i < frame.length; i++) {
      const s = Math.max(-1, Math.min(1, frame[i]))
      pcm[off++] = s < 0 ? s * 0x8000 : s * 0x7fff
    }
  }
  const bytes = new Uint8Array(pcm.buffer)
  let binary = ''
  for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i])
  return btoa(binary)
}

export function createPlayer(onSpeaking) {
  const ctx = new AudioContext({ sampleRate: 24000 })
  ctx.resume()

  let nextTime = 0
  let sources = []
  let quiet = null

  const started = () => {
    if (quiet) {
      clearTimeout(quiet)
      quiet = null
    }
    if (sources.length === 1 && onSpeaking) onSpeaking(true)
  }
  const ended = () => {
    if (sources.length === 0 && !quiet) {
      quiet = setTimeout(() => {
        quiet = null
        onSpeaking?.(false)
      }, 300)
    }
  }

  return {
    play(b64) {
      const binary = atob(b64)
      const bytes = new Uint8Array(binary.length)
      for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
      const pcm = new Int16Array(bytes.buffer)

      const buf = ctx.createBuffer(1, pcm.length, 24000)
      const channel = buf.getChannelData(0)
      for (let i = 0; i < pcm.length; i++) channel[i] = pcm[i] / 0x8000

      const src = ctx.createBufferSource()
      src.buffer = buf
      src.connect(ctx.destination)
      nextTime = Math.max(nextTime, ctx.currentTime)
      src.start(nextTime)
      nextTime += buf.duration

      sources.push(src)
      started()
      src.onended = () => {
        sources = sources.filter((s) => s !== src)
        ended()
      }
    },

    flush() {
      for (const s of sources) {
        try {
          s.stop()
        } catch {}
      }
      sources = []
      nextTime = 0
      if (quiet) {
        clearTimeout(quiet)
        quiet = null
      }
      onSpeaking?.(false)
    },
    close() {
      if (quiet) clearTimeout(quiet)
      ctx.close()
    },

    finish() {
      const waitFor = nextTime - ctx.currentTime
      if (sources.length === 0 || waitFor <= 0) {
        this.close()
        return
      }
      setTimeout(() => this.close(), waitFor * 1000 + 200)
    },
  }
}
