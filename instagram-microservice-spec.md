# Техническое задание: Микросервис Instagram для 7Analytics

**Автор:** Isfandiyor Djamalutdinov  
**Версия:** 1.0  
**Дата:** 22.12.2024  
**Для:** Go микросервис  
**Исполнитель:** @Вадим Галкин

---

## Оглавление

1. [Модуль "Директ"](#часть-1-модуль-директ-instagram-messaging)
2. [Модуль "Комментарии"](#часть-2-модуль-комментарии)
3. [Модуль "Календарь публикаций"](#часть-3-модуль-календарь-публикаций-отложенный-постинг)
4. [Данные для отчётов](#часть-4-данные-для-отчётов)
5. [Instagram API](#часть-5-instagram-api--какие-api-использовать)

---

## Часть 1: Модуль "Директ" (Instagram Messaging)

### 1.1 Управление диалогами

#### Получение списка диалогов

| Поле                          | Описание                               |
| ----------------------------- | -------------------------------------- |
| `id`                          | Идентификатор диалога                  |
| `participant.name`            | Имя собеседника                        |
| `participant.username`        | Username собеседника                   |
| `participant.avatar`          | URL аватара                            |
| `participant.followers_count` | Количество подписчиков (если доступно) |
| `last_message.text`           | Текст последнего сообщения             |
| `last_message.time`           | Время последнего сообщения             |
| `last_message.is_from_me`     | Исходящее ли сообщение                 |
| `unread_count`                | Количество непрочитанных               |
| `account_username`            | Username привязанного аккаунта         |

**Требования:**

- Сортировка по дате последнего сообщения (новые сверху)
- Поддержка пагинации (offset/limit или cursor)
- Фильтр по аккаунту (если несколько подключённых)

#### Поиск по диалогам

- Поиск по имени/username отправителя
- Поиск по тексту сообщений

#### Закрепление диалогов

- Сохранение списка закреплённых диалогов для пользователя
- Закрепление/открепление диалога
- Закреплённые диалоги отображаются сверху списка

### 1.2 Сообщения

#### Получение истории сообщений

| Поле         | Описание                |
| ------------ | ----------------------- |
| `id`         | Идентификатор сообщения |
| `text`       | Текст сообщения         |
| `media_url`  | URL медиа (если есть)   |
| `media_type` | Тип медиа (image/video) |
| `timestamp`  | Время отправки          |
| `is_from_me` | Исходящее ли сообщение  |

**Требования:**

- Сортировка по дате
- Пагинация
- Группировка по датам (для UI)

#### Отправка сообщений

**Входные данные:**

- `conversation_id` — ID диалога
- `text` — текст сообщения (опционально)
- `media` — массив файлов/URL (опционально)
- `account_id` — ID аккаунта отправителя

**Валидация:**

- Минимум текст ИЛИ медиа
- Ограничение на размер медиа (по требованиям Instagram API)

#### Получение информации о собеседнике

- Username, полное имя
- URL аватара
- Количество подписчиков (если API отдаёт)

### 1.3 Шаблоны сообщений

#### Структура шаблона

| Поле          | Тип      | Описание                 |
| ------------- | -------- | ------------------------ |
| `id`          | string   | Идентификатор            |
| `title`       | string   | Название шаблона         |
| `content`     | string   | Текст шаблона            |
| `images`      | string[] | URL изображений          |
| `icon`        | string   | URL иконки (опционально) |
| `usage_count` | number   | Счётчик использований    |
| `created_at`  | datetime | Дата создания            |
| `updated_at`  | datetime | Дата обновления          |

#### Операции

- **Создание** — название, текст, изображения
- **Редактирование** — все поля
- **Удаление**
- **Получение списка** — с сортировкой по `usage_count` или дате
- **Инкремент использования** — при отправке сообщения с шаблоном

### 1.4 Статистика директа

#### Метрики (за выбранный период)

| Метрика               | Описание                                               |
| --------------------- | ------------------------------------------------------ |
| `total_dialogs`       | Общее количество диалогов                              |
| `new_dialogs`         | Количество новых диалогов                              |
| `unique_users`        | Количество уникальных пользователей                    |
| `first_response_time` | Среднее время первого ответа (минуты, секунды)         |
| `avg_response_time`   | Среднее время ответа (минуты, секунды)                 |
| `busiest_time`        | Самое загруженное время (день недели + временной слот) |

#### Тепловая карта активности

Структура данных для heatmap:

```json
{
  "heatmap": [
    {
      "time_slot": "00:00-03:00",
      "days": [
        { "day": "mon", "messages_count": 5 },
        { "day": "tue", "messages_count": 8 }
      ]
    }
  ]
}
```

**Временные слоты:** 00:00-03:00, 03:00-06:00, 06:00-09:00, 09:00-12:00, 12:00-15:00, 15:00-18:00, 18:00-21:00, 21:00-00:00

**Дни:** пн, вт, ср, чт, пт, сб, вс

---

## Часть 2: Модуль "Комментарии"

### 2.1 Публикации с комментариями

#### Получение списка публикаций

| Поле                 | Описание                       |
| -------------------- | ------------------------------ |
| `id`                 | ID публикации                  |
| `account_username`   | Username аккаунта              |
| `thumbnail`          | URL превью изображения         |
| `caption_preview`    | Первые 100 символов текста     |
| `published_at`       | Дата публикации                |
| `comments_count`     | Общее количество комментариев  |
| `new_comments_count` | Количество новых/непрочитанных |

**Требования:**

- Сортировка: по дате или по количеству комментариев
- Пагинация
- Фильтр по аккаунту

#### Поиск

- По тексту публикации
- По username аккаунта

### 2.2 Управление комментариями

#### Получение комментариев к публикации

| Поле                | Описание                    |
| ------------------- | --------------------------- |
| `id`                | ID комментария              |
| `author.username`   | Username автора             |
| `author.avatar`     | URL аватара                 |
| `text`              | Текст комментария           |
| `timestamp`         | Время комментария           |
| `date`              | Дата (для группировки)      |
| `is_new`            | Новый/непрочитанный         |
| `is_own_reply`      | Наш ответ (от аккаунта)     |
| `reply_to_username` | Кому ответ (если это reply) |

**Требования:**

- Группировка по датам
- Сортировка по времени
- Пагинация

#### Ответ на комментарий

**Входные данные:**

- `comment_id` — ID комментария для ответа
- `text` — текст ответа
- `send_to_direct` — boolean, дополнительно отправить в директ

**Логика:**

1. Отправить reply как комментарий в Instagram
2. Если `send_to_direct=true`, также отправить в директ автору

#### Модерация комментариев

- **Удаление комментария** — `DELETE /comments/{id}`
- **Скрытие комментария** — `POST /comments/{id}/hide`

### 2.3 Шаблоны для комментариев

Можно использовать общую систему шаблонов с директом или создать отдельную коллекцию.

**Рекомендация:** общая система с добавлением поля `type: 'direct' | 'comment' | 'both'`

---

## Часть 3: Модуль "Календарь публикаций" (Отложенный постинг)

### 3.1 Структура публикации

| Поле            | Тип      | Описание                                |
| --------------- | -------- | --------------------------------------- |
| `id`            | string   | Идентификатор                           |
| `account_id`    | string   | ID Instagram аккаунта                   |
| `type`          | enum     | post / story / reel                     |
| `status`        | enum     | draft / scheduled / published / error   |
| `caption`       | string   | Текст публикации (до 1100 символов)     |
| `media`         | object[] | Массив медиафайлов                      |
| `media[].url`   | string   | URL файла                               |
| `media[].type`  | string   | image / video                           |
| `media[].order` | number   | Порядок в карусели                      |
| `scheduled_at`  | datetime | Дата/время публикации                   |
| `published_at`  | datetime | Фактическое время публикации            |
| `error_message` | string   | Сообщение об ошибке (если status=error) |
| `created_at`    | datetime | Дата создания                           |
| `updated_at`    | datetime | Дата обновления                         |

### 3.2 Операции с публикациями

#### Создание отложенной публикации

**Входные данные:**

- `account_id` — ID Instagram аккаунта
- `type` — тип публикации
- `caption` — текст (для post/reel)
- `media` — массив файлов
- `scheduled_at` — дата/время публикации

**Валидация:**

- Минимум 1 медиафайл
- Для post: до 10 изображений (карусель)
- Для story/reel: 1 медиафайл
- Caption: до 1100 символов
- `scheduled_at` должно быть в будущем

#### Редактирование публикации

- Разрешено только для статусов `draft` и `scheduled`
- Все поля можно изменить

#### Удаление публикации

- Отмена запланированной публикации
- Удаление черновика

#### Сохранение черновика

- `scheduled_at = null`
- `status = draft`

### 3.3 Получение данных для календаря

**Запрос:**

```
GET /publications?year=2025&month=12&type=all&status=all&account_id=xxx
```

**Параметры:**

- `year`, `month` — период
- `type` — post / story / reel / all
- `status` — scheduled / published / error / all
- `account_id` — фильтр по аккаунту

**Ответ:**

```json
{
  "publications": [
    {
      "id": "post-123",
      "type": "post",
      "status": "scheduled",
      "thumbnail": "https://...",
      "title": "7TECH ikki kamerali...",
      "scheduled_at": "2025-12-25T10:21:00Z"
    }
  ]
}
```

### 3.4 Статусы публикаций

| Статус      | Описание          | Цвет (для UI)       |
| ----------- | ----------------- | ------------------- |
| `draft`     | Черновик          | —                   |
| `scheduled` | Запланировано     | #FF8E00 (оранжевый) |
| `published` | Опубликовано      | #00A63D (зелёный)   |
| `error`     | Ошибка публикации | #FF0000 (красный)   |

### 3.5 Публикация контента

#### Автоматическая публикация по расписанию

**Логика:**

1. Cron job проверяет публикации где `status=scheduled` и `scheduled_at <= now()`
2. Отправляет контент через Instagram API
3. При успехе: `status=published`, `published_at=now()`
4. При ошибке: `status=error`, `error_message=<текст ошибки>`

#### Загрузка медиафайлов

**Endpoint:** `POST /media/upload`

**Требования:**

- Поддержка форматов: JPEG, PNG, MP4
- Ограничение размера: по требованиям Instagram
- Хранение: S3 или аналогичное хранилище
- Возврат URL загруженного файла

---

## Часть 4: Данные для отчётов

> Генерация PDF/отчётов выполняется на фронтенде. Бэкенд только отдаёт агрегированные данные.

### 4.1 Отчёт по директу

| Метрика               | Описание                      |
| --------------------- | ----------------------------- |
| `total_dialogs`       | Количество диалогов за период |
| `new_dialogs`         | Новые диалоги                 |
| `avg_response_time`   | Среднее время ответа          |
| `first_response_time` | Время первого ответа          |
| `activity_by_day`     | Активность по дням недели     |
| `activity_by_hour`    | Активность по часам           |

### 4.2 Отчёт по комментариям

| Метрика                 | Описание                            |
| ----------------------- | ----------------------------------- |
| `total_comments`        | Общее количество комментариев       |
| `replied_comments`      | Количество ответов от аккаунта      |
| `top_posts`             | Топ публикаций по комментариям      |
| `avg_comments_per_post` | Среднее кол-во комментариев на пост |

### 4.3 Отчёт по публикациям

| Метрика           | Описание                            |
| ----------------- | ----------------------------------- |
| `scheduled_count` | Запланировано публикаций            |
| `published_count` | Успешно опубликовано                |
| `error_count`     | С ошибками                          |
| `by_type`         | Разбивка по типам (post/story/reel) |

---

## Часть 5: Instagram API — Какие API использовать

### ⚠️ ВАЖНО: Новый Instagram API with Instagram Login (Июль 2024)

С 23 июля 2024 Meta запустила новый **Instagram API with Instagram Login**, который не требует привязки к Facebook Page.

**Преимущества нового API:**

- Упрощённый онбординг — только Instagram Login, без Facebook
- Instagram Business/Creator аккаунт не нужно связывать с Facebook Page
- Упрощённые permissions — только Instagram-related
- Поддержка: Messaging, Comments, Content Publishing, Insights

**Рекомендация Meta:** Использовать новый Instagram API with Instagram Login вместо старого подхода через Facebook.

### 5.1 Для Директа — Instagram Messenger API

**Документация:** [Instagram Messaging - Messenger Platform](https://developers.facebook.com/docs/messenger-platform/instagram)

**Требования:**

- Instagram Business аккаунт
- Подключение через Facebook Login
- Одобрение приложения в Meta

**Возможности:**

- Получение входящих сообщений
- Отправка сообщений (текст, медиа)
- Получение истории диалогов

**Permissions:**

- `instagram_manage_messages`

### 5.2 Для Комментариев — Instagram Graph API

**Документация:** [IG Comment - Instagram Platform](https://developers.facebook.com/docs/instagram-api/reference/ig-comment)

**Endpoints:**

| Метод  | Endpoint                      | Описание               |
| ------ | ----------------------------- | ---------------------- |
| GET    | `/{media-id}/comments`        | Получение комментариев |
| POST   | `/{comment-id}/replies`       | Ответ на комментарий   |
| DELETE | `/{comment-id}`               | Удаление комментария   |
| POST   | `/{comment-id}` + `hide=true` | Скрытие комментария    |

**Permissions:**

- `instagram_basic`
- `instagram_manage_comments`

### 5.3 Для Публикаций — Instagram Content Publishing API

**Документация:** [Publish Content - Instagram Platform](https://developers.facebook.com/docs/instagram-api/guides/content-publishing)

**Поддерживаемые типы:**

| Тип        | Поддержка | Примечание                   |
| ---------- | --------- | ---------------------------- |
| Feed Posts | ✅        | Изображения, видео, карусели |
| Reels      | ✅        | С 2022 года                  |
| Stories    | ✅        | С 2023 года                  |

**Процесс публикации:**

1. Создание контейнера: `POST /{ig-user-id}/media`
2. Проверка статуса: `GET /{container-id}?fields=status_code`
3. Публикация: `POST /{ig-user-id}/media_publish`

**Ограничения:**

- До 25 публикаций в день через API
- Instagram Business/Creator аккаунт обязателен

**Permissions:**

- `instagram_basic`
- `instagram_content_publish`

### 5.4 Общие требования к интеграции

#### Необходимые Permissions

```
instagram_basic
instagram_content_publish
instagram_manage_comments
instagram_manage_messages
pages_manage_metadata
pages_read_engagement
```

> **Примечание:** При использовании нового Instagram API with Instagram Login permissions `pages_manage_metadata` и `pages_read_engagement` не требуются.

#### Access Token

- Long-lived User Access Token
- Refresh перед истечением (60 дней)
- Хранение в существующем Laravel-бэкенде

---

## Примечания

1. **Привязка аккаунтов** — уже реализована в Laravel-бэкенде, токены доступны
2. **Генерация отчётов** — выполняется на фронтенде, бэкенд только отдаёт агрегированные данные
3. **Фоновая синхронизация и Webhooks** — в текущей версии не требуются
4. **Статусы публикаций** — определены в типах фронтенда: `published`, `scheduled`, `error`, `draft`
5. Все пользователи с подключённым аккаунтом могут получать сообщения и инициировать диалог

---

## Источники

### Официальная документация Meta

- [Instagram API with Instagram Login](https://developers.facebook.com/docs/instagram-api/instagram-login) — новый API (рекомендуется)
- [Instagram Graph API](https://developers.facebook.com/docs/instagram-api) — общая документация
- [Content Publishing API](https://developers.facebook.com/docs/instagram-api/guides/content-publishing)

### Статьи и руководства

- Meta Releases New Instagram API with Instagram Login - Swipe Insight
- Instagram's New API Without Facebook - Postly
- Content Publishing Implementation Guide - GitHub Gist
- Instagram Graph API Guide 2025 - Elfsight
- Comment Moderation - NapoleonCat
