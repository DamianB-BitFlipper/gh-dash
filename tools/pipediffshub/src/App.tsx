import { ReviewUI } from './diffshub-copied/ReviewUI';
import { useHeartbeat } from './useHeartbeat';
import { useEffect } from 'react';

const DEFAULT_TITLE = 'PipeDiffsHub';

export function App() {
  useHeartbeat();
  useDocumentTitle();

  return (
    <div className="flex h-dvh flex-col">
      <ReviewUI path="/piped-diff" />
    </div>
  );
}

function useDocumentTitle() {
  useEffect(() => {
    const controller = new AbortController();

    async function updateTitle() {
      try {
        const response = await fetch('/meta', {
          cache: 'no-store',
          signal: controller.signal,
        });
        if (!response.ok) return;

        const meta = (await response.json()) as { title?: unknown };
        const title = typeof meta.title === 'string' ? meta.title.trim() : '';
        document.title = title === '' ? DEFAULT_TITLE : title;
      } catch (error) {
        if (!controller.signal.aborted) {
          document.title = DEFAULT_TITLE;
        }
      }
    }

    updateTitle();
    return () => controller.abort();
  }, []);
}
