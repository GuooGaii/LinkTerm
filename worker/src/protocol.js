export const MAX_MESSAGE = 96 * 1024;
export function validMessage(role, msg) {
  if (!msg || typeof msg !== 'object') return false;
  if (role === 'agent') {
    if (msg.type === 'output') return typeof msg.data === 'string' && /^[A-Za-z0-9+/]*={0,2}$/.test(msg.data) && typeof msg.epoch === 'string';
    return msg.type === 'system' && ['ready', 'error'].includes(msg.data?.cmd) && typeof msg.epoch === 'string';
  }
  if (msg.type === 'ack') return typeof msg.epoch === 'string';
  if (msg.type === 'input') return typeof msg.data === 'string' && msg.data.length <= 16384 && typeof msg.epoch === 'string';
  if (msg.type !== 'system') return false;
  const { cmd, args } = msg.data || {};
  if (cmd === 'attach' || cmd === 'resize') return args && Number.isInteger(args.cols) && Number.isInteger(args.rows) && args.cols >= 2 && args.cols <= 500 && args.rows >= 2 && args.rows <= 200 && typeof msg.epoch === 'string';
  return false;
}
