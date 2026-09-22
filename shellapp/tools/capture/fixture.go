package main

import (
    "os"
    "os/exec"
    "path/filepath"
)

// buildFixture creates the project the video is shot in: a small API
// collection, some notes, and a git repo with history, a second branch, a
// stash and uncommitted work, so every Git section has something to show.
func buildFixture(root, baseURL string) error {
    hd := filepath.Join(root, "hittable")
    for _, d := range []string{hd, filepath.Join(hd, "users"), filepath.Join(hd, "notes")} {
        if err := os.MkdirAll(d, 0o755); err != nil {
            return err
        }
    }
    write := func(p, s string) error { return os.WriteFile(filepath.Join(root, p), []byte(s), 0o644) }

    if err := write("hittable/env.json", `{
  "BASE_URL": "`+baseURL+`",
  "AUTH_TOKEN": "demo-token-7f3a"
}
`); err != nil {
        return err
    }
    if err := write("hittable/users/list.hit", `{
  "method": "GET",
  "url": "<<BASE_URL>>/users",
  "headers": { "Accept": "application/json" },
  "params": { "_limit": "5" },
  "body": "",
  "response": null
}
`); err != nil {
        return err
    }
    if err := write("hittable/users/create.hit", `{
  "method": "POST",
  "url": "<<BASE_URL>>/users",
  "headers": { "Content-Type": "application/json" },
  "params": {},
  "body": "{\n  \"name\": \"Ada Lovelace\",\n  \"email\": \"ada@example.com\"\n}",
  "response": null
}
`); err != nil {
        return err
    }
    if err := write("hittable/notes/api.md", `# API notes

The collection is plain JSON on disk, shared with the web app.

## Conventions

- Every URL goes through `+"`<<BASE_URL>>`"+` so environments swap cleanly.
- Auth is a bearer token from `+"`env.json`"+`.

| Endpoint     | Method | Auth |
| ------------ | ------ | ---- |
| /users       | GET    | no   |
| /users       | POST   | yes  |
| /users/:id   | PATCH  | yes  |

`+"```mermaid"+`
graph LR
  Client --> API
  API --> DB
`+"```"+`

Run a request with ctrl+r and the response lands in the same file.
`); err != nil {
        return err
    }
    if err := write("README.md", "# demo\n\nA small collection used for the hittable walkthrough.\n"); err != nil {
        return err
    }

    git := func(args ...string) error {
        cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
        cmd.Env = append(os.Environ(),
            "GIT_AUTHOR_NAME=Ada", "GIT_AUTHOR_EMAIL=ada@example.com",
            "GIT_COMMITTER_NAME=Ada", "GIT_COMMITTER_EMAIL=ada@example.com")
        return cmd.Run()
    }
    for _, args := range [][]string{
        {"init", "-q", "-b", "main"},
        {"config", "user.name", "Ada"},
        {"config", "user.email", "ada@example.com"},
        {"add", "."},
        {"commit", "-q", "-m", "feat: users collection and API notes"},
    } {
        if err := git(args...); err != nil {
            return err
        }
    }
    // A little history, so the Commits and Branches sections are not empty.
    if err := write("hittable/users/list.hit", `{
  "method": "GET",
  "url": "<<BASE_URL>>/users",
  "headers": { "Accept": "application/json" },
  "params": { "_limit": "10" },
  "body": "",
  "response": null
}
`); err != nil {
        return err
    }
    for _, args := range [][]string{
        {"commit", "-qam", "chore: raise the user page size to 10"},
        {"branch", "feature/pagination"},
        {"branch", "fix/auth-header"},
    } {
        if err := git(args...); err != nil {
            return err
        }
    }
    // Uncommitted work for the Status section to stage and diff.
    if err := write("hittable/notes/api.md", `# API notes

The collection is plain JSON on disk, shared with the web app.

## Conventions

- Every URL goes through `+"`<<BASE_URL>>`"+` so environments swap cleanly.
- Auth is a bearer token from `+"`env.json`"+`.
- Responses are written back into the same .hit file.

| Endpoint     | Method | Auth |
| ------------ | ------ | ---- |
| /users       | GET    | no   |
| /users       | POST   | yes  |
| /users/:id   | PATCH  | yes  |
| /users/:id   | DELETE | yes  |

`+"```mermaid"+`
graph LR
  Client --> API
  API --> DB
  API --> Cache
`+"```"+`

Run a request with ctrl+r and the response lands in the same file.
`); err != nil {
        return err
    }
    if err := write("hittable/users/create.hit", `{
  "method": "POST",
  "url": "<<BASE_URL>>/users",
  "headers": {
    "Content-Type": "application/json",
    "Authorization": "Bearer <<AUTH_TOKEN>>"
  },
  "params": {},
  "body": "{\n  \"name\": \"Ada Lovelace\",\n  \"email\": \"ada@example.com\"\n}",
  "response": null
}
`); err != nil {
        return err
    }
    return write("hittable/users/delete.hit", `{
  "method": "DELETE",
  "url": "<<BASE_URL>>/users/3",
  "headers": { "Authorization": "Bearer <<AUTH_TOKEN>>" },
  "params": {},
  "body": "",
  "response": null
}
`)
}
