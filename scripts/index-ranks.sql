WITH
    ranks_definition (rank_index, min_days) AS (
        -- Define your ranks directly in a CTE with named columns
        VALUES
            (1, 0),    -- Baby
            (2, 7),    -- Novice
            (3, 30),   -- Developing
            (4, 60),   -- Advancing
            (5, 90),   -- Proficient
            (6, 150),  -- Competent
            (7, 210),  -- Capable
            (8, 270),  -- Solid
            (9, 330),  -- Excellent
            (10, 390), -- Formidable
            (11, 450), -- Outstanding
            (12, 510), -- Brilliant
            (13, 570), -- Magnificent
            (14, 630), -- WorldClass
            (15, 690), -- Supernatural
            (16, 750), -- Titanic
            (17, 810), -- ExtraTerrestrial
            (18, 870), -- Mythical
            (19, 930), -- Magical
            (20, 990), -- Utopian
            (21, 1050) -- Divine
    ),
    user_effective_days AS (
        -- For each user, calculate their effective days
        SELECT
            u.id,
            JULIANDAY('now') - JULIANDAY(
                (SELECT MIN(created_at) FROM workouts WHERE user_id = u.id)
            ) as total_days,
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
    ),
    rank_determination AS (
        -- Determine the appropriate rank based on effective days
        SELECT
            ued.id,
            ued.total_days - ued.inactive_days as effective_days,
            (
                SELECT MAX(rd.rank_index)
                FROM ranks_definition rd -- Use the CTE here
                WHERE rd.min_days <= (ued.total_days - ued.inactive_days)
            ) as new_rank
        FROM user_effective_days ued
    )
UPDATE users
SET rank = rd.new_rank
FROM rank_determination rd
WHERE users.id = rd.id;
