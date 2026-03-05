-- Generic street preferences framework for OSRM routing penalty customization.
--
-- This module provides the machinery for applying speed penalties and bonuses
-- to road segments based on node IDs, street names, or coordinate areas.
-- Region-specific penalty data lives in a separate file (e.g. penalties/example-region.lua)
-- and is loaded at the bottom of this module if present.
--
-- To use:
--   1. Copy penalties/example-region.lua to penalties/my-region.lua
--   2. Add your node/street/coordinate penalties there
--   3. Set "penalty_file" in your region.json to point to it
--   4. The Docker mount will make it available at /opt/region_penalties.lua

local street_preferences = {}

-- Track applied penalties and diagnostics
street_preferences.node_coord_map = {}
street_preferences.node_coord_count = 0
street_preferences.node_lookup_failures = 0
street_preferences.node_coord_collisions = 0
street_preferences.node_lookup_fallbacks = 0
street_preferences.node_coord_missing = 0
street_preferences.node_apply_calls = 0
street_preferences.applied_penalties = {}

-- Debug logging function
local function debug_log(message)
  io.stderr:write("[STREET_PREFS] " .. message .. "\n")
  io.stderr:flush()
end

local function coord_key(lat, lon)
  return string.format("%.6f,%.6f", lat, lon)
end

local function format_coord(value)
  if value == nil then
    return "nil"
  end
  return string.format("%.6f", value)
end

local function safe_get_field(obj, field)
  if obj == nil then
    return nil
  end
  local ok, value = pcall(function()
    return obj[field]
  end)
  if ok then
    return value
  end
  return nil
end

local function get_storage(profile)
  local storage = profile.__street_preferences_state
  if not storage then
    storage = {
      node_coord_map = {},
      node_coord_count = 0,
      node_lookup_failures = 0,
      node_coord_collisions = 0,
      node_lookup_fallbacks = 0,
      node_coord_missing = 0,
      node_apply_calls = 0,
      segment_logging_started = false,
      applied_penalties = {},
    }
    profile.__street_preferences_state = storage
  end
  return storage
end

local function sync_exports(storage)
  street_preferences.node_coord_map = storage.node_coord_map
  street_preferences.node_coord_count = storage.node_coord_count
  street_preferences.node_lookup_failures = storage.node_lookup_failures
  street_preferences.node_coord_collisions = storage.node_coord_collisions
  street_preferences.node_lookup_fallbacks = storage.node_lookup_fallbacks
  street_preferences.node_coord_missing = storage.node_coord_missing
  street_preferences.node_apply_calls = storage.node_apply_calls
  street_preferences.applied_penalties = storage.applied_penalties
end

local function store_node_coord(storage, key, lat, lon, node_id)
  if not key or not node_id then
    return
  end

  local existing = storage.node_coord_map[key]
  if existing == node_id then
    return
  end

  if existing == nil then
    storage.node_coord_count = storage.node_coord_count + 1
    local count = storage.node_coord_count
    if count <= 5 or count % 50000 == 0 then
      debug_log(string.format(
        "NODE MAP: stored %d unique coordinates (latest node %d at %s,%s)",
        count, node_id, format_coord(lat), format_coord(lon)))
    end
  else
    storage.node_coord_collisions = storage.node_coord_collisions + 1
    local collisions = storage.node_coord_collisions
    if collisions <= 5 then
      debug_log(string.format(
        "NODE MAP COLLISION %d: coords %s,%s replaced node %s with %s",
        collisions, format_coord(lat), format_coord(lon),
        tostring(existing), tostring(node_id)))
    end
  end

  storage.node_coord_map[key] = node_id
  sync_exports(storage)
end

local function extract_node_id_from_endpoint(endpoint)
  if not endpoint then
    return nil
  end

  local candidate_fields = {"node_id", "id", "node", "osm_node_id", "original_osm_id"}

  for _, field in ipairs(candidate_fields) do
    local value = safe_get_field(endpoint, field)
    if value and type(value) ~= "function" then
      return value
    end
  end

  for _, field in ipairs(candidate_fields) do
    local getter = safe_get_field(endpoint, "get_" .. field) or safe_get_field(endpoint, field)
    if type(getter) == "function" then
      local ok, value = pcall(getter, endpoint)
      if ok and value then
        return value
      end
    end
  end

  return nil
