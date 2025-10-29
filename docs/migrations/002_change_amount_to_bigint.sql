-- Change amount column from int4 to int8 to support larger values
-- This migration is safe to run on production without downtime
ALTER TABLE "expenses" ALTER COLUMN "amount" TYPE int8;

