#!/bin/bash
# SoloQuest Core Flow Smoke Test
# Usage: ./scripts/smoke_core_flow.sh [BASE_URL]

BASE_URL="${1:-http://localhost:9000}"
PASS=0
FAIL=0

check() {
  local desc="$1"
  local url="$2"
  local method="${3:-GET}"
  local body="$4"

  if [ "$method" = "POST" ] && [ -n "$body" ]; then
    resp=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$url" -H "Content-Type: application/json" -d "$body")
  elif [ "$method" = "POST" ]; then
    resp=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$url")
  else
    resp=$(curl -s -o /dev/null -w "%{http_code}" "$url")
  fi

  if [ "$resp" -ge 200 ] && [ "$resp" -lt 300 ]; then
    echo "  PASS  $desc ($resp)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL  $desc ($resp)"
    FAIL=$((FAIL + 1))
  fi
}

echo "SoloQuest Smoke Test"
echo "Base URL: $BASE_URL"
echo "---"

# Health
check "GET /health" "$BASE_URL/health"
check "GET /health/db" "$BASE_URL/health/db"
check "GET /health/models" "$BASE_URL/health/models"

# Users
check "GET /api/users/me" "$BASE_URL/api/users/me"
check "GET /api/users/me/daily-status" "$BASE_URL/api/users/me/daily-status"

# Onboarding
check "GET /api/onboarding/status" "$BASE_URL/api/onboarding/status"

# Quests
check "GET /api/quests" "$BASE_URL/api/quests"

# Progress
check "GET /api/progress" "$BASE_URL/api/progress"
check "GET /api/progress/weekly-chart" "$BASE_URL/api/progress/weekly-chart"
check "GET /api/progress/xp-history" "$BASE_URL/api/progress/xp-history"

# Logs
check "GET /api/logs" "$BASE_URL/api/logs"

# Check-ins
check "GET /api/checkins/today" "$BASE_URL/api/checkins/today"

# Reviews
check "GET /api/reviews/today" "$BASE_URL/api/reviews/today"
check "GET /api/reviews/summary" "$BASE_URL/api/reviews/summary"

# Rewards
check "GET /api/rewards" "$BASE_URL/api/rewards"
check "GET /api/rewards/redemptions" "$BASE_URL/api/rewards/redemptions"

# Settings
check "GET /api/settings" "$BASE_URL/api/settings"

echo "---"
echo "Results: $PASS passed, $FAIL failed"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
