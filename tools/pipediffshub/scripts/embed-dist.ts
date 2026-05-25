import { readdir } from 'node:fs/promises';
import { join, relative, sep } from 'node:path';

const root = join(import.meta.dir, '..');
const distDir = join(root, 'dist');
const outputPath = join(root, 'src', 'embedded-dist.ts');

interface EmbeddedAsset {
  contentType: string;
  data: string;
}

async function main() {
  const assets: Record<string, EmbeddedAsset> = {};
  await addFiles(distDir, assets);

  const moduleText = `export interface EmbeddedAsset {\n  contentType: string;\n  data: string;\n}\n\nexport const EMBEDDED_DIST: Record<string, EmbeddedAsset> = ${JSON.stringify(
    assets
  )};\n`;
  await Bun.write(outputPath, moduleText);
}

async function addFiles(
  directory: string,
  assets: Record<string, EmbeddedAsset>
): Promise<void> {
  const entries = await readdir(directory, { withFileTypes: true });
  for (const entry of entries) {
    const filePath = join(directory, entry.name);
    if (entry.isDirectory()) {
      await addFiles(filePath, assets);
      continue;
    }
    if (!entry.isFile()) continue;

    const routePath = `/${relative(distDir, filePath).split(sep).join('/')}`;
    const bytes = await Bun.file(filePath).arrayBuffer();
    assets[routePath] = {
      contentType: getContentType(filePath),
      data: Buffer.from(bytes).toString('base64'),
    };
  }
}

function getContentType(path: string): string {
  if (path.endsWith('.html')) return 'text/html; charset=utf-8';
  if (path.endsWith('.css')) return 'text/css; charset=utf-8';
  if (path.endsWith('.js')) return 'text/javascript; charset=utf-8';
  if (path.endsWith('.json')) return 'application/json; charset=utf-8';
  if (path.endsWith('.svg')) return 'image/svg+xml';
  if (path.endsWith('.png')) return 'image/png';
  if (path.endsWith('.ico')) return 'image/x-icon';
  if (path.endsWith('.wasm')) return 'application/wasm';
  return 'application/octet-stream';
}

await main();
