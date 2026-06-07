# telegram-mcub-go

A Go equivalent of [Telethon-MCUB](https://github.com/mcub/telethon-mcub) — a Telegram MTProto client library with MCUB-specific extensions, built on top of [gotd/td](https://github.com/gotd/td).

## Features

- **Middleware chains** — event middleware and request middleware
- **Protection profiles** — `off`, `safe`, `strict`, `custom` with `ProtectionPolicy`
- **Forum topic helpers** — `IterTopics`, `GetTopics`, `CreateTopic`, `SendToTopic`, `SendFileToTopic`, `IterTopicMessages`
- **History export** — `IterHistoryBatches`, `ExportHistory`, `HistoryExportResult`, `FileTransferState`
- **Resumable file transfers** — `DownloadFile` and `SendFile` with `Resume=true`, `ResumeKey`, `StateStore`
- **Reaction methods** — `SendReaction`, `GetMessageReactionsList`, `SetDefaultReaction`
- **Colored buttons** — `ButtonStylePrimary`, `ButtonStyleSuccess`, `ButtonStyleDanger`
- **JoinRequest events** — `JoinRequest`, `JoinRequestFilter`
- **Session management** — `FileSessionStorage`, `StateStore`

## Installation

```bash
go get github.com/nulls-brawl-site/telegram-mcub-go
```

## Quick start

```go
package main

import (
    "context"
    "log"

    "github.com/nulls-brawl-site/telegram-mcub-go/client"
    "github.com/nulls-brawl-site/telegram-mcub-go/events"
    "github.com/nulls-brawl-site/telegram-mcub-go/session"
    "github.com/nulls-brawl-site/telegram-mcub-go/types"
)

func main() {
    store, _ := session.NewFileSessionStorage("session.json")

    c, err := client.New(client.Options{
        AppID:          12345,
        AppHash:        "your_app_hash",
        Session:        store.Storage(),
        ProtectionMode: types.ProtectionSafe,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Add a logging middleware.
    c.AddEventMiddleware(func(ctx context.Context, e events.Event, next events.Handler) error {
        log.Printf("event: %s", e.EventType())
        return next(ctx, e)
    })

    // Handle new messages.
    c.AddEventHandler(events.NewMessageFilter(), func(ctx context.Context, e events.Event) error {
        msg := e.(*events.NewMessage)
        log.Printf("message from %d: %s", msg.SenderID, msg.Text())
        return nil
    })

    ctx := context.Background()
    if err := c.Run(ctx, func(ctx context.Context) error {
        if err := c.AuthenticateAsBot(ctx, "BOT_TOKEN"); err != nil {
            return err
        }
        me, _ := c.GetMe(ctx)
        log.Printf("logged in as @%s", me.Username)
        <-ctx.Done()
        return nil
    }); err != nil {
        log.Fatal(err)
    }
}
```

## Protection modes

| Mode       | Flood wait | Retries | Auto-reconnect | Rate limit |
|------------|-----------|---------|----------------|------------|
| `off`      | none      | 0       | no             | none       |
| `safe`     | 60 s      | 3       | yes            | 30 req/s   |
| `strict`   | 300 s     | 5       | yes            | 10 req/s   |
| `custom`   | configurable                                       |

## Resumable downloads

```go
stateStore, _ := session.NewStateStore(".mcub-states")

err := c.DownloadFile(ctx, client.DownloadParams{
    Location:   inputFileLocation,
    DestPath:   "/tmp/myfile.mp4",
    Resume:     true,
    ResumeKey:  "myfile-download",
    StateStore: stateStore,
    ProgressFunc: func(done, total int64) {
        log.Printf("downloaded %d bytes", done)
    },
})
```

## Forum topics

```go
// List all topics.
err := c.IterTopics(ctx, channelID, 50, func(batch []*client.Topic) bool {
    for _, t := range batch {
        log.Printf("topic %d: %s", t.ID, t.Title)
    }
    return true
})

// Send to a topic.
_, err = c.SendToTopic(ctx, client.SendToTopicParams{
    ChannelID: channelID,
    TopicID:   topicID,
    Text:      "Hello, topic!",
})
```

## License

MIT
