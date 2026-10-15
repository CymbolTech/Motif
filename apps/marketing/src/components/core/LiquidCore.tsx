'use client';

import { useEffect, useRef, useState } from 'react';
import { getSnapshot, subscribeGame } from '@/game/store';
import { FRAGMENT, VERTEX } from './shader';

// The droplet enters from the trigger's side, arcs up in front of the cage,
// then dives through an opening into the heart.
const DROP_START = { x: 0.72, y: -0.52, z: 1.35 };
const DROP_BEND = { x: 0.82, y: 0.55, z: 1.5 };
const DROP_SECONDS = 1.25;
const RIPPLE_SECONDS = 1.4;
const HEART_MIN = 0.36;
const HEART_MAX = 0.54;
const HEART_PER_ACHIEVEMENT = 0.012;

const easeInOutCubic = (t: number) => (t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2);
const smoothstep = (a: number, b: number, t: number) => {
  const x = Math.min(1, Math.max(0, (t - a) / (b - a)));
  return x * x * (3 - 2 * x);
};

function dropAt(progress: number) {
  const t = easeInOutCubic(progress);
  const a = (1 - t) * (1 - t);
  const b = 2 * (1 - t) * t;
  return {
    x: a * DROP_START.x + b * DROP_BEND.x,
    y: a * DROP_START.y + b * DROP_BEND.y,
    z: a * DROP_START.z + b * DROP_BEND.z,
    radius: 0.09 * smoothstep(0, 0.12, progress) * (1 - smoothstep(0.7, 1, progress)),
  };
}

function rotation(yaw: number, pitch: number) {
  const cy = Math.cos(yaw);
  const sy = Math.sin(yaw);
  const cp = Math.cos(pitch);
  const sp = Math.sin(pitch);
  // Column-major Ry(yaw) * Rx(pitch).
  return new Float32Array([cy, 0, -sy, sy * sp, cp, cy * sp, sy * cp, -sp, cy * cp]);
}

function compile(gl: WebGL2RenderingContext, type: number, source: string) {
  const shader = gl.createShader(type);
  if (!shader) return null;
  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    console.warn(gl.getShaderInfoLog(shader));
    gl.deleteShader(shader);
    return null;
  }
  return shader;
}

function heartFor(achievements: number) {
  return Math.min(HEART_MAX, HEART_MIN + achievements * HEART_PER_ACHIEVEMENT);
}

