UPDATE users
SET rank_updated_at = (
    SELECT e.created_at
    FROM events e
    WHERE e.user_id = users.id  -- Reference the outer table by its name
    AND e.event = 'rejoinedGroup'
    ORDER BY e.created_at DESC
    LIMIT 1
)
WHERE EXISTS (
    SELECT 1
    FROM events e
    WHERE e.user_id = users.id  -- Reference the outer table by its name
    AND e.event = 'rejoinedGroup'
);
