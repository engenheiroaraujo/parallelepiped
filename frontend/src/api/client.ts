// Error contract shared with internal/platform/bridge: a rejected bound call
// carries a JSON envelope {code, message, details} as its error string.

export type ErrorCode = 'invalid_input' | 'not_found' | 'conflict' | 'unavailable' | 'internal';

export class AppError extends Error {
  constructor(
    readonly code: ErrorCode,
    message: string,
    readonly details: Record<string, string> = {},
  ) {
    super(message);
  }
}

const userMessages: Record<ErrorCode, string> = {
  invalid_input: 'Dados inválidos.',
  not_found: 'Item não encontrado.',
  conflict: 'Já existe uma nota com esse caminho.',
  unavailable: 'Nenhum cofre aberto.',
  internal: 'Erro inesperado. Consulte o log do aplicativo.',
};

function toAppError(raw: unknown): AppError {
  if (raw instanceof AppError) return raw;
  const text = raw instanceof Error ? raw.message : String(raw);
  try {
    const envelope = JSON.parse(text) as { code?: ErrorCode; message?: string; details?: Record<string, string> };
    if (envelope.code) return new AppError(envelope.code, envelope.message ?? '', envelope.details ?? {});
  } catch {
    // Not an envelope: an error raised by Wails itself.
  }
  return new AppError('internal', text);
}

/** Awaits a bound Go call, normalising any rejection into an AppError. */
export async function call<T>(promise: Promise<T>): Promise<T> {
  try {
    return await promise;
  } catch (raw) {
    throw toAppError(raw);
  }
}

/** Text to show the user for any error. Field details are already in Portuguese. */
export function describeError(err: unknown): string {
  const appError = toAppError(err);
  const details = Object.values(appError.details);
  if (details.length > 0) return details.join(' ');
  return userMessages[appError.code] ?? userMessages.internal;
}
