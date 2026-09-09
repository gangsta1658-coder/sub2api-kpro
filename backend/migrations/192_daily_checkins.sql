CREATE TABLE daily_checkins (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    checkin_date DATE NOT NULL,
    reward DECIMAL(20,8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT daily_checkins_user_date_key UNIQUE (user_id, checkin_date)
);

CREATE INDEX daily_checkins_user_id_idx ON daily_checkins (user_id);
