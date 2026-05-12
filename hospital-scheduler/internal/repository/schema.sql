-- 科室表
CREATE TABLE IF NOT EXISTS departments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    code       TEXT NOT NULL UNIQUE,
    is_active  INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 班次类型表
CREATE TABLE IF NOT EXISTS shift_types (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    code         TEXT NOT NULL UNIQUE,
    name         TEXT NOT NULL,
    start_hour   INTEGER NOT NULL,
    start_minute INTEGER NOT NULL DEFAULT 0,
    end_hour     INTEGER NOT NULL,
    end_minute   INTEGER NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 员工表
CREATE TABLE IF NOT EXISTS staff (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    employee_no   TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    role          TEXT NOT NULL CHECK(role IN ('DOCTOR','NURSE','INTERN')),
    department_id INTEGER NOT NULL REFERENCES departments(id),
    is_active     INTEGER NOT NULL DEFAULT 1,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 员工资质表 (多对多)
CREATE TABLE IF NOT EXISTS staff_qualifications (
    staff_id      INTEGER NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
    qualification TEXT NOT NULL,
    PRIMARY KEY (staff_id, qualification)
);

-- 排班格 (Slot)
CREATE TABLE IF NOT EXISTS slots (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    department_id  INTEGER NOT NULL REFERENCES departments(id),
    shift_type_id  INTEGER NOT NULL REFERENCES shift_types(id),
    date           DATE NOT NULL,
    required_staff INTEGER NOT NULL DEFAULT 1,
    required_role  TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'OPEN'
                   CHECK(status IN ('OPEN','FILLED','LOCKED','CANCELED')),
    assigned_count INTEGER NOT NULL DEFAULT 0,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Slot所需资质
CREATE TABLE IF NOT EXISTS slot_qualifications (
    slot_id       INTEGER NOT NULL REFERENCES slots(id) ON DELETE CASCADE,
    qualification TEXT NOT NULL,
    PRIMARY KEY (slot_id, qualification)
);

-- 排班记录 (Assignment)
CREATE TABLE IF NOT EXISTS assignments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    staff_id   INTEGER NOT NULL REFERENCES staff(id),
    slot_id    INTEGER NOT NULL REFERENCES slots(id),
    status     TEXT NOT NULL DEFAULT 'ACTIVE'
               CHECK(status IN ('ACTIVE','CANCELED','EMERGENCY')),
    source     TEXT NOT NULL DEFAULT 'MANUAL'
               CHECK(source IN ('AUTO','MANUAL','EMERGENCY','SWAP')),
    note       TEXT NOT NULL DEFAULT '',
    created_by INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(staff_id, slot_id)
);

-- 工时账户
CREATE TABLE IF NOT EXISTS workload_accounts (
    staff_id                INTEGER PRIMARY KEY REFERENCES staff(id),
    total_hours             REAL NOT NULL DEFAULT 0,
    month_hours             REAL NOT NULL DEFAULT 0,
    week_hours              REAL NOT NULL DEFAULT 0,
    consecutive_shifts      INTEGER NOT NULL DEFAULT 0,
    last_shift_end          DATETIME,
    night_shifts_this_month INTEGER NOT NULL DEFAULT 0,
    updated_at              DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 换班申请
CREATE TABLE IF NOT EXISTS swap_requests (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    requester_id      INTEGER NOT NULL REFERENCES staff(id),
    requester_slot_id INTEGER NOT NULL REFERENCES slots(id),
    target_staff_id   INTEGER REFERENCES staff(id),
    reason            TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'PENDING'
                      CHECK(status IN ('PENDING','APPROVED','REJECTED')),
    review_note       TEXT NOT NULL DEFAULT '',
    reviewed_by       INTEGER REFERENCES staff(id),
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 规则配置表
CREATE TABLE IF NOT EXISTS rule_configs (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 审计日志
CREATE TABLE IF NOT EXISTS audit_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type TEXT NOT NULL,
    entity_id   INTEGER NOT NULL,
    action      TEXT NOT NULL,
    old_value   TEXT NOT NULL DEFAULT '',
    new_value   TEXT NOT NULL DEFAULT '',
    operator_id INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_assignments_staff_date
    ON assignments(staff_id, created_at);
CREATE INDEX IF NOT EXISTS idx_assignments_slot
    ON assignments(slot_id, status);
CREATE INDEX IF NOT EXISTS idx_slots_dept_date
    ON slots(department_id, date, status);
CREATE INDEX IF NOT EXISTS idx_staff_dept
    ON staff(department_id, is_active);
CREATE INDEX IF NOT EXISTS idx_swap_requester
    ON swap_requests(requester_id, status);
