// hittable extension host.
//
// Runs VS Code extensions under plain Node and gives them a `vscode` module.
// Everything that needs the user interface (messages, pickers, edits, status
// bar, diagnostics, webviews) is forwarded to the Go app as JSON-RPC over
// stdin/stdout, one JSON object per line. Extensions' own stdout writes and
// console output are captured and sent as log messages, so they can never
// corrupt that channel.
'use strict';

const Module = require('module');
const path = require('path');
const fs = require('fs');
const fsp = require('fs/promises');
const os = require('os');
const util = require('util');
const crypto = require('crypto');
const readline = require('readline');

// ---------------------------------------------------------------- RPC

const rpcWrite = process.stdout.write.bind(process.stdout);
function send(msg) { rpcWrite(JSON.stringify(msg) + '\n'); }

function log(level, ...args) {
  const text = args.map(a => (typeof a === 'string' ? a : util.inspect(a, { depth: 4 }))).join(' ');
  send({ method: 'log', params: { level, text } });
}
process.stdout.write = function (chunk, enc, cb) {
  log('stdout', String(chunk).replace(/\n$/, ''));
  if (typeof enc === 'function') enc(); else if (typeof cb === 'function') cb();
  return true;
};
console.log = (...a) => log('info', ...a);
console.info = (...a) => log('info', ...a);
console.debug = (...a) => log('debug', ...a);
console.warn = (...a) => log('warn', ...a);
console.error = (...a) => log('error', ...a);
console.trace = (...a) => log('debug', ...a, new Error().stack);
process.on('uncaughtException', e => log('error', 'uncaught exception:', e && e.stack || String(e)));
process.on('unhandledRejection', e => log('error', 'unhandled rejection:', e && e.stack || String(e)));

let nextId = 1;
const pending = new Map();
function request(method, params) {
  const id = nextId++;
  send({ id, method, params });
  return new Promise((resolve, reject) => pending.set(id, { resolve, reject }));
}
function notify(method, params) { send({ method, params }); }

const handlers = Object.create(null);
const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
rl.on('line', line => {
  if (!line.trim()) return;
  let msg;
  try { msg = JSON.parse(line); } catch (e) { log('error', 'bad message from app:', line.slice(0, 200)); return; }
  if (msg.method) {
    const h = handlers[msg.method];
    if (msg.id === undefined || msg.id === null) {
      if (h) Promise.resolve().then(() => h(msg.params || {})).catch(e => log('error', msg.method, e && e.stack || e));
      return;
    }
    if (!h) { send({ id: msg.id, error: { message: 'unknown method ' + msg.method } }); return; }
    Promise.resolve().then(() => h(msg.params || {})).then(
      result => send({ id: msg.id, result: result === undefined ? null : result }),
      err => send({ id: msg.id, error: { message: String(err && err.message || err) } }));
    return;
  }
  const p = pending.get(msg.id);
  if (!p) return;
  pending.delete(msg.id);
  if (msg.error) p.reject(new Error(msg.error.message)); else p.resolve(msg.result);
});
// The app closing our stdin means it exited; never outlive it.
rl.on('close', () => { shutdown().finally(() => process.exit(0)); });

// ---------------------------------------------------------------- basics

class Disposable {
  constructor(fn) { this._fn = fn; }
  static from(...items) { return new Disposable(() => { for (const d of items) if (d && typeof d.dispose === 'function') d.dispose(); }); }
  dispose() { if (this._fn) { const f = this._fn; this._fn = undefined; try { f(); } catch (e) { log('error', e); } } }
}

class EventEmitter {
  constructor() { this._listeners = []; }
  get event() {
    if (!this._event) {
      this._event = (listener, thisArg, disposables) => {
        const entry = { listener, thisArg };
        this._listeners.push(entry);
        const d = new Disposable(() => { const i = this._listeners.indexOf(entry); if (i >= 0) this._listeners.splice(i, 1); });
        if (Array.isArray(disposables)) disposables.push(d);
        return d;
      };
    }
    return this._event;
  }
  fire(data) {
    for (const { listener, thisArg } of this._listeners.slice()) {
      try { listener.call(thisArg, data); } catch (e) { log('error', 'event listener failed:', e && e.stack || e); }
    }
  }
  dispose() { this._listeners = []; }
}
const noEvent = () => new Disposable(() => {});

class CancellationError extends Error { constructor() { super('Canceled'); this.name = 'Canceled'; } }
class CancellationTokenSource {
  constructor() {
    this._emitter = new EventEmitter();
    const self = this;
    this.token = { isCancellationRequested: false, onCancellationRequested: this._emitter.event };
    this._cancel = () => { if (!self.token.isCancellationRequested) { self.token.isCancellationRequested = true; self._emitter.fire(); } };
  }
  cancel() { this._cancel(); }
  dispose() { this._emitter.dispose(); }
}
const noToken = new CancellationTokenSource().token;

// ---------------------------------------------------------------- Uri

