import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import WS from 'vitest-websocket-mock'
import { useVoiceSession } from './useVoiceSession.js'

const player = { play: vi.fn(), flush: vi.fn(), finish: vi.fn(), close: vi.fn() }
const mic = { speak: null }

vi.mock('./audio.js', () => ({
  createPlayer: () => player,
  startCapture: async (onChunk) => {
    mic.speak = (b64) => act(() => onChunk(b64))
    return Object.assign(() => {}, { label: 'Fake Mic' })
  },
}))

let server

beforeEach(() => {
  server = new WS(`ws://${location.host}/api/v1/voice`, { jsonProtocol: true })
})

afterEach(() => {
  WS.clean()
  vi.clearAllMocks()
})

async function talking() {
  const { result } = renderHook(() => useVoiceSession())
  await act(async () => {
    await result.current.start()
  })
  await server.connected
  await says({ type: 'ready' })
  await waitFor(() => expect(result.current.status).toBe('talking'))
  return result
}

async function says(msg) {
  await act(async () => {
    server.send(msg)
  })
}

function transcript(index, role, text) {
  return { type: 'turn', turn: { index, role, text, closed: false } }
}

describe('useVoiceSession', () => {
  it('carries a whole conversation through to a saved entry', async () => {
    const result = await talking()
    const day = { day_rating: 7, emotion: 'content' }
    const entry = { ...day, transcript: 'Me: a good day\n' }

    await says(transcript(0, 'user', 'A good day.'))
    await says({ type: 'draft', draft: day, complete: true })

    expect(result.current.turns[0].text).toBe('A good day.')
    expect(result.current.draft).toEqual(day)
    expect(result.current.complete).toBe(true)

    act(() => result.current.finish())
    await expect(server).toReceiveMessage({ type: 'end' })

    await says({ type: 'concluded', draft: entry, code: '' })
    expect(result.current.status).toBe('review')
    expect(result.current.entry).toEqual(entry)

    act(() => result.current.save())
    const saved = await server.nextMessage
    expect(saved.type).toBe('save')
    expect(saved.at).toBeTruthy()

    await says({ type: 'filed', filed: { ...entry, url: 'https://notion.so/p' } })
    expect(result.current.status).toBe('saved')
    expect(result.current.result.url).toBe('https://notion.so/p')
  })

  it('leaves nothing of the last conversation behind when a draft is discarded', async () => {
    const result = await talking()
    const entry = { day_rating: 7, emotion: 'content', transcript: 'Me: a good day\n' }

    await says(transcript(0, 'user', 'A good day.'))
    await says({ type: 'draft', draft: { day_rating: 7, emotion: 'content' }, complete: true })
    await says({ type: 'concluded', draft: entry, code: 'incomplete' })

    act(() => result.current.discard())
    await says({ type: 'discarded' })

    expect(result.current.status).toBe('idle')
    expect(result.current.turns).toEqual([])
    expect(result.current.draft).toBeNull()
    expect(result.current.entry).toBeNull()
    expect(result.current.entryNote).toBe('')
    expect(result.current.complete).toBe(false)
    expect(result.current.incomplete).toBe(false)
    expect(result.current.asking).toBe('')
  })

  it('hands the floor back when finishing has to wait', async () => {
    const result = await talking()

    act(() => result.current.finish())
    expect(result.current.status).toBe('saving')

    await says({ type: 'asking' })

    expect(result.current.status).toBe('talking')
    expect(result.current.asking).not.toBe('')
  })

  it('marks a concluded draft that is not an entry', async () => {
    const result = await talking()

    await says({
      type: 'concluded',
      draft: { transcript: 'Me: a quiet day\n' },
      code: 'incomplete',
    })

    expect(result.current.status).toBe('review')
    expect(result.current.incomplete).toBe(true)
    expect(result.current.entryNote).not.toBe('')
  })

  it('keeps the entry when the socket dies during review', async () => {
    const result = await talking()
    const entry = { day_rating: 7, emotion: 'content', transcript: 'Me: a good day\n' }

    await says({ type: 'concluded', draft: entry, code: '' })
    await act(async () => {
      server.close()
    })

    expect(result.current.status).toBe('review')
    expect(result.current.entry).toEqual(entry)
    expect(result.current.entryNote).not.toBe('')
  })

  it('fails when the socket dies mid-conversation', async () => {
    const result = await talking()

    await act(async () => {
      server.close()
    })

    expect(result.current.status).toBe('error')
    expect(result.current.error).not.toBe('')
  })

  it('replaces a growing turn instead of appending to it', async () => {
    const result = await talking()

    await says(transcript(0, 'user', 'Today was'))
    await says(transcript(0, 'user', 'Today was a good day.'))
    await says(transcript(1, 'companion', 'Glad to hear it.'))

    expect(result.current.turns).toHaveLength(2)
    expect(result.current.turns[0].text).toBe('Today was a good day.')
    expect(result.current.turns[1].role).toBe('companion')
  })

  it('translates an error code into something a person can read', async () => {
    const result = await talking()

    await says({ type: 'error', code: 'nothing_said' })

    expect(result.current.status).toBe('error')
    expect(result.current.error).not.toBe('')
    expect(result.current.error).not.toContain('nothing_said')
  })

  it('plays what the companion says and drops it when interrupted', async () => {
    const result = await talking()

    await says({ type: 'audio', data: 'AAAA' })
    await says({ type: 'interrupted' })

    expect(player.play).toHaveBeenCalledWith('AAAA')
    expect(player.flush).toHaveBeenCalled()
    expect(result.current.status).toBe('talking')
  })

  it('opens the socket at the moment this browser says it is', async () => {
    await talking()

    const [client] = server.server.clients()
    const now = new URL(client.url).searchParams.get('now')
    expect(now).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{2}:\d{2}$/)
  })

  it('stops the companion hearing or answering while paused', async () => {
    const result = await talking()

    mic.speak('before')
    await expect(server).toReceiveMessage({ type: 'audio', data: 'before' })

    act(() => result.current.togglePause())
    expect(result.current.paused).toBe(true)
    expect(player.flush).toHaveBeenCalled()

    mic.speak('during')
    await says({ type: 'audio', data: 'ignore-me' })

    expect(player.play).not.toHaveBeenCalledWith('ignore-me')

    act(() => result.current.togglePause())
    expect(result.current.paused).toBe(false)

    mic.speak('after')
    await expect(server).toReceiveMessage({ type: 'audio', data: 'after' })
  })

  it('never starts a new conversation still paused', async () => {
    const result = await talking()

    act(() => result.current.togglePause())
    expect(result.current.paused).toBe(true)

    await act(async () => {
      await result.current.start()
    })

    expect(result.current.paused).toBe(false)
  })

  it('goes home from a finished entry without keeping any of it', async () => {
    const result = await talking()
    const entry = { day_rating: 7, emotion: 'content', transcript: 'Me: a good day\n' }

    await says(transcript(0, 'user', 'A good day.'))
    await says({ type: 'concluded', draft: entry, code: '' })
    await says({ type: 'filed', filed: { ...entry, url: 'https://notion.so/p' } })
    expect(result.current.status).toBe('saved')

    act(() => result.current.home())

    expect(result.current.status).toBe('idle')
    expect(result.current.turns).toEqual([])
    expect(result.current.entry).toBeNull()
    expect(result.current.result).toBeNull()
  })

  it('offers to end a turn only when there is one to end', async () => {
    const result = await talking()

    expect(result.current.midSentence).toBe(false)

    await says(transcript(0, 'companion', 'Hi, how was your day?'))
    expect(result.current.midSentence).toBe(false)

    await says(transcript(1, 'user', 'It was'))
    expect(result.current.midSentence).toBe(true)

    await says(transcript(2, 'companion', 'Go on.'))
    expect(result.current.midSentence).toBe(false)
  })
})
