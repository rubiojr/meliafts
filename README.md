# Melia Email Database CLI

A [Melia](https://melia.buxjr.com) email search CLI (very much a vibecoded WiP).

## Install

```
go install github.com/rubiojr/meliafts/cmd/ms@latest
```

## Run

```
ms --db ~/.config/melia/melia.db search subject:hello
ms search   subject:privacy isn't dead
1 message
───────────────────────────────────────

[U  ] 2024-03-04 18:50  EFFector List <editor@eff.org>
      Privacy Isn’t Dead. Far From It. | EFFector 36.3
      This is a friendly message from the Electronic Frontier Foundation. EFFector 36.3 Privacy Isn’t Dea…
```

MeliaFTS supports Gmail style filters:

```
ms search subject:invoice unread:
ms search sender:bob flagged: attachments:
ms search newer:7d body:kubernetes
ms search after:2024-01-01 older:1month from:bob
ms search -- subject:invoice -subject:draft
```

`ms search --help` for a quick overview.

## TUI

```
ms tui
```

![](/docs/tui.png)

TUI supports themes (`--theme`), `ms tui --help` to list them.
