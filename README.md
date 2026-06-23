# Melia Email Database CLI

A [Melia](https://melia.buxjr.com) email search CLI (very much a vibecoded WiP).

![](/docs/meliafts.gif)

## Install

```
go install github.com/rubiojr/meliafts/cmd/ms
```

## Run

```
ms --db ~/.config/melia/melia.db search subject:hello
```
ms search   subject:privacy isn't dead
1 message
───────────────────────────────────────

[U  ] 2024-03-04 18:50  EFFector List <editor@eff.org>
      Privacy Isn’t Dead. Far From It. | EFFector 36.3
      This is a friendly message from the Electronic Frontier Foundation. EFFector 36.3 Privacy Isn’t Dea…
```

## TUI

```
ms tui
```

![](/docs/tui.png)
