-- How often (in seconds) the passenger and driver apps' live maps refresh
-- their own position and nearby-driver markers. Configurable from the admin
-- panel instead of hardcoded in each app, so it can be tuned without a
-- release — e.g. turned down if it's putting too much load on the API/OSM
-- tile servers, or turned up for tighter tracking.
ALTER TABLE platform_settings ADD COLUMN map_poll_seconds INTEGER NOT NULL DEFAULT 60;
