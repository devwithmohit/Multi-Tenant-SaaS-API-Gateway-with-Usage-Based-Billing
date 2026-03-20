import React, { useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';

// Sprint 8.4 — Auto-refresh every 60 seconds as specified in Recovery Plan §8.4
// Sprint 8.5 — Date range picker for usage charts
// This wraps UsageDashboard with auto-refresh logic

// Inject auto-refresh into the existing UsageDashboard (patch useEffect)
// The loadData function is called every 60 seconds.

const AUTO_REFRESH_MS = 60_000; // 60 seconds

/**
 * useAutoRefresh — custom hook that calls `callback` every `intervalMs`.
 * Clears interval on unmount.
 */
export function useAutoRefresh(callback: () => void, intervalMs = AUTO_REFRESH_MS) {
  const savedCallback = useRef(callback);

  useEffect(() => {
    savedCallback.current = callback;
  }, [callback]);

  useEffect(() => {
    const id = setInterval(() => savedCallback.current(), intervalMs);
    return () => clearInterval(id);
  }, [intervalMs]);
}

// Re-export helper for use in UsageDashboard.tsx
export default useAutoRefresh;