end

local function resolve_segment_node(storage, endpoint, lat, lon, role)
  local key = nil
  if lat and lon then
    key = coord_key(lat, lon)
    local node_id = storage.node_coord_map[key]
    if node_id then
      return node_id
    end
  end

  local fallback_id = extract_node_id_from_endpoint(endpoint)
  if fallback_id then
    if key then
      store_node_coord(storage, key, lat, lon, fallback_id)
    end

    storage.node_lookup_fallbacks = storage.node_lookup_fallbacks + 1
    local fallback_count = storage.node_lookup_fallbacks
    if fallback_count <= 5 or fallback_count % 1000 == 0 then
      debug_log(string.format(
        "LOOKUP FALLBACK %d: using %s node %s (coords %s,%s; map size=%d)",
        fallback_count, role, tostring(fallback_id),
        format_coord(lat), format_coord(lon), storage.node_coord_count))
    end

    sync_exports(storage)
    return fallback_id
  end

  storage.node_lookup_failures = storage.node_lookup_failures + 1
  local failures = storage.node_lookup_failures
  if failures <= 5 or failures % 1000 == 0 then
    debug_log(string.format(
      "LOOKUP MISS %d: no node found for %s coords %s,%s (map size=%d)",
      failures, role, format_coord(lat), format_coord(lon), storage.node_coord_count))
  end

  sync_exports(storage)
  return nil
end

-- Penalty configuration: speed multipliers
street_preferences.penalties = {
  high = 0.2,         -- Heavy penalty (80% speed reduction)
  medium = 0.5,       -- Medium penalty (50% speed reduction)
  low = 0.7,          -- Light penalty (30% speed reduction)
  bonus_low = 1.3,    -- Light bonus (30% speed increase)
  bonus_medium = 1.5, -- Medium bonus (50% speed increase)
  bonus_high = 2.0    -- High bonus (100% speed increase)
}

-- Region-specific data tables (populated by region penalty files)
street_preferences.street_rules = {}
street_preferences.node_penalties = {}

-- Validation
local function fail_configuration(message)
  error("[STREET_PREFS] configuration error: " .. message)
end

local function validate_configuration()
  for index, rule in ipairs(street_preferences.street_rules) do
    if not street_preferences.penalties[rule.penalty] then
      fail_configuration(string.format(
        "street rule %d ('%s') references unknown penalty '%s'",
        index, rule.description or rule.pattern or "(no description)",
        tostring(rule.penalty)))
    end
  end

  for node_id, config in pairs(street_preferences.node_penalties) do
    if type(node_id) ~= "number" then
      fail_configuration(string.format(
        "node penalty key '%s' must be a number (OSM node ID)",
        tostring(node_id)))
    end

    if not street_preferences.penalties[config.penalty] then
      fail_configuration(string.format(
        "node penalty %d ('%s') references unknown penalty '%s'",
        node_id, config.name or "(unnamed node)", tostring(config.penalty)))
    end

    if config.extra_duration_seconds and config.extra_duration_seconds < 0 then
      fail_configuration(string.format(
        "node penalty %d ('%s') has negative extra_duration_seconds %.2f",
        node_id, config.name or "(unnamed node)", config.extra_duration_seconds))
    end
  end

  if street_preferences.coordinate_areas then
    for index, area in ipairs(street_preferences.coordinate_areas) do
      if area.lat_min and area.lat_max and area.lat_min > area.lat_max then
        fail_configuration(string.format(
          "coordinate area %d ('%s') has lat_min > lat_max (%.6f > %.6f)",
          index, area.name or "(unnamed area)", area.lat_min, area.lat_max))
      end
      if area.lon_min and area.lon_max and area.lon_min > area.lon_max then
        fail_configuration(string.format(
          "coordinate area %d ('%s') has lon_min > lon_max (%.6f > %.6f)",
          index, area.name or "(unnamed area)", area.lon_min, area.lon_max))
      end
      if not street_preferences.penalties[area.penalty] then
        fail_configuration(string.format(
          "coordinate area %d ('%s') references unknown penalty '%s'",
          index, area.name or "(unnamed area)", tostring(area.penalty)))
      end
    end
  end
