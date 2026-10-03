# Деплой сайта коворкинга на Realistic Nova (31.129.99.35)

Правила машины (соблюдать):

- Порты берём только нестандартные, высокие. Сосед занял 49253 — мы берём **49254** (весь проект промазан этим портом, менять в `deploy/coworking.service`).
- Никаких `shutdown`, `reboot`, `halt`, `poweroff`.
- Проект живёт в podman-контейнере, данные — наружу (bind-mount `/data`).
- Сайт — systemd-служба пользователя (`systemctl --user` + `loginctl enable-linger`), переживает логаут.
- Свой юзер `fedos` в `/home`, проект в `/home/fedos/projects/coworking`.
- Не жрём машину: лимиты контейнеру уже заданы (`--cpus=0.5 --memory=512m`).
- На сервере работаем руками: команды ниже — человеку, не агенту.

---

## Шаг 1. Заход на сервер и первый порядок

```bash
ssh root@31.129.99.35        # пароль от друга
passwd root                  # сменить — пароль засветился в переписке
```

## Шаг 2. Свой юзер

```bash
useradd -m -s /bin/bash fedos
passwd fedos
usermod -aG wheel fedos
```

С локальной машины скопировать ключ, чтобы дальше ходить под `fedos` без пароля:

```bash
# локально (Windows: из Git Bash)
ssh-keygen -t ed25519 -C "fedos@31.129.99.35"
ssh-copy-id fedos@31.129.99.35
```

Дальше все шаги — под `fedos`.

## Шаг 3. Служба пользователя переживает логаут

```bash
loginctl enable-linger fedos
loginctl show-user fedos | grep Linger   # должно быть Linger=yes
```

## Шаг 4. Проект

```bash
mkdir -p ~/projects
cd ~/projects
git clone https://github.com/<твой-логин>/coworking.git
```

> Если репозиторий приватный — сделай публичным или настрой deploy-ключ.

## Шаг 5. Сборка образа

```bash
cd ~/projects/coworking
podman build -t localhost/coworking:latest .
```

Проверка перед установкой службы:

```bash
mkdir -p ~/projects/coworking/data
podman run --rm --name coworking -p 49254:8081 \
  -v ~/projects/coworking/data:/data \
  localhost/coworking:latest &
sleep 4
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:49254/   # 200
kill %1
```

## Шаг 6. systemd-служба

```bash
mkdir -p ~/.config/systemd/user
cp ~/projects/coworking/deploy/coworking.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now coworking
systemctl --user status coworking
```

Смотреть логи:

```bash
journalctl --user -u coworking -f
```

## Шаг 7. Проверка снаружи

```bash
curl -s -o /dev/null -w "%{http_code}\n" http://31.129.99.35:49254/
```

Если фаервол не пускает (под root):

```bash
ufw allow 49254/tcp
```

Сайт: **http://31.129.99.35:49254/** (заголовок, футер со счётчиком посещений, `/booking`).

## Обновление сайта

```bash
cd ~/projects/coworking
git pull                                   # локально коммитим и пушим, здесь только подтягиваем
podman build -t localhost/coworking:latest .
systemctl --user restart coworking
```

## Полезное

- Лимиты уже в службе: 0.5 CPU / 512 МБ / 256 процессов. Посмотреть: `podman stats --no-stream`.
- Диск: `df -h`, `du -sh ~/projects`.
- Журнал systemd самограничен, в бесконечный файл ничего не пишем.
- Не занимаем чужие порты; свой порт держим в одном месте — `deploy/coworking.service`.
- БД (Лаба 3): PostgreSQL поднимем вторым контейнером, том `~/projects/coworking/data` уже примонтирован.
