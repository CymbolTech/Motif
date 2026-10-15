// Signed-distance raymarcher for the engine core: a glazed gyroid lattice
// clipped to a sphere (the cage a 3D printer builds) around a liquid orange
// heart that lights the cage from inside. Each processed event arrives as a
// droplet that melts into the heart and sends a ring of light out through the
// lattice; each achievement makes the heart larger and brighter.

export const VERTEX = `#version 300 es
in vec2 aPosition;
void main() {
  gl_Position = vec4(aPosition, 0.0, 1.0);
}`;

export const FRAGMENT = `#version 300 es
precision highp float;
out vec4 outColor;

uniform vec2 uRes;
uniform float uTime;
uniform mat3 uRot;
uniform float uCore;
uniform float uFlash;
uniform vec4 uDrop;
uniform float uRipple;
uniform float uChrome;

const vec3 GLAZE = vec3(0.86, 0.85, 0.82);
const vec3 INK = vec3(0.114, 0.114, 0.106);
const vec3 SIGNAL = vec3(1.0, 0.239, 0.0);
const vec3 EMBER = vec3(1.0, 0.42, 0.12);
const float FAR = 8.0;
const float FREQ = 5.4;

float smin(float a, float b, float k) {
  float h = clamp(0.5 + 0.5 * (b - a) / k, 0.0, 1.0);
  return mix(b, a, h) - k * h * (1.0 - h);
}

float gyroid(vec3 p) {
  return dot(sin(p), cos(p.yzx));
}

// Distance bound for the gyroid sheet. The 0.6 factor keeps the field
// conservative so the march never steps through a thin wall.
float gyroidDist(vec3 p) {
  return abs(gyroid(p)) * 0.6;
}

// The cage: a gyroid shell clipped by a sphere, hollow inside, breathing slowly.
float cageDist(vec3 q) {
  float sphere = length(q) - 1.0;
  float wall = 0.028 + 0.005 * sin(uTime * 0.6);
  float shell = gyroidDist(q * FREQ + vec3(0.0, uTime * 0.1, 0.0)) / FREQ - wall;
  float hollow = 0.6 - length(q);
  return max(max(sphere, shell), hollow);
}

float heartDist(vec3 q) {
  float wobble = sin(q.x * 6.0 + uTime * 1.1) * sin(q.y * 6.0 + uTime * 0.9) * sin(q.z * 6.0 + uTime * 0.7);
  return length(q) - uCore - 0.014 * wobble;
}

float dropDist(vec3 p) {
  return uDrop.w > 0.0 ? length(p - uDrop.xyz) - uDrop.w : FAR;
}

float liquidDist(vec3 p, vec3 q) {
  return smin(heartDist(q), dropDist(p), 0.22);
}

float map(vec3 p) {
  vec3 q = uRot * p;
  return min(cageDist(q), liquidDist(p, q));
}

vec3 calcNormal(vec3 p, float t) {
  const vec2 k = vec2(1.0, -1.0);
  float e = 0.0008 * t + 0.001;
  return normalize(k.xyy * map(p + k.xyy * e) + k.yyx * map(p + k.yyx * e) + k.yxy * map(p + k.yxy * e) + k.xxx * map(p + k.xxx * e));
}

float calcAO(vec3 p, vec3 n) {
  float occ = 0.0;
  float sca = 1.0;
  for (int i = 0; i < 5; i++) {
    float h = 0.015 + 0.06 * float(i);
    occ += (h - map(p + h * n)) * sca;
    sca *= 0.75;
  }
  return clamp(1.0 - 3.0 * occ, 0.0, 1.0);
}

float hash(vec2 p) {
  return fract(sin(dot(p, vec2(12.9898, 78.233))) * 43758.5453);
}

// Soft shadow with a per-pixel jittered start and the previous-step estimate,
// so the penumbra is smooth instead of quantised into contour rings.
float softShadow(vec3 ro, vec3 rd) {
  float res = 1.0;
  float ph = 1e3;
  float t = 0.02 + 0.02 * hash(gl_FragCoord.xy);
  for (int i = 0; i < 32; i++) {
    float h = map(ro + rd * t);
    float y = h * h / (2.0 * ph);
    float d = sqrt(max(h * h - y * y, 0.0));
    res = min(res, 10.0 * d / max(1e-4, t - y));
    ph = h;
    t += clamp(h, 0.01, 0.12);
    if (res < 0.005 || t > 2.5) break;
  }
  return clamp(res, 0.0, 1.0);
}

// A photo studio: bright top softbox, a long strip on the left, an orange gel on the right.
vec3 studio(vec3 r) {
  vec3 top = vec3(0.96, 0.955, 0.94);
  vec3 horizon = vec3(0.7, 0.69, 0.66);
  vec3 floorCol = vec3(0.12, 0.12, 0.115);
  vec3 col = r.y > 0.0 ? mix(horizon, top, smoothstep(0.0, 0.8, r.y)) : mix(horizon, floorCol, smoothstep(0.0, 0.45, -r.y));
  col += 1.4 * smoothstep(0.9, 0.985, dot(r, normalize(vec3(-0.6, 0.75, 0.35))));
  col += 0.9 * smoothstep(0.985, 0.998, dot(r, normalize(vec3(-0.95, 0.1, 0.2))));
  col += SIGNAL * 0.9 * smoothstep(0.965, 0.995, dot(r, normalize(vec3(0.85, 0.05, 0.5))));
  return col;
}

vec3 shade(vec3 p, vec3 rd, float t) {
  vec3 n = calcNormal(p, t);
  vec3 q = uRot * p;
  float cage = cageDist(q);
  float liquidD = liquidDist(p, q);
  float liquid = smoothstep(0.015, -0.015, liquidD - cage);
  float radius = length(q);
  float depth = clamp((radius - 0.6) / 0.4, 0.0, 1.0);

  vec3 key = normalize(vec3(-0.5, 0.85, 0.55));
  vec3 rim = normalize(vec3(0.75, 0.25, -0.65));
  vec3 h = normalize(key - rd);
  vec3 r = reflect(rd, n);
  float ndv = max(dot(n, -rd), 0.0);
  float fre = 0.04 + 0.96 * pow(1.0 - ndv, 5.0);
  float dif = clamp(dot(n, key), 0.0, 1.0);
  float sha = softShadow(p + n * 0.01, key);
  float ao = calcAO(p, n);
  float sky = 0.5 + 0.5 * n.y;

  // The heart is a point light at the centre of the cage.
  vec3 toHeart = -normalize(p);
  float heartPower = (0.7 + 1.2 * (uCore - 0.3)) * (1.0 + 0.8 * uFlash);
  float falloff = heartPower / (1.0 + 9.0 * max(length(p) - uCore, 0.0) * max(length(p) - uCore, 0.0));
  float facing = max(dot(n, toHeart), 0.0);
  // Thin walls let the heart's light through, like a lantern.
  float thickness = clamp(-map(p - n * 0.06) / 0.06, 0.0, 1.0);
  float through = (1.0 - thickness) * max(dot(-n, toHeart), 0.0) * 0.35;
  // A ring of light that runs outward through the lattice when a droplet lands.
  float front = uCore + uRipple * 0.62;
  float ripple = uRipple > 0.0 ? exp(-pow((radius - front) / 0.06, 2.0)) * (1.0 - uRipple) : 0.0;

  // Glazed ceramic lattice.
  vec3 glaze = GLAZE * (0.18 + 0.2 * sky + 0.82 * dif * sha) * (0.45 + 0.55 * ao);
  glaze += vec3(1.0, 0.97, 0.92) * pow(max(dot(n, h), 0.0), 70.0) * 0.45 * sha;
  glaze += studio(r) * fre * 0.55 * ao;
  glaze += vec3(0.92, 0.93, 0.95) * pow(1.0 - ndv, 2.5) * max(dot(n, rim), 0.0) * 0.55;
  glaze += EMBER * (facing + through) * falloff * (0.35 + 0.65 * ao);
  glaze += SIGNAL * ripple * 1.2;

  // Chrome finish: the same lights, mirrored.
  vec3 film = 0.5 + 0.5 * cos(6.2831 * (pow(1.0 - ndv, 3.0) * 1.2 + vec3(0.0, 0.33, 0.67)) + dot(n, vec3(1.0, 0.5, 0.2)) * 2.0);
  vec3 chrome = studio(r) * mix(vec3(0.95), film, 0.18) * (0.5 + 0.5 * ao) * mix(0.65, 1.0, depth);
  chrome += EMBER * facing * falloff * 0.8 + SIGNAL * ripple;

  // Liquid heart: saturated body, studio reflections by fresnel, a hot core.
  float heartFre = pow(1.0 - ndv, 2.0);
  vec3 heartCol = SIGNAL * (0.62 + 0.45 * dif) * (0.8 + 0.2 * ao);
  heartCol += EMBER * pow(ndv, 3.0) * 0.45 * (1.0 + uFlash);
  heartCol = mix(heartCol, studio(r) * vec3(1.0, 0.58, 0.4), heartFre * 0.5);
  heartCol += pow(max(dot(n, h), 0.0), 150.0) * 1.5;

  vec3 lattice = mix(glaze, chrome, uChrome);
  return mix(lattice, heartCol, liquid);
}

void main() {
  vec2 uv = (gl_FragCoord.xy - 0.5 * uRes) / uRes.y;
  vec3 ro = vec3(0.0, 0.05, 3.4);
  vec3 rd = normalize(vec3(uv, -1.45));

  float t = 0.0;
  float hit = -1.0;
  float bestRatio = 1e3;
  float bestT = 0.0;
  // Skip straight to the bounding sphere, grown to include a droplet in flight.
  float bound = uDrop.w > 0.0 ? max(1.1, length(uDrop.xyz) + uDrop.w + 0.05) : 1.1;
  float b = dot(ro, rd);
  float c = dot(ro, ro) - bound * bound;
  float disc = b * b - c;
  if (disc > 0.0) {
    t = max(0.0, -b - sqrt(disc));
    float tFar = -b + sqrt(disc);
    float previousT = t;
    for (int i = 0; i < 160; i++) {
      float d = map(ro + rd * t);
      if (d < 0.0006 * t) {
        // Bisect between the last two samples to land exactly on the surface.
        float lo = previousT;
        float hi = t;
        for (int j = 0; j < 3; j++) {
          float mid = 0.5 * (lo + hi);
          if (map(ro + rd * mid) < 0.0006 * mid) hi = mid;
          else lo = mid;
        }
        hit = hi;
        break;
      }
      if (d / t < bestRatio) {
        bestRatio = d / t;
        bestT = t;
      }
      previousT = t;
      t += d * 0.7;
      if (t > tFar) break;
    }
  }

  if (hit > 0.0) {
    outColor = vec4(shade(ro + rd * hit, rd, hit), 1.0);
    return;
  }

  float shadow = 0.0;
  if (rd.y < 0.0) {
    float ft = (-1.15 - ro.y) / rd.y;
    vec3 fp = ro + rd * ft;
    shadow = 0.22 * (1.0 - smoothstep(0.05, 1.1, length(fp.xz * vec2(1.0, 1.3))));
    // Fade before the canvas edge so the shadow never ends in a hard line.
    shadow *= smoothstep(-0.5, -0.4, uv.y);
  }
  vec4 ground = vec4(INK * shadow, shadow);
  float pixel = 1.0 / (1.45 * uRes.y);
  float coverage = 1.0 - smoothstep(0.0, pixel * 1.5, bestRatio);
  if (coverage > 0.0 && bestT > 0.0) {
    vec3 edge = shade(ro + rd * bestT, rd, bestT);
    outColor = vec4(edge * coverage, coverage) + ground * (1.0 - coverage);
    return;
  }
  outColor = ground;
}`;
