/**
 * Simple HTML/CSS/JS reformatter for source view readability.
 * Handles minified HTML by adding proper line breaks and indentation.
 */

const SELF_CLOSING_TAGS = new Set([
  'area', 'base', 'br', 'col', 'embed', 'hr', 'img', 'input',
  'link', 'meta', 'param', 'source', 'track', 'wbr'
]);

const BLOCK_TAGS = new Set([
  'html', 'head', 'body', 'div', 'section', 'article', 'aside',
  'header', 'footer', 'nav', 'main', 'form', 'fieldset', 'legend',
  'table', 'thead', 'tbody', 'tfoot', 'tr', 'td', 'th',
  'ul', 'ol', 'li', 'dl', 'dt', 'dd',
  'p', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
  'blockquote', 'pre', 'figure', 'figcaption',
  'details', 'summary'
]);

interface FormatOptions {
  indentSize?: number;
  maxLineLength?: number;
}

function formatAttributes(attrs: string): string {
  if (!attrs.trim()) return '';

  const parts: string[] = [];
  let remaining = attrs;

  while (remaining.length > 0) {
    // Skip whitespace
    const wsMatch = remaining.match(/^(\s+)/);
    if (wsMatch) {
      parts.push(wsMatch[1]);
      remaining = remaining.slice(wsMatch[1].length);
      continue;
    }

    // Attribute name
    const attrMatch = remaining.match(/^([\w-:]+)(=)?/);
    if (attrMatch) {
      parts.push(attrMatch[1]);
      remaining = remaining.slice(attrMatch[1].length);

      if (attrMatch[2] === '=') {
        parts.push('=');
        remaining = remaining.slice(1);

        // Attribute value
        if (remaining.startsWith('"')) {
          const endQuote = remaining.indexOf('"', 1);
          if (endQuote !== -1) {
            parts.push(remaining.slice(0, endQuote + 1));
            remaining = remaining.slice(endQuote + 1);
          } else {
            parts.push(remaining);
            break;
          }
        } else if (remaining.startsWith("'")) {
          const endQuote = remaining.indexOf("'", 1);
          if (endQuote !== -1) {
            parts.push(remaining.slice(0, endQuote + 1));
            remaining = remaining.slice(endQuote + 1);
          } else {
            parts.push(remaining);
            break;
          }
        } else {
          const valMatch = remaining.match(/^([^\s>]+)/);
          if (valMatch) {
            parts.push(valMatch[1]);
            remaining = remaining.slice(valMatch[1].length);
          }
        }
      }
    } else {
      parts.push(remaining[0]);
      remaining = remaining.slice(1);
    }
  }

  return parts.join('');
}

