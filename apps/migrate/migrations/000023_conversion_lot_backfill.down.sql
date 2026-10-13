-- Historical allocations and lots cannot be safely reversed after subsequent
-- spends. Reapplying the up migration is idempotent.
SELECT 1;
