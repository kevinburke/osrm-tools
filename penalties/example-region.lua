-- Example region-specific penalty file.
--
-- Copy this file and customize it for your region. Set "penalty_file" in your
-- region.json to point to your copy.
--
-- Available penalty levels (defined in street_preferences.lua):
--   high         = 0.2  (80% speed reduction)
--   medium       = 0.5  (50% speed reduction)
--   low          = 0.7  (30% speed reduction)
--   bonus_low    = 1.3  (30% speed increase)
--   bonus_medium = 1.5  (50% speed increase)
--   bonus_high   = 2.0  (100% speed increase)
--
-- HOW TO FIND OSM NODE IDs:
--   1. Go to https://www.openstreetmap.org
--   2. Navigate to your area of interest
--   3. Click "Edit" to open the editor
--   4. Select a node (intersection point) - the ID appears in the sidebar
--   5. Use just the number as the key below
--
-- This file should return a table with one or more of these keys:
--   node_penalties    - table mapping OSM node IDs to penalty configs
--   street_rules      - array of {pattern, penalty, description} tables
--   coordinate_areas  - array of bounding box penalty areas

local region = {}

-- Node-based penalties (most precise)
region.node_penalties = {
  -- Example: penalize a dangerous overpass
  -- [417767169] = {
  --   name = "Dangerous Overpass",
  --   penalty = "high",
  --   extra_duration_seconds = 240,
  --   description = "No bike infrastructure on overpass"
  -- },

  -- Example: bonus for a low-stress bike route
  -- [57823162] = {
  --   name = "Protected Bike Lane",
  --   penalty = "bonus_high",
  --   description = "Route people along the protected bike lane"
  -- },
}

-- Street name pattern rules
region.street_rules = {
  -- Example: penalize a busy road
  -- {pattern = "highway 101", penalty = "high", description = "Major highway"},
}

-- Coordinate area penalties (less precise, use node IDs when possible)
-- region.coordinate_areas = {
--   {
--     name = "Downtown Core",
--     lat_min = 37.780,
--     lat_max = 37.790,
--     lon_min = -122.420,
--     lon_max = -122.410,
--     penalty = "low",
--     description = "Congested area"
--   },
-- }

return region