end

-- Apply penalty/bonus to segment duration/weight
local function apply_penalty_to_segment(segment, penalty_key, context)
  local multiplier = street_preferences.penalties[penalty_key]
  if not multiplier then
    fail_configuration(string.format("segment penalty attempted with unknown key '%s'", tostring(penalty_key)))
  end

  if multiplier <= 0 then
    fail_configuration(string.format("penalty '%s' has non-positive multiplier %.3f", tostring(penalty_key), multiplier))
  end

  local duration_factor = 1 / multiplier

  local old_duration = segment.duration or 0
  local old_weight = segment.weight or 0

  if segment.duration and segment.duration > 0 then
    segment.duration = segment.duration * duration_factor
  end

  if segment.weight and segment.weight > 0 then
    segment.weight = segment.weight * duration_factor
  end

  debug_log(string.format("Applied %s penalty (multiplier %.2fx => duration factor %.2fx) to %s: duration %.2f->%.2f weight %.2f->%.2f",
    penalty_key, multiplier, duration_factor, context or "segment",
    old_duration, segment.duration or 0, old_weight, segment.weight or 0))
end

-- Apply a full penalty configuration (speed + extra duration) to a segment
local function apply_penalty_config(segment, config, context)
  local is_bonus = config.penalty and string.match(config.penalty, "bonus")
  local action_word = is_bonus and "bonus" or "penalty"

  local changes_made = false
  local original_duration = segment.duration
  local original_weight = segment.weight

  if config.penalty then
    apply_penalty_to_segment(segment, config.penalty, context)
    changes_made = true
  end

  if config.extra_duration_seconds and config.extra_duration_seconds ~= 0 then
    if segment.duration == nil then
      fail_configuration(string.format("segment missing duration when applying extra_duration_seconds to %s", context))
    end
    if segment.weight == nil then
      fail_configuration(string.format("segment missing weight when applying extra_duration_seconds to %s", context))
    end

    local old_duration = segment.duration
    segment.duration = segment.duration + config.extra_duration_seconds
    segment.weight = segment.weight * (segment.duration / old_duration)

    debug_log(string.format("Added %.2fs to %s: duration %.2f->%.2f",
      config.extra_duration_seconds, context, old_duration, segment.duration))
    changes_made = true
  end

  if not changes_made then
    fail_configuration(string.format("%s BUG: apply_penalty_config called for %s but no changes applied",
      string.upper(action_word), context))
  end

  if changes_made and segment.duration == original_duration and segment.weight == original_weight then
    fail_configuration(string.format("%s BUG: values unchanged after applying to %s",
      string.upper(action_word), context))
  end
end

-- Apply street name-based routing preferences to a way
function street_preferences.apply(way, result)
  local name = way:get_value_by_key("name")
  if not name then
    return
  end

  local name_lower = string.lower(name)

  for _, rule in ipairs(street_preferences.street_rules) do
    if string.find(name_lower, rule.pattern) then
      debug_log(string.format("Matched rule pattern '%s' for way '%s'", rule.pattern, name))

      local multiplier = street_preferences.penalties[rule.penalty]
      if multiplier and result.forward_speed and result.forward_speed > 0 then
        result.forward_speed = result.forward_speed * multiplier
      end
      if multiplier and result.backward_speed and result.backward_speed > 0 then
        result.backward_speed = result.backward_speed * multiplier
      end
      break
    end
  end
end

-- Node processing stub (penalties are handled at segment level)
function street_preferences.apply_node(profile, node, result)
  -- All node-based penalties are handled in apply_segment
end

-- Check if a coordinate point is within a bounding box
local function point_in_bounds(lat, lon, area)
  return lat >= area.lat_min and lat <= area.lat_max and
         lon >= area.lon_min and lon <= area.lon_max
end