const isWin = process.platform === 'win32';
const uriRe = /^(([^:/?#]+?):)?(\/\/([^/?#]*))?([^?#]*)(\?([^#]*))?(#(.*))?/;
function dec(s) { try { return decodeURIComponent(s); } catch { return s; } }
function encPath(p) {
  return p.split('/').map(seg => encodeURIComponent(seg).replace(/[!'()*]/g, c => '%' + c.charCodeAt(0).toString(16).toUpperCase())).join('/');
}

class Uri {
  constructor(scheme, authority, p, query, fragment) {
    this.scheme = scheme || 'file';
    this.authority = authority || '';
    this.path = p || '';
    this.query = query || '';
    this.fragment = fragment || '';
    if (this.scheme === 'file' && this.path && this.path[0] !== '/') this.path = '/' + this.path;
  }
  static parse(value) {
    const m = uriRe.exec(String(value));
    if (!m) return new Uri('file', '', '', '', '');
    return new Uri(m[2] || 'file', dec(m[4] || ''), dec(m[5] || ''), dec(m[7] || ''), dec(m[9] || ''));
  }
  static file(p) {
    let authority = '';
    p = String(p);
    if (isWin) p = p.replace(/\\/g, '/');
    if (p.startsWith('//')) {
      const i = p.indexOf('/', 2);
      if (i === -1) { authority = p.slice(2); p = '/'; } else { authority = p.slice(2, i); p = p.slice(i) || '/'; }
    }
    if (p[0] !== '/') p = '/' + p;
    return new Uri('file', authority, p, '', '');
  }
  static from(c) { return new Uri(c.scheme, c.authority, c.path, c.query, c.fragment); }
  static joinPath(base, ...segs) { return base.with({ path: path.posix.join(base.path || '/', ...segs) }); }
  static isUri(x) { return x instanceof Uri; }
  get fsPath() {
    let p;
    if (this.authority && this.path.length > 1 && this.scheme === 'file') p = '//' + this.authority + this.path;
    else if (/^\/[a-zA-Z]:/.test(this.path)) p = this.path[1].toLowerCase() + this.path.slice(2);
    else p = this.path;
    if (isWin) p = p.replace(/\//g, '\\');
    return p;
  }
  with(c) {
    return new Uri(
      c.scheme !== undefined ? c.scheme : this.scheme,
      c.authority !== undefined ? c.authority : this.authority,
      c.path !== undefined ? c.path : this.path,
      c.query !== undefined ? c.query : this.query,
      c.fragment !== undefined ? c.fragment : this.fragment);
  }
  toString(skipEncoding) {
    let p = this.path;
    if (/^\/[a-zA-Z]:/.test(p)) p = '/' + p[1].toLowerCase() + p.slice(2);
    const ep = skipEncoding ? p : encPath(p);
    let s = this.scheme + ':';
    // The authority keeps its ':' (port) and '@' (user) unencoded, as VS Code does.
    if (this.authority || this.scheme === 'file') s += '//' + (skipEncoding ? this.authority : encodeURIComponent(this.authority).replace(/%3A/gi, ':').replace(/%40/g, '@'));
    s += ep;
    if (this.query) s += '?' + (skipEncoding ? this.query : encodeURIComponent(this.query));
    if (this.fragment) s += '#' + (skipEncoding ? this.fragment : encodeURIComponent(this.fragment));
    return s;
  }
  toJSON() { return { $mid: 1, scheme: this.scheme, authority: this.authority, path: this.path, query: this.query, fragment: this.fragment, fsPath: this.fsPath, external: this.toString() }; }
  static revive(d) { return d instanceof Uri ? d : d ? Uri.from(d) : d; }
}
function toUri(x) {
  if (x instanceof Uri) return x;
  if (typeof x === 'string') return /^[a-zA-Z][\w+.-]+:/.test(x) && !/^[a-zA-Z]:[\\/]/.test(x) ? Uri.parse(x) : Uri.file(x);
  if (x && typeof x === 'object' && x.scheme) return Uri.from(x);
  return x;
}
function uriKey(u) { u = toUri(u); return u.scheme === 'file' ? 'file:' + path.resolve(u.fsPath) : u.toString(); }

// ---------------------------------------------------------------- geometry

class Position {
  constructor(line, character) {
    if (line < 0 || character < 0) throw new Error('Illegal argument: line/character must be non-negative');
    this.line = line; this.character = character;
  }
  isBefore(o) { return this.line < o.line || (this.line === o.line && this.character < o.character); }
  isBeforeOrEqual(o) { return this.isBefore(o) || this.isEqual(o); }
  isAfter(o) { return !this.isBeforeOrEqual(o); }
  isAfterOrEqual(o) { return !this.isBefore(o); }
  isEqual(o) { return this.line === o.line && this.character === o.character; }
  compareTo(o) { return this.isBefore(o) ? -1 : this.isEqual(o) ? 0 : 1; }
  translate(a, b) {
    if (typeof a === 'object' && a) return new Position(this.line + (a.lineDelta || 0), this.character + (a.characterDelta || 0));
    return new Position(this.line + (a || 0), this.character + (b || 0));
  }
  with(a, b) {
    if (typeof a === 'object' && a) return new Position(a.line !== undefined ? a.line : this.line, a.character !== undefined ? a.character : this.character);
    return new Position(a !== undefined ? a : this.line, b !== undefined ? b : this.character);
  }
  toJSON() { return { line: this.line, character: this.character }; }
}
const P = p => p instanceof Position ? p : new Position(p.line, p.character);

class Range {
  constructor(a, b, c, d) {
    let s, e;
    if (typeof a === 'number') { s = new Position(a, b); e = new Position(c, d); } else { s = P(a); e = P(b); }
    if (e.isBefore(s)) { this.start = e; this.end = s; } else { this.start = s; this.end = e; }
  }
  get isEmpty() { return this.start.isEqual(this.end); }
  get isSingleLine() { return this.start.line === this.end.line; }
  contains(x) {
    if (x instanceof Range) return this.contains(x.start) && this.contains(x.end);
    x = P(x); return !x.isBefore(this.start) && !this.end.isBefore(x);
  }
  isEqual(o) { return this.start.isEqual(o.start) && this.end.isEqual(o.end); }
  intersection(o) {
    const s = this.start.isAfter(o.start) ? this.start : o.start;
    const e = this.end.isBefore(o.end) ? this.end : o.end;
    return s.isAfter(e) ? undefined : new Range(s, e);
  }
  union(o) { return new Range(this.start.isBefore(o.start) ? this.start : o.start, this.end.isAfter(o.end) ? this.end : o.end); }
  with(a, b) {
    if (a && !(a instanceof Position) && typeof a === 'object') return new Range(a.start || this.start, a.end || this.end);
    return new Range(a || this.start, b || this.end);
  }
  toJSON() { return { start: this.start.toJSON(), end: this.end.toJSON() }; }
}
const R = r => r instanceof Range ? r : new Range(P(r.start), P(r.end));

class Selection extends Range {
  constructor(a, b, c, d) {
    let anchor, active;
    if (typeof a === 'number') { anchor = new Position(a, b); active = new Position(c, d); } else { anchor = P(a); active = P(b); }
    super(anchor, active);
    this.anchor = anchor; this.active = active;
  }
  get isReversed() { return this.anchor.isAfter(this.active); }
}

class Location {
  constructor(uri, rangeOrPos) { this.uri = toUri(uri); this.range = rangeOrPos instanceof Position ? new Range(rangeOrPos, rangeOrPos) : rangeOrPos; }
}

const EndOfLine = { LF: 1, CRLF: 2 };
class TextEdit {
  constructor(range, newText) { this.range = range; this.newText = newText; }
  static replace(r, t) { return new TextEdit(r, t); }
  static insert(p, t) { return new TextEdit(new Range(p, p), t); }
  static delete(r) { return new TextEdit(r, ''); }
  static setEndOfLine(eol) { const e = new TextEdit(new Range(0, 0, 0, 0), ''); e.newEol = eol; return e; }
}
class SnippetString {
  constructor(value) { this.value = value || ''; this._tab = 1; }
  static isSnippetString(x) { return x instanceof SnippetString; }
  appendText(s) { this.value += String(s).replace(/[$}\\]/g, '\\$&'); return this; }
  appendTabstop(n) { this.value += '$' + (n === undefined ? this._tab++ : n); return this; }
  appendPlaceholder(v, n) {
    if (typeof v === 'function') { const nested = new SnippetString(); nested._tab = this._tab; v(nested); this._tab = nested._tab; v = nested.value; } else v = String(v).replace(/[$}\\]/g, '\\$&');
    this.value += '${' + (n === undefined ? this._tab++ : n) + ':' + v + '}'; return this;
  }
  appendChoice(vals, n) { this.value += '${' + (n === undefined ? this._tab++ : n) + '|' + vals.join(',') + '|}'; return this; }
  appendVariable(name, def) { this.value += '${' + name + (def ? ':' + def : '') + '}'; return this; }
}
class SnippetTextEdit { constructor(range, snippet) { this.range = range; this.snippet = snippet; } static replace(r, s) { return new SnippetTextEdit(r, s); } static insert(p, s) { return new SnippetTextEdit(new Range(p, p), s); } }

class WorkspaceEdit {
  constructor() { this._entries = []; this._files = []; }
  _list(uri) { const k = uriKey(uri); let e = this._entries.find(x => x.key === k); if (!e) { e = { key: k, uri: toUri(uri), edits: [] }; this._entries.push(e); } return e.edits; }
  replace(uri, range, newText) { this._list(uri).push(new TextEdit(range, newText)); }
  insert(uri, pos, newText) { this._list(uri).push(new TextEdit(new Range(pos, pos), newText)); }
  delete(uri, range) { this._list(uri).push(new TextEdit(range, '')); }
  set(uri, edits) {
    const list = this._list(uri); list.length = 0;
    for (const e of edits || []) { const x = Array.isArray(e) ? e[0] : e; if (x) list.push(x instanceof SnippetTextEdit ? Object.assign(new TextEdit(x.range, x.snippet.value), { snippet: true }) : x); }
  }
  has(uri) { const k = uriKey(uri); return this._entries.some(e => e.key === k); }
  get(uri) { const k = uriKey(uri); const e = this._entries.find(x => x.key === k); return e ? e.edits.slice() : []; }
  entries() { return this._entries.map(e => [e.uri, e.edits.slice()]); }
  get size() { return this._entries.length + this._files.length; }
  createFile(uri, opts) { this._files.push({ op: 'create', path: toUri(uri).fsPath, overwrite: !!(opts && opts.overwrite), ignoreIfExists: !!(opts && opts.ignoreIfExists), contents: opts && opts.contents }); }
  deleteFile(uri, opts) { this._files.push({ op: 'delete', path: toUri(uri).fsPath, recursive: !!(opts && opts.recursive), ignoreIfNotExists: !!(opts && opts.ignoreIfNotExists) }); }
  renameFile(a, b, opts) { this._files.push({ op: 'rename', path: toUri(a).fsPath, to: toUri(b).fsPath, overwrite: !!(opts && opts.overwrite) }); }
}

// ---------------------------------------------------------------- language types

const DiagnosticSeverity = { Error: 0, Warning: 1, Information: 2, Hint: 3 };
const DiagnosticTag = { Unnecessary: 1, Deprecated: 2 };
class Diagnostic { constructor(range, message, severity) { this.range = range; this.message = message; this.severity = severity === undefined ? DiagnosticSeverity.Error : severity; } }
class DiagnosticRelatedInformation { constructor(location, message) { this.location = location; this.message = message; } }

const CompletionItemKind = { Text: 0, Method: 1, Function: 2, Constructor: 3, Field: 4, Variable: 5, Class: 6, Interface: 7, Module: 8, Property: 9, Unit: 10, Value: 11, Enum: 12, Keyword: 13, Snippet: 14, Color: 15, File: 16, Reference: 17, Folder: 18, EnumMember: 19, Constant: 20, Struct: 21, Event: 22, Operator: 23, TypeParameter: 24, User: 25, Issue: 26 };
const CompletionItemTag = { Deprecated: 1 };
const CompletionTriggerKind = { Invoke: 0, TriggerCharacter: 1, TriggerForIncompleteCompletions: 2 };
class CompletionItem { constructor(label, kind) { this.label = label; this.kind = kind; } }
class CompletionList { constructor(items, isIncomplete) { this.items = items || []; this.isIncomplete = !!isIncomplete; } }

class MarkdownString {
  constructor(value, supportThemeIcons) { this.value = value || ''; this.supportThemeIcons = !!supportThemeIcons; this.isTrusted = false; this.supportHtml = false; }
  appendText(s) { this.value += String(s).replace(/[\\`*_{}[\]()#+\-.!]/g, '\\$&').replace(/\n/g, '\n\n'); return this; }
  appendMarkdown(s) { this.value += s; return this; }
  appendCodeblock(code, lang) { this.value += '\n```' + (lang || '') + '\n' + code + '\n```\n'; return this; }
}
class Hover { constructor(contents, range) { this.contents = Array.isArray(contents) ? contents : [contents]; this.range = range; } }
class VerboseHover extends Hover {}
const SymbolKind = { File: 0, Module: 1, Namespace: 2, Package: 3, Class: 4, Method: 5, Property: 6, Field: 7, Constructor: 8, Enum: 9, Interface: 10, Function: 11, Variable: 12, Constant: 13, String: 14, Number: 15, Boolean: 16, Array: 17, Object: 18, Key: 19, Null: 20, EnumMember: 21, Struct: 22, Event: 23, Operator: 24, TypeParameter: 25 };
const SymbolTag = { Deprecated: 1 };
class SymbolInformation {
  constructor(name, kind, a, b, c) {
    this.name = name; this.kind = kind;
    if (a instanceof Location) { this.location = a; this.containerName = b; } else { this.containerName = a; this.location = new Location(b, c); }
  }
}
class DocumentSymbol { constructor(name, detail, kind, range, selectionRange) { Object.assign(this, { name, detail, kind, range, selectionRange, children: [] }); } }
class CodeActionKind {
  constructor(value) { this.value = value; }
  append(p) { return new CodeActionKind(this.value ? this.value + '.' + p : p); }
  intersects(o) { return this.contains(o) || o.contains(this); }
  contains(o) { return this.value === o.value || o.value.startsWith(this.value + '.') || this.value === ''; }
}
CodeActionKind.Empty = new CodeActionKind('');
for (const [k, v] of Object.entries({ QuickFix: 'quickfix', Refactor: 'refactor', RefactorExtract: 'refactor.extract', RefactorInline: 'refactor.inline', RefactorMove: 'refactor.move', RefactorRewrite: 'refactor.rewrite', Source: 'source', SourceOrganizeImports: 'source.organizeImports', SourceFixAll: 'source.fixAll', Notebook: 'notebook' })) CodeActionKind[k] = new CodeActionKind(v);
const CodeActionTriggerKind = { Invoke: 1, Automatic: 2 };
class CodeAction { constructor(title, kind) { this.title = title; this.kind = kind; } }
class CodeLens { constructor(range, command) { this.range = range; this.command = command; } get isResolved() { return !!this.command; } }
class SignatureHelp { constructor() { this.signatures = []; this.activeSignature = 0; this.activeParameter = 0; } }
class SignatureInformation { constructor(label, documentation) { this.label = label; this.documentation = documentation; this.parameters = []; } }
class ParameterInformation { constructor(label, documentation) { this.label = label; this.documentation = documentation; } }
const SignatureHelpTriggerKind = { Invoke: 1, TriggerCharacter: 2, ContentChange: 3 };
const DocumentHighlightKind = { Text: 0, Read: 1, Write: 2 };
class DocumentHighlight { constructor(range, kind) { this.range = range; this.kind = kind === undefined ? DocumentHighlightKind.Text : kind; } }
class DocumentLink { constructor(range, target) { this.range = range; this.target = target; } }
class Color { constructor(r, g, b, a) { Object.assign(this, { red: r, green: g, blue: b, alpha: a }); } }
class ColorInformation { constructor(range, color) { this.range = range; this.color = color; } }
class ColorPresentation { constructor(label) { this.label = label; } }
const FoldingRangeKind = { Comment: 1, Imports: 2, Region: 3 };
class FoldingRange { constructor(start, end, kind) { Object.assign(this, { start, end, kind }); } }
class SelectionRange { constructor(range, parent) { this.range = range; this.parent = parent; } }
class CallHierarchyItem { constructor(kind, name, detail, uri, range, selectionRange) { Object.assign(this, { kind, name, detail, uri, range, selectionRange }); } }
class CallHierarchyIncomingCall { constructor(from, fromRanges) { this.from = from; this.fromRanges = fromRanges; } }
class CallHierarchyOutgoingCall { constructor(to, fromRanges) { this.to = to; this.fromRanges = fromRanges; } }
class TypeHierarchyItem { constructor(kind, name, detail, uri, range, selectionRange) { Object.assign(this, { kind, name, detail, uri, range, selectionRange }); } }
class SemanticTokensLegend { constructor(tokenTypes, tokenModifiers) { this.tokenTypes = tokenTypes; this.tokenModifiers = tokenModifiers || []; } }
class SemanticTokens { constructor(data, resultId) { this.data = data; this.resultId = resultId; } }
class SemanticTokensEdit { constructor(start, deleteCount, data) { Object.assign(this, { start, deleteCount, data }); } }
class SemanticTokensEdits { constructor(edits, resultId) { this.edits = edits; this.resultId = resultId; } }
class SemanticTokensBuilder {
  constructor(legend) { this._legend = legend; this._data = []; this._prevLine = 0; this._prevChar = 0; }
  push(a, b, c, d, e) {
    let line, ch, len, type, mods = 0;
    if (typeof a === 'number') { line = a; ch = b; len = c; type = d; mods = e || 0; } else {
      const r = a; line = r.start.line; ch = r.start.character; len = r.end.character - r.start.character;
      type = this._legend.tokenTypes.indexOf(b); for (const m of c || []) { const i = this._legend.tokenModifiers.indexOf(m); if (i >= 0) mods |= 1 << i; }
    }
    const dl = line - this._prevLine; const dc = dl === 0 ? ch - this._prevChar : ch;
    this._data.push(dl, dc, len, type, mods); this._prevLine = line; this._prevChar = ch;
  }
  build(resultId) { return new SemanticTokens(new Uint32Array(this._data), resultId); }
}
class LinkedEditingRanges { constructor(ranges, wordPattern) { this.ranges = ranges; this.wordPattern = wordPattern; } }
class InlineValueText { constructor(range, text) { this.range = range; this.text = text; } }
class InlineValueVariableLookup { constructor(range, variableName, caseSensitiveLookup) { Object.assign(this, { range, variableName, caseSensitiveLookup }); } }
class InlineValueEvaluatableExpression { constructor(range, expression) { this.range = range; this.expression = expression; } }
class EvaluatableExpression { constructor(range, expression) { this.range = range; this.expression = expression; } }
const InlayHintKind = { Type: 1, Parameter: 2 };
class InlayHint { constructor(position, label, kind) { Object.assign(this, { position, label, kind }); } }
class InlayHintLabelPart { constructor(value) { this.value = value; } }
class InlineCompletionItem { constructor(insertText, range, command) { Object.assign(this, { insertText, range, command }); } }
class InlineCompletionList { constructor(items) { this.items = items; } }
const InlineCompletionTriggerKind = { Invoke: 0, Automatic: 1 };
class DocumentDropEdit { constructor(insertText) { this.insertText = insertText; } }
class DocumentPasteEdit { constructor(insertText, title, kind) { Object.assign(this, { insertText, title, kind }); } }
class DocumentDropOrPasteEditKind { constructor(value) { this.value = value; } append(p) { return new DocumentDropOrPasteEditKind(this.value + '.' + p); } }
DocumentDropOrPasteEditKind.Empty = new DocumentDropOrPasteEditKind('');
DocumentDropOrPasteEditKind.Text = new DocumentDropOrPasteEditKind('text');
class DataTransferItem { constructor(value) { this.value = value; } asString() { return Promise.resolve(typeof this.value === 'string' ? this.value : JSON.stringify(this.value)); } asFile() { return undefined; } }
class DataTransfer { constructor() { this._m = new Map(); } get(m) { return this._m.get(m); } set(m, v) { this._m.set(m, v); } forEach(f, t) { this._m.forEach((v, k) => f.call(t, v, k, this)); } [Symbol.iterator]() { return this._m.entries(); } }

class ThemeColor { constructor(id) { this.id = id; } }
class ThemeIcon { constructor(id, color) { this.id = id; this.color = color; } }
ThemeIcon.File = new ThemeIcon('file'); ThemeIcon.Folder = new ThemeIcon('folder');
const TreeItemCollapsibleState = { None: 0, Collapsed: 1, Expanded: 2 };
class TreeItem {
  constructor(a, collapsibleState) {
    if (a instanceof Uri) this.resourceUri = a; else this.label = a;
    this.collapsibleState = collapsibleState === undefined ? TreeItemCollapsibleState.None : collapsibleState;
  }
}
class TreeItemCheckboxState {}
TreeItemCheckboxState.Unchecked = 0; TreeItemCheckboxState.Checked = 1;

class RelativePattern {
  constructor(base, pattern) {
    if (typeof base === 'string') { this.baseUri = Uri.file(base); } else if (base && base.uri) { this.baseUri = base.uri; } else this.baseUri = toUri(base);
    this.base = this.baseUri.fsPath; this.pattern = pattern;
  }
}

// Task, notebook, test and other data classes: present so `instanceof` and
// `new` work in code that only touches them conditionally.
class Task { constructor(def, scope, name, source, execution, problemMatchers) { Object.assign(this, { definition: def, scope, name, source, execution, problemMatchers, isBackground: false, presentationOptions: {}, runOptions: {} }); } }
class ShellExecution { constructor(a, b, c) { Object.assign(this, { commandLine: typeof a === 'string' && !Array.isArray(b) ? a : undefined, command: a, args: Array.isArray(b) ? b : undefined, options: Array.isArray(b) ? c : b }); } }
class ProcessExecution { constructor(process, a, b) { Object.assign(this, { process, args: Array.isArray(a) ? a : [], options: Array.isArray(a) ? b : a }); } }
class CustomExecution { constructor(cb) { this.callback = cb; } }
class TaskGroup { constructor(id, label) { this.id = id; this.label = label; } }
TaskGroup.Clean = new TaskGroup('clean', 'Clean'); TaskGroup.Build = new TaskGroup('build', 'Build'); TaskGroup.Rebuild = new TaskGroup('rebuild', 'Rebuild'); TaskGroup.Test = new TaskGroup('test', 'Test');
class NotebookRange { constructor(start, end) { this.start = start; this.end = end; } get isEmpty() { return this.start === this.end; } }
class NotebookCellData { constructor(kind, value, languageId) { Object.assign(this, { kind, value, languageId }); } }
class NotebookData { constructor(cells) { this.cells = cells; } }
class NotebookCellOutputItem { constructor(data, mime) { this.data = data; this.mime = mime; } static text(v, m) { return new NotebookCellOutputItem(Buffer.from(String(v)), m || 'text/plain'); } static json(v, m) { return new NotebookCellOutputItem(Buffer.from(JSON.stringify(v)), m || 'text/x-json'); } static error(e) { return NotebookCellOutputItem.json({ name: e.name, message: e.message, stack: e.stack }, 'application/vnd.code.notebook.error'); } }
class NotebookCellOutput { constructor(items, metadata) { this.items = items; this.metadata = metadata; } }
class NotebookEdit { static replaceCells() { return new NotebookEdit(); } static insertCells() { return new NotebookEdit(); } static deleteCells() { return new NotebookEdit(); } static updateCellMetadata() { return new NotebookEdit(); } static updateNotebookMetadata() { return new NotebookEdit(); } }
class TestMessage { constructor(message) { this.message = message; } static diff(message, expected, actual) { return Object.assign(new TestMessage(message), { expectedOutput: expected, actualOutput: actual }); } }
class TestTag { constructor(id) { this.id = id; } }
class TestRunRequest { constructor(include, exclude, profile) { Object.assign(this, { include, exclude, profile }); } }
class TabInputText { constructor(uri) { this.uri = uri; } }
class TabInputTextDiff { constructor(original, modified) { this.original = original; this.modified = modified; } }
class TabInputCustom { constructor(uri, viewType) { this.uri = uri; this.viewType = viewType; } }
class TabInputWebview { constructor(viewType) { this.viewType = viewType; } }
class TabInputNotebook { constructor(uri, notebookType) { this.uri = uri; this.notebookType = notebookType; } }
class TabInputNotebookDiff { constructor(original, modified, notebookType) { Object.assign(this, { original, modified, notebookType }); } }
class TabInputTerminal {}
class TerminalLink { constructor(startIndex, length, tooltip) { Object.assign(this, { startIndex, length, tooltip }); } }
class TerminalProfile { constructor(options) { this.options = options; } }
class FileDecoration { constructor(badge, tooltip, color) { Object.assign(this, { badge, tooltip, color }); } }
class QuickInputButtons {}
QuickInputButtons.Back = { iconPath: new ThemeIcon('arrow-left'), tooltip: 'Back' };
class LanguageModelChatMessage { static User(c) { return { role: 1, content: c }; } static Assistant(c) { return { role: 2, content: c }; } }
class LanguageModelTextPart { constructor(value) { this.value = value; } }
class LanguageModelError extends Error { static NotFound(m) { return new LanguageModelError(m); } static NoPermissions(m) { return new LanguageModelError(m); } static Blocked(m) { return new LanguageModelError(m); } }

class FileSystemError extends Error {
  constructor(m, code) { super(m instanceof Uri ? m.toString() : String(m || '')); this.code = code || 'Unknown'; this.name = (code || 'Unknown') + ' (FileSystemError)'; }
  static FileNotFound(m) { return new FileSystemError(m, 'FileNotFound'); }
  static FileExists(m) { return new FileSystemError(m, 'FileExists'); }
  static FileNotADirectory(m) { return new FileSystemError(m, 'FileNotADirectory'); }
  static FileIsADirectory(m) { return new FileSystemError(m, 'FileIsADirectory'); }
  static NoPermissions(m) { return new FileSystemError(m, 'NoPermissions'); }
  static Unavailable(m) { return new FileSystemError(m, 'Unavailable'); }
}
const FileType = { Unknown: 0, File: 1, Directory: 2, SymbolicLink: 64 };
const FilePermission = { Readonly: 1 };
const FileChangeType = { Changed: 1, Created: 2, Deleted: 3 };

const enums = {
  StatusBarAlignment: { Left: 1, Right: 2 },
  ViewColumn: { Active: -1, Beside: -2, One: 1, Two: 2, Three: 3, Four: 4, Five: 5, Six: 6, Seven: 7, Eight: 8, Nine: 9 },
  TextEditorRevealType: { Default: 0, InCenter: 1, InCenterIfOutsideViewport: 2, AtTop: 3 },
  TextEditorCursorStyle: { Line: 1, Block: 2, Underline: 3, LineThin: 4, BlockOutline: 5, UnderlineThin: 6 },
  TextEditorLineNumbersStyle: { Off: 0, On: 1, Relative: 2, Interval: 3 },
  TextEditorSelectionChangeKind: { Keyboard: 1, Mouse: 2, Command: 3 },
  DecorationRangeBehavior: { OpenOpen: 0, ClosedClosed: 1, OpenClosed: 2, ClosedOpen: 3 },
  OverviewRulerLane: { Left: 1, Center: 2, Right: 4, Full: 7 },
  ProgressLocation: { SourceControl: 1, Window: 10, Notification: 15 },
  ConfigurationTarget: { Global: 1, Workspace: 2, WorkspaceFolder: 3 },
  UIKind: { Desktop: 1, Web: 2 },
  ExtensionKind: { UI: 1, Workspace: 2 },
  ExtensionMode: { Production: 1, Development: 2, Test: 3 },
  LogLevel: { Off: 0, Trace: 1, Debug: 2, Info: 3, Warning: 4, Error: 5 },
  ColorThemeKind: { Light: 1, Dark: 2, HighContrast: 3, HighContrastLight: 4 },
  QuickPickItemKind: { Separator: -1, Default: 0 },
  TextDocumentSaveReason: { Manual: 1, AfterDelay: 2, FocusOut: 3 },
  TextDocumentChangeReason: { Undo: 1, Redo: 2 },
  LanguageStatusSeverity: { Information: 0, Warning: 1, Error: 2 },
  InputBoxValidationSeverity: { Info: 1, Warning: 2, Error: 3 },
  TerminalLocation: { Panel: 1, Editor: 2 },
  TerminalExitReason: { Unknown: 0, Shutdown: 1, Process: 2, User: 3, Extension: 4 },
  TerminalShellExecutionCommandLineConfidence: { Low: 0, Medium: 1, High: 2 },
  EnvironmentVariableMutatorType: { Replace: 1, Append: 2, Prepend: 3 },
  TaskRevealKind: { Always: 1, Silent: 2, Never: 3 },
  TaskPanelKind: { Shared: 1, Dedicated: 2, New: 3 },
  TaskScope: { Global: 1, Workspace: 2 },
  ShellQuoting: { Escape: 1, Strong: 2, Weak: 3 },
  NotebookCellKind: { Markup: 1, Code: 2 },
  NotebookCellStatusBarAlignment: { Left: 1, Right: 2 },
  NotebookEditorRevealType: { Default: 0, InCenter: 1, InCenterIfOutsideViewport: 2, AtTop: 3 },
  NotebookControllerAffinity: { Default: 1, Preferred: 2 },
  CommentMode: { Editing: 0, Preview: 1 },
  CommentThreadCollapsibleState: { Collapsed: 0, Expanded: 1 },
  CommentThreadState: { Unresolved: 0, Resolved: 1 },
  TestRunProfileKind: { Run: 1, Debug: 2, Coverage: 3 },
  TestResultState: { Queued: 1, Running: 2, Passed: 3, Failed: 4, Skipped: 5, Errored: 6 },
  DebugConsoleMode: { Separate: 0, MergeWithParent: 1 },
  DebugConfigurationProviderTriggerKind: { Initial: 1, Dynamic: 2 },
  SyntaxTokenType: { Other: 0, Comment: 1, String: 2, RegEx: 3 },
  IndentAction: { None: 0, Indent: 1, IndentOutdent: 2, Outdent: 3 },
  PortAutoForwardAction: { Notify: 1, OpenBrowser: 2, OpenPreview: 3, Silent: 4, Ignore: 5 },
  ChatResultFeedbackKind: { Unhelpful: 0, Helpful: 1 },
  LanguageModelChatMessageRole: { User: 1, Assistant: 2 },
  CompletionItemLabelDetails: undefined,
};

// ---------------------------------------------------------------- glob

const globCache = new Map();
function globToRegExp(glob) {
  let g = String(glob);
  if (globCache.has(g)) return globCache.get(g);
  let re = '';
  for (let i = 0; i < g.length; i++) {
    const c = g[i];
    if (c === '*') {
      if (g[i + 1] === '*') {
        i++;
        if (g[i + 1] === '/') { i++; re += '(?:.*/)?'; } else re += '.*';
      } else re += '[^/]*';
    } else if (c === '?') re += '[^/]';
    else if (c === '{') {
      const end = g.indexOf('}', i);
      if (end < 0) { re += '\\{'; continue; }
      re += '(?:' + g.slice(i + 1, end).split(',').map(s => globToRegExp(s).source.replace(/^\^|\$$/g, '')).join('|') + ')';
      i = end;
    } else if (c === '[') {
      const end = g.indexOf(']', i);
      if (end < 0) { re += '\\['; continue; }
      re += '[' + g.slice(i + 1, end).replace(/^!/, '^') + ']';
      i = end;
    } else re += /[.+^$()|\\]/.test(c) ? '\\' + c : c;
  }
  const r = new RegExp('^' + re + '$', isWin ? 'i' : '');
  globCache.set(g, r);
  return r;
}
// globMatch tests a path against a glob string or RelativePattern. A bare
// pattern without a slash matches the file name anywhere, as VS Code does
// for "*.yaml"-style document selectors.
function globMatch(pattern, fsPath) {
  let base = null, pat = pattern;
  if (pattern && typeof pattern === 'object') { base = pattern.baseUri ? pattern.baseUri.fsPath : pattern.base; pat = pattern.pattern; }
  let p = fsPath.replace(/\\/g, '/');
  if (base) {
    const b = base.replace(/\\/g, '/').replace(/\/$/, '');
    if (!p.startsWith(b + '/')) return false;
    p = p.slice(b.length + 1);
    return globToRegExp(pat).test(p);
  }
  const re = globToRegExp(pat);
  if (re.test(p)) return true;
  if (!String(pat).includes('/')) return re.test(p.slice(p.lastIndexOf('/') + 1));
  return re.test(p.replace(/^\//, '')) || globToRegExp('**/' + pat).test(p);
}

// ---------------------------------------------------------------- state

const S = {
  root: '',
  rootUri: null,
  folders: [],
  exts: [],           // { id, path, manifest, nls, active, exports, activating, contributedEvents }
  settings: { user: {}, workspace: {} },
  defaults: {},
  docs: new Map(),    // uriKey -> Doc
  editors: [],
  activeEditor: undefined,
  commands: new Map(),// id -> { fn, thisArg, ext }
  providers: Object.create(null), // kind -> [{ selector, provider, extra, ext }]
  diagnostics: new Map(), // owner -> Map(uriKey -> {uri, list})
  contexts: Object.create(null),
  trees: new Map(),   // viewId -> { provider, handles: Map, view }
  webviewViews: new Map(), // viewId -> provider
  webviews: new Map(),// handle -> panel record
  contentProviders: new Map(), // scheme -> provider
  fsProviders: new Map(),
  uriHandlers: [],
  webviewBase: '',
  extData: '',
  machineId: '',
  workspaceHash: '',
  languages: new Set(),
  caches: new Map(),  // result id -> { items, providers }
  tokens: new Map(),  // request token -> CancellationTokenSource
  warned: new Set(),
  statusSeq: 0,
};

const ev = {
  openDoc: new EventEmitter(), closeDoc: new EventEmitter(), changeDoc: new EventEmitter(), saveDoc: new EventEmitter(), willSaveDoc: new EventEmitter(),
  activeEditor: new EventEmitter(), visibleEditors: new EventEmitter(), selection: new EventEmitter(), visibleRanges: new EventEmitter(), editorOptions: new EventEmitter(),
  config: new EventEmitter(), diagnostics: new EventEmitter(), folders: new EventEmitter(), extensions: new EventEmitter(),
  createFiles: new EventEmitter(), deleteFiles: new EventEmitter(), renameFiles: new EventEmitter(),
  willCreateFiles: new EventEmitter(), willDeleteFiles: new EventEmitter(), willRenameFiles: new EventEmitter(),
  tabs: new EventEmitter(), tabGroups: new EventEmitter(), openTerminal: new EventEmitter(), closeTerminal: new EventEmitter(),
  windowState: new EventEmitter(), theme: new EventEmitter(), trust: new EventEmitter(),
};

function warnOnce(key, ext) {
  const k = (ext ? ext.id + ':' : '') + key;
  if (S.warned.has(k)) return;
  S.warned.add(k);
  log('warn', `${ext ? ext.id : 'extension'} used vscode.${key}, which hittable does not support yet`);
}

// ---------------------------------------------------------------- documents

class Doc {
  constructor(uri, languageId, version, text) {
    this.uri = uri; this.languageId = languageId; this.version = version; this.isClosed = false; this.isUntitled = uri.scheme === 'untitled';
    this.isDirty = false; this.encoding = 'utf8';
    this._set(text);
  }
  _set(text) {
    this._text = text;
    this._lines = text.split(/\r\n|\r|\n/);
    this.eol = text.includes('\r\n') ? EndOfLine.CRLF : EndOfLine.LF;
    this._starts = [0];
    const re = /\r\n|\r|\n/g; let m;
    while ((m = re.exec(text))) this._starts.push(m.index + m[0].length);
  }
  get fileName() { return this.uri.fsPath; }
  get lineCount() { return this._lines.length; }
  getText(range) {
    if (!range) return this._text;
    const r = this.validateRange(R(range));
    return this._text.slice(this.offsetAt(r.start), this.offsetAt(r.end));
  }
  lineAt(x) {
    const line = typeof x === 'number' ? x : x.line;
    if (line < 0 || line >= this._lines.length) throw new Error('Illegal value for `line`');
    const text = this._lines[line];
    const fnw = text.search(/\S/);
    const nextStart = line + 1 < this._starts.length ? this._starts[line + 1] : this._text.length;
    return {
      lineNumber: line, text,
      range: new Range(line, 0, line, text.length),
      rangeIncludingLineBreak: line + 1 < this._lines.length ? new Range(line, 0, line + 1, 0) : new Range(line, 0, line, text.length),
      firstNonWhitespaceCharacterIndex: fnw < 0 ? text.length : fnw,
      isEmptyOrWhitespace: fnw < 0,
      _end: nextStart,
    };
  }
  offsetAt(pos) {
    pos = this.validatePosition(P(pos));
    return this._starts[pos.line] + pos.character;
  }
  positionAt(offset) {
    offset = Math.max(0, Math.min(Math.floor(offset), this._text.length));
    let lo = 0, hi = this._starts.length - 1;
    while (lo < hi) { const mid = (lo + hi + 1) >> 1; if (this._starts[mid] <= offset) lo = mid; else hi = mid - 1; }
    return new Position(lo, Math.min(offset - this._starts[lo], this._lines[lo].length));
  }
  validatePosition(p) {
    if (p.line < 0) return new Position(0, 0);
    if (p.line >= this._lines.length) { const l = this._lines.length - 1; return new Position(l, this._lines[l].length); }
    const len = this._lines[p.line].length;
    return p.character > len ? new Position(p.line, len) : p.character < 0 ? new Position(p.line, 0) : p;
  }
  validateRange(r) { return new Range(this.validatePosition(r.start), this.validatePosition(r.end)); }
  getWordRangeAtPosition(pos, regex) {
    pos = this.validatePosition(P(pos));
    const text = this._lines[pos.line];
    let re = regex || wordPatternFor(this.languageId);
    re = new RegExp(re.source, re.flags.includes('g') ? re.flags : re.flags + 'g');
    let m;
    while ((m = re.exec(text))) {
      const s = m.index, e = s + m[0].length;
      if (s <= pos.character && pos.character <= e && e > s) return new Range(pos.line, s, pos.line, e);
      if (m[0].length === 0) re.lastIndex++;
    }
    return undefined;
  }
  save() { return request('workspace/save', { path: this.uri.fsPath }).then(() => true, () => false); }
}
const languageConfigs = new Map();
function wordPatternFor(lang) {
  const c = languageConfigs.get(lang);
  return (c && c.wordPattern) || /(-?\d*\.\d\w*)|([^`~!@#$%^&*()\-=+[{\]}\\|;:'",.<>/?\s]+)/g;
}

function docFor(uri) { return S.docs.get(uriKey(uri)); }

function makeEditor(doc, selections) {
  const ed = {
    document: doc,
    selections: selections && selections.length ? selections : [new Selection(0, 0, 0, 0)],
    get selection() { return this.selections[0]; },
    set selection(s) { this.selections = [s]; },
    visibleRanges: [new Range(0, 0, Math.max(0, doc.lineCount - 1), 0)],
    options: { tabSize: 2, insertSpaces: true, cursorStyle: 2, lineNumbers: 1 },
    viewColumn: 1,
    async edit(cb) {
      const edits = [];
      const builder = {
        replace: (loc, text) => edits.push({ range: loc instanceof Position ? new Range(loc, loc) : R(loc), newText: text }),
        insert: (p, text) => edits.push({ range: new Range(p, p), newText: text }),
        delete: (loc) => edits.push({ range: R(loc), newText: '' }),
        setEndOfLine: () => {},
      };
      cb(builder);
      const we = new WorkspaceEdit();
      we.set(doc.uri, edits.map(e => new TextEdit(e.range, e.newText)));
      return applyWorkspaceEdit(we);
    },
    async insertSnippet(snippet, where) {
      const value = snippet instanceof SnippetString ? snippet.value : String(snippet);
      const targets = where === undefined ? this.selections : Array.isArray(where) ? where : [where];
      const edits = targets.map(t => ({ path: doc.uri.fsPath, range: rangeOut(t instanceof Position ? new Range(t, t) : R(t)), newText: value, snippet: true }));
      return !!(await request('workspace/applyEdit', { edits, files: [] }));
    },
    setDecorations(type, items) {
      const ranges = (items || []).map(x => rangeOut(x instanceof Range ? x : R(x.range)));
      notify('editor/decorations', { path: doc.uri.fsPath, key: type.key, ranges, options: type._options });
    },
    revealRange(range) { notify('editor/reveal', { path: doc.uri.fsPath, range: rangeOut(R(range)) }); },
    show() { request('window/showTextDocument', { path: doc.uri.fsPath }).catch(() => {}); },
    hide() {},
  };
  return ed;
}

function rangeOut(r) { return { start: { line: r.start.line, character: r.start.character }, end: { line: r.end.line, character: r.end.character } }; }
function posOut(p) { return { line: p.line, character: p.character }; }

async function applyWorkspaceEdit(we) {
  const edits = [];
  for (const [uri, list] of we.entries()) {
    for (const e of list) {
      if (!e || !e.range) continue;
      edits.push({ path: toUri(uri).fsPath, range: rangeOut(R(e.range)), newText: e.newText !== undefined ? e.newText : (e.snippet && e.snippet.value) || '', snippet: !!e.snippet || e instanceof SnippetTextEdit });
    }
  }
  const files = (we._files || []).map(f => Object.assign({}, f, { contents: f.contents ? Buffer.from(f.contents).toString('base64') : undefined }));
  if (!edits.length && !files.length) return true;
  try { return !!(await request('workspace/applyEdit', { edits, files })); } catch (e) { log('error', 'applyEdit failed:', e.message); return false; }
}

// ---------------------------------------------------------------- settings

function flattenSettings(obj, prefix, out) {
  for (const [k, v] of Object.entries(obj || {})) {
    if (k.startsWith('[')) { out[k] = v; continue; } // language overrides stay nested
    out[prefix + k] = v;
  }
  return out;
}
function settingsLayers() { return [flattenSettings(S.settings.workspace, '', {}), flattenSettings(S.settings.user, '', {})]; }

function collectDefaults() {
  const d = {};
  for (const ext of S.exts) {
    let conf = ext.manifest.contributes && ext.manifest.contributes.configuration;
    if (!conf) continue;
    if (!Array.isArray(conf)) conf = [conf];
    for (const c of conf) for (const [k, v] of Object.entries((c && c.properties) || {})) if (v && 'default' in v) d[k] = v.default;
  }
  S.defaults = d;
}

function lookup(key, lang) {
  const layers = settingsLayers();
  if (lang) {
    for (const layer of layers) { const o = layer['[' + lang + ']']; if (o && key in o) return clone(o[key]); }
  }
  for (const layer of layers) if (key in layer) return mergeObjects(clone(layer[key]), key, layers);
  // A section asked for as a whole: assemble it from its dotted children.
  const prefix = key + '.';
  let found = false; const obj = {};
  const sources = [S.defaults, ...layers.slice().reverse()];
  for (const src of sources) for (const [k, v] of Object.entries(src)) {
    if (k.startsWith(prefix)) { setPath(obj, k.slice(prefix.length), clone(v)); found = true; }
  }
  if (key in S.defaults) {
    const dv = clone(S.defaults[key]);
    if (found && dv && typeof dv === 'object' && !Array.isArray(dv)) return Object.assign(dv, obj);
    return dv;
  }
  return found ? obj : undefined;
}
function mergeObjects(v, key, layers) {
  const dv = S.defaults[key];
  if (v && dv && typeof v === 'object' && typeof dv === 'object' && !Array.isArray(v) && !Array.isArray(dv)) return Object.assign(clone(dv), v);
  return v;
}
function setPath(obj, dotted, v) {
  const parts = dotted.split('.'); let o = obj;
  for (let i = 0; i < parts.length - 1; i++) { if (typeof o[parts[i]] !== 'object' || o[parts[i]] === null) o[parts[i]] = {}; o = o[parts[i]]; }
  o[parts[parts.length - 1]] = v;
}
function clone(v) { return v === undefined ? v : JSON.parse(JSON.stringify(v)); }

function getConfiguration(section, scope) {
  let lang;
  if (scope && scope.languageId) lang = scope.languageId;
  else if (scope && scope.uri) { const d = docFor(scope.uri); if (d) lang = d.languageId; }
  else if (scope instanceof Uri) { const d = docFor(scope); if (d) lang = d.languageId; }
  const full = k => (section ? section + (k ? '.' + k : '') : k);
  const base = section ? lookup(section, lang) : undefined;
  const cfg = {
    get(key, def) {
      const v = lookup(full(key), lang);
      return v === undefined ? def : v;
    },
    has(key) { return lookup(full(key), lang) !== undefined; },
    inspect(key) {
      const k = full(key); const [ws, user] = settingsLayers();
      return { key: k, defaultValue: clone(S.defaults[k]), globalValue: clone(user[k]), workspaceValue: clone(ws[k]), workspaceFolderValue: undefined };
    },
    update(key, value, target) {
      const k = full(key);
      const where = target === 2 || target === 3 || target === false ? 'workspace' : 'user';
      return request('config/update', { key: k, value: value === undefined ? null : value, target: where }).then(() => {
        const layer = S.settings[where] || (S.settings[where] = {});
        if (value === undefined) delete layer[k]; else layer[k] = value;
      });
    },
  };
  if (base && typeof base === 'object' && !Array.isArray(base)) for (const [k, v] of Object.entries(base)) if (!(k in cfg)) cfg[k] = v;
  return Object.freeze(cfg);
}

// ---------------------------------------------------------------- selectors & providers

function selectorScore(sel, doc) {
  if (!sel) return 0;
  if (Array.isArray(sel)) return Math.max(0, ...sel.map(s => selectorScore(s, doc)));
  if (typeof sel === 'string') return sel === '*' ? 5 : sel === doc.languageId ? 10 : 0;
  let score = 0;
  if (sel.notebookType) return 0;
  if (sel.language) { if (sel.language === doc.languageId) score = 10; else if (sel.language === '*') score = 5; else return 0; }
  if (sel.scheme) { if (sel.scheme === doc.uri.scheme) score = Math.max(score, 10); else if (sel.scheme === '*') score = Math.max(score, 5); else return 0; }
  if (sel.pattern) { if (globMatch(sel.pattern, doc.uri.fsPath)) score = Math.max(score, 10); else return 0; }
  return score;
}

function addProvider(kind, ext, selector, provider, extra) {
  const list = S.providers[kind] || (S.providers[kind] = []);
  const entry = { selector, provider, extra: extra || [], ext };
  list.push(entry);
  if (kind === 'completion' || kind === 'signatureHelp' || kind === 'onTypeFormatting') pushTriggers();
  return new Disposable(() => { const i = list.indexOf(entry); if (i >= 0) list.splice(i, 1); if (kind === 'completion' || kind === 'signatureHelp') pushTriggers(); });
}
function providersFor(kind, doc) {
  return (S.providers[kind] || [])
    .map(p => ({ p, score: selectorScore(p.selector, doc) }))
    .filter(x => x.score > 0)
    .sort((a, b) => b.score - a.score)
    .map(x => x.p);
}
function pushTriggers() {
  const completion = {}, signature = {};
  for (const p of S.providers.completion || []) for (const ch of p.extra) if (typeof ch === 'string') langsOf(p.selector).forEach(l => (completion[l] = completion[l] || new Set()).add(ch));
  for (const p of S.providers.signatureHelp || []) {
    let chars = [];
    const m = p.extra[0];
    if (m && typeof m === 'object' && !Array.isArray(m)) chars = [...(m.triggerCharacters || []), ...(m.retriggerCharacters || [])]; else chars = p.extra.filter(x => typeof x === 'string');
    langsOf(p.selector).forEach(l => chars.forEach(ch => (signature[l] = signature[l] || new Set()).add(ch)));
  }
  const out = o => Object.fromEntries(Object.entries(o).map(([k, v]) => [k, [...v]]));
  notify('languages/triggers', { completion: out(completion), signature: out(signature) });
}
function langsOf(sel) {
  if (!sel) return [];
  if (Array.isArray(sel)) return sel.flatMap(langsOf);
  if (typeof sel === 'string') return [sel];
  return [sel.language || '*'];
}

function mdText(x) {
  if (x === undefined || x === null) return '';
  if (typeof x === 'string') return x;
  if (x instanceof MarkdownString || (x && typeof x.value === 'string' && x.language === undefined)) return x.value;
  if (x && x.language !== undefined) return '```' + x.language + '\n' + x.value + '\n```';
  return String(x);
}
function plainLabel(l) { return typeof l === 'string' ? l : l && l.label ? l.label : String(l); }

function locationsOut(res) {
  if (!res) return [];
  if (!Array.isArray(res)) res = [res];
  return res.filter(Boolean).map(l => {
    if (l.targetUri) return { path: toUri(l.targetUri).fsPath, range: rangeOut(R(l.targetSelectionRange || l.targetRange)) };
    return { path: toUri(l.uri).fsPath, range: rangeOut(R(l.range)) };
  });
}

function editsOut(doc, edits) {
  return (edits || []).filter(e => e && e.range).map(e => ({ path: doc.uri.fsPath, range: rangeOut(R(e.range)), newText: e.newText || '' }));
}

function completionItemOut(it, i) {
  let insert = it.insertText, snippet = false;
  if (insert instanceof SnippetString || (insert && typeof insert === 'object' && typeof insert.value === 'string')) { insert = insert.value; snippet = true; }
  if (insert === undefined) insert = plainLabel(it.label);
  let range = it.range;
  if (range && range.inserting) range = range.inserting;
  const label = plainLabel(it.label);
  const detail = it.detail || (it.label && typeof it.label === 'object' ? [it.label.detail, it.label.description].filter(Boolean).join(' ') : '');
  return {
    index: i, label, kind: it.kind === undefined ? CompletionItemKind.Text : it.kind,
    detail: detail || '', documentation: mdText(it.documentation),
    sortText: it.sortText || label, filterText: it.filterText || label,
    insertText: String(insert), snippet, preselect: !!it.preselect,
    range: range ? rangeOut(R(range)) : null,
    additionalTextEdits: (it.additionalTextEdits || []).map(e => ({ range: rangeOut(R(e.range)), newText: e.newText })),
    command: it.command ? { command: it.command.command, title: it.command.title } : null,
    deprecated: !!(it.tags && it.tags.includes(1)),
  };
}

// withToken runs fn with a cancellation token the app can cancel by id.
async function withToken(tokenId, ms, fn) {
  const src = new CancellationTokenSource();
  if (tokenId) S.tokens.set(tokenId, src);
  const timer = setTimeout(() => src.cancel(), ms);
  try { return await fn(src.token); } finally { clearTimeout(timer); if (tokenId) S.tokens.delete(tokenId); src.dispose(); }
}
handlers['cancel'] = ({ token }) => { const s = S.tokens.get(token); if (s) s.cancel(); };

function settle(promises) { return Promise.all(promises.map(p => Promise.resolve(p).catch(e => { if (!(e instanceof CancellationError) && !(e && e.name === 'Canceled')) log('warn', 'provider failed:', e && e.message || e); return undefined; }))); }

let resultSeq = 1;
function cacheResult(value) { const id = resultSeq++; S.caches.set(id, value); if (S.caches.size > 20) S.caches.delete(S.caches.keys().next().value); return id; }

handlers['provide/completion'] = async ({ path: p, position, triggerCharacter, token }) => {
  const doc = docFor(p); if (!doc) return { items: [] };
  await activateByEvent('onLanguage:' + doc.languageId);
  const pos = P(position);
  const ps = providersFor('completion', doc).filter(x => !triggerCharacter || x.extra.includes(triggerCharacter));
  return withToken(token, 8000, async tok => {
    const ctx = { triggerKind: triggerCharacter ? CompletionTriggerKind.TriggerCharacter : CompletionTriggerKind.Invoke, triggerCharacter };
    const results = await settle(ps.map(x => x.provider.provideCompletionItems(doc, pos, tok, ctx)));
    const raw = []; const owners = [];
    let incomplete = false;
    results.forEach((r, i) => {
      if (!r) return;
      const items = Array.isArray(r) ? r : r.items || [];
      if (!Array.isArray(r) && r.isIncomplete) incomplete = true;
      for (const it of items) { raw.push(it); owners.push(ps[i]); }
    });
    const limited = raw.slice(0, 2000);
    const id = cacheResult({ items: limited, owners, doc });
    return { resultId: id, isIncomplete: incomplete, items: limited.map(completionItemOut) };
  });
};

handlers['completion/resolve'] = async ({ resultId, index }) => {
  const c = S.caches.get(resultId); if (!c) return null;
  let it = c.items[index]; const owner = c.owners[index];
  if (!it) return null;
  if (owner && owner.provider.resolveCompletionItem) {
    try { const r = await withToken(null, 3000, tok => owner.provider.resolveCompletionItem(it, tok)); if (r) it = r; } catch (e) { log('warn', 'resolveCompletionItem failed:', e.message); }
  }
  return completionItemOut(it, index);
};

handlers['provide/hover'] = async ({ path: p, position, token }) => {
  const doc = docFor(p); if (!doc) return null;
  const pos = P(position);
  return withToken(token, 5000, async tok => {
    const rs = await settle(providersFor('hover', doc).map(x => x.provider.provideHover(doc, pos, tok)));
    const parts = [];
    for (const h of rs) if (h) for (const c of (Array.isArray(h.contents) ? h.contents : [h.contents])) { const t = mdText(c); if (t.trim()) parts.push(t); }
    return parts.length ? { markdown: parts.join('\n\n---\n\n') } : null;
  });
};

const locationKinds = { definition: 'provideDefinition', typeDefinition: 'provideTypeDefinition', declaration: 'provideDeclaration', implementation: 'provideImplementation', references: 'provideReferences' };
for (const [kind, method] of Object.entries(locationKinds)) {
  handlers['provide/' + kind] = async ({ path: p, position, token }) => {
    const doc = docFor(p); if (!doc) return [];
    const pos = P(position);
    return withToken(token, 8000, async tok => {
      const args = kind === 'references' ? [doc, pos, { includeDeclaration: true }, tok] : [doc, pos, tok];
      const rs = await settle(providersFor(kind, doc).map(x => x.provider[method](...args)));
      return rs.flatMap(locationsOut);
    });
  };
}

handlers['provide/formatting'] = async ({ path: p, range, tabSize, insertSpaces, token }) => {
  const doc = docFor(p); if (!doc) return [];
  const opts = { tabSize: tabSize || 2, insertSpaces: insertSpaces !== false };
  return withToken(token, 10000, async tok => {
    if (range) {
      for (const x of providersFor('rangeFormatting', doc)) {
        const r = await settle([x.provider.provideDocumentRangeFormattingEdits(doc, R(range), opts, tok)]);
        if (r[0]) return editsOut(doc, r[0]);
      }
    }
    for (const x of providersFor('formatting', doc)) {
      const r = await settle([x.provider.provideDocumentFormattingEdits(doc, opts, tok)]);
      if (r[0]) return editsOut(doc, r[0]);
    }
    for (const x of providersFor('rangeFormatting', doc)) {
      const full = new Range(new Position(0, 0), doc.lineAt(doc.lineCount - 1).range.end);
      const r = await settle([x.provider.provideDocumentRangeFormattingEdits(doc, full, opts, tok)]);
      if (r[0]) return editsOut(doc, r[0]);
    }
    return null; // no formatter
  });
};

handlers['provide/codeActions'] = async ({ path: p, range, token }) => {
  const doc = docFor(p); if (!doc) return { items: [] };
  const rng = R(range);
  const diags = [];
  for (const m of S.diagnostics.values()) { const e = m.get(uriKey(doc.uri)); if (e) for (const d of e.list) if (d.range.intersection(rng)) diags.push(d); }
  return withToken(token, 8000, async tok => {
    const ps = providersFor('codeActions', doc);
    const rs = await settle(ps.map(x => x.provider.provideCodeActions(doc, rng, { diagnostics: diags, only: undefined, triggerKind: CodeActionTriggerKind.Invoke }, tok)));
    const items = [], owners = [];
    rs.forEach((r, i) => { for (const a of r || []) if (a && !a.disabled) { items.push(a); owners.push(ps[i]); } });
    const id = cacheResult({ items, owners, doc });
    return { resultId: id, items: items.map((a, i) => ({ index: i, title: a.title, kind: a.kind ? a.kind.value : '', preferred: !!a.isPreferred })) };
  });
};

handlers['codeAction/apply'] = async ({ resultId, index }) => {
  const c = S.caches.get(resultId); if (!c) return false;
  let a = c.items[index]; const owner = c.owners[index];
  if (!a) return false;
  if (typeof a.command === 'string') return execute(a.command, ...(a.arguments || [])).then(() => true);
  if (!a.edit && owner && owner.provider.resolveCodeAction) {
    try { a = (await withToken(null, 5000, tok => owner.provider.resolveCodeAction(a, tok))) || a; } catch (e) { log('warn', 'resolveCodeAction failed:', e.message); }
  }
  if (a.edit) await applyWorkspaceEdit(a.edit);
  if (a.command) await execute(a.command.command, ...(a.command.arguments || []));
  return true;
};

handlers['provide/rename'] = async ({ path: p, position, newName, token }) => {
  const doc = docFor(p); if (!doc) return { ok: false, message: 'document is not open' };
  const pos = P(position);
  const ps = providersFor('rename', doc);
  if (!ps.length) return { ok: false, message: 'no rename provider for ' + doc.languageId };
  return withToken(token, 15000, async tok => {
    for (const x of ps) {
      try {
        const we = await x.provider.provideRenameEdits(doc, pos, newName, tok);
        if (we) return { ok: await applyWorkspaceEdit(we) };
      } catch (e) { return { ok: false, message: e.message || String(e) }; }
    }
    return { ok: false, message: 'nothing to rename here' };
  });
};

handlers['provide/prepareRename'] = async ({ path: p, position }) => {
  const doc = docFor(p); if (!doc) return null;
  const pos = P(position);
  for (const x of providersFor('rename', doc)) {
    if (!x.provider.prepareRename) break;
    try {
      const r = await x.provider.prepareRename(doc, pos, noToken);
      if (r) { const range = r.range ? R(r.range) : R(r); return { placeholder: r.placeholder || doc.getText(range) }; }
    } catch (e) { return { error: e.message || String(e) }; }
  }
  const w = doc.getWordRangeAtPosition(pos);
  return { placeholder: w ? doc.getText(w) : '' };
};

handlers['provide/signatureHelp'] = async ({ path: p, position, triggerCharacter, token }) => {
  const doc = docFor(p); if (!doc) return null;
  const pos = P(position);
  return withToken(token, 4000, async tok => {
    for (const x of providersFor('signatureHelp', doc)) {
      const [r] = await settle([x.provider.provideSignatureHelp(doc, pos, tok, { triggerKind: triggerCharacter ? 2 : 1, triggerCharacter, isRetrigger: false })]);
      if (r && r.signatures && r.signatures.length) {
        const s = r.signatures[Math.min(r.activeSignature || 0, r.signatures.length - 1)];
        const ap = s.activeParameter !== undefined ? s.activeParameter : r.activeParameter;
        const param = s.parameters && s.parameters[ap];
        let pl = param ? param.label : '';
        if (Array.isArray(pl)) pl = s.label.slice(pl[0], pl[1]);
        return { label: s.label, parameter: pl || '', documentation: mdText(s.documentation) };
      }
    }
    return null;
  });
};

handlers['provide/documentSymbols'] = async ({ path: p, token }) => {
  const doc = docFor(p); if (!doc) return [];
  return withToken(token, 8000, async tok => {
    const out = [];
    const walk = (s, depth) => {
      const range = s.selectionRange || s.range || (s.location && s.location.range);
      out.push({ name: s.name, detail: s.detail || s.containerName || '', kind: s.kind, depth, range: rangeOut(R(range)) });
      for (const c of s.children || []) walk(c, depth + 1);
    };
    for (const x of providersFor('documentSymbol', doc)) {
      const [r] = await settle([x.provider.provideDocumentSymbols(doc, tok)]);
      if (r && r.length) { r.forEach(s => walk(s, 0)); break; }
    }
    return out;
  });
};

// ---------------------------------------------------------------- diagnostics

function diagOut(d) {
  return {
    range: rangeOut(R(d.range)), severity: d.severity === undefined ? 0 : d.severity, message: d.message,
    source: d.source || '', code: d.code && typeof d.code === 'object' ? String(d.code.value) : d.code !== undefined ? String(d.code) : '',
  };
}
const pendingDiag = new Map();
let diagTimer = null;
function publishDiagnostics(owner, uri) {
  pendingDiag.set(owner + '\u0000' + uriKey(uri), { owner, uri });
  if (diagTimer) return;
  // Language servers publish in bursts; one batched message per tick keeps
  // a whole-project lint from flooding the UI.
  diagTimer = setTimeout(() => {
    diagTimer = null;
    const batch = [];
    const changed = [];
    for (const { owner, uri } of pendingDiag.values()) {
      const m = S.diagnostics.get(owner);
      const e = m && m.get(uriKey(uri));
      batch.push({ owner, path: toUri(uri).fsPath, diagnostics: e ? e.list.map(diagOut) : [] });
      changed.push(toUri(uri));
    }
    pendingDiag.clear();
    notify('diagnostics/set', { batch });
    ev.diagnostics.fire({ uris: changed });
  }, 50);
}
let diagSeq = 0;
function createDiagnosticCollection(name) {
  const owner = name || ('diagnostics' + (++diagSeq));
  if (!S.diagnostics.has(owner)) S.diagnostics.set(owner, new Map());
  const m = () => S.diagnostics.get(owner) || (S.diagnostics.set(owner, new Map()), S.diagnostics.get(owner));
  const col = {
    name: owner,
    set(a, b) {
      if (Array.isArray(a)) {
        const grouped = new Map();
        for (const [u, ds] of a) { const k = uriKey(u); if (!grouped.has(k)) grouped.set(k, { uri: toUri(u), list: [] }); if (ds) grouped.get(k).list.push(...ds); else grouped.get(k).list = []; }
        for (const [k, v] of grouped) { m().set(k, v); publishDiagnostics(owner, v.uri); }
        return;
      }
      const u = toUri(a);
      if (!b || !b.length) m().delete(uriKey(u)); else m().set(uriKey(u), { uri: u, list: b.slice() });
      publishDiagnostics(owner, u);
    },
    delete(u) { m().delete(uriKey(u)); publishDiagnostics(owner, toUri(u)); },
    clear() { for (const v of m().values()) publishDiagnostics(owner, v.uri); m().clear(); },
    forEach(cb, thisArg) { for (const v of m().values()) cb.call(thisArg, v.uri, v.list, col); },
    get(u) { const e = m().get(uriKey(u)); return e ? e.list.slice() : []; },
    has(u) { return m().has(uriKey(u)); },
    dispose() { col.clear(); S.diagnostics.delete(owner); },
    [Symbol.iterator]: function* () { for (const v of m().values()) yield [v.uri, v.list]; },
  };
  return col;
}
function getDiagnostics(uri) {
  if (uri) { const out = []; for (const m of S.diagnostics.values()) { const e = m.get(uriKey(uri)); if (e) out.push(...e.list); } return out; }
  const all = new Map();
  for (const m of S.diagnostics.values()) for (const [k, v] of m) { if (!all.has(k)) all.set(k, [v.uri, []]); all.get(k)[1].push(...v.list); }
  return [...all.values()];
}

// ---------------------------------------------------------------- commands

async function execute(id, ...args) {
  const c = S.commands.get(id);
  if (c) return c.fn.apply(c.thisArg, args);
  // An extension that contributes this command but has not run yet.
  await activateByEvent('onCommand:' + id);
  const c2 = S.commands.get(id);
  if (c2) return c2.fn.apply(c2.thisArg, args);
  const b = builtinCommands[id];
  if (b) return b(...args);
  return request('commands/execute', { id, args: args.map(argOut) });
}
function argOut(a) {
  if (a instanceof Uri) return { $uri: a.fsPath || a.toString() };
  try { return JSON.parse(JSON.stringify(a === undefined ? null : a)); } catch { return null; }
}

const builtinCommands = {
  setContext: (key, value) => { S.contexts[key] = value; notify('context/set', { key, value: argOut(value) }); },
  'vscode.open': async (uri, opts) => { uri = toUri(uri); if (uri.scheme === 'file') return request('window/showTextDocument', { path: uri.fsPath }); return request('env/openExternal', { uri: uri.toString(true) }); },
  'vscode.openWith': async uri => builtinCommands['vscode.open'](uri),
  'vscode.openFolder': async () => { await request('window/showMessage', { severity: 'info', message: 'Opening another folder is done from the shell: run `hittable <folder>`.', items: [] }); },
  'vscode.diff': async (a, b) => request('window/showTextDocument', { path: toUri(b).fsPath }),
  'workbench.action.reloadWindow': () => notify('host/restart', {}),
  'workbench.action.openSettings': () => request('commands/execute', { id: 'workbench.action.openSettings', args: [] }),
  'editor.action.triggerSuggest': () => {},
  'editor.action.triggerParameterHints': () => {},
  'editor.action.formatDocument': () => request('commands/execute', { id: 'editor.action.formatDocument', args: [] }),
  'vscode.executeCompletionItemProvider': async (uri, pos, trigger) => {
    const r = await handlers['provide/completion']({ path: toUri(uri).fsPath, position: pos, triggerCharacter: trigger });
    const c = S.caches.get(r.resultId); return new CompletionList(c ? c.items : [], r.isIncomplete);
  },
  'vscode.executeHoverProvider': async (uri, pos) => {
    const doc = docFor(uri); if (!doc) return [];
    return (await settle(providersFor('hover', doc).map(x => x.provider.provideHover(doc, P(pos), noToken)))).filter(Boolean);
  },
  'vscode.executeDefinitionProvider': async (uri, pos) => {
    const doc = docFor(uri); if (!doc) return [];
    return (await settle(providersFor('definition', doc).map(x => x.provider.provideDefinition(doc, P(pos), noToken)))).flat().filter(Boolean);
  },
  'vscode.executeFormatDocumentProvider': async (uri, opts) => {
    const doc = docFor(uri); if (!doc) return [];
    for (const x of providersFor('formatting', doc)) { const [r] = await settle([x.provider.provideDocumentFormattingEdits(doc, opts || { tabSize: 2, insertSpaces: true }, noToken)]); if (r) return r; }
    return [];
  },
  'vscode.executeDocumentSymbolProvider': async uri => {
    const doc = docFor(uri) || await openTextDocument(uri); if (!doc) return [];
    for (const x of providersFor('documentSymbol', doc)) { const [r] = await settle([x.provider.provideDocumentSymbols(doc, noToken)]); if (r && r.length) return r; }
    return [];
  },
};

handlers['command/execute'] = async ({ id, args }) => {
  const res = await execute(id, ...(args || []).map(reviveArg));
  try { return JSON.parse(JSON.stringify(res === undefined ? null : res)); } catch { return null; }
};
function reviveArg(a) { if (a && typeof a === 'object' && a.$uri) return Uri.file(a.$uri); return a; }
handlers['commands/list'] = () => [...S.commands.keys()];

// ---------------------------------------------------------------- windows & UI

function messageFn(severity) {
  return async (message, ...rest) => {
    let opts = {};
    if (rest.length && rest[0] && typeof rest[0] === 'object' && !('title' in rest[0]) && !Array.isArray(rest[0])) opts = rest.shift();
    const items = rest.flat().filter(x => x !== undefined);
    const titles = items.map(x => typeof x === 'string' ? x : x.title);
    const idx = await request('window/showMessage', { severity, message: String(message), detail: opts.detail || '', modal: !!opts.modal, items: titles });
    return idx === null || idx === undefined || idx < 0 ? undefined : items[idx];
  };
}

async function showQuickPick(items, options, token) {
  items = await items;
  if (!items || !items.length) return undefined;
  options = options || {};
  const wire = items.map(it => typeof it === 'string' ? { label: it } : { label: stripIcons(it.label), description: stripIcons(it.description || ''), detail: stripIcons(it.detail || ''), picked: !!it.picked, separator: it.kind === -1 });
  const res = await request('window/showQuickPick', { items: wire, placeHolder: options.placeHolder || '', title: options.title || '', canPickMany: !!options.canPickMany });
  if (res === null || res === undefined) return undefined;
  if (options.canPickMany) return (res || []).map(i => items[i]);
  const picked = items[res];
  if (picked !== undefined && options.onDidSelectItem) try { options.onDidSelectItem(picked); } catch {}
  return picked;
}
function stripIcons(s) { return String(s || '').replace(/\$\([\w~-]+\)\s*/g, '').trim(); }

async function showInputBox(options) {
  options = options || {};
  let value = options.value || '';
  let prompt = options.prompt || '';
  for (let attempt = 0; attempt < 10; attempt++) {
    const res = await request('window/showInputBox', { prompt, placeHolder: options.placeHolder || '', value, password: !!options.password, title: options.title || '' });
    if (res === null || res === undefined) return undefined;
    if (!options.validateInput) return res;
    let msg = await options.validateInput(res);
    if (msg && typeof msg === 'object') msg = msg.severity === 3 || msg.severity === undefined ? msg.message : null;
    if (!msg) return res;
    value = res; prompt = '⚠ ' + msg + (options.prompt ? ' — ' + options.prompt : '');
  }
  return undefined;
}

function createQuickPick() {
  const accept = new EventEmitter(), sel = new EventEmitter(), hide = new EventEmitter(), val = new EventEmitter(), act = new EventEmitter(), btn = new EventEmitter(), itemBtn = new EventEmitter();
  const qp = {
    items: [], activeItems: [], selectedItems: [], value: '', placeholder: '', title: '', canSelectMany: false, matchOnDescription: false, matchOnDetail: false,
    busy: false, enabled: true, ignoreFocusOut: false, step: undefined, totalSteps: undefined, buttons: [], keepScrollPosition: false, sortByLabel: true,
    onDidAccept: accept.event, onDidChangeSelection: sel.event, onDidHide: hide.event, onDidChangeValue: val.event, onDidChangeActive: act.event, onDidTriggerButton: btn.event, onDidTriggerItemButton: itemBtn.event,
    async show() {
      // Items may arrive a moment after show() (they are filled while busy).
      for (let i = 0; i < 50 && !qp.items.length && qp.busy; i++) await new Promise(r => setTimeout(r, 100));
      const r = await showQuickPick(qp.items, { placeHolder: qp.placeholder, title: qp.title, canPickMany: qp.canSelectMany });
      if (r === undefined) { hide.fire(); return; }
      qp.selectedItems = Array.isArray(r) ? r : [r]; qp.activeItems = qp.selectedItems;
      act.fire(qp.activeItems); sel.fire(qp.selectedItems); accept.fire();
    },
    hide() { hide.fire(); }, dispose() {},
  };
  return qp;
}
function createInputBox() {
  const accept = new EventEmitter(), hide = new EventEmitter(), val = new EventEmitter(), btn = new EventEmitter();
  const ib = {
    value: '', placeholder: '', prompt: '', title: '', password: false, validationMessage: undefined, busy: false, enabled: true, ignoreFocusOut: false, buttons: [], valueSelection: undefined,
    onDidAccept: accept.event, onDidHide: hide.event, onDidChangeValue: val.event, onDidTriggerButton: btn.event,
    async show() {
      const r = await showInputBox({ value: ib.value, placeHolder: ib.placeholder, prompt: ib.prompt, title: ib.title, password: ib.password });
      if (r === undefined) { hide.fire(); return; }
      ib.value = r; val.fire(r); accept.fire();
    },
    hide() { hide.fire(); }, dispose() {},
  };
  return ib;
}

let channelSeq = 0;
function createOutputChannel(name, opts) {
  const isLog = opts && typeof opts === 'object' && opts.log;
  const key = name || ('output' + (++channelSeq));
  const lvl = new EventEmitter();
  const ch = {
    name: key,
    append(s) { notify('output/append', { channel: key, text: String(s) }); },
    appendLine(s) { notify('output/append', { channel: key, text: String(s) + '\n' }); },
    replace(s) { notify('output/clear', { channel: key }); ch.append(s); },
    clear() { notify('output/clear', { channel: key }); },
    show() { notify('output/show', { channel: key }); },
    hide() {}, dispose() {},
  };
  if (isLog) {
    const stamp = level => (...args) => ch.appendLine(`${new Date().toISOString()} [${level}] ${util.format(...args)}`);
    Object.assign(ch, { logLevel: 3, onDidChangeLogLevel: lvl.event, trace: stamp('trace'), debug: stamp('debug'), info: stamp('info'), warn: stamp('warning'), error: (e, ...a) => stamp('error')(e instanceof Error ? e.stack : e, ...a) });
  }
  return ch;
}

function createStatusBarItem(a, b, c) {
  let id, alignment, priority;
  if (typeof a === 'string') { id = a; alignment = b; priority = c; } else { alignment = a; priority = b; }
  const key = 'sb' + (++S.statusSeq);
  let shown = false;
  const st = { id: id || key, name: undefined, text: '', tooltip: undefined, command: undefined, color: undefined, backgroundColor: undefined, accessibilityInformation: undefined };
  const push = () => notify('statusBar/set', { key, text: stripIcons(st.text), tooltip: typeof st.tooltip === 'string' ? st.tooltip : st.tooltip ? mdText(st.tooltip) : '', command: typeof st.command === 'string' ? st.command : st.command ? st.command.command : '', args: st.command && typeof st.command === 'object' ? (st.command.arguments || []).map(argOut) : [], visible: shown, alignment: alignment === 2 ? 'right' : 'left', priority: priority || 0, error: !!(st.backgroundColor && String(st.backgroundColor.id).includes('error')) });
  const item = new Proxy(st, {
    set(t, p, v) { t[p] = v; if (shown) push(); return true; },
  });
  Object.assign(st, {
    alignment: alignment || 1, priority,
    show() { shown = true; push(); }, hide() { shown = false; push(); }, dispose() { shown = false; push(); },
  });
  return item;
}

let progressSeq = 0;
async function withProgress(options, task) {
  const key = 'p' + (++progressSeq);
  const src = new CancellationTokenSource();
  notify('progress/start', { key, title: options.title || '', location: options.location });
  try {
    return await task({ report: v => notify('progress/report', { key, message: v && v.message || '', increment: v && v.increment || 0 }) }, src.token);
  } finally { notify('progress/end', { key }); }
}

let decoSeq = 0;
function createTextEditorDecorationType(options) {
  const key = 'deco' + (++decoSeq);
  const o = options || {};
  const pick = x => x && (typeof x === 'string' ? x : x.id ? 'theme:' + x.id : undefined);
  const _options = { color: pick(o.color), backgroundColor: pick(o.backgroundColor), fontWeight: o.fontWeight, fontStyle: o.fontStyle, textDecoration: o.textDecoration, after: o.after ? { contentText: o.after.contentText, color: pick(o.after.color) } : undefined, before: o.before ? { contentText: o.before.contentText } : undefined, isWholeLine: !!o.isWholeLine, gutter: !!o.gutterIconPath };
  return { key, _options, dispose() { notify('editor/decorations', { key, dispose: true }); } };
}

// Tree views are walked on demand from the app's picker.
function registerTree(ext, viewId, provider) {
  const rec = S.trees.get(viewId) || { handles: new Map(), seq: 0 };
  rec.provider = provider; rec.ext = ext;
  S.trees.set(viewId, rec);
  notify('views/registered', { id: viewId });
  if (provider.onDidChangeTreeData) provider.onDidChangeTreeData(() => notify('views/changed', { id: viewId }));
  return rec;
}
handlers['tree/children'] = async ({ viewId, handle }) => {
  let rec = S.trees.get(viewId);
  if (!rec) { await activateByEvent('onView:' + viewId); rec = S.trees.get(viewId); }
  if (!rec || !rec.provider) return { error: 'no view ' + viewId + ' is registered' };
  const parent = handle ? rec.handles.get(handle) : undefined;
  const kids = (await rec.provider.getChildren(parent)) || [];
  const out = [];
  for (const k of kids) {
    const ti = await rec.provider.getTreeItem(k);
    const h = viewId + '/' + (++rec.seq);
    rec.handles.set(h, k);
    let label = ti.label;
    if (label && typeof label === 'object') label = label.label;
    if (!label && ti.resourceUri) label = path.basename(ti.resourceUri.fsPath);
    out.push({
      handle: h, label: stripIcons(label || ''), description: typeof ti.description === 'string' ? ti.description : '',
      tooltip: typeof ti.tooltip === 'string' ? ti.tooltip : ti.tooltip ? mdText(ti.tooltip) : '',
      collapsible: (ti.collapsibleState || 0) > 0,
      command: ti.command ? { command: ti.command.command, args: (ti.command.arguments || []).map(argOut) } : null,
      path: ti.resourceUri ? ti.resourceUri.fsPath : '',
    });
  }
  return { items: out };
};
handlers['tree/command'] = async ({ viewId, handle }) => {
  const rec = S.trees.get(viewId); if (!rec) return null;
  const el = rec.handles.get(handle); if (el === undefined) return null;
  const ti = await rec.provider.getTreeItem(el);
  if (ti.command) await execute(ti.command.command, ...(ti.command.arguments || []));
  return null;
};

// ---------------------------------------------------------------- webviews

let webviewSeq = 0;
function makeWebview(handle, ext, options) {
  const recv = new EventEmitter();
  let html = '';
  const wv = {
    options: options || {},
    get html() { return html; },
    set html(v) { html = String(v); notify('webview/html', { handle, html }); },
    onDidReceiveMessage: recv.event,
    postMessage(message) { notify('webview/post', { handle, message: argOut(message) }); return Promise.resolve(true); },
    asWebviewUri(uri) { uri = toUri(uri); return Uri.parse(S.webviewBase + '/res/' + handle + encPath(uri.fsPath.replace(/\\/g, '/').replace(/^([a-zA-Z]):/, '/$1:'))); },
    get cspSource() { return S.webviewBase.replace(/\/[^/]*$/, ''); },
  };
  return { wv, recv };
}
function createWebviewPanel(ext, viewType, title, showOptions, options) {
  const handle = 'wv' + (++webviewSeq);
  const { wv, recv } = makeWebview(handle, ext, options);
  const disposeEv = new EventEmitter(), stateEv = new EventEmitter();
  let disposed = false, t = title;
  const roots = ((options && options.localResourceRoots) || [Uri.file(ext.path), ...S.folders.map(f => f.uri)]).map(u => toUri(u).fsPath);
  const panel = {
    viewType, webview: wv, options: options || {}, viewColumn: 1, active: true, visible: true, iconPath: undefined,
    get title() { return t; }, set title(v) { t = v; notify('webview/title', { handle, title: v }); },
    onDidDispose: disposeEv.event, onDidChangeViewState: stateEv.event,
    reveal() { notify('webview/reveal', { handle }); },
    dispose() { if (disposed) return; disposed = true; S.webviews.delete(handle); notify('webview/dispose', { handle }); disposeEv.fire(); },
  };
  S.webviews.set(handle, { panel, recv, disposeEv });
  notify('webview/create', { handle, viewType, title, extensionId: ext.id, enableScripts: !!(options && options.enableScripts), roots });
  return panel;
}
handlers['webview/message'] = ({ handle, message }) => { const w = S.webviews.get(handle); if (w) w.recv.fire(message); };
handlers['webview/closed'] = ({ handle }) => { const w = S.webviews.get(handle); if (w) w.panel.dispose(); };
handlers['webview/openView'] = async ({ viewId }) => {
  let provider = S.webviewViews.get(viewId);
  if (!provider) { await activateByEvent('onView:' + viewId); provider = S.webviewViews.get(viewId); }
  if (!provider) return { error: 'no webview view ' + viewId + ' is registered' };
  const ext = provider.ext;
  const handle = 'wv' + (++webviewSeq);
  const { wv, recv } = makeWebview(handle, ext, {});
  const disposeEv = new EventEmitter(), visEv = new EventEmitter();
  const view = { viewType: viewId, webview: wv, title: viewId, description: undefined, badge: undefined, visible: true, onDidDispose: disposeEv.event, onDidChangeVisibility: visEv.event, show() { notify('webview/reveal', { handle }); } };
  S.webviews.set(handle, { panel: { dispose() { disposeEv.fire(); notify('webview/dispose', { handle }); } }, recv, disposeEv });
  notify('webview/create', { handle, viewType: viewId, title: viewId, extensionId: ext.id, enableScripts: true, roots: [ext.path, ...S.folders.map(f => f.uri.fsPath)] });
  await provider.provider.resolveWebviewView(view, { state: undefined }, noToken);
  return { handle };
};

// ---------------------------------------------------------------- workspace fs

function fsError(e, uri) {
  if (!e) return e;
  switch (e.code) {
    case 'ENOENT': return FileSystemError.FileNotFound(uri);
    case 'EEXIST': return FileSystemError.FileExists(uri);
    case 'ENOTDIR': return FileSystemError.FileNotADirectory(uri);
    case 'EISDIR': return FileSystemError.FileIsADirectory(uri);
    case 'EACCES': case 'EPERM': return FileSystemError.NoPermissions(uri);
  }
  return e;
}
const wfs = {
  async stat(uri) {
    uri = toUri(uri);
    const prov = S.fsProviders.get(uri.scheme); if (prov) return prov.stat(uri);
    try {
      const st = await fsp.lstat(uri.fsPath);
      let type = st.isDirectory() ? FileType.Directory : st.isFile() ? FileType.File : FileType.Unknown;
      if (st.isSymbolicLink()) { try { const t = await fsp.stat(uri.fsPath); type = (t.isDirectory() ? FileType.Directory : FileType.File) | FileType.SymbolicLink; } catch { type = FileType.SymbolicLink; } }
      return { type, ctime: st.ctimeMs, mtime: st.mtimeMs, size: st.size };
    } catch (e) { throw fsError(e, uri); }
  },
  async readDirectory(uri) {
    uri = toUri(uri);
    const prov = S.fsProviders.get(uri.scheme); if (prov) return prov.readDirectory(uri);
    try { return (await fsp.readdir(uri.fsPath, { withFileTypes: true })).map(d => [d.name, d.isDirectory() ? FileType.Directory : d.isFile() ? FileType.File : d.isSymbolicLink() ? FileType.SymbolicLink : FileType.Unknown]); } catch (e) { throw fsError(e, uri); }
  },
  async createDirectory(uri) { uri = toUri(uri); const prov = S.fsProviders.get(uri.scheme); if (prov) return prov.createDirectory(uri); await fsp.mkdir(uri.fsPath, { recursive: true }); },
  async readFile(uri) {
    uri = toUri(uri);
    const prov = S.fsProviders.get(uri.scheme); if (prov) return prov.readFile(uri);
    const d = docFor(uri);
    if (d && d.isDirty) return Buffer.from(d.getText());
    try { return new Uint8Array(await fsp.readFile(uri.fsPath)); } catch (e) { throw fsError(e, uri); }
  },
  async writeFile(uri, content) { uri = toUri(uri); const prov = S.fsProviders.get(uri.scheme); if (prov) return prov.writeFile(uri, content, { create: true, overwrite: true }); await fsp.mkdir(path.dirname(uri.fsPath), { recursive: true }); await fsp.writeFile(uri.fsPath, content); notify('fs/changed', { path: uri.fsPath }); },
  async delete(uri, opts) { uri = toUri(uri); const prov = S.fsProviders.get(uri.scheme); if (prov) return prov.delete(uri, opts || {}); try { await fsp.rm(uri.fsPath, { recursive: !!(opts && opts.recursive), force: false }); } catch (e) { throw fsError(e, uri); } notify('fs/changed', { path: uri.fsPath }); },
  async rename(a, b, opts) {
    a = toUri(a); b = toUri(b);
    if (!(opts && opts.overwrite)) { try { await fsp.access(b.fsPath); throw FileSystemError.FileExists(b); } catch (e) { if (e instanceof FileSystemError) throw e; } }
    await fsp.rename(a.fsPath, b.fsPath); notify('fs/changed', { path: b.fsPath });
  },
  async copy(a, b, opts) { a = toUri(a); b = toUri(b); await fsp.cp(a.fsPath, b.fsPath, { recursive: true, force: !!(opts && opts.overwrite), errorOnExist: !(opts && opts.overwrite) }); },
  isWritableFileSystem(scheme) { return scheme === 'file' ? true : S.fsProviders.has(scheme) ? !S.fsProviders.get(scheme).isReadonly : undefined; },
};

const skipDirs = new Set(['.git', 'node_modules', '.hg', '.svn', '.next', 'dist', 'build', 'out', 'target', 'vendor', '.cache']);
async function findFiles(include, exclude, maxResults, token) {
  const roots = include && typeof include === 'object' && (include.baseUri || include.base) ? [include.baseUri ? include.baseUri.fsPath : include.base] : S.folders.map(f => f.uri.fsPath);
  const max = maxResults || 10000;
  const out = [];
  const ex = exclude === null ? null : exclude || '**/{node_modules,.git}/**';
  for (const root of roots) {
    const stack = [root];
    let visited = 0;
    while (stack.length && out.length < max && visited < 200000) {
      if (token && token.isCancellationRequested) return out;
      const dir = stack.pop();
      let entries; try { entries = await fsp.readdir(dir, { withFileTypes: true }); } catch { continue; }
      for (const d of entries) {
        visited++;
        const full = path.join(dir, d.name);
        if (d.isDirectory()) { if (!skipDirs.has(d.name) || (ex === null)) stack.push(full); continue; }
        const rel = path.relative(root, full).replace(/\\/g, '/');
        const pat = include && typeof include === 'object' ? include.pattern : include;
        if (!globToRegExp(pat).test(rel)) continue;
        if (ex && globMatch(typeof ex === 'object' ? ex : ex, rel)) continue;
        out.push(Uri.file(full));
        if (out.length >= max) break;
      }
    }
  }
  return out;
}

// One recursive watcher per root, fanned out to every FileSystemWatcher.
const watchers = new Set();
let rootWatcher = null;
function ensureRootWatcher() {
  if (rootWatcher || !S.root) return;
  try {
    rootWatcher = fs.watch(S.root, { recursive: true }, (type, name) => {
      if (!name) return;
      const rel = String(name).replace(/\\/g, '/');
      if (rel.split('/').some(s => s === '.git' || s === 'node_modules')) return;
      const full = path.join(S.root, rel);
      fs.stat(full, (err) => {
        const kind = err ? 'delete' : type === 'rename' ? 'create' : 'change';
        for (const w of watchers) w._emit(kind, full);
      });
    });
    rootWatcher.on('error', e => log('warn', 'file watcher stopped:', e.message));
  } catch (e) { log('warn', 'file watching is unavailable:', e.message); }
}
function createFileSystemWatcher(pattern, ignoreCreate, ignoreChange, ignoreDelete) {
  if (pattern && typeof pattern === 'object' && pattern.ignoreCreateEvents !== undefined) { ignoreCreate = pattern.ignoreCreateEvents; ignoreChange = pattern.ignoreChangeEvents; ignoreDelete = pattern.ignoreDeleteEvents; pattern = pattern.globPattern; }
  ensureRootWatcher();
  const c = new EventEmitter(), ch = new EventEmitter(), d = new EventEmitter();
  const w = {
    ignoreCreateEvents: !!ignoreCreate, ignoreChangeEvents: !!ignoreChange, ignoreDeleteEvents: !!ignoreDelete,
    onDidCreate: c.event, onDidChange: ch.event, onDidDelete: d.event,
    _emit(kind, full) {
      if (!globMatch(pattern, full)) return;
      const u = Uri.file(full);
      if (kind === 'create' && !w.ignoreCreateEvents) c.fire(u);
      if (kind === 'change' && !w.ignoreChangeEvents) ch.fire(u);
      if (kind === 'delete' && !w.ignoreDeleteEvents) d.fire(u);
    },
    dispose() { watchers.delete(w); },
  };
  watchers.add(w);
  return w;
}

async function openTextDocument(arg) {
  if (arg === undefined || (arg && typeof arg === 'object' && !(arg instanceof Uri) && !arg.scheme && ('content' in arg || 'language' in arg))) {
    const o = arg || {};
    const uri = new Uri('untitled', '', '/Untitled-' + (++untitledSeq), '', '');
    const doc = new Doc(uri, o.language || 'plaintext', 1, o.content || '');
    S.docs.set(uriKey(uri), doc); ev.openDoc.fire(doc);
    return doc;
  }
  const uri = toUri(arg);
  const existing = docFor(uri);
  if (existing) return existing;
  let text;
  const cp = S.contentProviders.get(uri.scheme);
  if (cp) text = (await cp.provideTextDocumentContent(uri, noToken)) || '';
  else if (S.fsProviders.has(uri.scheme)) text = Buffer.from(await S.fsProviders.get(uri.scheme).readFile(uri)).toString('utf8');
  else {
    try { text = await fsp.readFile(uri.fsPath, 'utf8'); } catch (e) { throw new Error(`cannot open ${uri.fsPath}: ${e.message}`); }
  }
  const lang = await request('languages/idFor', { path: uri.fsPath }).catch(() => 'plaintext');
  const doc = new Doc(uri, lang || 'plaintext', 1, text);
  S.docs.set(uriKey(uri), doc);
  ev.openDoc.fire(doc);
  activateByEvent('onLanguage:' + doc.languageId);
  return doc;
}
let untitledSeq = 0;

// ---------------------------------------------------------------- document sync from the app

function changeFor(doc, newText) {
  const old = doc.getText();
  let s = 0; const max = Math.min(old.length, newText.length);
  while (s < max && old.charCodeAt(s) === newText.charCodeAt(s)) s++;
  // Never split a surrogate pair.
  if (s > 0 && s < old.length && (old.charCodeAt(s) & 0xFC00) === 0xDC00) s--;
  let eo = old.length, en = newText.length;
  while (eo > s && en > s && old.charCodeAt(eo - 1) === newText.charCodeAt(en - 1)) { eo--; en--; }
  if (eo < old.length && (old.charCodeAt(eo) & 0xFC00) === 0xDC00 && eo > s) { eo--; en--; }
  return { range: new Range(doc.positionAt(s), doc.positionAt(eo)), rangeOffset: s, rangeLength: eo - s, text: newText.slice(s, en) };
}

handlers['doc/open'] = async ({ path: p, languageId, version, text }) => {
  const uri = Uri.file(p);
  const k = uriKey(uri);
  const existing = S.docs.get(k);
  if (existing && !existing.isClosed) {
    if (existing.getText() !== text) handlers['doc/change']({ path: p, version, text });
    return;
  }
  const doc = new Doc(uri, languageId || 'plaintext', version || 1, text);
  S.docs.set(k, doc);
  ev.openDoc.fire(doc);
  await activateByEvent('onLanguage:' + doc.languageId);
};
handlers['doc/change'] = ({ path: p, version, text }) => {
  const doc = docFor(Uri.file(p)); if (!doc) return;
  if (doc.getText() === text) return;
  const change = changeFor(doc, text);
  doc._set(text); doc.version = version || doc.version + 1; doc.isDirty = true;
  ev.changeDoc.fire({ document: doc, contentChanges: [change], reason: undefined });
};
handlers['doc/save'] = ({ path: p }) => {
  const doc = docFor(Uri.file(p)); if (!doc) return;
  ev.willSaveDoc.fire({ document: doc, reason: 1, waitUntil() {} });
  doc.isDirty = false;
  ev.saveDoc.fire(doc);
};
handlers['doc/close'] = ({ path: p }) => {
  const k = uriKey(Uri.file(p));
  const doc = S.docs.get(k); if (!doc) return;
  S.docs.delete(k); doc.isClosed = true;
  S.editors = S.editors.filter(e => e.document !== doc);
  if (S.activeEditor && S.activeEditor.document === doc) { S.activeEditor = undefined; ev.activeEditor.fire(undefined); }
  ev.visibleEditors.fire(S.editors.slice());
  ev.closeDoc.fire(doc);
  ev.tabs.fire({ opened: [], closed: [], changed: [] });
};
handlers['doc/language'] = ({ path: p, languageId }) => {
  const doc = docFor(Uri.file(p)); if (!doc || doc.languageId === languageId) return;
  ev.closeDoc.fire(doc); doc.languageId = languageId; ev.openDoc.fire(doc);
  return activateByEvent('onLanguage:' + languageId);
};
handlers['editor/active'] = ({ path: p, selections }) => {
  if (!p) { if (S.activeEditor) { S.activeEditor = undefined; ev.activeEditor.fire(undefined); } return; }
  const doc = docFor(Uri.file(p)); if (!doc) return;
  const sels = (selections || []).map(s => new Selection(P(s.anchor), P(s.active)));
  let ed = S.editors.find(e => e.document === doc);
  const isNew = !ed;
  if (!ed) { ed = makeEditor(doc, sels); S.editors = [ed]; }
  else if (sels.length) ed.selections = sels;
  if (S.activeEditor !== ed) { S.activeEditor = ed; ev.activeEditor.fire(ed); ev.tabs.fire({ opened: isNew ? [tabFor(ed)] : [], closed: [], changed: [tabFor(ed)] }); }
  if (isNew) ev.visibleEditors.fire(S.editors.slice());
  if (sels.length) ev.selection.fire({ textEditor: ed, selections: ed.selections, kind: 1 });
};
handlers['config/changed'] = ({ settings }) => {
  const before = JSON.stringify(S.settings);
  const old = S.settings;
  S.settings = settings || { user: {}, workspace: {} };
  if (JSON.stringify(S.settings) === before) return;
  const changedKeys = new Set();
  const flatOld = Object.assign({}, flattenSettings(old.user, '', {}), flattenSettings(old.workspace, '', {}));
  const flatNew = Object.assign({}, flattenSettings(S.settings.user, '', {}), flattenSettings(S.settings.workspace, '', {}));
  for (const k of new Set([...Object.keys(flatOld), ...Object.keys(flatNew)])) if (JSON.stringify(flatOld[k]) !== JSON.stringify(flatNew[k])) changedKeys.add(k);
  ev.config.fire({ affectsConfiguration: (section) => [...changedKeys].some(k => k === section || k.startsWith(section + '.') || section.startsWith(k + '.') || (k.startsWith('[') && typeof flatNew[k] === 'object')) });
};

function tabFor(ed) { return { input: new TabInputText(ed.document.uri), label: path.basename(ed.document.fileName), isActive: ed === S.activeEditor, isDirty: ed.document.isDirty, isPinned: false, isPreview: false, group: tabGroup }; }
const tabGroup = { get tabs() { return S.editors.map(tabFor); }, get activeTab() { return S.activeEditor ? tabFor(S.activeEditor) : undefined; }, isActive: true, viewColumn: 1 };

// ---------------------------------------------------------------- extensions & activation

function extensionObject(ext) {
  return {
    id: ext.manifest.publisher + '.' + ext.manifest.name, extensionUri: Uri.file(ext.path), extensionPath: ext.path,
    get isActive() { return ext.active; }, packageJSON: ext.manifest, extensionKind: 2,
    get exports() { return ext.exports; },
    activate: () => activateExt(ext).then(() => ext.exports),
  };
}

function memento(file) {
  let data = {};
  try { data = JSON.parse(fs.readFileSync(file, 'utf8')); } catch {}
  let timer = null;
  const flush = () => { timer = null; try { fs.mkdirSync(path.dirname(file), { recursive: true }); fs.writeFileSync(file, JSON.stringify(data)); } catch (e) { log('warn', 'saving state failed:', e.message); } };
  return {
    get(k, d) { return k in data ? clone(data[k]) : d; },
    update(k, v) { if (v === undefined) delete data[k]; else data[k] = clone(v); if (!timer) timer = setTimeout(flush, 200); return Promise.resolve(); },
    keys() { return Object.keys(data); },
    setKeysForSync() {},
    _flush: () => { if (timer) { clearTimeout(timer); flush(); } },
  };
}
const mementos = [];

// Secrets live in a 0600 file under the app's data folder. There is no OS
// keychain binding without native code; the file is readable by this user only.
let secretsCache = null;
function secretsFile() { return path.join(S.extData, 'secrets.json'); }
function loadSecrets() { if (!secretsCache) { try { secretsCache = JSON.parse(fs.readFileSync(secretsFile(), 'utf8')); } catch { secretsCache = {}; } } return secretsCache; }
function saveSecrets() { fs.mkdirSync(path.dirname(secretsFile()), { recursive: true }); fs.writeFileSync(secretsFile(), JSON.stringify(secretsCache), { mode: 0o600 }); try { fs.chmodSync(secretsFile(), 0o600); } catch {} }

function makeContext(ext) {
  const gs = memento(path.join(S.extData, 'globalState', ext.id + '.json'));
  const ws = memento(path.join(S.extData, 'workspaceState', S.workspaceHash, ext.id + '.json'));
  mementos.push(gs, ws);
  const globalStorage = path.join(S.extData, 'globalStorage', ext.id);
  const storage = path.join(S.extData, 'workspaceStorage', S.workspaceHash, ext.id);
  const logDir = path.join(S.extData, 'logs', ext.id);
  for (const d of [globalStorage, logDir]) { try { fs.mkdirSync(d, { recursive: true }); } catch {} }
  const secretEv = new EventEmitter();
  const envColl = () => {
    const m = new Map();
    return { persistent: true, description: undefined, replace(k, v) { m.set(k, { value: v, type: 1 }); }, append(k, v) { m.set(k, { value: v, type: 2 }); }, prepend(k, v) { m.set(k, { value: v, type: 3 }); }, get: k => m.get(k), forEach(cb, t) { m.forEach((v, k) => cb.call(t, k, v, this)); }, delete: k => m.delete(k), clear: () => m.clear(), getScoped() { return envColl(); }, [Symbol.iterator]: () => m.entries() };
  };
  return {
    subscriptions: [],
    extensionPath: ext.path, extensionUri: Uri.file(ext.path),
    asAbsolutePath: rel => path.join(ext.path, rel),
    storagePath: storage, storageUri: Uri.file(storage),
    globalStoragePath: globalStorage, globalStorageUri: Uri.file(globalStorage),
    logPath: logDir, logUri: Uri.file(logDir),
    globalState: gs, workspaceState: ws,
    secrets: {
      get: async k => loadSecrets()[ext.id + '/' + k],
      store: async (k, v) => { loadSecrets()[ext.id + '/' + k] = v; saveSecrets(); secretEv.fire({ key: k }); },
      delete: async k => { delete loadSecrets()[ext.id + '/' + k]; saveSecrets(); secretEv.fire({ key: k }); },
      keys: async () => Object.keys(loadSecrets()).filter(k => k.startsWith(ext.id + '/')).map(k => k.slice(ext.id.length + 1)),
      onDidChange: secretEv.event,
    },
    extensionMode: 1,
    extension: extensionObject(ext),
    environmentVariableCollection: envColl(),
    languageModelAccessInformation: { onDidChange: noEvent, canSendRequest: () => false },
  };
}

function eventsOf(ext) {
  if (ext._events) return ext._events;
  const m = ext.manifest; const c = m.contributes || {};
  const evs = new Set(m.activationEvents || []);
  for (const cmd of c.commands || []) evs.add('onCommand:' + cmd.command);
  for (const l of c.languages || []) evs.add('onLanguage:' + l.id);
  for (const list of Object.values(c.views || {})) for (const v of list || []) evs.add('onView:' + v.id);
  for (const ce of c.customEditors || []) evs.add('onCustomEditor:' + ce.viewType);
  for (const a of c.authentication || []) evs.add('onAuthenticationRequest:' + a.id);
  ext._events = evs;
  return evs;
}

// resolveMain finds the entry file: "main" may omit .js / .cjs / .mjs.
function resolveMain(ext) {
  const base = path.resolve(ext.path, ext.manifest.main);
  for (const c of [base, base + '.js', base + '.cjs', base + '.mjs', path.join(base, 'index.js')]) {
    try { if (fs.statSync(c).isFile()) return c; } catch {}
  }
  return base;
}

async function activateByEvent(event) {
  const todo = S.exts.filter(e => e.manifest.main && !e.active && !e.failed && (eventsOf(e).has(event) || eventsOf(e).has('*')));
  await Promise.all(todo.map(activateExt));
}

function activateExt(ext) {
  if (ext.activating) return ext.activating;
  ext.activating = (async () => {
    for (const dep of ext.manifest.extensionDependencies || []) {
      const d = S.exts.find(e => e.id === String(dep).toLowerCase());
      if (d) await activateExt(d);
    }
    if (!ext.manifest.main) { ext.active = true; return; }
    const started = Date.now();
    notify('ext/activating', { id: ext.id });
    try {
      const main = resolveMain(ext);
      let mod;
      try { mod = require(main); } catch (e) {
        if (!e || !/ERR_REQUIRE_(ESM|ASYNC_MODULE)/.test(e.code || '')) throw e;
        mod = await import(require('url').pathToFileURL(main).href);
      }
      if (mod && mod.default && typeof mod.activate !== 'function' && typeof mod.default.activate === 'function') mod = mod.default;
      ext.context = makeContext(ext);
      ext.module = mod;
      if (typeof mod.activate === 'function') {
        const slow = setTimeout(() => log('warn', `${ext.id} is taking a long time to activate`), 10000);
        try { ext.exports = await mod.activate(ext.context); } finally { clearTimeout(slow); }
      }
      ext.active = true;
      notify('ext/activated', { id: ext.id, ms: Date.now() - started });
    } catch (e) {
      ext.failed = true;
      log('error', `activating ${ext.id} failed:`, e && e.stack || e);
      notify('ext/failed', { id: ext.id, error: String(e && e.message || e) });
    }
  })();
  return ext.activating;
}

async function shutdown() {
  for (const ext of S.exts) {
    if (!ext.active) continue;
    try { if (ext.module && typeof ext.module.deactivate === 'function') await Promise.race([ext.module.deactivate(), new Promise(r => setTimeout(r, 3000))]); } catch (e) { log('warn', `deactivating ${ext.id}:`, e.message); }
    for (const d of (ext.context && ext.context.subscriptions) || []) { try { d && d.dispose && d.dispose(); } catch {} }
  }
  for (const m of mementos) m._flush();
}
handlers['shutdown'] = async () => { await shutdown(); setTimeout(() => process.exit(0), 50); return true; };
handlers['activate'] = ({ event }) => activateByEvent(event);
handlers['debug/providers'] = () => Object.fromEntries(Object.entries(S.providers).map(([k, v]) => [k, v.map(x => JSON.stringify(x.selector))]));
handlers['status'] = () => S.exts.map(e => ({ id: e.id, active: !!e.active, failed: !!e.failed }));

// workspaceContains:<glob> is checked once at start, over a bounded walk.
async function checkWorkspaceContains() {
  const wanted = [];
  for (const ext of S.exts) for (const e of eventsOf(ext)) if (e.startsWith('workspaceContains:')) wanted.push([ext, e.slice('workspaceContains:'.length)]);
  if (!wanted.length || !S.root) return;
  const files = await findFiles('**/*', undefined, 20000);
  for (const [ext, glob] of wanted) {
    if (ext.active) continue;
    if (files.some(u => globToRegExp(glob).test(path.relative(S.root, u.fsPath).replace(/\\/g, '/')))) activateExt(ext);
  }
}

// ---------------------------------------------------------------- the vscode module

function stubNamespace(name, ext, obj) {
  return new Proxy(obj, {
    get(t, p) {
      if (p in t || typeof p === 'symbol' || p === 'then' || p === 'toJSON' || p === '__esModule' || p === 'default') return t[p];
      warnOnce(name + '.' + String(p), ext);
      if (/^(register|create)/.test(String(p))) return () => new Disposable(() => {});
      if (/^onDid|^onWill/.test(String(p))) return noEvent;
      return undefined;
    },
  });
}

function buildApi(ext) {
  const reg = kind => (selector, provider, ...extra) => addProvider(kind, ext, selector, provider, extra);
  const languages = {
    createDiagnosticCollection,
    getDiagnostics,
    onDidChangeDiagnostics: ev.diagnostics.event,
    getLanguages: async () => [...S.languages],
    setTextDocumentLanguage: async (doc, lang) => { await handlers['doc/language']({ path: doc.uri.fsPath, languageId: lang }); notify('doc/languageChanged', { path: doc.uri.fsPath, languageId: lang }); return doc; },
    match: (sel, doc) => selectorScore(sel, doc),
    setLanguageConfiguration: (lang, conf) => { languageConfigs.set(lang, conf || {}); return new Disposable(() => languageConfigs.delete(lang)); },
    createLanguageStatusItem: (id, selector) => {
      const item = createStatusBarItem('lang.' + id, 2, 0);
      const wrapper = { id, selector, severity: 0, name: undefined, busy: false, accessibilityInformation: undefined, detail: '',
        get text() { return item.text; }, set text(v) { item.text = v; item.show(); },
        get command() { return item.command; }, set command(v) { item.command = v; },
        dispose() { item.dispose(); } };
      return wrapper;
    },
    registerCompletionItemProvider: reg('completion'),
    registerHoverProvider: reg('hover'),
    registerDefinitionProvider: reg('definition'),
    registerTypeDefinitionProvider: reg('typeDefinition'),
    registerDeclarationProvider: reg('declaration'),
    registerImplementationProvider: reg('implementation'),
    registerReferenceProvider: reg('references'),
    registerDocumentFormattingEditProvider: reg('formatting'),
    registerDocumentRangeFormattingEditProvider: reg('rangeFormatting'),
    registerOnTypeFormattingEditProvider: reg('onTypeFormatting'),
    registerCodeActionsProvider: reg('codeActions'),
    registerRenameProvider: reg('rename'),
    registerSignatureHelpProvider: reg('signatureHelp'),
    registerDocumentSymbolProvider: reg('documentSymbol'),
    registerWorkspaceSymbolProvider: (provider) => addProvider('workspaceSymbol', ext, '*', provider, []),
    registerCodeLensProvider: reg('codeLens'),
    registerDocumentHighlightProvider: reg('documentHighlight'),
    registerDocumentLinkProvider: reg('documentLink'),
    registerColorProvider: reg('color'),
    registerFoldingRangeProvider: reg('folding'),
    registerSelectionRangeProvider: reg('selectionRange'),
    registerCallHierarchyProvider: reg('callHierarchy'),
    registerTypeHierarchyProvider: reg('typeHierarchy'),
    registerDocumentSemanticTokensProvider: reg('semanticTokens'),
    registerDocumentRangeSemanticTokensProvider: reg('rangeSemanticTokens'),
    registerInlayHintsProvider: reg('inlayHints'),
    registerLinkedEditingRangeProvider: reg('linkedEditing'),
    registerInlineValuesProvider: reg('inlineValues'),
    registerInlineCompletionItemProvider: reg('inlineCompletion'),
    registerEvaluatableExpressionProvider: reg('evaluatable'),
    registerDocumentDropEditProvider: reg('drop'),
    registerDocumentPasteEditProvider: reg('paste'),
  };
  const window = {
    get activeTextEditor() { return S.activeEditor; },
    get visibleTextEditors() { return S.editors.slice(); },
    onDidChangeActiveTextEditor: ev.activeEditor.event,
    onDidChangeVisibleTextEditors: ev.visibleEditors.event,
    onDidChangeTextEditorSelection: ev.selection.event,
    onDidChangeTextEditorVisibleRanges: ev.visibleRanges.event,
    onDidChangeTextEditorOptions: ev.editorOptions.event,
    onDidChangeTextEditorViewColumn: noEvent,
    onDidChangeWindowState: ev.windowState.event,
    state: { focused: true, active: true },
    activeColorTheme: { kind: 2 },
    onDidChangeActiveColorTheme: ev.theme.event,
    showInformationMessage: messageFn('info'),
    showWarningMessage: messageFn('warning'),
    showErrorMessage: messageFn('error'),
    showQuickPick, showInputBox, createQuickPick, createInputBox,
    createOutputChannel,
    createStatusBarItem,
    setStatusBarMessage: (text, hide) => {
      notify('statusBar/message', { text: stripIcons(text), timeout: typeof hide === 'number' ? hide : 0 });
      if (hide && typeof hide.then === 'function') hide.then(() => notify('statusBar/message', { text: '', timeout: 0 }));
      return new Disposable(() => notify('statusBar/message', { text: '', timeout: 0 }));
    },
    withProgress,
    withScmProgress: task => task({ report() {} }),
    showTextDocument: async (a, b) => {
      const doc = a && a.getText ? a : await openTextDocument(a);
      const sel = b && b.selection ? R(b.selection) : undefined;
      await request('window/showTextDocument', { path: doc.uri.fsPath, selection: sel ? rangeOut(sel) : null });
      let ed = S.editors.find(e => e.document === doc);
      if (!ed) { ed = makeEditor(doc, sel ? [new Selection(sel.start, sel.end)] : undefined); S.editors = [ed]; }
      S.activeEditor = ed;
      return ed;
    },
    createTextEditorDecorationType,
    registerTreeDataProvider: (id, provider) => { registerTree(ext, id, provider); return new Disposable(() => S.trees.delete(id)); },
    createTreeView: (id, options) => {
      registerTree(ext, id, options.treeDataProvider);
      const e = () => new EventEmitter().event;
      return { visible: false, selection: [], message: undefined, title: id, description: undefined, badge: undefined, onDidExpandElement: e(), onDidCollapseElement: e(), onDidChangeSelection: e(), onDidChangeVisibility: e(), onDidChangeCheckboxState: e(), reveal: async () => {}, dispose() { S.trees.delete(id); } };
    },
    createWebviewPanel: (viewType, title, showOptions, options) => createWebviewPanel(ext, viewType, title, showOptions, options),
    registerWebviewViewProvider: (viewId, provider) => { S.webviewViews.set(viewId, { provider, ext }); notify('views/registered', { id: viewId, webview: true }); return new Disposable(() => S.webviewViews.delete(viewId)); },
    registerWebviewPanelSerializer: () => new Disposable(() => {}),
    registerCustomEditorProvider: (viewType) => { warnOnce('window.registerCustomEditorProvider (' + viewType + ')', ext); return new Disposable(() => {}); },
    registerUriHandler: h => { S.uriHandlers.push(h); return new Disposable(() => {}); },
    registerFileDecorationProvider: () => new Disposable(() => {}),
    registerTerminalLinkProvider: () => new Disposable(() => {}),
    registerTerminalProfileProvider: () => new Disposable(() => {}),
    createTerminal: (a, b, c) => {
      const o = typeof a === 'object' && a ? a : { name: a, shellPath: b, shellArgs: c };
      const id = 't' + (++termSeq);
      notify('terminal/create', { id, name: o.name || 'extension', cwd: o.cwd ? toUri(o.cwd).fsPath || String(o.cwd) : '', shellPath: o.shellPath || '', shellArgs: Array.isArray(o.shellArgs) ? o.shellArgs : o.shellArgs ? [o.shellArgs] : [] });
      const t = { name: o.name || 'extension', processId: Promise.resolve(undefined), creationOptions: o, exitStatus: undefined, state: { isInteractedWith: false }, shellIntegration: undefined,
        sendText(text, addNewLine) { notify('terminal/send', { id, text: String(text) + (addNewLine === false ? '' : '\n') }); },
        show() { notify('terminal/show', { id }); }, hide() {}, dispose() {} };
      ev.openTerminal.fire(t);
      return t;
    },
    get terminals() { return []; },
    activeTerminal: undefined,
    onDidOpenTerminal: ev.openTerminal.event, onDidCloseTerminal: ev.closeTerminal.event, onDidChangeActiveTerminal: noEvent, onDidChangeTerminalState: noEvent,
    onDidStartTerminalShellExecution: noEvent, onDidEndTerminalShellExecution: noEvent, onDidChangeTerminalShellIntegration: noEvent,
    showOpenDialog: async (o) => { const r = await showInputBox({ prompt: (o && o.title) || (o && o.openLabel) || 'Path to open', value: o && o.defaultUri ? toUri(o.defaultUri).fsPath : S.root }); return r ? [Uri.file(path.resolve(S.root, r))] : undefined; },
    showSaveDialog: async (o) => { const r = await showInputBox({ prompt: (o && o.title) || 'Save as', value: o && o.defaultUri ? toUri(o.defaultUri).fsPath : S.root }); return r ? Uri.file(path.resolve(S.root, r)) : undefined; },
    showWorkspaceFolderPick: async () => S.folders[0],
    tabGroups: { get all() { return [tabGroup]; }, get activeTabGroup() { return tabGroup; }, onDidChangeTabs: ev.tabs.event, onDidChangeTabGroups: ev.tabGroups.event, close: async () => false },
    get visibleNotebookEditors() { return []; }, activeNotebookEditor: undefined, onDidChangeActiveNotebookEditor: noEvent, onDidChangeVisibleNotebookEditors: noEvent,
    onDidChangeNotebookEditorSelection: noEvent, onDidChangeNotebookEditorVisibleRanges: noEvent,
  };
  const workspace = {
    get workspaceFolders() { return S.folders.length ? S.folders.slice() : undefined; },
    get name() { return S.folders.length ? S.folders[0].name : undefined; },
    get rootPath() { return S.root || undefined; },
    workspaceFile: undefined,
    getWorkspaceFolder: uri => { const p = toUri(uri).fsPath; return S.folders.find(f => p === f.uri.fsPath || p.startsWith(f.uri.fsPath + path.sep)); },
    asRelativePath: (x, includeFolder) => { const p = typeof x === 'string' ? x : toUri(x).fsPath; if (!S.root || !(p === S.root || p.startsWith(S.root + path.sep))) return p; const rel = path.relative(S.root, p).replace(/\\/g, '/'); return includeFolder && S.folders.length > 1 ? S.folders[0].name + '/' + rel : rel; },
    onDidChangeWorkspaceFolders: ev.folders.event,
    updateWorkspaceFolders: () => false,
    get textDocuments() { return [...S.docs.values()]; },
    openTextDocument,
    onDidOpenTextDocument: ev.openDoc.event,
    onDidCloseTextDocument: ev.closeDoc.event,
    onDidChangeTextDocument: ev.changeDoc.event,
    onDidSaveTextDocument: ev.saveDoc.event,
    onWillSaveTextDocument: ev.willSaveDoc.event,
    onDidCreateFiles: ev.createFiles.event, onDidDeleteFiles: ev.deleteFiles.event, onDidRenameFiles: ev.renameFiles.event,
    onWillCreateFiles: ev.willCreateFiles.event, onWillDeleteFiles: ev.willDeleteFiles.event, onWillRenameFiles: ev.willRenameFiles.event,
    getConfiguration,
    onDidChangeConfiguration: ev.config.event,
    fs: wfs,
    findFiles,
    createFileSystemWatcher,
    applyEdit: applyWorkspaceEdit,
    saveAll: async () => { await request('workspace/saveAll', {}); return true; },
    save: async uri => { await request('workspace/save', { path: toUri(uri).fsPath }); return toUri(uri); },
    saveAs: async () => undefined,
    registerTextDocumentContentProvider: (scheme, provider) => { S.contentProviders.set(scheme, provider); return new Disposable(() => S.contentProviders.delete(scheme)); },
    registerFileSystemProvider: (scheme, provider, options) => { provider.isReadonly = !!(options && options.isReadonly); S.fsProviders.set(scheme, provider); return new Disposable(() => S.fsProviders.delete(scheme)); },
    registerTaskProvider: () => new Disposable(() => {}),
    get isTrusted() { return true; },
    onDidGrantWorkspaceTrust: ev.trust.event,
    get notebookDocuments() { return []; },
    onDidOpenNotebookDocument: noEvent, onDidCloseNotebookDocument: noEvent, onDidChangeNotebookDocument: noEvent, onDidSaveNotebookDocument: noEvent, onWillSaveNotebookDocument: noEvent,
    registerNotebookSerializer: () => new Disposable(() => {}),
    openNotebookDocument: async () => { throw new Error('notebooks are not supported in hittable'); },
    decode: async (content) => Buffer.from(content).toString('utf8'),
    encode: async (text) => Buffer.from(text, 'utf8'),
  };
  const commands = {
    registerCommand: (id, fn, thisArg) => {
      S.commands.set(id, { fn, thisArg, ext });
      notify('commands/registered', { id, extensionId: ext.id });
      return new Disposable(() => { if (S.commands.get(id) && S.commands.get(id).fn === fn) S.commands.delete(id); });
    },
    registerTextEditorCommand: (id, fn, thisArg) => commands.registerCommand(id, (...args) => {
      const ed = S.activeEditor; if (!ed) return;
      const edits = [];
      const builder = { replace: (l, t) => edits.push(new TextEdit(l instanceof Position ? new Range(l, l) : R(l), t)), insert: (p, t) => edits.push(new TextEdit(new Range(p, p), t)), delete: l => edits.push(new TextEdit(R(l), '')), setEndOfLine() {} };
      const r = fn.call(thisArg, ed, builder, ...args);
      return Promise.resolve(r).then(() => { if (edits.length) { const we = new WorkspaceEdit(); we.set(ed.document.uri, edits); return applyWorkspaceEdit(we); } });
    }, thisArg),
    executeCommand: (id, ...args) => execute(id, ...args),
    getCommands: async () => [...S.commands.keys(), ...Object.keys(builtinCommands)],
    registerDiffInformationCommand: () => new Disposable(() => {}),
  };
  const env = {
    appName: 'Hittable', appRoot: __dirname, appHost: 'terminal', uriScheme: 'hittable', language: 'en',
    get machineId() { return S.machineId; }, sessionId: crypto.randomUUID(), remoteName: undefined, isNewAppInstall: false,
    shell: process.env.SHELL || (isWin ? 'cmd.exe' : '/bin/sh'), uiKind: 1, isTelemetryEnabled: false,
    onDidChangeTelemetryEnabled: noEvent, onDidChangeShell: noEvent, logLevel: 3, onDidChangeLogLevel: noEvent,
    clipboard: { readText: () => request('clipboard/read', {}).then(v => v || ''), writeText: t => request('clipboard/write', { text: String(t) }).then(() => {}) },
    openExternal: async uri => { await request('env/openExternal', { uri: toUri(uri).toString(true) }); return true; },
    asExternalUri: async uri => toUri(uri),
    createTelemetryLogger: () => ({ logUsage() {}, logError() {}, dispose() {}, onDidChangeEnableStates: noEvent, isUsageEnabled: false, isErrorsEnabled: false }),
  };
  const extensions = {
    getExtension: id => { const e = S.exts.find(x => x.id === String(id).toLowerCase()); return e ? extensionObject(e) : undefined; },
    get all() { return S.exts.map(extensionObject); },
    get allAcrossExtensionHosts() { return S.exts.map(extensionObject); },
    onDidChange: ev.extensions.event,
  };
  const debug = {
    activeDebugSession: undefined, activeDebugConsole: { append() {}, appendLine() {} }, activeStackItem: undefined, breakpoints: [],
    onDidChangeActiveDebugSession: noEvent, onDidStartDebugSession: noEvent, onDidReceiveDebugSessionCustomEvent: noEvent, onDidTerminateDebugSession: noEvent, onDidChangeBreakpoints: noEvent, onDidChangeActiveStackItem: noEvent,
    registerDebugConfigurationProvider: () => new Disposable(() => {}), registerDebugAdapterDescriptorFactory: () => new Disposable(() => {}), registerDebugAdapterTrackerFactory: () => new Disposable(() => {}),
    startDebugging: async () => { await messageFn('warning')('Debugging is not available in hittable.'); return false; },
    stopDebugging: async () => {}, addBreakpoints() {}, removeBreakpoints() {}, asDebugSourceUri: s => Uri.file(s.path || ''),
  };
  const tasks = { registerTaskProvider: () => new Disposable(() => {}), fetchTasks: async () => [], executeTask: async () => { throw new Error('tasks are not supported in hittable'); }, taskExecutions: [], onDidStartTask: noEvent, onDidEndTask: noEvent, onDidStartTaskProcess: noEvent, onDidEndTaskProcess: noEvent };
  const scm = { createSourceControl: (id, label, rootUri) => ({ id, label, rootUri, inputBox: { value: '', placeholder: '', enabled: true, visible: true }, count: 0, quickDiffProvider: undefined, commitTemplate: undefined, acceptInputCommand: undefined, statusBarCommands: undefined, createResourceGroup: (gid, glabel) => ({ id: gid, label: glabel, resourceStates: [], hideWhenEmpty: false, dispose() {} }), dispose() {} }), inputBox: { value: '' } };
  const comments = { createCommentController: (id, label) => ({ id, label, options: undefined, commentingRangeProvider: undefined, createCommentThread: () => ({ comments: [], dispose() {} }), dispose() {} }) };
  const authentication = {
    getSession: async (providerId, scopes, options) => {
      if (options && options.createIfNone) throw new Error(`Signing in to ${providerId} is not available in hittable`);
      return undefined;
    },
    getAccounts: async () => [], onDidChangeSessions: noEvent, registerAuthenticationProvider: () => new Disposable(() => {}),
  };
  const testItems = () => { const m = new Map(); return { get size() { return m.size; }, add: i => m.set(i.id, i), delete: id => m.delete(id), get: id => m.get(id), replace: items => { m.clear(); items.forEach(i => m.set(i.id, i)); }, forEach: (cb, t) => m.forEach(v => cb.call(t, v, this)), [Symbol.iterator]: () => m.entries() }; };
  const tests = {
    createTestController: (id, label) => ({ id, label, items: testItems(), refreshHandler: undefined, resolveHandler: undefined,
      createRunProfile: (l, kind, handler) => ({ label: l, kind, runHandler: handler, isDefault: false, dispose() {} }),
      createTestItem: (iid, ilabel, uri) => ({ id: iid, label: ilabel, uri, children: testItems(), canResolveChildren: false, busy: false, tags: [], range: undefined, error: undefined }),
      createTestRun: () => ({ name: '', token: noToken, isPersisted: false, enqueued() {}, started() {}, skipped() {}, failed() {}, errored() {}, passed() {}, appendOutput() {}, end() {}, addCoverage() {}, onDidDispose: noEvent }),
      invalidateTestResults() {}, dispose() {} }),
  };
  const notebooks = { createNotebookController: (id, type, label) => ({ id, notebookType: type, label, supportedLanguages: [], executeHandler: undefined, createNotebookCellExecution: () => ({ start() {}, end() {}, replaceOutput: async () => {}, appendOutput: async () => {} }), updateNotebookAffinity() {}, onDidChangeSelectedNotebooks: noEvent, dispose() {} }), registerNotebookCellStatusBarItemProvider: () => new Disposable(() => {}), createRendererMessaging: () => ({ onDidReceiveMessage: noEvent, postMessage: async () => false }) };
  const l10n = {
    t: (first, ...args) => {
      let message = first, named = {};
      if (first && typeof first === 'object') { message = first.message; args = Array.isArray(first.args) ? first.args : []; if (first.args && !Array.isArray(first.args)) named = first.args; }
      if (args.length === 1 && args[0] && typeof args[0] === 'object' && !Array.isArray(args[0])) named = args[0];
      return String(message).replace(/\{(\w+)\}/g, (m, k) => (/^\d+$/.test(k) ? (args[+k] !== undefined ? String(args[+k]) : m) : (named[k] !== undefined ? String(named[k]) : m)));
    },
    bundle: undefined, uri: undefined,
  };
  const lm = { selectChatModels: async () => [], registerTool: () => new Disposable(() => {}), invokeTool: async () => { throw new Error('language models are not available in hittable'); }, tools: [], onDidChangeChatModels: noEvent, registerMcpServerDefinitionProvider: () => new Disposable(() => {}) };
  const chat = { createChatParticipant: (id) => ({ id, iconPath: undefined, requestHandler: undefined, followupProvider: undefined, onDidReceiveFeedback: noEvent, dispose() {} }) };

  const api = {
    version: S.apiVersion,
    Disposable, EventEmitter, CancellationTokenSource, CancellationError, Uri, Position, Range, Selection, Location, TextEdit, WorkspaceEdit, SnippetString, SnippetTextEdit,
    Diagnostic, DiagnosticSeverity, DiagnosticTag, DiagnosticRelatedInformation, CompletionItem, CompletionList, CompletionItemKind, CompletionItemTag, CompletionTriggerKind,
    MarkdownString, Hover, VerboseHover, SymbolKind, SymbolTag, SymbolInformation, DocumentSymbol, CodeActionKind, CodeActionTriggerKind, CodeAction, CodeLens,
    SignatureHelp, SignatureInformation, ParameterInformation, SignatureHelpTriggerKind, DocumentHighlight, DocumentHighlightKind, DocumentLink, Color, ColorInformation, ColorPresentation,
    FoldingRange, FoldingRangeKind, SelectionRange, CallHierarchyItem, CallHierarchyIncomingCall, CallHierarchyOutgoingCall, TypeHierarchyItem,
    SemanticTokensLegend, SemanticTokens, SemanticTokensEdit, SemanticTokensEdits, SemanticTokensBuilder, LinkedEditingRanges,
    InlineValueText, InlineValueVariableLookup, InlineValueEvaluatableExpression, EvaluatableExpression, InlayHint, InlayHintKind, InlayHintLabelPart,
    InlineCompletionItem, InlineCompletionList, InlineCompletionTriggerKind, DocumentDropEdit, DocumentPasteEdit, DocumentDropOrPasteEditKind, DataTransfer, DataTransferItem,
    ThemeColor, ThemeIcon, TreeItem, TreeItemCollapsibleState, TreeItemCheckboxState, RelativePattern, EndOfLine,
    Task, ShellExecution, ProcessExecution, CustomExecution, TaskGroup, NotebookRange, NotebookCellData, NotebookData, NotebookCellOutput, NotebookCellOutputItem, NotebookEdit,
    TestMessage, TestTag, TestRunRequest, TabInputText, TabInputTextDiff, TabInputCustom, TabInputWebview, TabInputNotebook, TabInputNotebookDiff, TabInputTerminal,
    TerminalLink, TerminalProfile, FileDecoration, QuickInputButtons, LanguageModelChatMessage, LanguageModelTextPart, LanguageModelError,
    FileSystemError, FileType, FilePermission, FileChangeType,
    ...enums,
    languages: stubNamespace('languages', ext, languages),
    window: stubNamespace('window', ext, window),
    workspace: stubNamespace('workspace', ext, workspace),
    commands: stubNamespace('commands', ext, commands),
    env: stubNamespace('env', ext, env),
    extensions: stubNamespace('extensions', ext, extensions),
    debug: stubNamespace('debug', ext, debug),
    tasks: stubNamespace('tasks', ext, tasks),
    scm: stubNamespace('scm', ext, scm),
    comments: stubNamespace('comments', ext, comments),
    authentication: stubNamespace('authentication', ext, authentication),
    tests: stubNamespace('tests', ext, tests),
    notebooks: stubNamespace('notebooks', ext, notebooks),
    l10n, lm: stubNamespace('lm', ext, lm), chat: stubNamespace('chat', ext, chat),
  };
  return buildApi.raw ? api : stubNamespace('', ext, api);
}
let termSeq = 0;
function buildApiTarget() { buildApi.raw = true; try { return buildApi(genericExtStub); } finally { buildApi.raw = false; } }
const genericExtStub = { id: 'unknown', path: '', manifest: { name: 'unknown', publisher: 'unknown' } };

const apis = new Map();
const genericExt = { id: 'unknown', path: '', manifest: { name: 'unknown', publisher: 'unknown' } };
function apiFor(filename) {
  let owner = genericExt;
  if (filename) {
    for (const e of S.exts) if (filename === e.path || filename.startsWith(e.path + path.sep)) { owner = e; break; }
  }
  if (!apis.has(owner.id)) apis.set(owner.id, buildApi(owner));
  return apis.get(owner.id);
}
const origLoad = Module._load;
Module._load = function (req, parent, isMain) {
  if (req === 'vscode') return apiFor(parent && parent.filename);
  return origLoad.apply(this, arguments);
};

// ES-module extensions (`import * as vscode from 'vscode'`) resolve
// 'vscode' to a generated module that re-exports the same per-extension API.
// The importer's file travels in the URL so each extension gets its own.
// Named exports must be static, so they are listed from a built API.
globalThis.__hittableApi = (file) => apiFor(file);
const apiKeys = () => Object.keys(buildApiTarget()).filter(k => /^[A-Za-z_$][\w$]*$/.test(k));
function vscodeModuleSource(parentFile) {
  const keys = apiKeys();
  return `const api = globalThis.__hittableApi(${JSON.stringify(parentFile)});\nexport default api;\n` +
    keys.map(k => `export const ${k} = api[${JSON.stringify(k)}];`).join('\n') + '\n';
}
try {
  const { pathToFileURL, fileURLToPath } = require('url');
  const resolveVscode = (spec, ctx) => {
    const parent = ctx && ctx.parentURL && ctx.parentURL.startsWith('file:') ? fileURLToPath(ctx.parentURL) : '';
    return { url: 'hittable-vscode:api?' + encodeURIComponent(parent), shortCircuit: true, format: 'module' };
  };
  const loadVscode = url => ({ format: 'module', source: vscodeModuleSource(decodeURIComponent(url.slice(url.indexOf('?') + 1))), shortCircuit: true });
  if (typeof Module.registerHooks === 'function') {
    // Node 22.15+/23.5+: synchronous hooks on this thread.
    Module.registerHooks({
      resolve(spec, ctx, next) { return spec === 'vscode' ? resolveVscode(spec, ctx) : next(spec, ctx); },
      load(url, ctx, next) { return url.startsWith('hittable-vscode:') ? loadVscode(url) : next(url, ctx); },
    });
  } else if (typeof Module.register === 'function') {
    // Older Node: off-thread hooks, which cannot see our API object, so the
    // generated source reads it from globalThis when it runs here.
    const keys = JSON.stringify(apiKeys());
    const hooks = `export async function resolve(spec, ctx, next) {
      if (spec !== 'vscode') return next(spec, ctx);
      const parent = ctx.parentURL && ctx.parentURL.startsWith('file:') ? new URL(ctx.parentURL).pathname : '';
      return { url: 'hittable-vscode:api?' + encodeURIComponent(decodeURIComponent(parent)), shortCircuit: true, format: 'module' };
    }
    export async function load(url, ctx, next) {
      if (!url.startsWith('hittable-vscode:')) return next(url, ctx);
      const parent = decodeURIComponent(url.slice(url.indexOf('?') + 1));
      const keys = ${keys};
      const source = 'const api = globalThis.__hittableApi(' + JSON.stringify(parent) + ');\\nexport default api;\\n' + keys.map(k => 'export const ' + k + ' = api[' + JSON.stringify(k) + '];').join('\\n');
      return { format: 'module', source, shortCircuit: true };
    }`;
    Module.register('data:text/javascript,' + encodeURIComponent(hooks));
  }
} catch (e) { log('warn', 'ES-module extensions are unavailable:', e.message); }

// ---------------------------------------------------------------- start

handlers['initialize'] = async (p) => {
  S.root = p.root || '';
  S.apiVersion = p.apiVersion || '1.144.0';
  S.webviewBase = p.webviewBase || '';
  S.extData = p.extData || path.join(os.homedir(), '.hittable', 'extdata');
  S.machineId = p.machineId || crypto.createHash('sha256').update(os.hostname()).digest('hex');
  S.workspaceHash = crypto.createHash('sha256').update(S.root).digest('hex').slice(0, 16);
  S.settings = p.settings || { user: {}, workspace: {} };
  if (S.root) { S.rootUri = Uri.file(S.root); S.folders = [{ uri: S.rootUri, name: path.basename(S.root), index: 0 }]; }
  // Node reports module file names with symlinks resolved (/var → /private/var
  // on macOS), so the real path is what `require('vscode')` callers match.
  const real = q => { try { return fs.realpathSync(q); } catch { return q; } };
  S.exts = (p.extensions || []).map(e => ({ id: e.id, path: real(e.path), manifest: e.manifest, nls: e.nls || {}, active: false }));
  for (const l of p.languages || []) S.languages.add(l);
  for (const e of S.exts) for (const l of (e.manifest.contributes && e.manifest.contributes.languages) || []) S.languages.add(l.id);
  collectDefaults();
  setTimeout(async () => {
    await activateByEvent('*');
    await activateByEvent('onStartupFinished');
    checkWorkspaceContains().catch(e => log('warn', 'workspaceContains:', e.message));
  }, 0);
  return { ok: true, pid: process.pid, node: process.version };
};

notify('ready', { pid: process.pid, node: process.version });
