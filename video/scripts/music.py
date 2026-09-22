#!/usr/bin/env python3
"""
The teaser's backing track: a bright four-on-the-floor loop over a I-V-vi-IV
progression at 124 BPM, so every cut in the video can land on a beat.

Synthesised from scratch with the standard library — the track is ours and
carries no licence. Drums are shaped noise and a pitch-swept sine rather than
samples, so there are no audio assets to ship.
"""
import math
import random
import struct
import sys
import wave

SR = 44100
BPM = 124.0
BEAT = 60.0 / BPM
BAR = BEAT * 4


def sec(n):
    return int(n * SR)


class Track:
    def __init__(self, length):
        self.buf = [0.0] * sec(length)

    def add(self, at, samples, gain=1.0):
        i = sec(at)
        for k, v in enumerate(samples):
            j = i + k
            if 0 <= j < len(self.buf):
                self.buf[j] += v * gain


def env(n, attack, decay, curve=2.5):
    """Percussive envelope: near-instant attack, exponential fall."""
    a = max(sec(attack), 1)
    d = max(sec(decay), 1)
    return [(i / a) if i < a else math.exp(-curve * (i - a) / d) for i in range(n)]


def kick():
    n = sec(0.30)
    e = env(n, 0.001, 0.11, 3.0)
    out, phase = [], 0.0
    for i in range(n):
        # The pitch sweep is what gives it the punch.
        f = 120.0 * math.exp(-27.0 * i / SR) + 47.0
        phase += 2 * math.pi * f / SR
        out.append(math.sin(phase) * e[i])
    return out


def clap():
    n = sec(0.26)
    e = env(n, 0.001, 0.08, 3.4)
    out, prev = [], 0.0
    for i in range(n):
        w = random.uniform(-1, 1)
        hp = w - prev  # one-pole high pass, so it cracks instead of thuds
        prev = w
        out.append((hp * 0.9 + math.sin(2 * math.pi * 190 * i / SR) * 0.25) * e[i])
    return out


def hat(open_=False):
    n = sec(0.16 if open_ else 0.055)
    e = env(n, 0.0005, 0.07 if open_ else 0.022, 4.0)
    out, prev = [], 0.0
    for i in range(n):
        w = random.uniform(-1, 1)
        hp = w - prev * 0.72
        prev = w
        out.append(hp * e[i])
    return out


def pluck(freq, dur, bright=0.5):
    """A saw softened by a one-pole filter: the cheerful top line."""
    n = sec(dur)
    e = env(n, 0.002, dur * 0.55, 3.0)
    out, phase, lp = [], 0.0, 0.0
    for i in range(n):
        phase = (phase + freq / SR) % 1.0
        raw = (2.0 * phase - 1.0) * bright + (1.0 if phase < 0.5 else -1.0) * (1 - bright)
        lp += (raw - lp) * 0.28
        out.append(lp * e[i])
    return out


def bass(freq, dur):
    n = sec(dur)
    e = env(n, 0.004, dur * 0.7, 2.2)
    out, phase, lp = [], 0.0, 0.0
    for i in range(n):
        phase = (phase + freq / SR) % 1.0
        sq = 1.0 if phase < 0.5 else -1.0
        sub = math.sin(2 * math.pi * freq * 0.5 * i / SR)
        lp += ((sq * 0.6 + sub * 0.8) - lp) * 0.12
        out.append(lp * e[i])
    return out


NOTE = {"C": 0, "D": 2, "E": 4, "F": 5, "G": 7, "A": 9, "B": 11}


def hz(name, octave):
    return 440.0 * 2 ** ((NOTE[name] + (octave - 4) * 12 - 9) / 12.0)


PROG = [("C", ["C", "E", "G"]), ("G", ["G", "B", "D"]),
        ("A", ["A", "C", "E"]), ("F", ["F", "A", "C"])]


def build(bars):
    random.seed(7)
    t = Track(bars * BAR + 1.5)
    for b in range(bars):
        at = b * BAR
        root, chord = PROG[b % 4]
        full = b >= 1   # drums land after a one-bar lift
        busy = b >= 2

        for beat in range(4):
            tb = at + beat * BEAT
            if full:
                t.add(tb, kick(), 0.95)
                t.add(tb, bass(hz(root, 2), BEAT * 0.9), 0.42)
            if full and beat in (1, 3):
                t.add(tb, clap(), 0.50)
                t.add(tb + BEAT * 0.5, bass(hz(root, 2), BEAT * 0.4), 0.22)
            for half in (0, 1):
                t.add(tb + half * BEAT / 2,
                      hat(open_=(half == 1 and beat == 3)),
                      0.20 if half else 0.13)

        for s in range(16):  # sixteenth-note arpeggio: the hook
            note = chord[[0, 1, 2, 1][s % 4]]
            g = (0.30 if busy else 0.18) * (1.25 if s % 4 == 0 else 1.0)
            t.add(at + s * (BAR / 16),
                  pluck(hz(note, 5 if (s % 8) < 4 else 6), BAR / 16 * 1.6), g)

        if busy:  # a pad underneath, so it is not only transients
            for note in chord:
                t.add(at, pluck(hz(note, 4), BAR * 0.98, bright=0.2), 0.10)
    return t.buf


def write(path, buf, fade_out=1.6):
    n = len(buf)
    norm = 0.89 / max(1e-9, max(abs(v) for v in buf))
    fo, fi = sec(fade_out), sec(0.02)
    frames = bytearray()
    for i, v in enumerate(buf):
        v *= norm
        if i > n - fo:
            v *= (n - i) / fo
        if i < fi:
            v *= i / fi
        v = math.tanh(v * 1.25) * 0.92  # soft clip: musical peaks, not square
        s = int(max(-1.0, min(1.0, v)) * 32767)
        frames += struct.pack("<hh", s, s)
    with wave.open(path, "wb") as w:
        w.setnchannels(2)
        w.setsampwidth(2)
        w.setframerate(SR)
        w.writeframes(bytes(frames))


if __name__ == "__main__":
    out = sys.argv[1] if len(sys.argv) > 1 else "public/beat.wav"
    bars = int(sys.argv[2]) if len(sys.argv) > 2 else 16
    write(out, build(bars))
    print(f"wrote {out} — {bars} bars, {bars * BAR:.1f}s at {BPM:.0f} BPM")
