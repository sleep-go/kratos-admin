import { createHash } from 'node:crypto'
import { createConnection } from 'node:net'

import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

const password = process.env.E2E_ADMIN_PASSWORD ?? 'Admin@123456'
const redisHost = process.env.E2E_REDIS_HOST ?? '127.0.0.1'
const redisPort = Number(process.env.E2E_REDIS_PORT ?? '6379')
const apiBaseURL = process.env.E2E_API_BASE_URL ?? 'http://127.0.0.1:8000'

type CaptchaResponse = { captchaId: string }

function redisGet(key: string) {
  return new Promise<string>((resolve, reject) => {
    const socket = createConnection({ host: redisHost, port: redisPort })
    let output = ''
    socket.setTimeout(5_000)
    socket.on('connect', () => {
      socket.write(`*2\r\n$3\r\nGET\r\n$${Buffer.byteLength(key)}\r\n${key}\r\n`)
    })
    socket.on('data', (chunk) => {
      output += chunk.toString('utf8')
      const match = output.match(/^\$(\d+)\r\n([\s\S]*)\r\n$/)
      if (!match || match[2].length !== Number(match[1])) return
      socket.end()
      resolve(match[2])
    })
    socket.on('timeout', () => socket.destroy(new Error('读取 Redis 验证码超时')))
    socket.on('error', reject)
  })
}

function solveCaptcha(id: string, wantedHash: string) {
  for (let value = 0; value <= 99_999; value++) {
    const answer = String(value).padStart(5, '0')
    const actual = createHash('sha256').update(`${id}:${answer}`).digest('hex')
    if (actual === wantedHash) return answer
  }
  throw new Error('无法解出 E2E 图形验证码')
}

async function captchaAnswer(id: string) {
  const hash = await redisGet(`auth:captcha:${id}`)
  return solveCaptcha(id, hash)
}

async function loginThroughUI(page: Page) {
  const captchaResponse = page.waitForResponse(
    (response) => response.url().includes('/api/v1/auth/captcha') && response.ok()
  )
  await page.goto('/login')
  const captcha = (await (await captchaResponse).json()) as CaptchaResponse
  await page.getByRole('textbox', { name: '账号' }).fill('admin')
  await page
    .getByRole('textbox', { name: '图形验证码' })
    .fill(await captchaAnswer(captcha.captchaId))
  await page.getByRole('textbox', { name: '密码' }).fill(password)
  await page.getByRole('button', { name: '安全登录' }).click()
  await expect(page.getByRole('heading', { name: '工作台' })).toBeVisible()
}

async function createBackgroundSession(request: APIRequestContext) {
  const captchaResponse = await request.get(`${apiBaseURL}/api/v1/auth/captcha`)
  expect(captchaResponse.ok()).toBeTruthy()
  const captcha = (await captchaResponse.json()) as CaptchaResponse
  const response = await request.post(`${apiBaseURL}/api/v1/auth/login`, {
    data: {
      identifier: 'admin',
      password,
      captchaId: captcha.captchaId,
      captchaCode: await captchaAnswer(captcha.captchaId),
      deviceName: 'E2E 备用设备'
    }
  })
  expect(response.ok()).toBeTruthy()
}

async function openTenantDialog(page: Page) {
  const desktopSwitcher = page.getByTestId('tenant-switcher')
  if (await desktopSwitcher.isVisible()) {
    await desktopSwitcher.click()
  } else {
    await page.getByRole('button', { name: '打开导航菜单' }).click()
    await page.getByTestId('mobile-tenant-switcher').click()
  }
  await expect(page.getByRole('dialog', { name: '切换租户' })).toBeVisible()
}

