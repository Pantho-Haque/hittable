"use client";

import { useMemo, useRef, useCallback, useEffect, useState } from "react";
import { formatHtml } from "@/utils/htmlFormatter";

interface HtmlSourceViewerProps {
  source: string;
  searchQuery?: string;
  highlightedLine?: number | null;
  onLineClick?: (line: number) => void;
}

interface FoldState {
  [lineNumber: number]: boolean; // true = collapsed
}

// Minimal HTML syntax highlighting - tags, attributes, strings, comments
function highlightHtmlLine(line: string): React.ReactNode[] {
  const parts: React.ReactNode[] = [];
  const remaining = line;
  let keyIdx = 0;

  let i = 0;
  while (i < remaining.length) {
    // HTML comment
    if (remaining.startsWith("<!--", i)) {
      const end = remaining.indexOf("-->", i + 4);
      if (end !== -1) {
        parts.push(<span key={keyIdx++} className="text-white/25 italic">{remaining.slice(i, end + 3)}</span>);
        i = end + 3;
        continue;
      }
      parts.push(<span key={keyIdx++} className="text-white/25 italic">{remaining.slice(i)}</span>);
      break;
    }

    // Opening tag or closing tag
    if (remaining[i] === "<") {
      // Find the end of the tag
      let j = i + 1;
      let inString = false;
      let stringChar = "";
      while (j < remaining.length) {
        if (inString) {
          if (remaining[j] === stringChar && remaining[j - 1] !== "\\") {
            inString = false;
          }
        } else {
          if (remaining[j] === '"' || remaining[j] === "'") {
            inString = true;
            stringChar = remaining[j];
          } else if (remaining[j] === ">") {
            break;
          }
        }
        j++;
      }
      if (j < remaining.length) j++; // include the >

      const tagContent = remaining.slice(i, j);

      // Parse the tag for highlighting
      const tagNodes = parseTagContent(tagContent);
      parts.push(...tagNodes);
      i = j;
      continue;
    }

    // HTML entity
    if (remaining[i] === "&") {
      const semiIdx = remaining.indexOf(";", i + 1);
      if (semiIdx !== -1 && semiIdx - i < 10) {
        parts.push(<span key={keyIdx++} className="text-purple-300">{remaining.slice(i, semiIdx + 1)}</span>);
        i = semiIdx + 1;
        continue;
      }
    }

    // Plain text
    let j = i + 1;
    while (j < remaining.length && remaining[j] !== "<" && remaining[j] !== "&") {
      j++;
    }
    parts.push(<span key={keyIdx++} className="text-white/70">{remaining.slice(i, j)}</span>);
    i = j;
  }

  return parts;
}

