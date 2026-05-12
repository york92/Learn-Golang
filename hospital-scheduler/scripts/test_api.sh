#!/usr/bin/env bash
# Hospital Scheduler - API smoke test script
set -e

BASE="http://localhost:8080/api/v1"
echo "=== Hospital Scheduler API Test ==="
echo "Base URL: $BASE"
echo ""

# Health check
echo "── Health ──────────────────────────────────────"
curl -s "$BASE/../health" | python3 -m json.tool 2>/dev/null || curl -s "$BASE/../health"
echo ""

# List departments
echo "── Departments ─────────────────────────────────"
curl -s "$BASE/departments" | python3 -m json.tool 2>/dev/null || curl -s "$BASE/departments"
echo ""

# List staff
echo "── Staff ───────────────────────────────────────"
curl -s "$BASE/staff" | python3 -m json.tool 2>/dev/null || curl -s "$BASE/staff"
echo ""

# List shift types
echo "── Shift Types ─────────────────────────────────"
curl -s "$BASE/shift-types" | python3 -m json.tool 2>/dev/null || curl -s "$BASE/shift-types"
echo ""

# Create a slot
TODAY=$(date +%Y-%m-%d)
echo "── Create Slot for $TODAY ───────────────────────"
SLOT=$(curl -s -X POST "$BASE/slots" \
  -H "Content-Type: application/json" \
  -d "{
    \"department_id\": 1,
    \"shift_type_id\": 1,
    \"date\": \"$TODAY\",
    \"required_staff\": 2,
    \"required_role\": \"NURSE\",
    \"required_quals\": [\"ICU\"]
  }")
echo $SLOT | python3 -m json.tool 2>/dev/null || echo $SLOT
SLOT_ID=$(echo $SLOT | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['id'])" 2>/dev/null || echo "1")
echo "Slot ID: $SLOT_ID"
echo ""

# Manual assign
echo "── Manual Assignment ────────────────────────────"
curl -s -X POST "$BASE/assignments" \
  -H "Content-Type: application/json" \
  -d "{\"staff_id\": 3, \"slot_id\": $SLOT_ID, \"created_by\": 1}" \
  | python3 -m json.tool 2>/dev/null
echo ""

# Create another slot for auto-schedule
TOMORROW=$(date -d "+1 day" +%Y-%m-%d 2>/dev/null || date -v+1d +%Y-%m-%d)
echo "── Create Slot for $TOMORROW ────────────────────"
curl -s -X POST "$BASE/slots" \
  -H "Content-Type: application/json" \
  -d "{
    \"department_id\": 1,
    \"shift_type_id\": 2,
    \"date\": \"$TOMORROW\",
    \"required_staff\": 1,
    \"required_role\": \"DOCTOR\",
    \"required_quals\": [\"EMERGENCY\"]
  }" | python3 -m json.tool 2>/dev/null
echo ""

# Auto schedule
echo "── Auto Schedule ───────────────────────────────"
curl -s -X POST "$BASE/schedule/auto" \
  -H "Content-Type: application/json" \
  -d "{\"department_id\": 1, \"from\": \"$TODAY\", \"to\": \"$TOMORROW\"}" \
  | python3 -m json.tool 2>/dev/null
echo ""

# Workload report
echo "── Workload Report ─────────────────────────────"
curl -s "$BASE/workload?department_id=1" | python3 -m json.tool 2>/dev/null
echo ""

# Emergency candidates
echo "── Emergency Candidates for Slot $SLOT_ID ───────"
curl -s "$BASE/emergency/candidates/$SLOT_ID" | python3 -m json.tool 2>/dev/null
echo ""

echo "=== Done ==="
