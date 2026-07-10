<p align="center">
  <img src="assets/banner.png" alt="happ-cli" width="800" />
</p>

# happ-cli

[English](README.md) | **Русский**

Терминальный VPN-клиент, совместимый с профилями подписок [HAPP](https://happ.su).
Забирает подписку, парсит share-ссылки (VLESS / VMess / Trojan / Shadowsocks) и
поднимает соединение через встроенный
[xray-core](https://github.com/XTLS/Xray-core) — как локальный прокси, системный
прокси или полноценный системный VPN (TUN).

Единый самодостаточный бинарник: xray-core и tun2socks встроены, внешних бинарей
не требуется.

## Возможности

| | happ-cli | Другие HAPP-клиенты |
|---|:---:|:---:|
| Единый бинарник без зависимостей | ✅ | ❌ |
| JSON-подписки (Incy-формат) | ✅ | ❌ |
| Системный TUN VPN | ✅ | ❌ (только прокси) |
| TUN-direct (ICMP/ping работает) | ✅ | ❌ |
| Поддержка роутеров (MIPS/ARM, iptables, обход LAN) | ✅ | ❌ |
| Авто-фейловер (round-robin по серверам) | ✅ | ❌ |
| Health-чеки с отслеживанием серверов | ✅ | ❌ |
| YAML-конфиг | ✅ | ❌ |
| Защита от DNS-утечек (опционально) | ✅ | ❌ |

## Поддерживаемые протоколы

| Протокол | Парсинг | Подключение | Транспорты | Шифрование |
|----------|:---:|:---:|------------|----------|
| **VLESS** | ✅ | ✅ | TCP, WS, gRPC, HTTP/2 | Reality, TLS, XTLS Vision |
| **VMess** | ✅ | ✅ | TCP, WS, gRPC, HTTP/2 | TLS |
| **Trojan** | ✅ | ✅ | TCP, WS | TLS |
| **Shadowsocks** | ✅ | ✅ | TCP | AEAD |
| **Hysteria2** | ✅ | ❌ | — | — |

## Режимы подключения

| Режим | Root | ICMP | DNS-прокси | Сценарий |
|------|:---:|:---:|:---:|----------|
| `proxy` | ❌ | ❌ | ✅ | Браузер/CLI через SOCKS5 |
| `proxy --system-proxy` | sudo | ❌ | ✅ | Системный прокси macOS (уживается с VPN) |
| `tun` | sudo | ❌ | ✅ | Полный системный VPN через tun2socks |
| **`tun-direct`** | sudo | ✅ | ✅ | Полный VPN с поддержкой ICMP (xray TUN, без SOCKS) |

## Как это устроено

```
subscription URL
      │  profile.Fetch (INCY-заголовки)
      ▼
base64 или JSON ──► link.Parse / json.go ──► []link.Server
                                            │ xray.BuildConfig
                                            ▼
                                    конфиг xray-core (JSON)
                                            │ xray.Start (встроенное ядро)
              ┌─────────────────────────────┼─────────────────────────────┐
              ▼                             ▼                             ▼
      proxy: SOCKS5/HTTP          --system-proxy: networksetup     tun: tun2socks
      на 127.0.0.1                ставит системный SOCKS/HTTP       + таблица маршрутов
      (без root)                  (sudo)                           (sudo, utun)
```

## Установка

### mise (рекомендуется)

Готовые бинарники публикуются в GitHub Releases. Ставятся через
[mise](https://mise.jdx.dev) — без установки Go. Внутри архива бинарник называется
`happ` (не `happ-cli`), поэтому укажи `exe=happ`:

```sh
mise use -g "github:aimuzov/happ-cli[exe=happ]@latest"
```

или зафиксировать в `mise.toml`:

```toml
[tools]
"github:aimuzov/happ-cli" = { version = "latest", exe = "happ" }
```

Бэкенд `ubi` работает так же и с теми же релизами, если он тебе привычнее:
`ubi:aimuzov/happ-cli[exe=happ]`.

> При частых установках задай `MISE_GITHUB_TOKEN` (или `GITHUB_TOKEN`), чтобы не
> упереться в лимиты GitHub API.

### Ручная загрузка

Скачай архив под свою ОС/архитектуру со страницы
[Releases](https://github.com/aimuzov/happ-cli/releases), распакуй и положи
бинарник `happ` в `PATH`.

| Система | Архитектура | Скачать |
|---------|-------------|---------|
| **Linux** | amd64 | [`happ-linux-amd64.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-linux-amd64.tar.gz) |
| | arm64 | [`happ-linux-arm64.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-linux-arm64.tar.gz) |
| | armv5 | [`happ-linux-armv5.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-linux-armv5.tar.gz) |
| | armv6 | [`happ-linux-armv6.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-linux-armv6.tar.gz) |
| | armv7 | [`happ-linux-armv7.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-linux-armv7.tar.gz) |
| | mips (softfloat) | [`happ-linux-mips.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-linux-mips.tar.gz) |
| | mipsle (softfloat) | [`happ-linux-mipsle.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-linux-mipsle.tar.gz) |
| **macOS** | amd64 (Intel) | [`happ-darwin-amd64.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-darwin-amd64.tar.gz) |
| | arm64 (Apple Silicon) | [`happ-darwin-arm64.tar.gz`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-darwin-arm64.tar.gz) |
| **Windows** | amd64 | [`happ-windows-amd64.zip`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-windows-amd64.zip) |
| | 386 | [`happ-windows-386.zip`](https://github.com/aimuzov/happ-cli/releases/latest/download/happ-windows-386.zip) |

### Из исходников

```sh
git clone https://github.com/aimuzov/happ-cli
cd happ-cli
go build -o happ ./cmd/happ/   # нужен Go 1.26+
```

Полученный бинарник `happ` самодостаточен.

> **`go install github.com/aimuzov/happ-cli@latest` не работает.** Сборке нужна
> директива `replace` в `go.mod` (примиряет xray-core и tun2socks по gvisor), а
> `go install pkg@version` игнорирует `replace`. Используй готовый бинарник либо
> клонируй и собирай.

## Использование

### Подписки

```sh
happ sub add https://panel.example/sub/TOKEN --name myvpn   # добавить (станет активной)
happ sub list                                               # список подписок
happ sub update [name]                                      # обновить (по умолчанию активную)
happ sub use <name>                                         # сделать подписку активной
happ sub rm <name>                                          # удалить
```

`sub list` показывает трафик и срок из заголовков подписки:

```
ACTIVE  NAME    TITLE       SERVERS  TRAFFIC          EXPIRES
*       myvpn   My VPN      12       12.4 GB / 200 GB  2026-09-01
```

### Серверы

```sh
happ list           # серверы активной подписки
happ list --sub x   # серверы конкретной подписки
```

```
#  PROTOCOL                 ADDRESS              TAG
1  vless                    de.example:443       🇩🇪 Германия
2  trojan                   nl.example:443       🇳🇱 Нидерланды
3  hysteria2 (unsupported)  hy.example:443       Fast HY2
```

### Подключение

`connect` работает в foreground до прерывания `Ctrl+C`. Аргумент `selector`
выбирает сервер: пусто = первый, число = индекс (1-based) из `happ list`, либо
подстрока тега без учёта регистра.

```sh
happ connect                 # первый сервер, режим proxy
happ connect 2               # сервер №2
happ connect germany         # первый сервер с тегом, содержащим "germany"

sudo happ connect 1 --system-proxy   # браузеры/приложения через системный прокси (без правки маршрутов)
sudo happ connect 1 --mode tun       # полноценный системный VPN
```

В обычном proxy-режиме настрой приложения на `socks5://127.0.0.1:10808`
(в Firefox включи «Proxy DNS when using SOCKS v5»).

### Флаги `connect`

| Флаг             | По умолчанию | Назначение                                                 |
| ---------------- | ------------ | ---------------------------------------------------------- |
| `-m, --mode`     | `proxy`      | `proxy` или `tun`                                          |
| `--socks`        | `10808`      | порт локального SOCKS5                                     |
| `--http`         | `10809`      | порт локального HTTP (режим proxy)                         |
| `--system-proxy` | `false`      | выставить системный прокси macOS (режим proxy, нужен sudo) |
| `--no-routing`    | `false`      | создать TUN без правки маршрутов |
| `--skip-firewall`  | `false`      | пропустить iptables-правила (tun/tun-direct) |
| `--health-check`   | `false`      | проверка соединения при старте + периодически; выход при ошибке |
| `--dns-proxy`       | `true`       | DNS через прокси (выключить: `--dns-proxy=false` для локального DNS) |
| `--sub`          | активная     | имя подписки                                               |

### Три способа завернуть трафик — сравнение

- **`connect` (proxy)** — только приложения, явно настроенные на
  `socks5://127.0.0.1:10808` (например, Firefox с remote DNS). Без root.
- **`connect --system-proxy`** — выставляет на всех включённых сетевых сервисах
  системный SOCKS (порт `--socks`) и HTTP/HTTPS (порт `--http`), поэтому
  Safari/Chrome и приложения, игнорирующие SOCKS, идут через прокси. **Не трогает**
  таблицу маршрутов, поэтому **уживается с другим активным VPN**. Нужен `sudo`;
  прежние настройки прокси восстанавливаются при выходе. Если сессию убили
  (`kill -9`) и прокси завис — сбросить командой `sudo happ system-proxy off`.
- **`connect --mode tun`** — полноценный системный VPN через utun, перехватывает
  весь трафик. Нужен `sudo`. Если параллельно активен другой VPN — сначала
  отключи его, чтобы туннели не дрались за маршруты/DNS.

### Прочие команды

```sh
happ config [selector]       # вывести сгенерированный конфиг xray-core (отладка)
happ system-proxy off        # аварийный сброс системного прокси (sudo)
```

## Конфигурация и хранение

Состояние (подписки и кэш ссылок) хранится в `state.json` в конфиг-каталоге
пользователя (`~/Library/Application Support/happ-cli` на macOS),
переопределяется глобальным флагом `--home`.

## Детали режима TUN

### macOS

1. адрес сервера резолвится в IP, и на каждый добавляется host-маршрут к текущему
   next-hop (физический шлюз либо интерфейс уже активного VPN) — чтобы соединение
   прокси с сервером не зациклилось обратно в туннель;
2. создаётся устройство `utun`, и tun2socks форвардит его трафик на локальный
   SOCKS, который отдаёт xray;
3. дефолтный маршрут перекрывается двумя `/1`-маршрутами на utun;
4. глобальный IPv6 заворачивается в `lo0` (блокируется);
5. при `Ctrl+C` все маршруты снимаются в обратном порядке.

### Linux

1. IP серверов пиннуются к текущему next-hop (как на macOS);
2. TUN-устройство создаётся через `/dev/net/tun`;
3. **локальные подсети сохраняются** — `192.168.0.0/16`, `172.16.0.0/12`,
   `10.0.0.0/8`, link-local и multicast остаются на физическом интерфейсе,
   чтобы LAN и админка роутера оставались доступны;
4. дефолтный маршрут перекрывается двумя `/1`-маршрутами на TUN-устройство;
5. флаг `--no-routing` пропускает правку маршрутов (только создаёт TUN),
   полезно на роутерах, где маршруты управляются внешне.

## Ограничения

- **Hysteria2** серверы парсятся и отображаются, но подключиться нельзя (xray-core не имеет outbound для Hysteria2).
- **`--system-proxy` только для macOS** (использует `networksetup`).
- **IPv6 заблокирован в TUN/TUN-direct** (путь прокси — IPv4); IPv6-only ресурсы недоступны.
- `connect` работает в **foreground**; фонового демона пока нет.
- `kill -9` пропускает очистку, но stale-маршруты чистятся автоматически при следующем `connect`. Для ручной очистки: `sudo happ cleanup-tun`.

## Структура проекта

| Пакет               | Назначение                                                   |
| ------------------- | ------------------------------------------------------------ |
| `cmd/happ`          | точка входа                                                  |
| `internal/check`    | проверка соединения (Cloudflare trace)                       |
| `internal/cli`      | команды cobra                                                |
| `internal/config`   | YAML-конфигурация                                            |
| `internal/device`   | идентификация устройства (HWID + UUID)                       |
| `internal/firewall` | iptables FORWARD (Linux)                                     |
| `internal/link`     | парсинг share-ссылок (vless/vmess/trojan/ss/hysteria2)       |
| `internal/profile`  | загрузка подписки, декод base64/JSON + заголовков            |
| `internal/store`    | хранение подписок, кэша ссылок, last-used трекинг            |
| `internal/tunnel`   | режим TUN: tun2socks + маршруты (macOS + Linux)              |
| `internal/sysproxy` | системный прокси macOS через networksetup                    |
| `internal/xray`     | сборка конфига xray-core, запуск встроенного ядра            |

## Разработка

```sh
go test ./...        # юнит-тесты + реальный end-to-end тест прокси
go vet ./...
```

Интеграционный тест xray поднимает реальный Shadowsocks-сервер и клиента,
собранного из `link.Server`, и проверяет, что HTTP-запрос через SOCKS-inbound
клиента доходит до цели сквозь прокси.

> xray-core и tun2socks требуют разные версии `gvisor.dev/gvisor`; директива
> `replace` в `go.mod` фиксирует gvisor на версии, с которой собираются оба. Не
> удаляй её — см. комментарий рядом.

### Релизы

Релизы собирает [GoReleaser](https://goreleaser.com) в CI по пушу тега:

```sh
git tag v0.1.0
git push origin v0.1.0
```

Workflow `build-release` (`.github/workflows/build-release.yml`) собирает
бинарники под 12 комбинаций ОС/архитектуры (linux/darwin/windows,
amd64/arm64/armv5-7/mips/mipsle) и публикует их в GitHub Releases. Там сборка
учитывает `replace` из `go.mod` (happ-cli — главный модуль). Локальный прогон:
`goreleaser release --clean --snapshot`.
