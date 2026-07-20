export interface ProviderField {
  key: string
  label: string
  secret?: boolean
  default?: string | boolean
  boolean?: boolean
}

const schemas: Record<string, ProviderField[]> = {
  'email:local': [],
  'email:smtp': [
    { key: 'address', label: 'SMTP 地址', default: 'smtp.example.com:587' },
    { key: 'host', label: '服务器域名', default: 'smtp.example.com' },
    { key: 'username', label: '用户名' },
    { key: 'password', label: '密码', secret: true },
    { key: 'from', label: '发件人地址' },
    { key: 'use_tls', label: '启用 STARTTLS', boolean: true, default: true }
  ],
  'sms:local': [],
  'sms:aliyun-sms': [
    { key: 'region', label: '地域', default: 'cn-hangzhou' },
    { key: 'endpoint', label: 'Endpoint', default: 'dysmsapi.aliyuncs.com' },
    { key: 'access_key_id', label: 'AccessKey ID' },
    { key: 'access_key_secret', label: 'AccessKey Secret', secret: true },
    { key: 'sign_name', label: '短信签名' },
    { key: 'template_code', label: '验证码模板 Code' }
  ],
  'storage:local': [],
  'storage:aliyun-oss': [
    { key: 'region', label: '地域', default: 'cn-hangzhou' },
    { key: 'endpoint', label: 'Endpoint（可选）' },
    { key: 'bucket', label: 'Bucket' },
    { key: 'access_key_id', label: 'AccessKey ID' },
    { key: 'access_key_secret', label: 'AccessKey Secret', secret: true },
    { key: 'security_token', label: 'STS Token（可选）', secret: true }
  ]
}

export function providerFields(providerType: string, providerName: string) {
  return schemas[`${providerType}:${providerName}`] ?? []
}

export function providerImplementations(providerType: string) {
  if (providerType === 'email') return ['local', 'smtp']
  if (providerType === 'sms') return ['local', 'aliyun-sms']
  if (providerType === 'storage') return ['local', 'aliyun-oss']
  return []
}

export function sanitizeProviderConfig(config: Record<string, unknown>) {
  return Object.fromEntries(
    Object.entries(config).filter(([key, value]) => {
      if (key.endsWith('_configured')) return false
      if (key.includes('secret') || key === 'password' || key === 'token') return value !== ''
      return true
    })
  )
}
