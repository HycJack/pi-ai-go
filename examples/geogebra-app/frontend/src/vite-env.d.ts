/// <reference types="vite/client" />

declare module 'react-katex' {
  import { ComponentType } from 'react';
  interface MathProps {
    math: string;
    block?: boolean;
    errorColor?: string;
    renderError?: (error: Error | string) => React.ReactNode;
    settings?: Record<string, unknown>;
  }
  export const InlineMath: ComponentType<MathProps>;
  export const BlockMath: ComponentType<MathProps>;
}
