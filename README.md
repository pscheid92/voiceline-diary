# voiceline diary

Talk about your day. A Gemini Live model holds the conversation, writes the entry
as you speak, and it goes to Notion once you say yes.

## Run it

Needs a Gemini API key ([AI Studio](https://aistudio.google.com/apikey)) and a
Notion integration ([here](https://www.notion.so/profile/integrations)) shared
with a database — any database; entries arrive as pages, no particular columns.
The 32-character ID is in the database URL.

```bash
cp .env.example .env      # three blanks: Gemini key, Notion token, database ID
make dev                  # http://localhost:5173
```

`make build && ./bin/voiceline-diary` gives the single binary with the frontend
embedded, on `:8080`. That step needs node.

```bash
make test        # go + frontend, no credentials
make test-live   # talks to the real model, costs a few cents
```

## What happens

You press start and talk. The model records into the entry with tools as things
are said — a rating out of ten, a mood, up to three things that went well, up to
three that went badly, anything on your mind for tomorrow — and you watch it fill
in beside the transcript.

Before the day can be filed it must own a rating and a mood, and that rule
answers whoever asks: the finish button and the model's own goodbye alike. Then
you read the entry and either save it or throw it away. Nothing reaches Notion
without that yes.

## How it is put together

```
browser ──websocket──▶ wire ──▶ session ──▶ gemini  (the companion)
                                    └────▶ notion  (the diary)
```

`session` is the product: one conversation, one goroutine, no locks. It owns the
day being written and is the only thing that changes it, which is what lets one
rule answer both the person and the model.

Ports are named for the actor that plays them, packages for what they are made
of — `session.Companion` is played by `internal/gemini`, `session.Diary` by
`internal/notion`, `session.User` by `internal/wire`. None of the three imports
`session`; they satisfy it structurally through `internal/diary` and
`internal/conversation`. `.golangci.yml` enforces that graph as a build rule.

In dependency order, nothing refers forward:

| | |
|---|---|
| `internal/diary` | what a day is, and every rule one is filed under |
| `internal/conversation` | the words two packages need for a live exchange |
| `internal/session` | the ports, the use cases, the events, the loop |
| `internal/wire` | the websocket and the protocol the browser matches |
| `internal/gemini` | the companion, its tools, and `prompt.md` |
| `internal/notion` | rendering an entry into a page |

Twenty minutes rather than two hours: `diary/day.go`, `session/ports.go`,
`session/usecases.go`, `session/session.go`, `gemini/prompt.md`. About 500 lines,
and the whole argument.

`prompt.md` is a file rather than a Go string because it is prose, and because it
turned out to be the most behaviour-critical thing here.

## Why there are live tests

Two things about a live model cannot be learned from a double, and both cost me
an afternoon. Declaring the recording tools `NON_BLOCKING` let the model end its
turn on a bare tool call and say nothing at all. A paragraph added to the prompt
about refusals made it so wary of recording that it stopped writing the day down.
Every unit test stayed green through both.

`internal/gemini/live_test.go` runs against the real model with the config that
ships, skipped without a key. There is also a test that reads both sides of the
wire protocol — the Go constants and the browser's strings — and fails if the
service can send a code the interface has no words for.

## Deploying

CI builds one image to `ghcr.io` tagged `YYYYMMDD-HHmmss-<sha>`. My cluster is
FluxCD-managed: an ImagePolicy picks up the newest tag and writes it back to the
manifests. The image is distroless, runs as nonroot, and is configured entirely
by environment.

## What I deliberately did not build

**You cannot edit an entry** — not while talking, not on the review screen. If
the model mishears you, the recourse is to say it again or discard the
conversation. This is the sharpest limitation and the first thing I would fix.

**The transcript is the speech recogniser's, verbatim**, mistakes included.

**No graceful shutdown.** SIGTERM ends conversations in flight.

**No resumption across a lost browser socket.** The Gemini side survives its own
connection being rotated; the browser's dropping ends the conversation and the
entry is offered as salvage.

**No persistence of its own.** No database. What is not filed to Notion is gone
when the tab closes, on purpose.

**The clock falls back.** The browser sends its own because it is the only one
that knows your timezone; if it sends nothing usable the server's is used, which
can date an entry to the wrong day near midnight.

**A refusal does not reliably reach you in words.** When a fourth good thing is
turned away the model is told why and mentions it about one time in four. The
entry on screen is the honest channel.