function parseTagContent(tag: string): React.ReactNode[] {
  const parts: React.ReactNode[] = [];
  let keyIdx = 0;

  // Extract tag name
  const tagMatch = tag.match(/^(<\/?)(\w[\w-]*)/);
  if (tagMatch) {
    parts.push(<span key={keyIdx++} className="text-cyan-300">{tagMatch[1]}</span>);
    parts.push(<span key={keyIdx++} className="text-cyan-300 font-semibold">{tagMatch[2]}</span>);

    let remaining = tag.slice(tagMatch[0].length);

    // Process attributes
    while (remaining.length > 0) {
      // Skip whitespace
      const wsMatch = remaining.match(/^(\s+)/);
      if (wsMatch) {
        parts.push(<span key={keyIdx++}>{wsMatch[1]}</span>);
        remaining = remaining.slice(wsMatch[1].length);
      }

      // Check for end of tag
      if (remaining.startsWith(">") || remaining.startsWith("/>")) {
        parts.push(<span key={keyIdx++} className="text-cyan-300">{remaining}</span>);
        break;
      }

      // Attribute name
      const attrMatch = remaining.match(/^([\w-:]+)(=)?/);
      if (attrMatch) {
        parts.push(<span key={keyIdx++} className="text-amber-300">{attrMatch[1]}</span>);
        remaining = remaining.slice(attrMatch[1].length);

        if (attrMatch[2] === "=") {
          parts.push(<span key={keyIdx++}>=</span>);
          remaining = remaining.slice(1);

          // Attribute value
          if (remaining.startsWith('"')) {
            const endQuote = remaining.indexOf('"', 1);
            if (endQuote !== -1) {
              parts.push(<span key={keyIdx++} className="text-emerald-300">{remaining.slice(0, endQuote + 1)}</span>);
              remaining = remaining.slice(endQuote + 1);
            } else {
              parts.push(<span key={keyIdx++} className="text-emerald-300">{remaining}</span>);
              break;
            }
          } else if (remaining.startsWith("'")) {
            const endQuote = remaining.indexOf("'", 1);
            if (endQuote !== -1) {
              parts.push(<span key={keyIdx++} className="text-emerald-300">{remaining.slice(0, endQuote + 1)}</span>);
              remaining = remaining.slice(endQuote + 1);
            } else {
              parts.push(<span key={keyIdx++} className="text-emerald-300">{remaining}</span>);
              break;
            }
          } else {
            // Unquoted value
            const valMatch = remaining.match(/^([^\s>]+)/);
            if (valMatch) {
              parts.push(<span key={keyIdx++} className="text-emerald-300">{valMatch[1]}</span>);
              remaining = remaining.slice(valMatch[1].length);
            }
          }
        }
      } else {
        // Unknown content, just output it
        parts.push(<span key={keyIdx++}>{remaining[0]}</span>);
        remaining = remaining.slice(1);
      }
    }
  } else {
    parts.push(<span key={keyIdx++} className="text-cyan-300">{tag}</span>);
  }

  return parts;
}