function formatTag(
  tag: string,
): { formatted: string; selfClosing: boolean; block: boolean } {
  const tagMatch = tag.match(/^(<\/?)(\w[\w-]*)((?:\s+[\w-:]+(?:=(?:"[^"]*"|'[^']*'|[^\s>]*))?)*\s*)(\/?)\s*>/);

  if (!tagMatch) {
    return { formatted: tag, selfClosing: false, block: false };
  }

  const [, openSlash, tagName, attrs, selfClose] = tagMatch;
  const lowerTag = tagName.toLowerCase();
  const isSelfClosing = selfClose === '/' || SELF_CLOSING_TAGS.has(lowerTag);
  const isBlock = BLOCK_TAGS.has(lowerTag);

  const formattedAttrs = formatAttributes(attrs);
  const formattedTag = `${openSlash}${tagName}${formattedAttrs}${selfClose}>`;

  return {
    formatted: formattedTag,
    selfClosing: isSelfClosing,
    block: isBlock
  };
}

export function formatHtml(html: string, options: FormatOptions = {}): string {
  const {
    indentSize = 2,
    maxLineLength = 100
  } = options;

  const indentStr = ' '.repeat(indentSize);
  let indent = 0;
  let i = 0;

  const lines: string[] = [];

  while (i < html.length) {
    // HTML comment
    if (html.startsWith('<!--', i)) {
      const end = html.indexOf('-->', i + 4);
      if (end !== -1) {
        const comment = html.slice(i, end + 3);
        // Multi-line comments stay as-is
        if (comment.includes('\n')) {
          lines.push(comment);
        } else if (comment.length > maxLineLength) {
          // Long comment - try to break it
          lines.push(indentStr.repeat(indent) + comment);
        } else {
          lines.push(indentStr.repeat(indent) + comment);
        }
        i = end + 3;
        continue;
      }
      lines.push(indentStr.repeat(indent) + html.slice(i));
      break;
    }

    // Script tag
    if (html.slice(i).match(/<script[\s>]/i)) {
      const scriptEnd = html.indexOf('</script>', i);
      if (scriptEnd !== -1) {
        const scriptContent = html.slice(i, scriptEnd + 9);
        const scriptTagMatch = scriptContent.match(/^(<script[^>]*>)([\s\S]*?)(<\/script>)$/i);

        if (scriptTagMatch) {
          const [, openTag, scriptBody, closeTag] = scriptTagMatch;
          const formattedOpen = indentStr.repeat(indent) + openTag.trim();
          lines.push(formattedOpen);

          // Format JavaScript content
          const formattedScript = formatJavaScript(scriptBody, indent + 1, indentStr);
          lines.push(formattedScript);

          lines.push(indentStr.repeat(indent) + closeTag);
        } else {
          lines.push(indentStr.repeat(indent) + scriptContent);
        }
        i = scriptEnd + 9;
        continue;
      }
    }

    // Style tag
    if (html.slice(i).match(/<style[\s>]/i)) {
      const styleEnd = html.indexOf('</style>', i);
      if (styleEnd !== -1) {
        const styleContent = html.slice(i, styleEnd + 8);
        const styleTagMatch = styleContent.match(/^(<style[^>]*>)([\s\S]*?)(<\/style>)$/i);

        if (styleTagMatch) {
          const [, openTag, cssBody, closeTag] = styleTagMatch;
          const formattedOpen = indentStr.repeat(indent) + openTag.trim();
          lines.push(formattedOpen);

          // Format CSS content
          const formattedCSS = formatCSS(cssBody, indent + 1, indentStr);
          lines.push(formattedCSS);

          lines.push(indentStr.repeat(indent) + closeTag);
        } else {
          lines.push(indentStr.repeat(indent) + styleContent);
        }
        i = styleEnd + 8;
        continue;
      }
    }

    // HTML tag
    if (html[i] === '<') {
      // Find the end of the tag
      let j = i + 1;
      let inString = false;
      let stringChar = '';

      while (j < html.length) {
        if (inString) {
          if (html[j] === stringChar && html[j - 1] !== '\\') {
            inString = false;
          }
        } else {
          if (html[j] === '"' || html[j] === "'") {
            inString = true;
            stringChar = html[j];
          } else if (html[j] === '>') {
            break;
          }
        }
        j++;
      }

      if (j < html.length) j++; // include the >

      const tagContent = html.slice(i, j);
      const { formatted, selfClosing, block } = formatTag(tagContent);

      // Adjust indent for closing tags
      if (tagContent.startsWith('</')) {
        indent = Math.max(0, indent - 1);
      }

      lines.push(indentStr.repeat(indent) + formatted);

      // Increase indent for block opening tags
      if (block && !selfClosing && !tagContent.startsWith('</')) {
        indent++;
      }

      i = j;
      continue;
    }

    // Text content
    let j = i + 1;
    while (j < html.length && html[j] !== '<') {
      j++;
    }

    const textContent = html.slice(i, j).trim();
    if (textContent) {
      // Check if text is on same line as tags
      const lastLine = lines[lines.length - 1] || '';
      if (lastLine.endsWith('>') && textContent.length < 50) {
        // Inline text - keep on same line if short
        lines[lines.length - 1] += textContent;
      } else {
        // Block text or long text - new line
        lines.push(indentStr.repeat(indent) + textContent);
      }
    }

    i = j;
  }

  return lines.join('\n');
}

function formatJavaScript(code: string, indent: number, indentStr: string): string {
  // Simple JS formatter - handle common cases
  const lines: string[] = [];
  let i = 0;

  while (i < code.length) {
    // Skip whitespace
    const wsMatch = code.slice(i).match(/^(\s+)/);
    if (wsMatch) {
      i += wsMatch[1].length;
      continue;
    }

    // String literal
    if (code[i] === '"' || code[i] === "'" || code[i] === '`') {
      const quote = code[i];
      let j = i + 1;
      while (j < code.length) {
        if (code[j] === quote && code[j - 1] !== '\\') break;
        if (code[j] === '\\' && j + 1 < code.length) j++;
        j++;
      }
      j++; // include closing quote
      const str = code.slice(i, j);
      if (lines.length > 0 && lines[lines.length - 1].trim()) {
        lines[lines.length - 1] += str;
      } else {
        lines.push(indentStr.repeat(indent) + str);
      }
      i = j;
      continue;
    }

    // Comment
    if (code.slice(i, i + 2) === '//') {
      const end = code.indexOf('\n', i);
      const comment = end !== -1 ? code.slice(i, end) : code.slice(i);
      lines.push(indentStr.repeat(indent) + comment.trim());
      i = end !== -1 ? end : code.length;
      continue;
    }

    if (code.slice(i, i + 2) === '/*') {
      const end = code.indexOf('*/', i + 2);
      const comment = end !== -1 ? code.slice(i, end + 2) : code.slice(i);
      if (comment.includes('\n')) {
        lines.push(indentStr.repeat(indent) + comment);
      } else {
        lines.push(indentStr.repeat(indent) + comment);
      }
      i = end !== -1 ? end + 2 : code.length;
      continue;
    }

    // Semicolon - end of statement
    if (code[i] === ';') {
      if (lines.length > 0) {
        lines[lines.length - 1] += ';';
      }
      i++;
      continue;
    }

    // Opening brace
    if (code[i] === '{') {
      if (lines.length > 0) {
        lines[lines.length - 1] += ' {';
      }
      indent++;
      i++;
      continue;
    }

    // Closing brace
    if (code[i] === '}') {
      indent = Math.max(0, indent - 1);
      lines.push(indentStr.repeat(indent) + '}');
      i++;
      continue;
    }

    // Other characters - collect until whitespace or special char
    let j = i;
    while (j < code.length && code[j] !== ' ' && code[j] !== '\t' && code[j] !== '\n' &&
           code[j] !== ';' && code[j] !== '{' && code[j] !== '}' &&
           code[j] !== '"' && code[j] !== "'" && code[j] !== '`') {
      j++;
    }

    if (j > i) {
      const token = code.slice(i, j);
      if (lines.length > 0 && lines[lines.length - 1].trim() && !lines[lines.length - 1].endsWith(' ')) {
        lines[lines.length - 1] += token;
      } else {
        lines.push(indentStr.repeat(indent) + token);
      }
      i = j;
      continue;
    }

    // Unknown character
    if (lines.length > 0) {
      lines[lines.length - 1] += code[i];
    } else {
      lines.push(indentStr.repeat(indent) + code[i]);
    }
    i++;
  }

  return lines.join('\n');
}

function formatCSS(code: string, indent: number, indentStr: string): string {
  const lines: string[] = [];
  let i = 0;

  while (i < code.length) {
    // Skip whitespace
    const wsMatch = code.slice(i).match(/^(\s+)/);
    if (wsMatch) {
      i += wsMatch[1].length;
      continue;
    }

    // Comment
    if (code.slice(i, i + 2) === '/*') {
      const end = code.indexOf('*/', i + 2);
      const comment = end !== -1 ? code.slice(i, end + 2) : code.slice(i);
      lines.push(indentStr.repeat(indent) + comment.trim());
      i = end !== -1 ? end + 2 : code.length;
      continue;
    }

    // Opening brace
    if (code[i] === '{') {
      if (lines.length > 0) {
        lines[lines.length - 1] += ' {';
      }
      indent++;
      i++;
      continue;
    }

    // Closing brace
    if (code[i] === '}') {
      indent = Math.max(0, indent - 1);
      lines.push(indentStr.repeat(indent) + '}');
      i++;
      continue;
    }

    // Semicolon
    if (code[i] === ';') {
      if (lines.length > 0) {
        lines[lines.length - 1] += ';';
      }
      i++;
      continue;
    }

    // Other content
    let j = i;
    while (j < code.length && code[j] !== '{' && code[j] !== '}' &&
           code[j] !== ';' && !code.slice(j, j + 2).match(/^\/\*/)) {
      j++;
    }

    if (j > i) {
      const content = code.slice(i, j).trim();
      if (content) {
        if (lines.length > 0 && lines[lines.length - 1].trim()) {
          lines[lines.length - 1] += ' ' + content;
        } else {
          lines.push(indentStr.repeat(indent) + content);
        }
      }
      i = j;
      continue;
    }

    i++;
  }

  return lines.join('\n');
}

/**
 * Find line number for an element by matching its opening tag in the source
 */
export function findElementSourceLine(
  tagName: string,
  attributes: Record<string, string>,
  sourceLines: string[]
): number | null {
  // Try to find the opening tag by matching tag name and key attributes
  for (let i = 0; i < sourceLines.length; i++) {
    const line = sourceLines[i];

    // Check if this line contains an opening tag with matching tag name
    const tagRegex = new RegExp(`<${tagName}[\\s>]`, 'i');
    if (!tagRegex.test(line)) continue;

    // Check for attribute matches
    let matchScore = 0;
    let requiredAttrs = 0;

    for (const key of Object.keys(attributes)) {
      // Skip data-line and other internal attributes
      if (key.startsWith('data-') || key === 'class') continue;

      requiredAttrs++;
      // Check if attribute exists in the line
      if (line.includes(`${key}=`) || line.includes(`${key} ="`)) {
        matchScore++;
      }
    }

    // If we matched enough attributes (or all if few), consider it a match
    if (requiredAttrs === 0 || matchScore >= Math.ceil(requiredAttrs * 0.5)) {
      return i;
    }
  }

  return null;
}