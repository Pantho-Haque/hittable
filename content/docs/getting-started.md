---
title: Getting started
description: From an empty folder to a sent request, in the browser and in the terminal, using the same files.
---

## 1. Make a project folder

Any folder works, including the root of an existing repository. Hittable keeps everything under a `hittable/` subfolder so it never collides with your code.

```sh
mkdir -p ~/code/my-api && cd ~/code/my-api
git init
```

## 2. In the browser

1. Open [hittable.vercel.app/hittable](https://hittable.vercel.app/hittable) in Chrome, Edge or another Chromium browser.
2. Click **Local** in the top bar to switch it to **Directory**, then **Choose folder** and pick `my-api`. The browser asks for read and write permission once; the handle is remembered, and on the next visit a single click re-grants it.
3. The app creates `hittable/env.json`, `hittable/testcollection/test.hit` and `hittable/notes/sample.md` if they are missing, then reads the folder into the explorer on the left.
4. Open `testcollection/test.hit`. It opens in **Text** mode as JSON; click **Runner** to get the URL bar, the Params / Headers / Body tabs and the response panel.
5. Press <kbd>Ctrl/Cmd+Enter</kbd> (or **Send**). The response appears below and is written into the file's `response` field, so the file on disk now carries the last result.

Requests to `http://localhost:…` need the [browser extension](/docs/browser-extension). Everything else goes through the server-side proxy.

If you skip the folder and stay in **Local** mode, collections live in the browser's storage instead and nothing is written to disk. That mode works in every browser, but it is not shared with the terminal app. See [Web app](/docs/web-app) for the difference.

## 3. In the terminal

Install the binary (macOS, Linux, Windows; details on the [terminal app page](/docs/shellapp)):

```sh
curl -fsSL https://raw.githubusercontent.com/Pantho-Haque/hittable/main/install.sh | sh
```

Open the same folder:

```sh
cd ~/code/my-api
hittable
```

The explorer shows the whole tree. Pick `hittable/testcollection/test.hit` with <kbd>↑</kbd> <kbd>↓</kbd> and <kbd>⏎</kbd>, press <kbd>ctrl+r</kbd> to send, <kbd>ctrl+t</kbd> to see the raw JSON, <kbd>?</kbd> for every key. If the browser was open on the same folder, the response you just got there is already in the file.

Had you started here instead, `hittable init` scaffolds the same three files the browser does.

## 4. Add a variable

Open `hittable/env.json` in either app and add a key:

```json
{
  "BASE_URL": "https://jsonplaceholder.typicode.com",
  "AUTH_TOKEN": ""
}
```

Then write `<<BASE_URL>>/posts/1` as a URL, or `Bearer <<AUTH_TOKEN>>` as a header value. Templates are resolved when the request is sent and never written back, so the file stays safe to commit. Names are letters, digits and underscores.

## 5. Add requests and folders

- **Browser:** right-click in the explorer, or use the **New File** and **New Folder** buttons in its toolbar. A new `.hit` file is seeded with an empty `GET`. <kbd>F2</kbd> renames, <kbd>Delete</kbd> deletes.
- **Terminal:** <kbd>ctrl+n</kbd> new file, <kbd>ctrl+f</kbd> new folder, <kbd>ctrl+e</kbd> rename, <kbd>ctrl+d</kbd> delete, or <kbd>x</kbd> for the context menu.

Folders under `hittable/` are collections and nest as deep as you like. The terminal app also imports an existing Postman or Insomnia export straight into the folder: `hittable -i collection.json`.

## 6. Commit it

```sh
git add hittable && git commit -m "feat(api): first requests"
```

Or do it from inside the terminal app: <kbd>ctrl+g</kbd> opens the [Git panel](/docs/shellapp-git), <kbd>s</kbd> stages, <kbd>c</kbd> drafts a conventional commit message from the staged diff, <kbd>ctrl+s</kbd> commits.

## What next

- Keep the keyboard close: [web shortcuts](/docs/web-app#keyboard-shortcuts), [terminal reference](/docs/shellapp-keyboard).
- Run your own instance so request traffic never touches a server you do not control: [Self-hosting](/docs/self-hosting).
- Let a small local model write your commit messages: [Commit messages with a local model](/docs/shellapp-ai).
