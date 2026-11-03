global_result = {}

local function validate_n(n)
    local num = tonumber(n) or 0
    if num < 1 then return 1 end
    if num > 50 then return 50 end
    return math.floor(num)
end

--- Generates a list of random strings from the provided options.
-- @param count (number) The number of items to return.
-- @param allow_duplicates (boolean) Whether items can be picked more than once.
--        Forced to true if count > number of options.
-- @param ... (string) The pool of text options to choose from.
-- @return (table) A table containing the randomly selected strings.
function random_text(is_repeatable, n, ...)
    n = validate_n(n)
    local options = { ... }
    local len = #options
    local result = {}

    local actual_allow_duplicates = is_repeatable or (n > len)

    if actual_allow_duplicates then
        for i = 1, n do
            table.insert(result, options[math.random(1, len)])
        end
    else
        for i = 1, n do
            local j = math.random(i, len)
            options[i], options[j] = options[j], options[i]
            table.insert(result, options[i])
        end
    end

    table.insert(global_result, result)
    return result
end

--- Generates a list of random numbers within a specified range.
-- @param count (number) The number of digits/numbers to return.
-- @param min (number) The lower bound (inclusive).
-- @param max (number) The upper bound (inclusive).
-- @param allow_duplicates (boolean) Whether numbers can repeat.
--        Forced to true if count > (max - min + 1).
-- @return (table) A table containing the random numbers.
function random_num(is_repeatable, n, min, max)
    n = validate_n(n)
    local range = max - min + 1
    local result = {}

    local actual_allow_duplicates = is_repeatable or (n > range)

    if actual_allow_duplicates then
        for _ = 1, n do
            table.insert(result, math.random(min, max))
        end
    else
        local pool = {}
        for i = 1, range do
            pool[i] = min + i - 1
        end

        for i = 1, n do
            local j = math.random(i, range)
            pool[i], pool[j] = pool[j], pool[i]
            table.insert(result, pool[i])
        end
    end

    table.insert(global_result, result)
    return result
end
