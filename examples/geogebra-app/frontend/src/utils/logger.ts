import { WriteLog } from '../../wailsjs/go/main/App';

export function log(level: 'debug' | 'info' | 'warn' | 'error', message: string, data?: any) {
  const msg = data ? `${message} ${JSON.stringify(data)}` : message;
  WriteLog(level.toUpperCase(), msg).catch(() => {});
  if (level === 'error') {
    console.error(msg);
  } else {
    console.log(`[${level.toUpperCase()}] ${msg}`);
  }
}
