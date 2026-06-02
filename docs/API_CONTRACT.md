# SoloQuest API Contract

Base URL: `http://localhost:9000`

## Time Standard

Backend returns all timestamps in UTC (ISO-8601). Frontend converts to local display time.

## Auth

Development mode uses dev user context middleware. Real JWT/Google login is not implemented yet.

---

## Health

### GET /health

```json
{ "status": "ok", "service": "soloquest-backend" }
```

### GET /health/db

```json
{ "status": "ok", "database": "connected" }
```

### GET /health/models

```json
{ "status": "ok", "models": ["user_profiles", "quests", ...] }
```

---

## Auth

### POST /api/auth/dev-login

```json
{
  "user": { ... },
  "access_token": "dev-token-...",
  "refresh_token": "dev-refresh-token-...",
  "token_type": "Bearer"
}
```

---

## Users

### GET /api/users/me

```json
{ "user": { "id": "...", "display_name": "...", "level": 1, "reward_points": 100, ... } }
```

### GET /api/users/dev

Same shape as `/api/users/me`.

### GET /api/users/me/daily-status

```json
{
  "user_id": "...",
  "date": "2026-06-01",
  "has_checked_in_today": false,
  "has_reviewed_today": false
}
```

---

## Onboarding

### POST /api/users/me/onboarding

Request:

```json
{
  "display_name": "Minh Thanh",
  "age": 25,
  "gender": "Nam",
  "height_cm": 170,
  "weight_kg": 65,
  "main_activity": "Engineer",
  "main_goals": ["Uống nước", "Học tập"],
  "quiet_after_time": "22:00"
}
```

Success (200):

```json
{
  "profile": { ... },
  "onboarding": { ... },
  "message": "onboarding saved successfully"
}
```

### GET /api/users/me/onboarding/status

```json
{
  "has_completed_onboarding": true,
  "user_id": "...",
  "profile_name": "Minh Thanh"
}
```

---

## Quests

### GET /api/quests?date=YYYY-MM-DD

```json
{
  "quests": [ ... ],
  "date": "2026-06-01"
}
```

### POST /api/quests/:id/start

Success (200):

```json
{ "quest": { "status": "active", ... }, "message": "quest started successfully" }
```

Errors: 400 (invalid id), 404 (not found), 409 (invalid status)

### POST /api/quests/:id/complete

Request: `{ "note": "Done!" }`

Success (200):

```json
{
  "quest": { ... },
  "exp_transaction": { ... },
  "reward_points_transaction": { ... },
  "profile": { ... },
  "message": "quest completed successfully"
}
```

Errors: 400 (invalid id), 404 (not found), 409 (already completed)

### POST /api/quests/:id/skip

Request: `{ "reason": "Đang bận" }`

```json
{ "quest": { ... }, "message": "quest skipped successfully" }
```

### POST /api/quests/:id/snooze

Request: `{ "minutes": 15 }`

```json
{ "quest": { ... }, "message": "quest snoozed successfully" }
```

Errors: 400 (invalid id/body/duration), 404 (not found), 409 (invalid status)

---

## Progress

### GET /api/progress

```json
{
  "level": 1,
  "current_level_exp": 10,
  "next_level_exp": 100,
  "total_exp": 10,
  "reward_points": 110,
  "streak_days": 0,
  "best_streak": 0,
  "streak_shields": 2,
  "total_completed_quests": 1,
  "total_skipped_quests": 0,
  "today_completed_quests": 1,
  "today_total_quests": 3,
  "today_completion_rate": 0.33,
  "weekly_completion_rate": 0.1,
  "completed_by_type": { "daily": 1 },
  "weekly_daily_data": [ ... ]
}
```

### GET /api/progress/weekly-chart

```json
{
  "week_start": "2026-06-01",
  "week_end": "2026-06-07",
  "items": [
    { "date": "2026-06-01", "day_label": "T2", "completed": 2, "planned": 5, "completion_rate": 0.4 }
  ]
}
```

### GET /api/progress/xp-history?currency=xp&limit=50&offset=0

Query params: `currency` (xp|reward_points|gem), `limit` (default 50, max 100), `offset`

```json
{
  "items": [
    {
      "id": "...",
      "amount": 10,
      "currency": "xp",
      "source": "quest_completion",
      "reference_id": "...",
      "description": "Hoàn thành quest: Uống nước",
      "balance_after": 110,
      "created_at": "2026-06-01T10:00:00Z"
    }
  ],
  "limit": 50,
  "offset": 0
}
```

