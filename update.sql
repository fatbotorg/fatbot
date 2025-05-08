WITH user_first_workouts AS (
    SELECT 
        user_id,
        MIN(created_at) as first_workout_time
    FROM workouts
    GROUP BY user_id
),
user_last_bans AS (
    SELECT 
        user_id,
        MAX(created_at) as last_ban_time
    FROM events
    WHERE event = 'ban'
    GROUP BY user_id
),
user_first_workout_after_ban AS (
    SELECT 
        w.user_id,
        MIN(w.created_at) as first_workout_after_ban
    FROM workouts w
    JOIN user_last_bans b ON w.user_id = b.user_id
    WHERE w.created_at > b.last_ban_time
    GROUP BY w.user_id
)
INSERT INTO events (user_id, event, created_at, updated_at)
SELECT 
    u.id as user_id,
    'rejoinedGroup' as event,
    COALESCE(
        fwab.first_workout_after_ban,
        fw.first_workout_time
    ) as created_at,
    COALESCE(
        fwab.first_workout_after_ban,
        fw.first_workout_time
    ) as updated_at
FROM users u
LEFT JOIN user_first_workouts fw ON u.id = fw.user_id
LEFT JOIN user_last_bans b ON u.id = b.user_id
LEFT JOIN user_first_workout_after_ban fwab ON u.id = fwab.user_id
WHERE fw.first_workout_time IS NOT NULL;
