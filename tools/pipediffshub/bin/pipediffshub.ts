#!/usr/bin/env bun

import { existsSync, realpathSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { Buffer } from 'node:buffer';

import { EMBEDDED_DIST } from '../src/embedded-dist';

const HEARTBEAT_TIMEOUT_MS = 2_000;
const CLOSE_GRACE_MS = 1_500;
const ANSI_ESCAPE_PATTERN = /[\u001B\u009B][[\]()#;?]*(?:(?:(?:[a-zA-Z\d]*(?:;[a-zA-Z\d]*)*)?\u0007)|(?:(?:\d{1,4}(?:;\d{0,4})*)?[\dA-PR-TZcf-nq-uy=><~]))/g;
const root = getProjectRoot();
const distDir = join(root, 'dist');
const indexPath = join(distDir, 'index.html');

function getProjectRoot(): string {
  const executableRoot = dirname(realpathSync(process.execPath));
  if (existsSync(join(executableRoot, 'dist', 'index.html'))) {
    return executableRoot;
  }

  return join(import.meta.dir, '..');
}

async function main() {
  const sourceURL = process.argv[2] ?? 'stdin://piped-diff';
  const rawDiff = await Bun.stdin.text();
  const diff = stripAnsi(rawDiff);
  let receivedHeartbeat = false;
  let lastHeartbeatAt = Date.now();
  let closeTimeout: Timer | undefined;
  if (diff.trim().length === 0) {
    console.error('pipediffshub: no diff received on stdin');
    process.exit(1);
  }

  debugLog(`stdin bytes: ${Buffer.byteLength(rawDiff)}`);
  debugLog(`normalized bytes: ${Buffer.byteLength(diff)}`);
  debugLog(`ansi detected: ${rawDiff !== diff}`);

  if (!hasEmbeddedDist() && !existsSync(indexPath)) {
    console.error(
      'pipediffshub: no embedded assets and dist/ is missing. Run `bun run build:all` first.'
    );
    process.exit(1);
  }

  let heartbeatCheck: Timer | undefined;
  const stopServer = (message: string) => {
    if (heartbeatCheck != null) {
      clearInterval(heartbeatCheck);
    }
    if (closeTimeout != null) {
      clearTimeout(closeTimeout);
    }
    console.error(message);
    server.stop(true);
    process.exit(0);
  };
  const server = Bun.serve({
    port: 0,
    hostname: '127.0.0.1',
    fetch(request) {
      const url = new URL(request.url);
      if (url.pathname === '/diff' || url.pathname === '/api/diff') {
        return new Response(diff, {
          headers: {
            'cache-control': 'no-store',
            'content-type': 'text/plain; charset=utf-8',
          },
        });
      }
      if (url.pathname === '/meta') {
        return Response.json({ sourceURL, title: '' }, { headers: { 'cache-control': 'no-store' } });
      }
      if (url.pathname === '/heartbeat') {
        receivedHeartbeat = true;
        lastHeartbeatAt = Date.now();
        if (closeTimeout != null) {
          clearTimeout(closeTimeout);
          closeTimeout = undefined;
        }
        return new Response(null, { status: 204 });
      }
      if (url.pathname === '/close') {
        if (closeTimeout == null) {
          closeTimeout = setTimeout(
            () => stopServer('pipediffshub: browser tab closed, stopping'),
            CLOSE_GRACE_MS
          );
        }
        return new Response(null, { status: 204 });
      }

      return serveStatic(url.pathname);
    },
  });

  heartbeatCheck = setInterval(() => {
    if (!receivedHeartbeat) return;
    if (Date.now() - lastHeartbeatAt < HEARTBEAT_TIMEOUT_MS) return;

    stopServer('pipediffshub: heartbeat stopped, stopping');
  }, 500);

  const browserURL = `http://${server.hostname}:${server.port}/`;
  if (process.env.PIPEDIFFSHUB_NO_OPEN !== '1') {
    await openBrowser(browserURL);
  }
  debugLog(`source: ${sourceURL}`);
  console.error(`pipediffshub: opened ${browserURL}`);
  console.error('pipediffshub: press Ctrl-C to stop');
}

function stripAnsi(value: string): string {
  return value.replace(ANSI_ESCAPE_PATTERN, '');
}

function debugLog(message: string) {
  if (process.env.PIPEDIFFSHUB_DEBUG !== '1') return;

  console.error(`pipediffshub debug: ${message}`);
}

function serveStatic(pathname: string): Response {
  const safePath = pathname === '/' ? '/index.html' : pathname;
  const embeddedAsset = EMBEDDED_DIST[safePath];
  if (embeddedAsset != null) {
    return new Response(Buffer.from(embeddedAsset.data, 'base64'), {
      headers: {
        'cache-control': 'no-store',
        'content-type': embeddedAsset.contentType,
      },
    });
  }

  const filePath = join(distDir, safePath);
  if (!filePath.startsWith(distDir)) {
    return new Response('Not found', { status: 404 });
  }

  const file = Bun.file(filePath);
  if (file.size > 0) {
    return new Response(file);
  }

  const embeddedIndex = EMBEDDED_DIST['/index.html'];
  if (embeddedIndex != null) {
    return new Response(Buffer.from(embeddedIndex.data, 'base64'), {
      headers: { 'content-type': embeddedIndex.contentType },
    });
  }

  return new Response(Bun.file(indexPath));
}

function hasEmbeddedDist(): boolean {
  return EMBEDDED_DIST['/index.html'] != null;
}

async function openBrowser(url: string): Promise<void> {
  const command =
    process.platform === 'darwin'
      ? 'open'
      : process.platform === 'win32'
        ? 'cmd'
        : 'xdg-open';
  const args = process.platform === 'win32' ? ['/c', 'start', '', url] : [url];
  const proc = Bun.spawn([command, ...args], {
    stdout: 'ignore',
    stderr: 'ignore',
  });
  await proc.exited;
}

await main();
