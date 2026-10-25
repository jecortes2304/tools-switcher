import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { readFile, mkdir } from 'node:fs/promises'
import { resolve, extname } from 'node:path'
import { chromium } from 'playwright'

const binding = await readFile(
  new URL(
    '../bindings/github.com/jecortes2304/tools-switcher/internal/desktop/service.ts',
    import.meta.url,
  ),
  'utf8',
)
const methods = new Map(
  [...binding.matchAll(/export function (\w+)\([^]*?\$Call.ByID\((\d+)/g)].map((m) => [
    Number(m[2]),
    m[1],
  ]),
)
const state = {
  settings: {
    theme: 'dark',
    vault: 'C:\\Users\\Developer\\.tools-switcher\\vault',
    startAtLogin: false,
    closeToTray: true,
    autoCheckUpdates: true,
  },
  repositories: [
    {
      id: 'temurin',
      name: 'Eclipse Temurin',
      kind: 'adoptium',
      source: 'https://api.adoptium.net/v3',
      enabled: true,
      count: 1,
      lastSync: '',
      error: '',
      tool: 'jdk',
    },
  ],
  installations: [
    {
      id: 'jdk21',
      name: 'Eclipse Temurin 21.0.9',
      version: '21.0.9',
      vendor: 'Eclipse Adoptium',
      repositoryId: 'temurin',
      path: 'C:\\Java\\temurin-21',
      managed: true,
      managedRoot: 'C:\\Java\\temurin-21',
      tool: 'jdk',
    },
    {
      id: 'jdk17',
      name: 'Eclipse Temurin 17.0.17',
      version: '17.0.17',
      vendor: 'Eclipse Adoptium',
      repositoryId: 'temurin',
      path: 'C:\\Java\\temurin-17',
      managed: false,
      managedRoot: '',
      tool: 'jdk',
    },
  ],
  downloads: [],
  activePath: 'C:\\Java\\temurin-21',
  resolvedJava: 'C:\\Java\\temurin-21\\bin\\java.exe',
  platform: 'windows',
  architecture: 'amd64',
}
let queued = []
const errors = []
const server = createServer(async (req, res) => {
  try {
    if (req.url.startsWith('/wails/runtime')) {
      const chunks = []
      for await (const chunk of req) chunks.push(chunk)
      const call = JSON.parse(Buffer.concat(chunks).toString())
      const method = methods.get(call.args?.methodID)
      const args = call.args?.args ?? []
      let result = null
      if (method === 'Snapshot') result = state
      else if (method === 'Version') result = '0.1.0'
      else if (method === 'Versions') result = [25, 21, 17, 11, 8]
      else if (method === 'Catalog')
        result = [
          {
            id: `package-${args[1]}`,
            repositoryId: args[0],
            name: `Eclipse Temurin ${args[1]}`,
            version: `${args[1]}.0.1`,
            os: 'windows',
            arch: 'amd64',
            size: 180000000,
          },
        ]
      else if (method === 'Queue') queued = args[0]
      else if (method === 'SaveSettings') state.settings = args[0]
      else if (method === 'Activate')
        state.activePath = state.installations.find((i) => i.id === args[0]).path
      else if (method === 'UpdateStatus')
        result = { status: 'idle', version: '', error: '', configured: false }
      else if (method === 'Variables') result = [{ name: 'JAVA_HOME', value: '' }]
      else if (method === 'AddRepository')
        state.repositories.push({
          ...state.repositories[0],
          id: 'custom',
          name: args[0] || 'Mi repositorio',
          source: args[1],
        })
      else if (method === 'RemoveInstallation')
        state.installations = state.installations.filter((i) => i.id !== args[0])
      res.writeHead(200, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify(result))
      return
    }
    const file = resolve(
      new URL('../dist', import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, '$1'),
      req.url === '/' ? 'index.html' : req.url.slice(1),
    )
    const content = await readFile(file)
    res.writeHead(200, {
      'Content-Type':
        { '.html': 'text/html', '.js': 'application/javascript', '.css': 'text/css' }[
          extname(file)
        ] ?? 'application/octet-stream',
    })
    res.end(content)
  } catch (error) {
    res.writeHead(500)
    res.end(String(error))
  }
})
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
const browser = await chromium.launch(
  process.env.PLAYWRIGHT_CHANNEL ? { channel: process.env.PLAYWRIGHT_CHANNEL } : {},
)
try {
  const page = await browser.newPage({
    viewport: { width: 400, height: 620 },
    deviceScaleFactor: 1,
  })
  page.on('pageerror', (error) => errors.push(error.message))
  await page.goto(`http://127.0.0.1:${server.address().port}`)
  await page.getByRole('heading', { name: 'Java Development Kits' }).waitFor()
  assert.equal(await page.locator('.jdk-card').count(), 2)
  await mkdir(new URL('../../artifacts/ui', import.meta.url), { recursive: true })
  const screenshot = (name) =>
    page.screenshot({
      animations: 'disabled',
      path: new URL(`../../artifacts/ui/${name}.png`, import.meta.url).pathname.replace(
        /^\/([A-Za-z]:)/,
        '$1',
      ),
    })
  await screenshot('jdks-dark')
  await page.getByRole('textbox', { name: 'Filtrar JDKs' }).fill('17.0')
  assert.equal(await page.locator('.jdk-card').count(), 1)
  await page.getByRole('button', { name: 'Activar', exact: true }).click()
  assert.equal(state.activePath, 'C:\\Java\\temurin-17')
  await page.getByRole('textbox', { name: 'Filtrar JDKs' }).fill('')
  await page.getByRole('button', { name: '+ Instalar' }).click()
  await page.getByText('Eclipse Temurin 21', { exact: true }).waitFor()
  await page.locator('.package-row input').check()
  await page.getByLabel('Versión de Java').selectOption('17')
  await page.getByText('Eclipse Temurin 17', { exact: true }).waitFor()
  await page.locator('.package-row input').check()
  await page.getByRole('button', { name: 'Instalar selección (2)' }).click()
  await page.locator('dialog').waitFor({ state: 'detached' })
  assert.deepEqual(queued.sort(), ['package-17', 'package-21'])
  await page.getByRole('button', { name: 'Repositorios', exact: true }).click()
  await page.getByRole('heading', { name: 'Añadir un catálogo' }).waitFor()
  await screenshot('repositories-dark')
  await page.getByLabel('Nombre opcional').fill('Mi espejo')
  await page.getByRole('button', { name: '+ Validar y añadir' }).click()
  await page.getByRole('heading', { name: 'Mi espejo' }).waitFor()
  await page.getByRole('button', { name: 'Ajustes', exact: true }).click()
  await page.getByRole('heading', { name: 'Apariencia' }).waitFor()
  await screenshot('settings-dark')
  await page.getByRole('button', { name: '☀ Frost' }).click()
  await page.waitForFunction(() => document.documentElement.dataset.theme === 'light')
  await screenshot('settings-light')
  await page.getByRole('button', { name: 'JDKs', exact: true }).click()
  await page.getByRole('button', { name: 'Opciones de Eclipse Temurin 17.0.17' }).click()
  await page.getByRole('button', { name: '× Desvincular JDK…' }).click()
  await page.getByRole('button', { name: 'Confirmar', exact: true }).click()
  await page.locator('dialog').waitFor({ state: 'detached' })
  assert.equal(state.installations.length, 1)
  await page.setViewportSize({ width: 360, height: 500 })
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false)
  assert.deepEqual(errors, [])
  console.log(
    'UI passed: navigation, filters, activation, multi-version queue, repositories, themes, confirmation, compact layout',
  )
} finally {
  await browser.close()
  server.close()
}
