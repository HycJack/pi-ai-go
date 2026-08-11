import React from 'react';

interface ToastProps {
  toasts: Array<{ id: string; message: string; type: string }>;
}

export default function Toast({ toasts }: ToastProps) {
  if (toasts.length === 0) return null;
  return (
    <div className="toast-container">
      {toasts.map(t => (
        <div key={t.id} className={`toast ${t.type}`}>{t.message}</div>
      ))}
    </div>
  );
}
