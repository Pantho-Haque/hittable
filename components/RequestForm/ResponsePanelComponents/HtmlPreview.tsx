"use client";

import { useEffect, useMemo, useRef } from "react";

export interface ElementInfo {
  tagName: string;
  attributes: Record<string, string>;
  outerHTML: string;
}

interface HtmlPreviewProps {
  html: string;
  /** The page's own URL (after redirects): relative CSS, images and scripts resolve against it. */
  baseUrl?: string;
  /** Run the page's own scripts. Off blocks them with a CSP; the inspector still runs. */
  scripts?: boolean;
  inspectMode?: boolean;
  onElementSelect?: (info: ElementInfo) => void;
}

/**
 * The frame is sandboxed WITHOUT allow-same-origin: it gets an opaque origin,
 * so the page's scripts can run but cannot touch this app, its storage or its
 * cookies, and calls to the app's API arrive with `Origin: null` and are
 * refused. No allow-modals (no alert spam), no top navigation; links open in a
 * new tab, outside the sandbox.
 */
export const PREVIEW_SANDBOX = "allow-scripts allow-popups allow-popups-to-escape-sandbox";

// Runs inside the preview. The frame is another origin, so the inspector
// cannot be driven from out here: it listens for on/off and posts back the
// element the user clicked.
const INSPECTOR = `(function(){
var on=false,box=null,cur=null,P=parent;
function hide(){if(box)box.style.display="none"}
addEventListener("message",function(e){
  if(e.source!==P||!e.data||e.data.type!=="hittable:inspect")return;
  on=!!e.data.on;
  if(!cur){cur=document.createElement("style");document.documentElement.appendChild(cur)}
  cur.textContent=on?"*{cursor:crosshair!important}":"";
  if(!on)hide();
});
document.addEventListener("mouseover",function(e){
  var t=e.target;if(!on||!t||t.nodeType!==1||t===box)return;
  if(!box){box=document.createElement("div");document.documentElement.appendChild(box)}
  var r=t.getBoundingClientRect();
  box.style.cssText="position:fixed;pointer-events:none;z-index:2147483647;border:2px solid #37d4e0;background:rgba(55,212,224,.12);top:"+r.top+"px;left:"+r.left+"px;width:"+r.width+"px;height:"+r.height+"px";
},true);
document.addEventListener("mouseout",function(e){if(!e.relatedTarget)hide()},true);
document.addEventListener("click",function(e){
  if(!on)return;e.preventDefault();e.stopPropagation();
  var t=e.target;if(!t||t.nodeType!==1)return;
  var a={};for(var i=0;i<t.attributes.length;i++)a[t.attributes[i].name]=t.attributes[i].value;
  P.postMessage({type:"hittable:picked",tagName:t.tagName.toLowerCase(),attributes:a,outerHTML:t.outerHTML.slice(0,500)},"*");
},true);
})();`;

/** The page with our head tags added: base URL, script policy, inspector. */
export function previewDocument(html: string, { baseUrl, scripts = true, nonce }: { baseUrl?: string; scripts?: boolean; nonce: string }): string {
  let base = "";
  if (baseUrl) {
    try {
      // URL.href percent-encodes quotes, so it is safe inside the attribute.
      base = `<base href="${new URL(baseUrl).href}" target="_blank">`;
    } catch {
      // not a URL: links stay relative to nothing, as before
    }
  }
  const csp = scripts ? "" : `<meta http-equiv="Content-Security-Policy" content="script-src 'nonce-${nonce}'">`;
  // Light unless the page says otherwise: unstyled HTML must not inherit the
  // app's dark scheme (dark text on a dark canvas). Page CSS comes later and wins.
  const scheme = "<style>:root{color-scheme:light}</style>";
  const head = `${csp}${base}${scheme}<script nonce="${nonce}">${INSPECTOR}</script>`;
  // Function replacers: a `$` in the URL must not be read as a pattern.
  // `<head` must not match `<header`.
  if (/<head(\s[^>]*)?>/i.test(html)) return html.replace(/<head(\s[^>]*)?>/i, (tag) => `${tag}${head}`);
  if (/<html(\s[^>]*)?>/i.test(html)) return html.replace(/<html(\s[^>]*)?>/i, (tag) => `${tag}<head>${head}</head>`);
  return `${head}${html}`;
}

export default function HtmlPreview({ html, baseUrl, scripts = true, inspectMode = false, onElementSelect }: HtmlPreviewProps) {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const nonce = useMemo(() => crypto.randomUUID().replace(/-/g, ""), []);
  const srcDoc = useMemo(() => previewDocument(html, { baseUrl, scripts, nonce }), [html, baseUrl, scripts, nonce]);

  const onSelectRef = useRef(onElementSelect);
  useEffect(() => {
    onSelectRef.current = onElementSelect;
  });

  // Picks come from the frame. Its page's own scripts can post too, so check
  // the sender and the shape; at worst a page fakes what the inspector shows.
  useEffect(() => {
    const onMessage = (e: MessageEvent) => {
      const d = e.data;
      if (e.source !== iframeRef.current?.contentWindow || d?.type !== "hittable:picked") return;
      if (typeof d.tagName !== "string" || typeof d.outerHTML !== "string" || typeof d.attributes !== "object" || !d.attributes) return;
      const attributes: Record<string, string> = {};
      for (const [k, v] of Object.entries(d.attributes)) if (typeof v === "string") attributes[k] = v;
      onSelectRef.current?.({ tagName: d.tagName, attributes, outerHTML: d.outerHTML.slice(0, 500) });
    };
    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, []);

  const sendMode = () => iframeRef.current?.contentWindow?.postMessage({ type: "hittable:inspect", on: inspectMode }, "*");
  useEffect(sendMode, [inspectMode]);

  return (
    <iframe
      ref={iframeRef}
      srcDoc={srcDoc}
      sandbox={PREVIEW_SANDBOX}
      title="HTML response preview"
      className="block h-full w-full border-0 bg-white"
      onLoad={sendMode}
    />
  );
}
