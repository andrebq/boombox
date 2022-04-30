local res = require('ctx').res
local req = require('ctx').req
local result = tonumber(req:param("a")) + tonumber(req:param("b"))
res:write_body('Result: '.. result)
