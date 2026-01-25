-- Atomic rate limit check and increment
-- This script checks both daily and minute limits, then increments if allowed
-- Returns: {allowed, daily_count, minute_count, reset_daily, reset_minute}

local daily_key = KEYS[1]    -- "ratelimit:org:{id}:daily:{YYYYMMDD}"
local minute_key = KEYS[2]   -- "ratelimit:org:{id}:minute:{timestamp}"

local daily_limit = tonumber(ARGV[1])
local minute_limit = tonumber(ARGV[2])
local burst = tonumber(ARGV[3])
local current_time = tonumber(ARGV[4])  -- Unix timestamp

-- Get current counts (returns 0 if key doesn't exist)
local daily_count = tonumber(redis.call('GET', daily_key) or "0")
local minute_count = tonumber(redis.call('GET', minute_key) or "0")

-- Check daily limit first (hard limit)
if daily_count >= daily_limit then
    local ttl = redis.call('TTL', daily_key)
    return {0, daily_count, minute_count, ttl, 0}  -- Deny: daily limit exceeded
end

-- Check minute limit with burst allowance
if minute_count >= (minute_limit + burst) then
    local ttl = redis.call('TTL', minute_key)
    return {0, daily_count, minute_count, 0, ttl}  -- Deny: minute limit exceeded
end

-- Both checks passed - increment counters
redis.call('INCR', daily_key)
redis.call('INCR', minute_key)

-- Set expiry if this is the first request
if daily_count == 0 then
    -- Expire at end of day (calculate seconds until midnight UTC)
    local seconds_in_day = 86400
    local seconds_since_midnight = current_time % seconds_in_day
    local ttl_daily = seconds_in_day - seconds_since_midnight
    redis.call('EXPIRE', daily_key, ttl_daily)
end

if minute_count == 0 then
    -- Expire after 120 seconds (2 minutes buffer for clock skew)
    redis.call('EXPIRE', minute_key, 120)
end

-- Get new TTLs
local ttl_daily = redis.call('TTL', daily_key)
local ttl_minute = redis.call('TTL', minute_key)

-- Return: allowed=1, new counts, reset times
return {1, daily_count + 1, minute_count + 1, ttl_daily, ttl_minute}
