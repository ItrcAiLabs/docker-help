# 🐳 Docker TUI (Terminal UI) in Go

A minimal terminal-based UI to manage and inspect Docker containers — written in pure Go using `tview` and the Docker SDK.

## 🚀 Features

* View all Docker containers (running, exited, etc.)
* See:

  * Name, Image, State, Status, Ports
  * Full container details (`docker inspect`)
  * Last 5 logs of each container
* Refresh container list with one key
* Works entirely in the terminal (no mouse required)

---

## 📦 Requirements

* Go 1.18+
* Docker daemon running (locally or remote via `DOCKER_HOST`)
* Git (for dependency fetching)

---

## 🔧 Installation

Clone the repo and run:

```bash
go mod tidy
go run .
```

Or build it:

```bash
go build -o docker-tui
./docker-tui
```

---

## 🎮 Controls

| Key         | Action                          |
| ----------- | ------------------------------- |
| ↑ / ↓       | Navigate container list         |
| `Enter`     | Show container details and logs |
| `r`         | Refresh container list          |
| `q` / `Esc` | Quit the app                    |

---

## 🧪 Example

```bash
docker run -d --name my-nginx -p 8080:80 nginx
go run .
```

You'll see `my-nginx` in the list, with ports, uptime, logs, and more.

---

## 📚 Dependencies

* [tview](https://github.com/rivo/tview) - Terminal UI framework
* [Docker Go SDK](https://github.com/moby/moby/tree/master/client)

---

## 📝 License

MIT — use freely.

