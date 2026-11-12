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
| **Hysteria2** | ✅ | ✅ | UDP/QUIC | TLS, obfs salamander |

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

> В `go.mod` нет директив `replace`, поэтому они больше не мешают `go install`:
> `go install github.com/aimuzov/happ-cli/cmd/happ@latest` работает по
> опубликованному тегу. Готовый бинарник или локальный `go build` всё равно
> проще.

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
3  hysteria2                hy.example:443       Fast HY2
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
| `--health-check`   | `false`      | проверка соединения при старте + периодически во всех режимах; выход при ошибке |
| `--check-interval`  | `60`         | секунд между проверками |
| `--check-url`       | Cloudflare trace | URL для стартовой проверки и периодических проб |
| `--dns-proxy`       | `true`       | DNS через прокси (выключить: `--dns-proxy=false` для локального DNS) |
| `--exclude`         | —            | домен или IPv4 CIDR, который идёт напрямую (можно повторять) |
| `--exclude-file`    | `<конфиг-каталог>/exclude.txt`, если есть | файл со списком исключений, по одному на строку |
| `--sub`          | активная     | имя подписки                                               |

Стартовая проверка повторяет попытки в пределах 10-секундного таймаута.
`--check-url` действует и для стартовой проверки, и для периодических проб,
включая `tun-direct`.

В режимах `tun` и `tun-direct` проверка маршрутизируемого пути резолвит хост из
`--check-url` **один раз, до изменения маршрутов**, а затем обращается к
полученному IPv4-адресу с сохранёнными Host/SNI. Поэтому роутер, у которого DNS
перестал работать после установки маршрутов, не может выставить рабочий туннель
мёртвым. Для разрыва нужны **три неудачные пробы подряд**; одиночный сбой
допускается. Ошибка DNS при проверке маршрутизируемого пути печатается как
предупреждение и никогда не откатывает маршруты.

### Исключения

Отправлять выбранные адреса **мимо** туннеля (напрямую):

```sh
sudo happ connect --mode tun --exclude google.com --exclude 10.0.0.0/8
sudo happ connect --mode tun --exclude-file ./my-excludes.txt
```

Список — это значения `--exclude` плюс строки файла. По одному на строку;
`#` начинает комментарий (вся строка или хвост после пробела):

```text
google.com              # google.com и все поддомены идут напрямую
full:example.com        # только example.com, без поддоменов
keyword:google          # подстрока в домене
regexp:^ads\.           # регулярное выражение по домену
10.0.0.0/8              # IPv4-сеть идёт напрямую
192.0.2.5               # одиночный IPv4, трактуется как /32
```

Голые имена превращаются в правила `domain:` (домен и все поддомены). Пустые
строки, комментарии и переводы строк Windows обрабатываются. Записи IPv6
отклоняются. Некорректные записи печатаются как
`warning: ignoring exclude entry "..."` в stderr и пропускаются — соединение
всё равно запускается.

Где лежит файл: `--exclude-file`, если задан (отсутствие явно указанного файла —
ошибка); иначе `exclude.txt` рядом с `config.yaml` загружается автоматически,
если существует. Те же значения можно задать в `config.yaml`:

```yaml
connect:
  exclude:
    - google.com
  exclude-file: ""
```

Как это работает: xray получает `routing.rules`, отправляющие совпавший трафик в
outbound `direct`. В `--mode tun` на Linux исходящее direct-соединение несёт
`streamSettings.sockopt.mark`, а туннель добавляет
`ip rule add fwmark <mark> lookup main pref 80`, чтобы такие соединения обходили
TUN-устройство, а не зацикливались в него. Исключённые сети дополнительно
получают правило ядра (`ip rule add to <cidr> lookup main pref 85..89`, максимум
пять сетей).

Ограничения:

- **Домены сопоставляются по sniffing'у** (TLS SNI / HTTP Host). ECH/ESNI или
  нераспознаваемый трафик по имени не матчится — работает только путь по IP,
  а домен, чей IP не исключён, продолжает идти через туннель.
- **Исключения на уровне ядра — только Linux `--mode tun`.** На macOS флаги
  принимаются, но обхода ядра нет, поэтому в `--mode tun` исключённый трафик
  может всё равно попасть в utun; используйте там режим proxy. В режимах proxy и
  `tun-direct` действуют только правила маршрутизации xray.
- **DNS по-прежнему идёт через туннель**, если не указан `--dns-proxy=false`:
  xray опрашивает 1.1.1.1/8.8.8.8 через proxy-outbound.

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

