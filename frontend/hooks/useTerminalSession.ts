import { useState, useCallback } from "react";
import { toast } from "react-hot-toast";

interface UseTerminalSessionReturn {
  connected: boolean;
  sessionId: string | undefined;
  podName: string | undefined;
  shouldDisconnect: boolean;
  error: string | null;
  handleConnect: (sessionId: string, podName?: string) => void;
  handleDisconnect: () => void;
  handleReconnect: () => void;
  handleError: (errorMessage: string) => void;
  clearError: () => void;
}

export function useTerminalSession(): UseTerminalSessionReturn {
  const [connected, setConnected] = useState(false);
  const [sessionId, setSessionId] = useState<string | undefined>();
  const [podName, setPodName] = useState<string | undefined>();
  const [shouldDisconnect, setShouldDisconnect] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleConnect = useCallback(
    (newSessionId: string, newPodName?: string) => {
      setSessionId(newSessionId);
      setPodName(newPodName);
      setConnected(true);
      setShouldDisconnect(false);
      setError(null);
      toast.success("Connected to terminal");
    },
    []
  );

  const handleDisconnect = useCallback(() => {
    setShouldDisconnect(true);
    setConnected(false);
    toast("Disconnected from terminal", { icon: "🔌" });
  }, []);

  const handleReconnect = useCallback(() => {
    setError(null);
    window.location.reload();
  }, []);

  const handleError = useCallback((errorMessage: string) => {
    setError(errorMessage);
    setConnected(false);
    toast.error(errorMessage);
  }, []);

  const clearError = useCallback(() => {
    setError(null);
  }, []);

  return {
    connected,
    sessionId,
    podName,
    shouldDisconnect,
    error,
    handleConnect,
    handleDisconnect,
    handleReconnect,
    handleError,
    clearError,
  };
}
