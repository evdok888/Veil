# Telegram Relay Bot

Анонимный посредник между владельцем бота и клиентами через Telegram. Клиенты пишут боту в личные сообщения, владелец отвечает через супергруппу с темами (Forum Topics).

## Возможности

- **Анонимность**: Все сообщения копируются через `copyMessage`, оригинальный отправитель не раскрывается
- **Темы**: Каждый клиент получает отдельную тему в супергруппе владельца
- **Синхронизация**: Редактирование и удаление сообщений синхронизируется между клиентом и владельцем
- **Блокировка**: Команды `/block`, `/end` для прекращения диалога с неадекватными клиентами
- **Rate Limiting**: Защита от флуда (настраивается)
- **Webhook/Polling**: Поддержка обоих режимов работы

## Структура проекта

```
telegram-relay-bot/
├── cmd/bot/main.go              # Точка входа
├── internal/
│   ├── config/config.go         # Загрузка .env
│   ├── db/db.go                 # База данных SQLite
│   ├── handlers/
│   │   ├── client.go            # Обработка сообщений от клиентов
│   │   ├── owner.go             # Обработка сообщений владельца
│   │   └── commands.go          # Команды /block, /end, /del
│   └── telegram/copy.go         # Обёртки над Telebot API
├── deploy/
│   ├── bot.service              # systemd unit
│   └── nginx.conf.example       # Конфиг nginx для webhook
├── .env.example                 # Шаблон конфигурации
├── go.mod
└── README.md
```

## Установка на VPS (Ubuntu 22.04)

### 1. Подготовка системы

```bash
# Обновление пакетов
sudo apt update && sudo apt upgrade -y

# Установка Go (если не установлен)
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Проверка версии
go version  # должна быть 1.22+

# Создание пользователя для бота
sudo useradd -r -s /bin/false relaybot
sudo mkdir -p /opt/relaybot
sudo chown relaybot:relaybot /opt/relaybot
```

### 2. Создание супергруппы Telegram

1. Создайте новую группу в Telegram, добавьте в неё бота
2. Превратите группу в **супергруппу** (отправьте любое сообщение)
3. Включите **Topics** (Forum):
   - Настройки группы → Topics → Включить
4. Сделайте бота **администратором** с правами:
   - Управление темами
   - Отправка сообщений
5. Удалите всех лишних участников (должны остаться только владелец и бот)

### 3. Получение ID

**SUPERGROUP_ID:**
- Перешлите сообщение из супергруппы боту @userinfobot
- Или отправьте сообщение в группу и посмотрите в логах бота (добавьте временный логгинг)
- ID супергруппы начинается с `-100` (например, `-1001234567890`)

**OWNER_USER_ID:**
- Напишите своему боту `/start`
- Используйте @userinfobot или посмотрите логи
- Это ваш числовой ID (например, `123456789`)

### 4. Настройка конфигурации

```bash
cd /opt/relaybot
sudo -u relaybot cp /path/to/project/.env.example .env
sudo -u relaybot nano .env
```

Заполните `.env`:

```env
BOT_TOKEN=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz  # от @BotFather
SUPERGROUP_ID=-1001234567890                    # ID супергруппы (обязательно с -100)
OWNER_USER_ID=123456789                         # Ваш ID
WEBHOOK_URL=https://yourdomain.com/webhook      # HTTPS URL для production
SERVER_ADDR=127.0.0.1:8443                      # Локальный порт
DB_PATH=/opt/relaybot/bot.db
RATE_LIMIT_PER_MINUTE=60
BLOCKED_MESSAGE=Диалог завершён. Вы не можете отправлять сообщения.
DEBUG_LOG=false
```

### 5. Сборка и установка

```bash
# Сборка бинарника
cd /workspace/telegram-relay-bot
go build -o bot ./cmd/bot

# Копирование на сервер
scp bot relaybot@your-server:/opt/relaybot/bot
scp deploy/bot.service relaybot@your-server:/tmp/bot.service

# Или локально (если уже на сервере)
sudo cp bot /opt/relaybot/bot
sudo chown relaybot:relaybot /opt/relaybot/bot
sudo chmod +x /opt/relaybot/bot
```