async function ensureTenantAndFeatures(page: Page) {
  await page.goto('/platform/tenants')
  await expect(page.getByRole('heading', { name: '租户管理' })).toBeVisible()
  if ((await page.locator('.el-table__row').count()) === 0) {
    await page.getByRole('button', { name: '新建' }).click()
    const dialog = page.getByRole('dialog', { name: '新建租户管理' })
    await dialog.getByLabel('租户编码').fill('e2e')
    await dialog.getByLabel('租户名称').fill('E2E 验收租户')
    await dialog.getByRole('button', { name: '保存' }).click()
    await expect(
      page.getByText('E2E 验收租户', { exact: true }).filter({ visible: true }).first()
    ).toBeVisible()
  }

  await page.goto('/permission/tenant-features')
  await expect(page.getByRole('heading', { name: '租户功能授权' })).toBeVisible()
  const tree = page.getByRole('tree')
  await expect(tree).toBeVisible()
  const checkboxLabels = tree.locator('.el-checkbox')
  for (let index = 0; index < (await checkboxLabels.count()); index++) {
    const checkbox = checkboxLabels.nth(index)
    if (!(await checkbox.locator('input').isChecked())) await checkbox.click()
  }
  await page.getByRole('button', { name: '保存授权' }).click()
  await expect(page.getByText(/租户功能授权已更新/)).toBeVisible()
}

test('管理员核心链路：登录、租户、权限、日志、文件和会话', async ({ page, request }) => {
  await loginThroughUI(page)

  await openTenantDialog(page)
  await page.getByRole('button', { name: /平台管理.*跨租户治理视角/ }).click()
  await expect(page).toHaveURL(/\/$/)
  await expect(page.getByText('平台治理视角，展示真实业务数据与安全动态。')).toBeVisible()
  await ensureTenantAndFeatures(page)

  await openTenantDialog(page)
  await page
    .getByRole('button', { name: /租户 ID/ })
    .first()
    .click()
  await expect(page.getByRole('heading', { name: '工作台' })).toBeVisible()

  await page.goto('/permission/roles')
  await expect(page.getByRole('heading', { name: '角色授权' })).toBeVisible()
  const roleCode = 'e2e_acceptance'
  const roleButton = page.getByRole('button', { name: new RegExp(roleCode) }).first()
  if ((await roleButton.count()) === 0) {
    await page.getByRole('button', { name: '新建角色' }).click()
    const roleDialog = page.getByRole('dialog', { name: '新建角色' })
    await roleDialog.getByLabel('角色名称').fill('E2E 验收角色')
    await roleDialog.getByLabel('角色编码').fill(roleCode)
    await roleDialog.getByRole('button', { name: '创建' }).click()
  } else {
    await roleButton.click()
  }
  await expect(roleButton).toBeVisible()
  const filePermissions = page.locator('.resource-list article').filter({ hasText: 'files' })
  for (const label of ['查看', '下载']) {
    const checkbox = filePermissions.locator('.el-checkbox').filter({ hasText: label })
    if (!(await checkbox.locator('input').isChecked())) await checkbox.click()
  }
  await page.getByRole('button', { name: '保存并生效' }).click()
  await expect(page.getByText(/角色授权与数据范围已生效/)).toBeVisible()

  await createBackgroundSession(request)
  await page.goto('/account')
  await expect(page.getByRole('heading', { name: '个人中心' })).toBeVisible()
  await expect(page.getByText('当前设备')).toBeVisible()
  const revokeButton = page.getByRole('button', { name: '撤销' }).first()
  await expect(revokeButton).toBeVisible()
  await revokeButton.click()
  await page
    .getByRole('dialog', { name: '撤销设备会话' })
    .getByRole('button', { name: 'OK' })
    .click()
  await expect(page.getByText(/设备会话已撤销/)).toBeVisible()

  await page.goto('/logs/audit')
  await expect(page.getByRole('heading', { name: '操作审计' })).toBeVisible()
  await page.getByPlaceholder('输入关键词搜索').fill('roles')
  await page.getByRole('button', { name: '查询' }).click()
  await expect(page.locator('.table-panel')).toBeVisible()

  await page.goto('/files')
  await expect(page.getByRole('heading', { name: '文件管理' })).toBeVisible()
  const filename = `e2e-${Date.now()}.txt`
  await page.getByLabel('选择文件直传').setInputFiles({
    name: filename,
    mimeType: 'text/plain',
    buffer: Buffer.from('kratos admin e2e')
  })
  await expect(page.getByText(filename)).toBeVisible()
  await expect(page.getByText('可用').last()).toBeVisible()
})
