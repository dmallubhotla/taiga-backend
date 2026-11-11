-- Why the workout is kept out of best efforts, routes and stats although its sport is scored, e.g. hand-entered or its points disagree with its totals; empty when it counts.
ALTER TABLE workouts
ADD COLUMN excluded_reason VARCHAR(64) NOT NULL DEFAULT '';
