import React from 'react';
import { cn } from '@/lib/utils';

interface ToastProps {
  toasts: Array<{ id: string; message: string; type: string }>;
}

export default function Toast({ toasts }: ToastProps) {
  if (toasts.length === 0) return null;

  return (
    <div className="fixed bottom-4 right-4 z-[200] flex flex-col gap-2">
      {toasts.map((t) => (
        <div
          key={t.id}
          className={cn(
            'animate-slide-in rounded-md px-4 py-2.5 text-sm font-medium shadow-lg',
            t.type === 'success' && 'bg-success text-success-foreground',
            t.type === 'error' && 'bg-destructive text-destructive-foreground',
            t.type === 'info' && 'bg-primary text-primary-foreground'
          )}
        >
          {t.message}
        </div>
      ))}
    </div>
  );
}
