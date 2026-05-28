import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { readFileSync } from 'node:fs';
import { fileURLToPath, URL } from 'node:url';
import { defineConfig, type Plugin } from 'vite';

// Dev-only plugin that mocks the backend endpoints the app normally gets from
// the Go server / bun CLI (`/api/diff`, `/diff`, `/meta`). It lets you run
// `bun run dev` and reproduce viewer bugs without rebuilding the Go binary.
// Point it at any patch file via DEV_DIFF_FILE, defaulting to dev/sample.diff.
function devDiffServer(): Plugin {
  const diffPath = fileURLToPath(
    new URL(
      process.env.DEV_DIFF_FILE ?? './dev/sample.diff',
      import.meta.url
    )
  );

  const readDiff = (): string => {
    try {
      return readFileSync(diffPath, 'utf8');
    } catch {
      return '';
    }
  };

  return {
    name: 'pipediffshub-dev-diff-server',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const url = (req.url ?? '').split('?')[0];
        if (url === '/api/diff' || url === '/diff') {
          res.setHeader('Cache-Control', 'no-store');
          res.setHeader('Content-Type', 'text/plain; charset=utf-8');
          res.end(readDiff());
          return;
        }
        if (url === '/meta') {
          res.setHeader('Cache-Control', 'no-store');
          res.setHeader('Content-Type', 'application/json; charset=utf-8');
          res.end(
            JSON.stringify({ sourceURL: 'dev://sample.diff', title: 'Dev Diff' })
          );
          return;
        }
        // Swallow heartbeat/close so the app's keepalive doesn't 404-spam.
        if (url === '/heartbeat' || url === '/close') {
          res.statusCode = 204;
          res.end();
          return;
        }
        next();
      });
    },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), devDiffServer()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    host: '127.0.0.1',
  },
});
