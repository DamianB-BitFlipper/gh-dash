import { ReviewUI } from './diffshub-copied/ReviewUI';
import { useHeartbeat } from './useHeartbeat';

export function App() {
  useHeartbeat();

  return (
    <div className="flex h-dvh flex-col">
      <ReviewUI path="/piped-diff" />
    </div>
  );
}
