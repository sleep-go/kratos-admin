import { describe, expect, it } from 'vitest'

import { providerFields, sanitizeProviderConfig } from './providerSchemas'

describe('providerSchemas', () => {
  it('provides the required Aliyun SMS fields without exposing stored secrets', () => {
    const fields = providerFields('sms', 'aliyun-sms')
    expect(fields.map((field) => field.key)).toEqual([
      'region',
      'endpoint',
      'access_key_id',
      'access_key_secret',
      'sign_name',
      'template_code'
    ])
    expect(fields.find((field) => field.key === 'access_key_secret')?.secret).toBe(true)
  })

  it('omits blank secret fields so updates keep the encrypted value', () => {
    expect(
      sanitizeProviderConfig({
        endpoint: 'dysmsapi.aliyuncs.com',
        access_key_secret: '',
        access_key_secret_configured: true
      })
    ).toEqual({ endpoint: 'dysmsapi.aliyuncs.com' })
  })
})
