-- What recorded the workout, e.g. a watch's product name or "Endomondo"; empty when the file doesn't say or the workout was entered by hand.
ALTER TABLE workouts
ADD COLUMN source TEXT NOT NULL DEFAULT '';
