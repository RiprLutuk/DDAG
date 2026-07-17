import test from 'node:test'
import assert from 'node:assert/strict'
import { joinGatewayPath } from './url-utils.js'

test('does not duplicate gateway prefix when endpoint already contains it', () => {
  assert.equal(
    joinGatewayPath('/api/v1', '/api/v1/brim/sites/ABC123'),
    '/api/v1/brim/sites/ABC123',
  )
})

test('joins a relative endpoint path to the gateway base', () => {
  assert.equal(joinGatewayPath('/api/v1', '/brim/sites/ABC123'), '/api/v1/brim/sites/ABC123')
})

test('supports absolute gateway base URLs', () => {
  assert.equal(
    joinGatewayPath('https://example.test/api/v1/', '/api/v1/brim/sites/ABC123'),
    'https://example.test/api/v1/brim/sites/ABC123',
  )
})
