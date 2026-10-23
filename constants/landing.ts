import { Globe, Database, Server, Terminal, Layout, Code2 } from "lucide-react";

export const PORTFOLIO_DATA = {
  hero: {
    name: "Pantho Haque",
    current_position: "Software Engineer I",
    company_name: "Pathao Ltd.",
    comment_one: "Building scalable, high-quality software — from pixel-perfect frontends to production-ready deployments. Currently architecting robust dashboards and shipping critical features powering logistics & rides.",
    comment_two: "Hittable is one of my projects built out of a passion for clean tooling, great Developer Experience, and a frustration with bloated API clients. Let's build something meaningful together.",
    photo: "/assets/images/avatar.png",
    contactLinks: [
      {
        icon: "github",
        label: "GitHub",
        href: "https://github.com/Pantho-Haque"
      },
      {
        icon: "linkedin",
        label: "LinkedIn",
        href: "https://www.linkedin.com/in/panthohaque/"
      },
      {
        icon: "mail",
        label: "Contact",
        href: "mailto:pantho.haque.dev@gmail.com"
      }
    ],
  },
};

export const LANDING_PAGE_DATA = {
  appInfo: {
    githubUrl: "https://github.com/Pantho-Haque/hittable",
    title: "API testing,",
    subtitle: "without the bloat.",
    introducing: "Hittable",
    description: "Hittable is a lightning-fast, keyboard-driven HTTP client that lives right in your browser. Collections, environments, and curl import—everything you need to build faster.",
  },
  features: [
    {
      icon: Globe,
      title: "Collections & Routes",
      desc: "Organize every API endpoint into named collections. Create, rename, delete — full control with instant feedback."
    },
    {
      icon: Database,
      title: "Environment Variables",
      desc: "Define <<KEY>> vars per collection. Switch between dev, staging, and prod without touching your requests."
    },
    {
      icon: Server,
      title: "CORS-Free Proxy",
      desc: "Requests route through a Next.js server-side proxy. No more CORS errors blocking your flow in the browser."
    },
    {
      icon: Terminal,
      title: "curl Import",
      desc: "Paste any curl command into the URL bar. Method, headers, and body auto-parse instantly."
    },
    {
      icon: Layout,
      title: "Persistent Storage",
      desc: "Collections save to localStorage automatically. Close the tab, come back later — everything is still there."
    },
    {
      icon: Code2,
      title: "Keyboard Driven",
      desc: "Ctrl+Enter to send, Ctrl+S to save. Every action has a shortcut — your mouse is optional."
    }
  ]
};
