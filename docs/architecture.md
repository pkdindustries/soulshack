# Soulshack Architecture

## System Overview

Soulshack is an IRC bot designed to bridge traditional IRC chat with modern LLM capabilities. It uses a modular architecture to handle IRC events, manage conversations, and invoke LLM agents.

A key feature is its **Unified Tool System**, which abstracts differences between native Go tools, shell scripts, and MCP servers, allowing the LLM to use them interchangeably.

## Component Diagram

<img src="images/diagram.png" alt="Soulshack Architecture" width="25%">

## Request Lifecycle

1.  **Event Reception**: A single `ALL_EVENTS` handler receives every IRC event from `girc`.
2.  **Early Exit**: The handler checks `Registry.Handles()` and drops events with no registered behaviors.
3.  **Context Creation**: A `ChatContext` is created, wrapping the event and configuration.
4.  **Behavior Dispatch**: The `Registry.Process()` method iterates registered behaviors for the event type. The first behavior whose `Check()` returns true wins — its `Execute()` runs and no further behaviors are evaluated.
5.  **Execution**:
    -   **Commands** (via `AddressedBehavior` / `NonAddressedBehavior`) are dispatched to the `CommandRegistry` or sent to the LLM.
    -   **Passive behaviors** (URL watcher, op watcher) call the LLM directly.
    -   **Lifecycle behaviors** (connected, nick/channel errors) handle join, retry, or fatal exit.

### Behavior Priority

Registration order in `run.go` determines priority (first-match-wins):

1.  Lifecycle: `Connected`, `NickError`, `ChannelError`
2.  Passive: `URL`, `Op`, `Join`
3.  Chat: `Addressed`, `NonAddressed`

For example, a non-addressed message containing a URL is handled by the URL behavior, not the non-addressed chat behavior.

## Key Interfaces

### `ChatContextInterface`
The primary interface passed to behaviors. It provides access to:
-   IRC operations (Reply, Join, Kick)
-   Configuration
-   User/Channel info
-   The conversation key: the channel, or the sender for private messages

### `Turn`
What commands and completions receive: a `ChatContextInterface` plus the
conversation for this turn (`turn.Conversation`). Turns are handed out by
`core.WithConversation`, which queues behind any turn already running for the
same key, and by `core.WithDetachedConversation`, which runs against a scratch
conversation that is discarded with the turn.

### `System`
Holds the singleton components:
-   `ToolRegistry`: Manages available tools.
-   `Memory`: Holds every conversation (`internal/memory`) — the transcript, its
    artifacts, the request token budget, and its idle expiry.
-   `Config`: The running settings (`config.Store`). Turns read a snapshot, so
    `/set` can change settings while other turns are reading them.
-   `LLM`: The configured LLM client.

### Conversations
pollytool's agent holds no transcript state across turns, so soulshack keeps
conversations as pollytool sessions: `internal/memory` keeps one `Conversation`
per key and hands it to one turn at a time. The turn leases the key's session,
loads its transcript, and appends what it generates; its agent stores large
tool output in the session's artifact store. The store is the SQLite database
named by `--sessiondb`, so conversations survive a restart, or an in-memory
one when that is empty. A conversation idle past `--sessionduration` starts over
on its next turn, and the store deletes abandoned ones.

The transcript is never trimmed. `maxcontext` is the request's
`MaxContextTokens`: polly projects each request to fit it, demoting old tool
results to artifact receipts and omitting the oldest exchanges, and fails a
request that still cannot fit. The agent's `read_transcript` reaches the
omitted exchanges.

What polly cannot shrink is bounded instead (`internal/llm/budget.go`). Polly's
context floor (the tool schemas, the system prompt and the omission marker) and
a reserve for the tool loop are carried whole; the rest of the budget is room.
The incoming message takes up to half of it (`messageShare`), never so much
that no floor-sized page is left, and is stored as an artifact beyond that.
Polly sizes each batch's pages to the room the request has left when it runs
them, and refuses recall calls beyond it. Both have a floor and a ceiling.
`llm.MinContext` is the budget the floors fit in, and startup, `/set` and
`/tools add` refuse a `maxcontext` below it. Every agent has an artifact store
for this: detached turns and child agents get a scratch session from
`Memory.WithScratch`, deleted when they finish. A turn that fails anyway, as
under a model window smaller than `maxcontext`, keeps what it generated and
replies with a plain `Stopped:` line.

### `LLM`
Abstracts the AI provider.
-   `ChatCompletionStream`: Takes a turn and request, returns a stream of strings.
