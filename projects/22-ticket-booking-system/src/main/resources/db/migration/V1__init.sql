CREATE TABLE IF NOT EXISTS events (
    id          VARCHAR(36) PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    venue       VARCHAR(255) NOT NULL,
    event_time  TIMESTAMPTZ  NOT NULL,
    total_seats INT          NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS seats (
    id          VARCHAR(36) PRIMARY KEY,
    event_id    VARCHAR(36)  NOT NULL REFERENCES events(id),
    seat_number VARCHAR(20)  NOT NULL,
    row_label   VARCHAR(10),
    section     VARCHAR(20),
    status      VARCHAR(20)  NOT NULL DEFAULT 'AVAILABLE',
    version     BIGINT       NOT NULL DEFAULT 0,
    CONSTRAINT uq_seat_event UNIQUE (event_id, section, row_label, seat_number)
);

CREATE INDEX IF NOT EXISTS idx_seats_event_status ON seats(event_id, status);

CREATE TABLE IF NOT EXISTS holds (
    id         VARCHAR(36) PRIMARY KEY,
    seat_id    VARCHAR(36)  NOT NULL REFERENCES seats(id),
    event_id   VARCHAR(36)  NOT NULL REFERENCES events(id),
    user_id    VARCHAR(128) NOT NULL,
    expires_at TIMESTAMPTZ  NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    status     VARCHAR(20)  NOT NULL DEFAULT 'ACTIVE'
);

CREATE INDEX IF NOT EXISTS idx_holds_seat_id    ON holds(seat_id);
CREATE INDEX IF NOT EXISTS idx_holds_expires_at ON holds(expires_at) WHERE status = 'ACTIVE';

CREATE TABLE IF NOT EXISTS bookings (
    id               VARCHAR(36) PRIMARY KEY,
    hold_id          VARCHAR(36)    NOT NULL UNIQUE REFERENCES holds(id),
    seat_id          VARCHAR(36)    NOT NULL REFERENCES seats(id),
    event_id         VARCHAR(36)    NOT NULL REFERENCES events(id),
    user_id          VARCHAR(128)   NOT NULL,
    amount           NUMERIC(10, 2) NOT NULL,
    payment_status   VARCHAR(20)    NOT NULL DEFAULT 'PENDING',
    idempotency_key  VARCHAR(128)   UNIQUE,
    created_at       TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bookings_user_id ON bookings(user_id);
