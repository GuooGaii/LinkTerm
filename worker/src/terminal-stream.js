// Preserve bytes: UTF-8 characters and ANSI escape sequences may span PTY reads.
export function decodeOutput(base64) {
  return Uint8Array.from(atob(base64), c => c.charCodeAt(0));
}
export function sameSize(a, b) { return a?.cols === b?.cols && a?.rows === b?.rows; }