---

## Logs

### GET /api/logs?type=questCompleted&date=YYYY-MM-DD&from=YYYY-MM-DD&to=YYYY-MM-DD&limit=50&offset=0

Query params: `type`, `date`, `from`, `to`, `limit`, `offset`

```json
{
  "items": [
    {
      "id": "...",
      "type": "questCompleted",
      "title": "Hoàn thành quest: Uống nước",
      "content": "+10 EXP",
      "quest_id": "...",
      "quest_type": "daily",
      "exp_changed": 10,
      "points_changed": 10,
      "created_at": "2026-06-01T10:00:00Z"
    }
  ],
  "limit": 50,
  "offset": 0
}
```

---

## Check-ins

### GET /api/checkins/today

```json
{
  "has_checked_in": true,
  "date": "2026-06-01",
  "checkin": { "id": "...", "energy_level": "high", "stress_level": "low", ... }
}
```

### GET /api/checkins?date=YYYY-MM-DD

Same shape as `/api/checkins/today`.

### POST /api/checkins

Request:

```json
{
  "energy_level": "high",
  "stress_level": "low",
  "focus_level": "medium",
  "day_intensity": "normal",
  "main_focus_today": "Backend work",
  "note": "Focus mode",
  "available_time_blocks": ["morning", "evening"]
}
```

Success (200):

```json
{ "item": { ... }, "message": "daily check-in saved successfully" }
```

---

## Reviews

### GET /api/reviews/today

```json
{
  "has_reviewed": true,
  "date": "2026-06-01",
  "review": { "id": "...", "mood": "good", ... }
}
```

### GET /api/reviews?date=YYYY-MM-DD

Same shape as `/api/reviews/today`.

### GET /api/reviews/summary?date=YYYY-MM-DD

```json
{
  "date": "2026-06-01",
  "completed_quest_count": 2,
  "skipped_quest_count": 0,
  "pending_quest_count": 3,
  "total_quest_count": 5,
  "earned_exp": 20,
  "completion_rate": 0.4,
  "completed_by_type": { "daily": 1, "water": 1 }
}
```

### POST /api/reviews

Request:

```json
{
  "mood": "good",
  "difficulty_rating": 3,
  "energy_level": 4,
  "satisfaction_level": 4,
  "helpful_quests": ["water"],
  "annoying_quests": [],
  "best_moment": "Hoàn thành API",
  "challenge": "Mệt buổi chiều",
  "improvement_tomorrow": "Chia task nhỏ hơn",
  "tomorrow_adjustments": ["more_breaks"],
  "note": "Ngày ổn"
}
```

Success (200):

```json
{ "item": { ... }, "summary": { ... }, "message": "daily review saved successfully" }
```

---

## Rewards

### GET /api/rewards

```json
{
  "items": [
    {
      "id": "...",
      "title": "Nghỉ ngơi 30 phút",
      "description": "",
      "type": "rest",
      "status": "available",
      "cost_points": 30,
      "icon_text": "🛋️",
      "claimed_at": null,
      "can_claim": true,
      "created_at": "...",
      "updated_at": "..."
    }
  ],
  "wallet": { "reward_points": 100 }
}
```

### POST /api/rewards/:id/claim

Success (200):

```json
{
  "reward": { "id": "...", "status": "claimed", "claimed_at": "...", "can_claim": false, ... },
  "redemption": { "id": "...", "reward_id": "...", "reward_title": "...", "points_spent": 30, ... },
  "transaction": { "id": "...", "amount": -30, "currency": "reward_points", "source": "reward_claim", ... },
  "profile": { "id": "...", "reward_points": 70 },
  "message": "reward claimed successfully"
}
```

Errors: 400 (invalid id), 404 (not found), 409 (already claimed / insufficient points)

### GET /api/rewards/redemptions?limit=50&offset=0

```json
{
  "items": [
    {
      "id": "...",
      "reward_id": "...",
      "reward_title": "Nghỉ ngơi 30 phút",
      "reward_type": "rest",
      "icon_text": "🛋️",
      "points_spent": 30,
      "created_at": "..."
    }
  ],
  "limit": 50,
  "offset": 0
}
```

---

## Settings

### GET /api/settings

```json
{ "settings": { "id": "...", "language": "vi", ... } }
```

---

## Error Response Pattern

All endpoints return errors in this shape:

```json
{ "error": "human-readable error message" }
```

HTTP status codes used: 400, 401, 404, 409, 500
