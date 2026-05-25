import { useEffect } from 'react';

const HEARTBEAT_INTERVAL_MS = 1_000;

export function useHeartbeat() {
  useEffect(() => {
    let stopped = false;

    const sendHeartbeat = () => {
      if (stopped) return;
      void fetch('/heartbeat', {
        body: '',
        cache: 'no-store',
        keepalive: true,
        method: 'POST',
      }).catch(() => {});
    };

    const notifyClose = () => {
      stopped = true;
      navigator.sendBeacon('/close', '');
    };

    sendHeartbeat();
    const interval = window.setInterval(sendHeartbeat, HEARTBEAT_INTERVAL_MS);
    window.addEventListener('pagehide', notifyClose);

    return () => {
      stopped = true;
      window.clearInterval(interval);
      window.removeEventListener('pagehide', notifyClose);
    };
  }, []);
}