-- Apply segment-level penalties based on node IDs
function street_preferences.apply_segment(profile, segment)
  if not segment then
    return
  end

  local storage = get_storage(profile)
  sync_exports(storage)
  if not storage.segment_logging_started then
    storage.segment_logging_started = true
    debug_log("SEGMENT PROCESSING: apply_segment active - checking node_id based penalties")
    sync_exports(storage)
  end

  storage.segment_calls = (storage.segment_calls or 0) + 1

  local source_node_id = segment.source_node_id
  if source_node_id then
    local source_node_str = tostring(source_node_id)
    local source_node_num = nil

    if type(source_node_id) == "userdata" then
      source_node_num = tonumber(source_node_str:match("%d+"))
    else
      source_node_num = tonumber(source_node_id)
    end

    if source_node_num then
      local source_config = street_preferences.node_penalties[source_node_num]
      if source_config then
        local context = string.format("source node %d (%s)", source_node_num, source_config.name or "unnamed")
        apply_penalty_config(segment, source_config, context)
        return
      end
    end
  end

  -- Legacy coordinate-based penalties (fallback)
  if street_preferences.coordinate_areas then
    local src_lat = segment.source and segment.source.lat
    local src_lon = segment.source and segment.source.lon
    local tgt_lat = segment.target and segment.target.lat
    local tgt_lon = segment.target and segment.target.lon

    if src_lat and src_lon and tgt_lat and tgt_lon then
      for _, area in ipairs(street_preferences.coordinate_areas) do
        if point_in_bounds(src_lat, src_lon, area) or point_in_bounds(tgt_lat, tgt_lon, area) then
          local coord_context = string.format("segment in coordinate area %s", area.name or "(unnamed)")
          debug_log("WARNING: Using legacy coordinate-based penalty: " .. coord_context)

          if area.penalty then
            apply_penalty_to_segment(segment, area.penalty, coord_context)
          end

          if area.extra_duration_seconds and area.extra_duration_seconds ~= 0 then
            local old_duration = segment.duration or 0
            if segment.duration then
              segment.duration = segment.duration + area.extra_duration_seconds
            end
            if segment.weight and old_duration > 0 then
              segment.weight = segment.weight * (segment.duration / old_duration)
            end
          end
          return
        end
      end
    end
  end
end

-- Utility functions for adding penalties at runtime
function street_preferences.add_node_penalty(node_id, name, penalty, extra_duration_seconds, description)
  street_preferences.node_penalties[node_id] = {
    name = name,
    penalty = penalty,
    extra_duration_seconds = extra_duration_seconds,
    description = description or ""
  }
  validate_configuration()
end

function street_preferences.add_street_rule(pattern, penalty, description)
  table.insert(street_preferences.street_rules, {
    pattern = pattern,
    penalty = penalty,
    description = description or ""
  })
  validate_configuration()
end

function street_preferences.add_coordinate_area(name, lat_min, lat_max, lon_min, lon_max, penalty, description)
  if not street_preferences.coordinate_areas then
    street_preferences.coordinate_areas = {}
  end
  table.insert(street_preferences.coordinate_areas, {
    name = name,
    lat_min = lat_min,
    lat_max = lat_max,
    lon_min = lon_min,
    lon_max = lon_max,
    penalty = penalty,
    description = description or ""
  })
  validate_configuration()
end

-- Load region-specific penalties if available
local region_penalties_path = "/opt/region_penalties.lua"
local f = io.open(region_penalties_path, "r")
if f then
  f:close()
  debug_log("Loading region penalties from: " .. region_penalties_path)
  local region = dofile(region_penalties_path)
  if region and type(region) == "table" then
    if region.node_penalties then
      for node_id, config in pairs(region.node_penalties) do
        street_preferences.node_penalties[node_id] = config
      end
    end
    if region.street_rules then
      for _, rule in ipairs(region.street_rules) do
        table.insert(street_preferences.street_rules, rule)
      end
    end
    if region.coordinate_areas then
      street_preferences.coordinate_areas = street_preferences.coordinate_areas or {}
      for _, area in ipairs(region.coordinate_areas) do
        table.insert(street_preferences.coordinate_areas, area)
      end
    end
  end
else
  debug_log("No region penalties file found at " .. region_penalties_path .. " (this is OK)")
end

-- Validate and log
validate_configuration()
debug_log("Street preferences module loaded")

local node_count = 0
for node_id, config in pairs(street_preferences.node_penalties) do
  node_count = node_count + 1
end
debug_log(string.format("Configured %d node penalties", node_count))
debug_log(string.format("Configured %d street name rules", #street_preferences.street_rules))

return street_preferences
