/**
 * Frontend logger — writes to ~/.pi-chat-app/logs/frontend-YYYY-MM-DD.log
 * via the backend WriteLog Wails binding, and also mirrors to the browser
 * console for development.
 *
 * Usage:
 *   import { log } from './utils/logger';
 *   log.info('message sent', { model });
 *   log.warn('empty reply detected');
 *   log.error('stream error', err);
 */

type LogLevel = 'DEBUG' | 'INFO' | 'WARN' | 'ERROR';

// WriteLog is injected by the Wails backend. It writes a line to the
// daily-rotated frontend log file. We declare it as a global to avoid
// a circular import with the wailsjs bindings.
declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          WriteLog?: (level: string, message: string) => Promise<void>;
        };
      };
    };
  }
}

function pad2(n: number): string {
  return n < 10 ? '0' + n : String(n);
}

function timestamp(): string {
  const d = new Date();
  return `${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}.${String(d.getMilliseconds()).padStart(3, '0')}`;
}

function emit(level: LogLevel, msg: string, extra?: unknown): void {
  const line = extra !== undefined ? `${msg} ${safeStringify(extra)}` : msg;
  const formatted = `${timestamp()} [${level}] ${line}`;

  // Mirror to console for dev convenience.
  switch (level) {
    case 'DEBUG': console.debug(formatted); break;
    case 'INFO':  console.info(formatted);  break;
    case 'WARN':  console.warn(formatted);  break;
    case 'ERROR': console.error(formatted); break;
  }

  // Send to backend file logger (fire-and-forget).
  try {
    window?.go?.main?.App?.WriteLog?.(level, line).catch(() => {});
  } catch {
    // window not available (SSR) — ignore.
  }
}

function safeStringify(v: unknown): string {
  if (v instanceof Error) return v.message + (v.stack ? '\n' + v.stack : '');
  try {
    return JSON.stringify(v);
  } catch {
    return String(v);
  }
}

export const log = {
  debug: (msg: string, extra?: unknown) => emit('DEBUG', msg, extra),
  info:  (msg: string, extra?: unknown) => emit('INFO', msg, extra),
  warn:  (msg: string, extra?: unknown) => emit('WARN', msg, extra),
  error: (msg: string, extra?: unknown) => emit('ERROR', msg, extra),
};
