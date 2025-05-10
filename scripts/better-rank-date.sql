WITH user_effective_days AS (
    -- For each user, calculate their effective days by:
    -- 1. Starting from their creation date
    -- 2. Subtracting inactive periods (time between ban and next workout)
    -- 3. Adding up the remaining active days
    SELECT
        u.id,
        JULIANDAY('now') - JULIANDAY(u.created_at) as total_days,
        COALESCE((
            SELECT SUM(
                JULIANDAY(COALESCE(
                    (SELECT MIN(w.created_at)
                     FROM workouts w
                     WHERE w.user_id = u.id
                     AND w.created_at > b.created_at),
                    'now' -- If no workout after ban, inactive until now
                )) - JULIANDAY(b.created_at)
            )
            FROM events b
            WHERE b.user_id = u.id
            AND b.event = 'ban'
        ), 0) as inactive_days
    FROM users u
)
UPDATE users
SET rank_updated_at = datetime('now', '-' || (ued.total_days - ued.inactive_days) || ' days')
FROM user_effective_days ued
WHERE users.id = ued.id;
