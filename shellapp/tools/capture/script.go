package main

import "time"

const (
    beat  = 900 * time.Millisecond
    hold  = 1500 * time.Millisecond
    think = 2200 * time.Millisecond
)

// script is the walkthrough, in order. Focus rectangles are in terminal cells
// and drive the zoom in the video; {0,0,0,0} means show the whole screen.
func script() []scene {
    return []scene{
        {
            Name: "Open", Caption: "hittable",
            Note:  "a terminal API client · .hit files, shared with the web app",
            Focus: [4]int{0, 0, 0, 0},
            act: func(s *session) {
                s.wait(think)
                s.hoverOn(" Git", 2, 0)
                s.hoverOn(" Find", 2, 0)
                s.hoverOn(" Terminal", 2, 0)
                s.wait(beat)
            },
        },
        {
            Name: "Explorer", Caption: "The whole project, one tree",
            Note:  "single click opens · git status colours every row",
            Focus: [4]int{0, 0, 34, 20},
            act: func(s *session) {
                s.clickOn("hittable/", 2, 0)
                s.wait(beat)
                s.clickOn("users/", 2, 0)
                s.wait(hold)
                s.hoverOn("notes/", 2, 0)
                s.wait(beat)
            },
        },
        {
            Name: "Open a request", Caption: "A request is just a file",
            Note:  "plain JSON on disk — the same files the web app opens",
            Focus: [4]int{0, 0, 0, 0},
            act: func(s *session) {
                s.clickOn("list.hit", 2, 0)
                s.wait(think)
            },
        },
        {
            Name: "URL bar", Caption: "Method, URL, environment",
            Note:  "<<BASE_URL>> resolves from env.json at send time",
            Focus: [4]int{34, 1, 98, 8},
            act: func(s *session) {
                s.clickOn("GET", 2, 0)
                s.wait(hold)
                s.key("esc")
                s.wait(beat)
            },
        },
        {
            Name: "Send", Caption: "Send it — ctrl+r",
            Note:  "status, timing and size, with the body highlighted",
            Focus: [4]int{34, 12, 98, 22},
            act: func(s *session) {
                s.key("ctrl+r")
                s.wait(think)
                s.wait(hold)
            },
        },
        {
            Name: "Response", Caption: "Read the response in place",
            Note:  "ctrl+f searches it · h flips to the headers",
            Focus: [4]int{34, 14, 98, 22},
            act: func(s *session) {
                s.clickOn("Grace", 0, 0)
                s.key("ctrl+f")
                s.typeText("engineer", 90*time.Millisecond)
                s.wait(hold)
                s.key("esc")
                s.key("h")
                s.wait(think)
                s.key("h")
                s.wait(beat)
            },
        },
        {
            Name: "Tabs", Caption: "Params, headers and body",
            Note:  "each tab is the full code editor, JSON validated as you type",
            Focus: [4]int{34, 4, 98, 16},
            act: func(s *session) {
                s.clickOn("[Headers]", 2, 0)
                s.wait(hold)
                s.clickOn("[Body]", 2, 0)
                s.wait(hold)
                s.clickOn("[Params]", 2, 0)
                s.wait(beat)
            },
        },
        {
            Name: "Raw view", Caption: "Or edit the raw file",
            Note:  "ctrl+t · one buffer, two views, autosaved either way",
            Focus: [4]int{34, 1, 98, 22},
            act: func(s *session) {
                s.key("ctrl+t")
                s.wait(think)
                s.key("ctrl+t")
                s.wait(beat)
            },
        },
        {
            Name: "Markdown", Caption: "Notes render beside the source",
            Note:  "ctrl+t cycles Text → Preview → Split · mermaid becomes a diagram",
            Focus: [4]int{0, 0, 0, 0},
            act: func(s *session) {
                s.key("ctrl+b")
                s.clickOn("notes/", 2, 0)
                s.wait(beat)
                s.clickOn("api.md", 2, 0)
                s.wait(hold)
                s.key("ctrl+t")
                s.wait(think)
                s.key("ctrl+t")
                s.wait(think)
            },
        },
        {
            Name: "Find", Caption: "Jump anywhere",
            Note:  "ctrl+p finds files · tab switches to live grep",
            Focus: [4]int{0, 0, 0, 0},
            act: func(s *session) {
                s.key("ctrl+p")
                s.typeText("crea", 130*time.Millisecond)
                s.wait(hold)
                s.key("tab")
                s.typeText("Bearer", 110*time.Millisecond)
                s.wait(think)
                s.key("esc")
                s.wait(beat)
            },
        },
        {
            Name: "Git status", Caption: "Git, without leaving the app",
            Note:  "stage with a click · [ ⟲ ] throws the change away",
            Focus: [4]int{0, 0, 0, 0},
            act: func(s *session) {
                s.key("alt+g")
                s.wait(think)
                s.hoverOn("+ stage all", 2, 0)
                s.wait(beat)
                // Arrow keys rather than clicks: clicking a row that is
                // already selected opens the file and closes the panel.
                s.keys("down", "down")
                s.wait(hold)
                s.key("down")
                s.wait(hold)
            },
        },
        {
            Name: "Split diff", Caption: "Side by side, wrapped, resizable",
            Note:  "v splits · z wraps long lines · drag the divider",
            Focus: [4]int{0, 13, 0, 21},
            act: func(s *session) {
                s.key("v")
                s.wait(think)
                s.key("z")
                s.wait(hold)
                s.key("z")
                s.wait(beat)
                s.key("v")
                s.wait(beat)
            },
        },
        {
            Name: "Stage and commit", Caption: "Stage, then commit",
            Note:  "the whole status list is clickable",
            Focus: [4]int{0, 1, 0, 14},
            act: func(s *session) {
                s.clickOn("+ stage all", 2, 0)
                s.wait(hold)
                s.key("c")
                s.typeText("docs: note the delete endpoint", 60*time.Millisecond)
                s.wait(hold)
                s.key("enter")
                s.wait(think)
            },
        },
        {
            Name: "History", Caption: "History, branches, stashes, blame",
            Note:  "every section is one key away",
            Focus: [4]int{0, 1, 0, 17},
            act: func(s *session) {
                s.key("2")
                s.wait(think)
                s.key("3")
                s.wait(hold)
                s.key("4")
                s.wait(hold)
                s.key("5")
                s.wait(think)
                s.key("1")
                s.wait(beat)
            },
        },
        {
            Name: "Terminal", Caption: "A real shell, in the same window",
            Note:  "ctrl+j · drag to select, ctrl+c to copy",
            Focus: [4]int{0, 18, 0, 18},
            act: func(s *session) {
                s.key("esc")
                s.key("ctrl+j")
                s.wait(2500 * time.Millisecond) // the login shell needs a moment
                // --no-pager: git would otherwise open less inside the panel.
                s.typeText("git --no-pager log --oneline -5", 55*time.Millisecond)
                s.key("enter")
                s.wait(think)
                s.typeText("ls hittable/users", 55*time.Millisecond)
                s.key("enter")
                s.wait(think)
            },
        },
        {
            Name: "Help", Caption: "Every shortcut, one key away",
            Note:  "? or F1",
            Focus: [4]int{0, 0, 0, 0},
            act: func(s *session) {
                s.key("ctrl+b")
                s.key("ctrl+j")
                s.wait(beat)
                s.key("F1")
                s.wait(think)
                s.wait(hold)
                s.key("esc")
                s.wait(beat)
            },
        },
        {
            Name: "Outro", Caption: "hittable",
            Note:  "go install · one binary · your files stay yours",
            Focus: [4]int{0, 0, 0, 0},
            act: func(s *session) {
                s.wait(think)
            },
        },
    }
}