### 6. Настройка systemd

```bash
# Копирование unit-файла
sudo cp /tmp/bot.service /etc/systemd/system/relaybot.service

# Перезагрузка systemd и запуск
sudo systemctl daemon-reload
sudo systemctl enable relaybot
sudo systemctl start relaybot

# Проверка статуса
sudo systemctl status relaybot
sudo journalctl -u relaybot -f  # Просмотр логов
```

### 7. Настройка nginx и HTTPS

```bash
# Установка nginx и certbot
sudo apt install nginx certbot python3-certbot-nginx -y

# Копирование конфига
sudo cp deploy/nginx.conf.example /etc/nginx/sites-available/relaybot
sudo ln -s /etc/nginx/sites-available/relaybot /etc/nginx/sites-enabled/

# Редактирование домена
sudo nano /etc/nginx/sites-available/relaybot
# Замените yourdomain.com на ваш домен

# Получение SSL сертификата
sudo certbot --nginx -d yourdomain.com

# Проверка и перезагрузка nginx
sudo nginx -t
sudo systemctl restart nginx
```

### 8. Настройка webhook

После запуска бота выполните один раз:

```bash
curl -X POST "https://api.telegram.org/bot<BOT_TOKEN>/setWebhook?url=https://yourdomain.com/webhook"
```

Или используйте встроенный механизм (при старте в режиме webhook бот сам установит webhook).

Проверка:
```bash
curl "https://api.telegram.org/bot<BOT_TOKEN>/getWebhookInfo"
```

### 9. Firewall (UFW)

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp        # SSH
sudo ufw allow 80/tcp        # HTTP (для Let's Encrypt)
sudo ufw allow 443/tcp       # HTTPS
sudo ufw enable
```

## Команды для владельца

Все команды работают **только** в супергруппе и **только** от владельца:

| Команда | Описание |
|---------|----------|
| `/block` (reply) | Заблокировать клиента, закрыть тему |
| `/end` (reply) | Завершить диалог (синоним /block) |
| `/unblock` (reply) | Разблокировать клиента, открыть тему |
| `/del` (reply) | Удалить сообщение у клиента и в группе |
| `/delete` (reply) | Синоним /del |
| `/help` | Показать справку |

Также доступны inline-кнопки под сообщениями:
- 🚫 Завершить диалог — блокирует клиента
- 🗑 Удалить — удаляет сообщение

## Известные ограничения Telegram Bot API

1. **Редактирование медиа**: Можно изменить только подпись (caption), но не сам файл. Если клиент "отредактировал" фото, Telegram присылает это как новое сообщение.

2. **Стикеры**: Не имеют подписи, редактирование невозможно. Только удаление.

3. **Голосовые сообщения**: Не поддерживают редактирование вообще.

4. **Ручное удаление**: Если владелец или клиент удаляет сообщение вручную (свайпом), бот не получает уведомление. Синхронизация возможна только через команду `/del` или inline-кнопку.

5. **Forward vs Copy**: Используется только `copyMessage` для анонимности. `forwardMessage` раскрыл бы профиль отправителя.

## Логирование

Бот логирует только метаданные:
- `user_id`, `message_id`
- Тип контента (текст, фото, голос и т.д.)
- Статус доставки

**Содержимое сообщений не логируется** (кроме режима отладки).

## Миграция на PostgreSQL

В будущем для замены SQLite на PostgreSQL:
1. Измените импорт в `internal/db/db.go` на `github.com/lib/pq`
2. Обновите DSN в конфиге
3. Адаптируйте SQL-запросы (синтаксис SQLite и PostgreSQL совместим на 95%)

## Troubleshooting

**Бот не отвечает:**
- Проверьте `sudo journalctl -u relaybot -f`
- Убедитесь, что токен верный
- Проверьте, что бот добавлен в супергруппу как админ

**Webhook не работает:**
- Проверьте сертификат: `openssl s_client -connect yourdomain.com:443`
- Убедитесь, что nginx проксирует на `127.0.0.1:8443`
- Проверьте `getWebhookInfo`

**Ошибки rate limit:**
- Увеличьте интервал между запросами
- Включите retry в коде (уже реализовано)

## Лицензия

MIT
