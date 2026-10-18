import { createRemoteJWKSet, jwtVerify } from 'jose';
const jwks = new Map();
export async function authenticate(request, env) {
  const url = new URL(request.url);
  if (url.pathname === '/agent') {
    const value = request.headers.get('Authorization') || '';
    const expected = `Bearer ${env.AGENT_TOKEN || ''}`;
    const enc = new TextEncoder();
    const a = await crypto.subtle.digest('SHA-256', enc.encode(value));
    const b = await crypto.subtle.digest('SHA-256', enc.encode(expected));
    return env.AGENT_TOKEN && crypto.subtle.timingSafeEqual(a, b) ? { role: 'agent' } : null;
  }
  const token = request.headers.get('Cf-Access-Jwt-Assertion');
  if (!token || !env.ACCESS_TEAM_DOMAIN || !env.ACCESS_AUD || !env.ALLOWED_EMAIL) return null;
  try {
    const issuer = `https://${env.ACCESS_TEAM_DOMAIN}`;
    if (!jwks.has(issuer)) jwks.set(issuer, createRemoteJWKSet(new URL(`${issuer}/cdn-cgi/access/certs`)));
    const { payload } = await jwtVerify(token, jwks.get(issuer), { issuer, audience: env.ACCESS_AUD, algorithms: ['RS256'] });
    return payload.email === env.ALLOWED_EMAIL ? { role: 'web', expires: payload.exp } : null;
  } catch { return null; }
}
