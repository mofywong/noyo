const { writeFileSync } = require('node:fs')
const { resolve } = require('node:path')
const upstream = require('../UPSTREAM.json')
writeFileSync(resolve(__dirname, '../app/dist/.noyo-build.json'), JSON.stringify({ buildId: upstream.buildId, upstreamSha: upstream.commit, base: process.env.NOYO_SCREEN_BASE || `/screen-editor/${upstream.buildId}/` }, null, 2) + '\n')