export default function HtmlSourceViewer({
  source,
  searchQuery,
  highlightedLine,
  onLineClick,
}: HtmlSourceViewerProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [foldState, setFoldState] = useState<FoldState>({});

  // Format the HTML source for readability
  const formattedSource = useMemo(() => {
    try {
      return formatHtml(source, { indentSize: 2, maxLineLength: 120 });
    } catch {
      // If formatting fails, use the original source
      return source;
    }
  }, [source]);

  const lines = useMemo(() => formattedSource.split("\n"), [formattedSource]);

  // Find foldable regions (opening tags with matching closing tags)
  const foldableRanges = useMemo(() => {
    const ranges: Array<{ start: number; end: number; tag: string }> = [];
    const stack: Array<{ line: number; tag: string }> = [];

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];

      // Match opening tags (not self-closing, not void elements)
      const openMatch = line.match(/<(\w[\w-]*)((?:\s+[\w-:]+(?:=(?:"[^"]*"|'[^']*'|[^\s>]*))?)*\s*)(\/?)\s*>/);
      if (openMatch && openMatch[3] !== "/") {
        const tagName = openMatch[1].toLowerCase();
        // Void elements don't need closing tags
        const voidElements = ["area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr"];
        if (!voidElements.includes(tagName)) {
          stack.push({ line: i, tag: tagName });
        }
      }

      // Match closing tags
      const closeMatch = line.match(/<\/(\w[\w-]*)\s*>/);
      if (closeMatch && stack.length > 0) {
        const tagName = closeMatch[1].toLowerCase();
        // Find matching opening tag
        for (let j = stack.length - 1; j >= 0; j--) {
          if (stack[j].tag === tagName) {
            if (stack[j].line < i - 1) { // Only fold if content spans multiple lines
              ranges.push({
                start: stack[j].line,
                end: i,
                tag: tagName,
              });
            }
            stack.splice(j, 1);
            break;
          }
        }
      }
    }

    return ranges;
  }, [lines]);

  // Check if a line is inside a folded region
  const isFolded = useCallback((lineNum: number): boolean => {
    for (const range of foldableRanges) {
      if (foldState[range.start] && lineNum > range.start && lineNum <= range.end) {
        return true;
      }
    }
    return false;
  }, [foldState, foldableRanges]);

  const toggleFold = useCallback((lineNum: number) => {
    setFoldState(prev => ({ ...prev, [lineNum]: !prev[lineNum] }));
  }, []);

  // Search highlighting
  const searchHighlights = useMemo(() => {
    if (!searchQuery) return new Map<number, number[]>();
    const highlights = new Map<number, number[]>();
    const q = searchQuery.toLowerCase();

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i].toLowerCase();
      const indices: number[] = [];
      let startIdx = 0;
      while (startIdx < line.length) {
        const idx = line.indexOf(q, startIdx);
        if (idx === -1) break;
        indices.push(idx);
        startIdx = idx + q.length;
      }
      if (indices.length > 0) {
        highlights.set(i, indices);
      }
    }
    return highlights;
  }, [lines, searchQuery]);

  // Scroll to highlighted line
  useEffect(() => {
    if (highlightedLine !== null && highlightedLine !== undefined) {
      const lineEl = containerRef.current?.querySelector(`[data-line="${highlightedLine}"]`);
      if (lineEl) {
        lineEl.scrollIntoView({ block: "center", behavior: "smooth" });
      }
    }
  }, [highlightedLine]);

  // Render a line with search highlighting
  const renderLine = useCallback((line: string, lineNum: number) => {
    const highlightIndices = searchHighlights.get(lineNum);
    if (!highlightIndices || highlightIndices.length === 0) {
      return highlightHtmlLine(line);
    }

    const parts: React.ReactNode[] = [];
    let lastIdx = 0;
    const q = searchQuery!.toLowerCase();

    for (const idx of highlightIndices) {
      if (idx > lastIdx) {
        parts.push(
          <span key={`t-${lastIdx}`} className="text-white/70">
            {line.slice(lastIdx, idx)}
          </span>
        );
      }
      parts.push(
        <mark
          key={`h-${idx}`}
          className="bg-yellow-400/30 text-yellow-200 rounded-sm px-0.5"
        >
          {line.slice(idx, idx + q.length)}
        </mark>
      );
      lastIdx = idx + q.length;
    }

    if (lastIdx < line.length) {
      parts.push(
        <span key={`t-${lastIdx}`} className="text-white/70">
          {line.slice(lastIdx)}
        </span>
      );
    }

    return parts;
  }, [searchHighlights, searchQuery]);

  return (
    <div ref={containerRef} className="font-mono text-[10px] md:text-[12px] leading-relaxed">
      {lines.map((line, idx) => {
        // Skip lines that are folded
        if (isFolded(idx)) return null;

        // Find if this line starts a foldable range
        const foldRange = foldableRanges.find(r => r.start === idx);
        const isFoldable = !!foldRange;
        const isCollapsed = foldState[idx];

        return (
          <div
            key={idx}
            data-line={idx}
            className={`flex items-start group ${
              highlightedLine === idx ? "bg-cyan-400/10" : ""
            } hover:bg-white/5 transition-colors`}
          >
            {/* Fold indicator + line number */}
            <div
              className="flex items-center gap-0 shrink-0 w-14 text-right pr-2 select-none border-r border-white/5"
              onClick={() => {
                if (isFoldable) toggleFold(idx);
                onLineClick?.(idx);
              }}
            >
              {isFoldable ? (
                <span className={`text-white/30 cursor-pointer hover:text-cyan-400 transition-colors mr-0.5 ${isCollapsed ? "-rotate-90" : ""}`}>
                  ▸
                </span>
              ) : (
                <span className="mr-2.5" />
              )}
              <span className="text-white/20 text-[10px]">
                {idx + 1}
              </span>
            </div>

            {/* Code content */}
            <div className="flex-1 pl-2 whitespace-pre overflow-x-auto">
              {renderLine(line, idx)}
              {isCollapsed && foldRange && (
                <span className="text-white/20 italic ml-2">
                  ... {foldRange.end - foldRange.start} lines hidden
                </span>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
