"use client";

import { useRef, useEffect, useState, useCallback, useMemo } from "react";

interface HtmlPreviewProps {
  html: string;
  requestUrl?: string;
  onElementSelect?: (info: ElementInfo) => void;
  inspectMode?: boolean;
}

export interface ElementInfo {
  tagName: string;
  attributes: Record<string, string>;
  outerHTML: string;
  sourceLine?: number;
}

export default function HtmlPreview({
  html,
  requestUrl,
  onElementSelect,
  inspectMode = false,
}: HtmlPreviewProps) {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const [ready, setReady] = useState(false);
  const highlightRef = useRef<HTMLDivElement | null>(null);

  // Build the HTML with base href injected
  const processedHtml = useMemo(() => {
    if (!requestUrl) return html;

    let baseUrl = "";
    try {
      const url = new URL(requestUrl);
      // Use the directory of the request URL as the base
      const pathParts = url.pathname.split("/");
      pathParts.pop(); // Remove the filename
      baseUrl = url.origin + pathParts.join("/") + "/";
    } catch {
      // If URL parsing fails, try to extract origin
      const match = requestUrl.match(/^(https?:\/\/[^/]+)/);
      if (match) {
        baseUrl = match[1] + "/";
      }
    }

    if (!baseUrl) return html;

    // Inject base tag if not already present
    if (html.toLowerCase().includes("<head")) {
      return html.replace(/(<head[^>]*>)/i, `$1\n<base href="${baseUrl}">`);
    } else if (html.toLowerCase().includes("<html")) {
      return html.replace(/(<html[^>]*>)/i, `$1\n<head><base href="${baseUrl}"></head>`);
    } else {
      return `<base href="${baseUrl}">\n${html}`;
    }
  }, [html, requestUrl]);

  // Create and manage the highlight overlay in the iframe
  const createHighlight = useCallback((doc: Document) => {
    if (highlightRef.current) {
      return highlightRef.current;
    }

    const el = doc.createElement('div');
    el.id = '__hittable_highlight';
    el.style.cssText = 'position:fixed;pointer-events:none;z-index:999999;border:2px solid #00e5cc;background:rgba(0,229,204,0.1);box-shadow:0 0 0 1px rgba(0,229,204,0.3);transition:all 0.05s ease;display:none;';
    doc.body.appendChild(el);
    highlightRef.current = el;
    return el;
  }, []);

  const updateHighlight = useCallback((el: Element | null) => {
    if (!iframeRef.current?.contentDocument || !el) {
      if (highlightRef.current) {
        highlightRef.current.style.display = 'none';
      }
      return;
    }

    const highlight = createHighlight(iframeRef.current.contentDocument);
    const rect = el.getBoundingClientRect();

    highlight.style.top = (rect.top + window.scrollY) + 'px';
    highlight.style.left = (rect.left + window.scrollX) + 'px';
    highlight.style.width = rect.width + 'px';
    highlight.style.height = rect.height + 'px';
    highlight.style.display = 'block';
  }, [createHighlight]);

  const hideHighlight = useCallback(() => {
    if (highlightRef.current) {
      highlightRef.current.style.display = 'none';
    }
  }, []);

  const getElementInfo = useCallback((el: Element): ElementInfo => {
    const attrs: Record<string, string> = {};
    for (const attr of Array.from(el.attributes)) {
      attrs[attr.name] = attr.value;
    }

    return {
      tagName: el.tagName.toLowerCase(),
      attributes: attrs,
      outerHTML: el.outerHTML.substring(0, 500),
    };
  }, []);

  // Handle mouse events in the iframe
  useEffect(() => {
    if (!inspectMode || !ready || !iframeRef.current?.contentDocument) {
      return;
    }

    const doc = iframeRef.current.contentDocument;

    const handleMouseMove = (e: MouseEvent) => {
      const target = e.target as Element;
      if (target && target !== doc.body && target !== doc.documentElement) {
        updateHighlight(target);
      }
    };

    const handleMouseLeave = () => {
      hideHighlight();
    };

    const handleClick = (e: MouseEvent) => {
      e.preventDefault();
      e.stopPropagation();

      const target = e.target as Element;
      if (target && target !== doc.body && target !== doc.documentElement) {
        const info = getElementInfo(target);
        onElementSelect?.(info);
      }
    };

    // Attach event listeners to the iframe's document
    doc.addEventListener('mousemove', handleMouseMove);
    doc.addEventListener('mouseleave', handleMouseLeave);
    doc.addEventListener('click', handleClick, { capture: true });

    return () => {
      doc.removeEventListener('mousemove', handleMouseMove);
      doc.removeEventListener('mouseleave', handleMouseLeave);
      doc.removeEventListener('click', handleClick, { capture: true });
      hideHighlight();
    };
  }, [inspectMode, ready, updateHighlight, hideHighlight, getElementInfo, onElementSelect]);

  // Handle iframe load
  const handleLoad = useCallback(() => {
    setReady(true);
  }, []);

  return (
    <div className="relative w-full h-full">
      <iframe
        ref={iframeRef}
        srcDoc={processedHtml}
        sandbox="allow-same-origin"
        title="HTML response preview"
        className="w-full h-full border-0 bg-white"
        onLoad={handleLoad}
        style={{ minHeight: 200 }}
      />
      {!ready && (
        <div className="absolute inset-0 flex items-center justify-center bg-[#0a1628]">
          <span className="text-white/30 text-[10px]">Loading preview…</span>
        </div>
      )}
    </div>
  );
}