/** The engine core: a gyroid cage whose liquid heart absorbs every event and grows with every achievement. */
export function LiquidCore({ className, label }: { className?: string; label: string }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [offline, setOffline] = useState(false);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const gl = canvas.getContext('webgl2', { antialias: false, premultipliedAlpha: true, alpha: true, powerPreference: 'high-performance' });
    if (!gl) {
      setOffline(true);
      return;
    }

    const vertex = compile(gl, gl.VERTEX_SHADER, VERTEX);
    const fragment = compile(gl, gl.FRAGMENT_SHADER, FRAGMENT);
    const program = gl.createProgram();
    if (!vertex || !fragment || !program) {
      setOffline(true);
      return;
    }
    gl.attachShader(program, vertex);
    gl.attachShader(program, fragment);
    gl.linkProgram(program);
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
      setOffline(true);
      return;
    }
    gl.useProgram(program);

    const buffer = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
    const position = gl.getAttribLocation(program, 'aPosition');
    gl.enableVertexAttribArray(position);
    gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0);

    const uniform = (name: string) => gl.getUniformLocation(program, name);
    const u = {
      res: uniform('uRes'),
      time: uniform('uTime'),
      rot: uniform('uRot'),
      core: uniform('uCore'),
      flash: uniform('uFlash'),
      drop: uniform('uDrop'),
      ripple: uniform('uRipple'),
      chrome: uniform('uChrome'),
    };

    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const coarse = window.matchMedia('(pointer: coarse)').matches;
    const dpr = window.devicePixelRatio || 1;
    const scale = coarse ? 1 : Math.min(dpr, 1.5);

    let heart = HEART_MIN;
    let heartTarget = HEART_MIN;
    let yaw = 0.5;
    let pitch = -0.3;
    let spin = 0;
    let dragging = false;
    let last = { x: 0, y: 0 };
    let drop = { progress: 1, active: false };
    let queued = 0;
    let ripple = 0;
    let flash = 0;
    let chrome = 0;
    let clock = 1;
    let previous = performance.now();
    let frame = 0;
    let visible = true;
    let seenPulse = getSnapshot().pulse;
    let seenAchievements = 0;
    let synced = false;

    const resize = () => {
      const width = Math.max(1, Math.round(canvas.clientWidth * scale));
      const height = Math.max(1, Math.round(canvas.clientHeight * scale));
      if (canvas.width !== width || canvas.height !== height) {
        canvas.width = width;
        canvas.height = height;
      }
      gl.viewport(0, 0, canvas.width, canvas.height);
    };

    const render = () => {
      resize();
      const droplet = dropAt(drop.progress);
      gl.uniform2f(u.res, canvas.width, canvas.height);
      gl.uniform1f(u.time, clock);
      gl.uniformMatrix3fv(u.rot, false, rotation(yaw, pitch));
      gl.uniform1f(u.core, heart);
      gl.uniform1f(u.flash, flash);
      gl.uniform4f(u.drop, droplet.x, droplet.y, droplet.z, drop.active ? droplet.radius : 0);
      gl.uniform1f(u.ripple, ripple);
      gl.uniform1f(u.chrome, chrome);
      gl.drawArrays(gl.TRIANGLES, 0, 3);
    };

    const step = (now: number) => {
      frame = 0;
      const dt = Math.min(0.05, (now - previous) / 1000);
      previous = now;
      if (!reduced) {
        clock += dt;
        if (!dragging) {
          yaw += (0.14 + spin) * dt;
          spin *= 0.94;
        }
      }
      if (drop.active) {
        drop.progress = Math.min(1, drop.progress + dt / DROP_SECONDS);
        // As the droplet finishes melting in, a ring of light starts out through the lattice.
        if (drop.progress > 0.82 && ripple === 0) ripple = 0.001;
        if (drop.progress >= 1) {
          drop.active = queued > 0;
          drop.progress = 0;
          queued = Math.max(0, queued - 1);
        }
      }
      if (ripple > 0) {
        ripple += dt / RIPPLE_SECONDS;
        if (ripple >= 1) ripple = 0;
      }
      flash = Math.max(0, flash - dt * 0.7);
      // Critically damped approach: the heart swells without overshooting.
      heart += (heartTarget - heart) * (1 - Math.exp(-dt * 2.6));
      const chromeTarget = getSnapshot().run.unlocks['chrome-core'] ? 1 : 0;
      chrome += (chromeTarget - chrome) * Math.min(1, dt * 3);
      render();
      const settling = drop.active || ripple > 0 || flash > 0 || Math.abs(chromeTarget - chrome) > 0.002 || Math.abs(heartTarget - heart) > 0.001;
      if (visible && (!reduced || settling || dragging)) frame = requestAnimationFrame(step);
    };

    const wake = () => {
      if (!frame && visible) {
        previous = performance.now();
        frame = requestAnimationFrame(step);
      }
    };

    const onStore = () => {
      const snapshot = getSnapshot();
      if (!snapshot.hydrated) return;
      const achievements = Object.keys(snapshot.run.unlocked).length;
      if (!synced) {
        synced = true;
        seenPulse = snapshot.pulse;
        seenAchievements = achievements;
        heart = heartTarget = heartFor(achievements);
        wake();
        return;
      }
      if (snapshot.pulse !== seenPulse) {
        const arrivals = Math.max(1, snapshot.pulse - seenPulse);
        seenPulse = snapshot.pulse;
        if (reduced) flash = Math.max(flash, 0.6);
        // Droplets queue instead of restarting, so a burst of events never makes one jump.
        else if (drop.active) queued = Math.min(queued + arrivals, 3);
        else {
          drop = { progress: 0, active: true };
          queued = Math.min(arrivals - 1, 3);
        }
      }
      if (achievements !== seenAchievements) {
        if (achievements > seenAchievements) flash = 1;
        seenAchievements = achievements;
        heartTarget = heartFor(achievements);
        if (reduced) heart = heartTarget;
      }
      wake();
    };

    const onDown = (event: PointerEvent) => {
      dragging = true;
      last = { x: event.clientX, y: event.clientY };
      canvas.setPointerCapture(event.pointerId);
      wake();
    };
    const onMove = (event: PointerEvent) => {
      if (!dragging) return;
      const dx = event.clientX - last.x;
      const dy = event.clientY - last.y;
      last = { x: event.clientX, y: event.clientY };
      yaw += dx * 0.008;
      pitch = Math.max(-1.2, Math.min(1.2, pitch + dy * 0.006));
      spin = dx * 0.05;
      if (reduced) render();
    };
    const onUp = () => {
      dragging = false;
    };

    canvas.addEventListener('pointerdown', onDown);
    canvas.addEventListener('pointermove', onMove);
    canvas.addEventListener('pointerup', onUp);
    canvas.addEventListener('pointercancel', onUp);

    const observer = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting && document.visibilityState === 'visible';
      if (visible) wake();
    });
    observer.observe(canvas);
    const onVisibility = () => {
      visible = document.visibilityState === 'visible';
      if (visible) wake();
    };
    document.addEventListener('visibilitychange', onVisibility);
    const resizeObserver = new ResizeObserver(() => {
      if (reduced) render();
    });
    resizeObserver.observe(canvas);
    const unsubscribe = subscribeGame(onStore);
    const onLost = (event: Event) => {
      event.preventDefault();
      setOffline(true);
    };
    canvas.addEventListener('webglcontextlost', onLost);

    onStore();
    render();
    wake();

    return () => {
      cancelAnimationFrame(frame);
      unsubscribe();
      observer.disconnect();
      resizeObserver.disconnect();
      document.removeEventListener('visibilitychange', onVisibility);
      canvas.removeEventListener('pointerdown', onDown);
      canvas.removeEventListener('pointermove', onMove);
      canvas.removeEventListener('pointerup', onUp);
      canvas.removeEventListener('pointercancel', onUp);
      canvas.removeEventListener('webglcontextlost', onLost);
      gl.deleteProgram(program);
      gl.deleteShader(vertex);
      gl.deleteShader(fragment);
      gl.deleteBuffer(buffer);
    };
  }, []);

  return (
    <div className={className} data-offline={offline || undefined}>
      <canvas ref={canvasRef} className="core-canvas" role="img" aria-label={label} />
      {offline && <p className="core-offline mono">Core offline · WebGL is unavailable in this browser</p>}
    </div>
  );
}
