import { adminHeaders } from '../src/shared/auth/session';

function assertDeepEqual(actual: unknown, expected: unknown, message: string) {
  const normalize = (value: unknown) => JSON.stringify(value, (_key, nested) => {
    if (!nested || typeof nested !== 'object' || Array.isArray(nested)) return nested;
    return Object.fromEntries(Object.entries(nested).sort(([left], [right]) => left.localeCompare(right)));
  });
  if (normalize(actual) !== normalize(expected)) throw new Error(message);
}

assertDeepEqual(adminHeaders(''), {}, 'signed-out requests must not carry a bootstrap admin key');
assertDeepEqual(adminHeaders('session-token', { 'Content-Type': 'application/json' }), {
  Authorization: 'Bearer session-token',
  'Content-Type': 'application/json',
}, 'signed-in requests must use the bearer session token');

console.log('dashboard session header tests passed');