Значения из `connect:` в `config.yaml` служат значениями по умолчанию для
соответствующих флагов `connect`; флаги командной строки их переопределяют.

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
3. **маршруты устанавливаются только после проверки связи** — с
   `--health-check` стартовая проверка идёт через SOCKS-прокси до любых
   изменений маршрутов, затем проверяется уже маршрутизируемый путь и при
   неудаче маршруты откатываются (проба обращается к IP, полученному до смены
   маршрутов, поэтому DNS роутера не может её провалить). Без
   `--health-check` проверки нет, и маршруты ставятся сразу;
4. **напрямую подключённые подсети остаются в main** — каждый префикс из
   `ip -4 route show proto kernel` (кроме TUN-устройства), плюс
   `169.254.0.0/16` и `224.0.0.0/4`, получает policy-rule (`pref 90..`) на
   таблицу `main` с приоритетом выше правила туннеля (`pref 32760`). Поэтому
   ответы LAN-клиентам, SSH к роутеру и его веб-интерфейс работают даже когда
   туннель лежит. Эти префиксы **не** копируются в таблицу 100 через
   WAN-шлюз (раньше это отправляло LAN-трафик провайдеру и ломало LAN↔LAN);
5. дефолтный маршрут перекрывается двумя `/1`-маршрутами на TUN-устройство;
6. флаг `--no-routing` пропускает правку маршрутов (только создаёт TUN),
   полезно на роутерах, где маршруты управляются внешне.

При настроенных исключениях (`--exclude`, `--exclude-file` или авто-загруженный
`exclude.txt`) правило `ip rule fwmark <mark> lookup main pref 80` уводит
помеченные direct-соединения xray мимо туннеля, а каждая исключённая сеть
получает `ip rule to <cidr> lookup main` с `pref 85..89` (не более пяти;
остальные сети пропускаются с предупреждением). `cleanupStaleTun` и teardown
туннеля удаляют prefs 80 и 85..89 вместе с LAN- и tunnel-правилами.

### Диагностика: в `-m tun` перестаёт работать LAN DNS

Если у LAN-клиентов ломается разрешение имён, а трафик через туннель идёт,
скорее всего upstream-резолвер роутера недоступен через туннель: приватный
DNS-сервер провайдера, завёрнутый в VPN, не отвечает. Направьте dnsmasq на
публичный резолвер (например, `1.1.1.1`) или добавьте этот сервер в
`--exclude`. Сам happ системный DNS не перехватывает.

## Ограничения

- **TLS `insecure=1` больше не поддерживается xray-core** (удалён 2026-06-01): для самоподписанного сервера добавьте в ссылку `pinSHA256`/`pcs` (hex sha256 листового сертификата), иначе соединение проверяется обычным образом и завершится ошибкой.
- **Hysteria2** поддерживает пароль (auth), TLS и `obfs=salamander`; port hopping (`udphop`) и brutal congestion control пока не подключены.
- **`connect` завершается с кодом 1, если туннель разорван health-check'ом** (после трёх неудачных проб подряд); Ctrl+C — код 0.
- **`--system-proxy` только для macOS** (использует `networksetup`).
- **IPv6 заблокирован в TUN/TUN-direct** (путь прокси — IPv4); IPv6-only ресурсы недоступны.
- **Исключения по домену работают только при sniffable-трафике**: правила используют TLS SNI / HTTP Host, поэтому ECH/ESNI и нераспознаваемый трафик откатываются к пути по IP. Записи IPv6 в исключениях не поддерживаются.
- **Исключения на уровне ядра — только Linux `--mode tun`**; на macOS действуют лишь правила маршрутизации xray, поэтому в `--mode tun` исключённый трафик может всё равно попасть в utun (используйте там режим proxy).
- **DNS по-прежнему идёт через туннель**, если не указан `--dns-proxy=false`.
- **`tun-direct` на Linux требует ручной настройки маршрутов**: TUN в xray-core только создаёт сетевой интерфейс (см. `proxy/tun/tun_linux.go`: "nothing more than create a network interface") и **не** устанавливает системные маршруты. В отличие от `--mode tun`, трафик сам не перенаправляется — добавьте маршруты/policy-rules вручную или используйте `--mode tun`.
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

> `gvisor.dev/gvisor` — транзитивная зависимость tun2socks, и tun2socks
> (>= v2.7.0) тянет снапшот, с которым собирается и xray-core, поэтому
> директива `replace` больше не нужна. Оставляй версию, которую выбирает MVS:
> снапшот gvisor от 2026-11-12 вообще не компилируется (в `pkg/tcpip/stack`
> файлы из двух разных пакетов), так что не гонись за `@latest`.

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
