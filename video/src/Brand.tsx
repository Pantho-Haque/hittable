import React from "react";

export const CYAN = "#00e5cc";
export const TILE = "#21222c";
export const BG = "#1a1b23";

/** The wordmark, letter-spaced the way the app draws it. */
export const Wordmark: React.FC<{ size?: number }> = ({ size = 64 }) => (
  <div
    style={{
      fontFamily: '"SF Mono", Menlo, monospace',
      fontSize: size,
      fontWeight: 700,
      letterSpacing: size * 0.35,
      color: CYAN,
      paddingLeft: size * 0.35,
    }}
  >
    HITTABLE
  </div>
);

/**
 * The logo tile: a rounded dark square with the italic H mark, matching the
 * block-art version the app prints on its welcome screen.
 */
export const Mark: React.FC<{ size?: number }> = ({ size = 180 }) => (
  <div
    style={{
      width: size,
      height: size,
      borderRadius: size * 0.18,
      background: TILE,
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      boxShadow: `0 0 ${size * 0.5}px rgba(0,229,204,0.18)`,
    }}
  >
    <svg width={size * 0.56} height={size * 0.56} viewBox="0 0 100 100">
      <g fill="#e8e8ee">
        <rect x="18" y="20" width="13" height="60" transform="skewX(-12)" />
        <rect x="58" y="20" width="13" height="60" transform="skewX(-12)" />
        <rect x="24" y="44" width="47" height="13" transform="skewX(-12)" />
      </g>
    </svg>
  </div>
);
